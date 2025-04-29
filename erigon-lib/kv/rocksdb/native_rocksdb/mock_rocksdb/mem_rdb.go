package mock_rocksdb

import (
	common2 "github.com/ledgerwatch/erigon-lib/kv/rocksdb/common"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/native_rocksdb"
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

func (db *MemoryRDB) Get(key []byte) (*common2.DBValue, error) {
	dbv, ok := db.storage.Get(key)
	if !ok {
		return nil, common2.ErrKeyNotExist
	}
	return dbv, nil
}

func (db *MemoryRDB) Close() {
	// do nothing
}

func (db *MemoryRDB) TransactionBegin(opts *grocksdb.WriteOptions, transactionOpts *grocksdb.TransactionOptions, oldTransaction *grocksdb.Transaction) native_rocksdb.RDBTransaction {
	return NewMemoryRTX(db)
}

func (db *MemoryRDB) GetMemStorage() map[string]*common2.DBValue {
	return db.storage.data
}

func (db *MemoryRDB) put(key []byte, value *common2.DBValue) error {
	db.storage.Put(key, value)
	return nil
}

func (db *MemoryRDB) get(key []byte) (*common2.DBValue, error) {
	value, ok := db.storage.Get(key)
	if !ok {
		return nil, common2.ErrKeyNotExist
	}
	return value, nil
}
