package compatible_rocksdb

import (
	"context"
	"fmt"
	"runtime"
	"unsafe"

	"github.com/ledgerwatch/erigon-lib/common/dbg"
	"github.com/ledgerwatch/erigon-lib/kv"
	rdbcommon "github.com/ledgerwatch/erigon-lib/kv/rocksdb/common"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/compatible_rocksdb/backend"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/compatible_rocksdb/backend/backend_type"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/compatible_rocksdb/backend/mock"
	"github.com/ledgerwatch/log/v3"
	"golang.org/x/sync/semaphore"
)

type CompatibleRocksDB struct {
	db backend.CompatibleBackend

	closeGuard *rdbcommon.CloseGuard

	readOnly  bool // todo: not used
	tablesCfg kv.TableCfg
	label     kv.Label // marker to distinct db instances - one process may open many databases. for example to collect metrics of only 1 database

	readTxLimiter  *semaphore.Weighted
	writeTxLimiter *semaphore.Weighted
	logger         log.Logger
	leakDetector   *dbg.LeakDetector
}

func NewCompatibleRocksDB(dbPath string, logger log.Logger, tablesCfg kv.TableCfg, label kv.Label, readTxLimiter *semaphore.Weighted, readOnly bool, backendType backend_type.CompatibleBackendType, options *rdbcommon.RocksDBOptions) (kv.RwDB, error) {
	if readTxLimiter == nil {
		targetSemCount := int64(runtime.GOMAXPROCS(-1)) - 1
		readTxLimiter = semaphore.NewWeighted(targetSemCount) // 1 less than max to allow unlocking to happen
	}
	writeTxLimiter := semaphore.NewWeighted(int64(runtime.GOMAXPROCS(-1)) - 1) // 1 less than max to allow unlocking to happen

	db, err := backendType.NewBackend(dbPath, readOnly, options)
	if err != nil {
		return nil, err
	}

	return &CompatibleRocksDB{
		db:             db,
		closeGuard:     rdbcommon.NewCloseGuard(),
		readOnly:       readOnly,
		tablesCfg:      tablesCfg,
		label:          label,
		readTxLimiter:  readTxLimiter,
		writeTxLimiter: writeTxLimiter,
		logger:         logger,
	}, nil
}

func (db *CompatibleRocksDB) GetMemStorage() map[string]*rdbcommon.DBValue {
	return db.db.(*mock.MemoryRDB).GetMemStorage()
}

func (db *CompatibleRocksDB) Close() {
	firstClose := db.closeGuard.Close()
	if firstClose {
		db.db.Close()
		db.db = nil
	}
}

func (db *CompatibleRocksDB) Get(table string, k []byte) (*rdbcommon.DBValue, error) {
	return db.db.GetCompatibleValue(table, k)
}

func (db *CompatibleRocksDB) ReadOnly() bool {
	return db.readOnly
}

func (db *CompatibleRocksDB) View(ctx context.Context, f func(tx kv.Tx) error) error {
	tx, err := db.BeginRo(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	return f(tx)
}

func (db *CompatibleRocksDB) BeginRo(ctx context.Context) (tx kv.Tx, err error) {
	return db.beginTx(ctx, db.readTxLimiter)
}

func (db *CompatibleRocksDB) AllTables() kv.TableCfg {
	return db.tablesCfg
}

func (db *CompatibleRocksDB) PageSize() uint64 {
	// note: no available call to PageSize for now
	panic("not supported")
}

func (db *CompatibleRocksDB) CHandle() unsafe.Pointer {
	// note: not support for CompatibleRocksDB
	panic("not supported")
}

func (db *CompatibleRocksDB) Update(ctx context.Context, f func(tx kv.RwTx) error) error {
	tx, err := db.BeginRw(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	if err = f(tx); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (db *CompatibleRocksDB) UpdateNosync(ctx context.Context, f func(tx kv.RwTx) error) error {
	tx, err := db.BeginRwNosync(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	if err = f(tx); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (db *CompatibleRocksDB) BeginRw(ctx context.Context) (tx kv.RwTx, err error) {
	return db.beginTx(ctx, db.writeTxLimiter)
}

func (db *CompatibleRocksDB) BeginRwNosync(ctx context.Context) (kv.RwTx, error) {
	// note: yztodo: there isn't a real `no sync` in rocksdb
	return db.BeginRw(ctx)
}

func (db *CompatibleRocksDB) beginTx(ctx context.Context, txLimiter *semaphore.Weighted) (tx kv.RwTx, err error) {
	// don't try to acquire if the context is already done
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
		// otherwise carry on
	}

	// will return nil err if context is cancelled (may appear to acquire the semaphore)
	if semErr := txLimiter.Acquire(ctx, 1); semErr != nil {
		return nil, fmt.Errorf("rocksdb.BeginTx: tx limiter error %w", semErr)
	}
	defer func() {
		if tx == nil {
			txLimiter.Release(1)
		}
	}()

	if !db.closeGuard.Reference() {
		return nil, fmt.Errorf("db closed")
	}
	defer func() {
		if tx == nil {
			db.closeGuard.DeReference()
		}
	}()

	id := db.leakDetector.Add()
	// todo: yztodo: not a real read only tx yet
	return newCompatibleTransaction(db, ctx, func() {
		db.closeGuard.DeReference()
		txLimiter.Release(1)
		db.leakDetector.Del(id)
	})
}
