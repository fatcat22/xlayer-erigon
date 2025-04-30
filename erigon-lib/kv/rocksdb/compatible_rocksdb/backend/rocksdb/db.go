package rocksdb

import (
	rdbcommon "github.com/ledgerwatch/erigon-lib/kv/rocksdb/common"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/compatible_rocksdb/backend"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/native_rocksdb"
	"github.com/linxGnu/grocksdb"
)

type CompatibleBackendRocksDB struct {
	native_rocksdb.NativeRocksDB

	defaultOpts *grocksdb.ReadOptions
}

func NewCompatibleBackendRocksDB(dbPath string, readOnly bool, options *rdbcommon.RocksDBOptions) (backend.CompatibleBackend, error) {
	db, err := native_rocksdb.NewNativeRocksDB(dbPath, readOnly, options)
	if err != nil {
		return nil, err
	}
	return &CompatibleBackendRocksDB{
		NativeRocksDB: db,
		defaultOpts:   grocksdb.NewDefaultReadOptions(),
	}, nil
}

////////////// implement NativeRocksDBBase //////////////

func (db *CompatibleBackendRocksDB) Close() {
	db.defaultOpts.Destroy()
	db.NativeRocksDB.Close()
}

////////////// implement CompatibleBackend //////////////

func (db *CompatibleBackendRocksDB) GetCompatibleValue(table string, key []byte) (*rdbcommon.DBValue, error) {
	v, err := db.NativeGet(db.defaultOpts, rdbcommon.MergeKey(table, key))
	if err != nil {
		return nil, err
	}
	return rdbcommon.DeserializeDBValue(v), nil
}

func (db *CompatibleBackendRocksDB) NewCompatibleTransaction(opts *grocksdb.WriteOptions, transactionOpts *grocksdb.TransactionOptions, oldTransaction *grocksdb.Transaction) backend.CompatibleBackendTransaction {
	return newCompatibleTransaction(db.NewNativeTransaction(opts, transactionOpts, oldTransaction))
}
