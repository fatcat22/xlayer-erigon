package native_rocksdb

import (
	"context"
	"github.com/ledgerwatch/erigon-lib/kv"
	rdbcommon "github.com/ledgerwatch/erigon-lib/kv/rocksdb/common"
	"github.com/linxGnu/grocksdb"
	"path"
	"unsafe"
)

type NativeRocksDBImpl struct {
	ctx     context.Context
	db      *grocksdb.TransactionDB
	options *rdbcommon.RocksDBOptions

	readOnly bool
}

func NewNativeRocksDB(dbPath string, readOnly bool, options *rdbcommon.RocksDBOptions) (*NativeRocksDBImpl, error) {
	db, err := grocksdb.OpenTransactionDb(options.Opts, options.TxOpts, path.Join(dbPath, "rocksdb"))

	return &NativeRocksDBImpl{
		db:      db,
		options: options,

		readOnly: readOnly,
	}, err
}

////////////// implement NativeRocksDBBase //////////////

func (db *NativeRocksDBImpl) Close() {
	db.db.Close()
	db.db = nil

	db.options.Destroy()
}

////////////// implement NativeRocksDB //////////////

func (db *NativeRocksDBImpl) NativeGet(opts *grocksdb.ReadOptions, key []byte) ([]byte, error) {
	s, err := db.db.Get(opts, key)
	if err != nil {
		return nil, err
	}
	defer s.Free()

	return rdbcommon.MoveSliceToBytes(s), nil
}

func (db *NativeRocksDBImpl) NewNativeTransaction(opts *grocksdb.WriteOptions, transactionOpts *grocksdb.TransactionOptions, oldTransaction *grocksdb.Transaction) NativeTransaction {
	return db.newTransaction(opts, transactionOpts, oldTransaction)
}

//func (db *NativeRocksDBImpl) Get(k []byte) (*rdbcommon.DBValue, error) {
//	ropts := grocksdb.NewDefaultReadOptions()
//	defer ropts.Destroy()
//	s, err := db.db.Get(ropts, k)
//	if err != nil {
//		return nil, err
//	}
//	defer s.Free()
//	return rdbcommon.DeserializeDBValue(rdbcommon.MoveSliceToBytes(s)), nil
//}

////////////// implement kv.RwDB //////////////

func (db *NativeRocksDBImpl) ReadOnly() bool {
	return db.readOnly
}

func (db *NativeRocksDBImpl) View(ctx context.Context, f func(tx kv.Tx) error) error {
	tx := db.newTransaction(nil, nil, nil)
	defer func() {
		tx.Rollback()
		tx.Destroy()
	}()

	return f(tx)
}

func (db *NativeRocksDBImpl) BeginRo(ctx context.Context) (kv.Tx, error) {
	return db.newTransaction(nil, nil, nil), nil
}

func (db *NativeRocksDBImpl) AllTables() kv.TableCfg {
	panic("NativeRocksDBImpl.AllTables is not supported")
}

func (db *NativeRocksDBImpl) PageSize() uint64 {
	panic("NativeRocksDBImpl.PageSize is not supported")
}

func (db *NativeRocksDBImpl) CHandle() unsafe.Pointer {
	panic("NativeRocksDBImpl.CHandle is not supported")
}

func (db *NativeRocksDBImpl) Update(ctx context.Context, f func(tx kv.RwTx) error) error {
	tx := db.newTransaction(nil, nil, nil)
	defer tx.Destroy()

	if err := f(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

func (db *NativeRocksDBImpl) UpdateNosync(ctx context.Context, f func(tx kv.RwTx) error) error {
	panic("NativeRocksDBImpl.UpdateNosync is not implemented")
}

func (db *NativeRocksDBImpl) BeginRw(ctx context.Context) (kv.RwTx, error) {
	return db.newTransaction(nil, nil, nil), nil
}

func (db *NativeRocksDBImpl) BeginRwNosync(ctx context.Context) (kv.RwTx, error) {
	panic("NativeRocksDBImpl.BeginRwNosync is not implemented")
}

////////////// internal methods //////////////

func (db *NativeRocksDBImpl) newTransaction(opts *grocksdb.WriteOptions, transactionOpts *grocksdb.TransactionOptions, oldTransaction *grocksdb.Transaction) *nativeTransaction {
	if opts == nil {
		opts = grocksdb.NewDefaultWriteOptions()
	}
	if transactionOpts == nil {
		transactionOpts = grocksdb.NewDefaultTransactionOptions()
	}

	return newNativeTransaction(db.ctx, db.db.TransactionBegin(opts, transactionOpts, oldTransaction))
}
