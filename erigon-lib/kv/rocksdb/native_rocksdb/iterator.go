package native_rocksdb

import (
	common2 "github.com/ledgerwatch/erigon-lib/kv/rocksdb/common"
	"github.com/linxGnu/grocksdb"
)

type nativeIterator struct {
	it    *grocksdb.Iterator
	ropts *grocksdb.ReadOptions
}

func newNativeIterator(beginPrefix, endPrefix []byte, tx *grocksdb.Transaction) *nativeIterator {
	ropts := grocksdb.NewDefaultReadOptions()
	if beginPrefix != nil {
		ropts.SetIterateLowerBound(beginPrefix)
	}
	if endPrefix != nil {
		ropts.SetIterateUpperBound(endPrefix)
	}

	return &nativeIterator{
		it:    tx.NewIterator(ropts),
		ropts: ropts,
	}
}

////////////// implement NativeIteratorBase //////////////

func (iter *nativeIterator) Valid() bool {
	return iter.it.Valid()
}

func (iter *nativeIterator) SeekToFirst() {
	iter.it.SeekToFirst()
}

func (iter *nativeIterator) SeekToLast() {
	iter.it.SeekToLast()
}

func (iter *nativeIterator) Next() {
	iter.it.Next()
}

func (iter *nativeIterator) Prev() {
	iter.it.Prev()
}

func (iter *nativeIterator) Close() {
	iter.it.Close()
	iter.ropts.Destroy()
}

func (iter *nativeIterator) Err() error {
	return iter.it.Err()
}

////////////// implement NativeIterator //////////////

func (iter *nativeIterator) NativeSeek(key []byte) {
	iter.it.Seek(key)
}

func (iter *nativeIterator) NativeKey() []byte {
	k := iter.it.Key()
	if !k.Exists() {
		return nil
	}
	return common2.MoveSliceToBytes(k)
}

func (iter *nativeIterator) NativeValue() []byte {
	v := iter.it.Value()
	if !v.Exists() {
		return nil
	}
	return common2.MoveSliceToBytes(v)
}
