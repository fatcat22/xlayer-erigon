package native_rocksdb

import (
	"github.com/linxGnu/grocksdb"
)

type NativeRocksDBBase interface {
	Close()
}

type NativeRocksDB interface {
	NativeRocksDBBase

	NativeGet(opts *grocksdb.ReadOptions, key []byte) ([]byte, error)
	NewNativeTransaction(opts *grocksdb.WriteOptions, transactionOpts *grocksdb.TransactionOptions, oldTransaction *grocksdb.Transaction) NativeTransaction
}

type NativeTransactionBase interface {
	Commit() error
	Rollback()
	Destroy()
}

type NativeTransaction interface {
	NativeTransactionBase

	NativeGet(opts *grocksdb.ReadOptions, key []byte) ([]byte, error)
	NativePut(key, value []byte) error
	NativeDelete(key []byte) error
	NewNativeIterator(beginPrefix, endPrefix []byte) NativeIterator
}

type NativeIteratorBase interface {
	Valid() bool
	SeekToFirst()
	SeekToLast()
	Next()
	Prev()
	Close()
	Err() error
}

type NativeIterator interface {
	NativeIteratorBase

	NativeSeek(key []byte)
	NativeKey() []byte
	NativeValue() []byte
}
