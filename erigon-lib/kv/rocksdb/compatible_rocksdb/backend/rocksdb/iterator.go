package rocksdb

import (
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/common"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/native_rocksdb"
)

type compatibleIterator struct {
	native_rocksdb.NativeIterator
}

func newCompatibleIterator(it native_rocksdb.NativeIterator) *compatibleIterator {
	return &compatibleIterator{
		NativeIterator: it,
	}
}

//////////// implement CompatibleBackendIterator //////////////

func (it *compatibleIterator) CompatibleSeek(table string, key []byte) {
	it.NativeSeek(common.MergeKey(table, key))
}

func (it *compatibleIterator) CompatibleKey() (string, []byte) {
	k := it.NativeKey()
	return common.SplitKey(k)
}

func (it *compatibleIterator) CompatibleValue() *common.DBValue {
	return common.DeserializeDBValue(it.NativeValue())
}
