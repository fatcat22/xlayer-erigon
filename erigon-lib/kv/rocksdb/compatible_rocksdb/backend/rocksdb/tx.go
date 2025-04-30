package rocksdb

import (
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/common"
	backend "github.com/ledgerwatch/erigon-lib/kv/rocksdb/compatible_rocksdb/backend"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/native_rocksdb"
	"github.com/linxGnu/grocksdb"
)

type compatibleTransaction struct {
	native_rocksdb.NativeTransaction
}

func newCompatibleTransaction(tx native_rocksdb.NativeTransaction) backend.CompatibleBackendTransaction {
	return &compatibleTransaction{NativeTransaction: tx}
}

//////////// implement CompatibleBackendTransaction //////////////

func (tx *compatibleTransaction) GetCompatibleValue(opts *grocksdb.ReadOptions, table string, key []byte) (*common.DBValue, error) {
	v, err := tx.NativeGet(opts, common.MergeKey(table, key))
	if err != nil {
		return nil, err
	}

	return common.DeserializeDBValue(v), nil
}

func (tx *compatibleTransaction) PutCompatibleValue(table string, key []byte, value *common.DBValue) error {
	return tx.NativePut(common.MergeKey(table, key), value.Serialize())
}

func (tx *compatibleTransaction) CompatibleDelete(table string, key []byte) error {
	return tx.NativeDelete(common.MergeKey(table, key))
}

func (tx *compatibleTransaction) NewCompatibleIterator(beginPrefix, endPrefix []byte) backend.CompatibleBackendIterator {
	return newCompatibleIterator(tx.NewNativeIterator(beginPrefix, endPrefix))
}
