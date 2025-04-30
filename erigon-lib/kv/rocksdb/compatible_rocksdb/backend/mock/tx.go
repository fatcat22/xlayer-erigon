package mock

import (
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/common"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/compatible_rocksdb/backend"
	"github.com/linxGnu/grocksdb"
)

type MemoryRTX struct {
	db *MemoryRDB
}

func NewMemoryRTX(db *MemoryRDB) *MemoryRTX {
	return &MemoryRTX{db: db}
}

////////////// implement NativeTransactionBase //////////////

func (rtx *MemoryRTX) Commit() error {
	return nil
}

func (rtx *MemoryRTX) Rollback() {
	// do nothing
}

func (rtx *MemoryRTX) Destroy() {
	// do nothing
}

////////////// implement CompatibleBackendTransaction //////////////

func (rtx *MemoryRTX) GetCompatibleValue(opts *grocksdb.ReadOptions, table string, key []byte) (*common.DBValue, error) {
	return rtx.db.get(table, key)
}

func (rtx *MemoryRTX) PutCompatibleValue(table string, key []byte, value *common.DBValue) error {
	// todo: should store data in MemoryRTX first
	return rtx.db.put(table, key, value)
}

func (rtx *MemoryRTX) CompatibleDelete(table string, key []byte) error {
	rtx.db.delete(table, key)
	return nil
}

func (rtx *MemoryRTX) NewCompatibleIterator(beginPrefix, endPrefix []byte) backend.CompatibleBackendIterator {
	return rtx.db.storage.NewIterator(beginPrefix, endPrefix)
}
