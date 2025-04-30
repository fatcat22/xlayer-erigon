package backend

import (
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/common"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/native_rocksdb"
	"github.com/linxGnu/grocksdb"
)

type CompatibleBackend interface {
	native_rocksdb.NativeRocksDBBase

	GetCompatibleValue(table string, key []byte) (*common.DBValue, error)
	NewCompatibleTransaction(opts *grocksdb.WriteOptions, transactionOpts *grocksdb.TransactionOptions, oldTransaction *grocksdb.Transaction) CompatibleBackendTransaction
}

type CompatibleBackendTransaction interface {
	native_rocksdb.NativeTransactionBase

	GetCompatibleValue(opts *grocksdb.ReadOptions, table string, key []byte) (*common.DBValue, error)
	PutCompatibleValue(table string, key []byte, value *common.DBValue) error
	CompatibleDelete(table string, key []byte) error
	NewCompatibleIterator(beginPrefix, endPrefix []byte) CompatibleBackendIterator
}

type CompatibleBackendIterator interface {
	native_rocksdb.NativeIteratorBase

	CompatibleSeek(table string, key []byte)
	CompatibleKey() (string, []byte)
	CompatibleValue() *common.DBValue
}
