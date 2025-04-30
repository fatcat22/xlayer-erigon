package mock

import (
	rdbcommon "github.com/ledgerwatch/erigon-lib/kv/rocksdb/common"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/compatible_rocksdb/backend"
	"github.com/linxGnu/grocksdb"
)

type MemoryRDB struct {
	storage *OrderedMap
}

func NewMemoryRDB() *MemoryRDB {
	return &MemoryRDB{
		storage: NewOrderedMap(),
	}
}

////////////// implement NativeRocksDBBase //////////////

func (db *MemoryRDB) Close() {
	// do nothing
}

////////////// implement CompatibleBackend //////////////

func (db *MemoryRDB) GetCompatibleValue(table string, key []byte) (*rdbcommon.DBValue, error) {
	return db.get(table, key)
}

func (db *MemoryRDB) NewCompatibleTransaction(opts *grocksdb.WriteOptions, transactionOpts *grocksdb.TransactionOptions, oldTransaction *grocksdb.Transaction) backend.CompatibleBackendTransaction {
	return NewMemoryRTX(db)
}

////////////// individual methods //////////////

func (db *MemoryRDB) GetMemStorage() map[string]*rdbcommon.DBValue {
	return db.storage.data
}

func (db *MemoryRDB) put(table string, key []byte, value *rdbcommon.DBValue) error {
	db.storage.Put(rdbcommon.MergeKey(table, key), value)
	return nil
}

func (db *MemoryRDB) get(table string, key []byte) (*rdbcommon.DBValue, error) {
	value, ok := db.storage.Get(rdbcommon.MergeKey(table, key))
	if !ok {
		return nil, rdbcommon.ErrKeyNotExist
	}
	return value, nil
}

func (db *MemoryRDB) delete(table string, key []byte) {
	db.storage.Delete(rdbcommon.MergeKey(table, key))
}
