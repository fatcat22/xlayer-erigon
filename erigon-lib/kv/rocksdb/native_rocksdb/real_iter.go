package native_rocksdb

import (
	common2 "github.com/ledgerwatch/erigon-lib/kv/rocksdb/common"
	"github.com/linxGnu/grocksdb"
)

type RealIterator struct {
	ropts *grocksdb.ReadOptions
	it    *grocksdb.Iterator
}

func newRealIterator(ropts *grocksdb.ReadOptions, it *grocksdb.Iterator) *RealIterator {
	return &RealIterator{
		ropts: ropts,
		it:    it,
	}
}

func (iter *RealIterator) Valid() bool {
	return iter.it.Valid()
}

func (iter *RealIterator) SeekToFirst() {
	iter.it.SeekToFirst()
}

func (iter *RealIterator) SeekToLast() {
	iter.it.SeekToLast()
}

func (iter *RealIterator) Next() {
	iter.it.Next()
}

func (iter *RealIterator) Prev() {
	iter.it.Prev()
}

func (iter *RealIterator) Seek(key []byte) {
	iter.it.Seek(key)
}

func (iter *RealIterator) Key() []byte {
	k := iter.it.Key()
	if !k.Exists() {
		return nil
	}
	return common2.MoveSliceToBytes(k)
}

func (iter *RealIterator) Value() *common2.DBValue {
	v := iter.it.Value()
	if !v.Exists() {
		return nil
	}
	return common2.DeserializeDBValue(common2.MoveSliceToBytes(v))
}

func (iter *RealIterator) Close() {
	iter.it.Close()
	iter.ropts.Destroy()
}

func (iter *RealIterator) Err() error {
	return iter.it.Err()
}
