package native_rocksdb

import (
	"bytes"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/common"
)

type nativeRwCursor struct {
	it NativeIterator
	tx NativeTransaction

	table string
}

func newNativeRwCursor(table string, tx NativeTransaction) *nativeRwCursor {
	beginPrefix := common.MergeKey(table, []byte{})
	endPrefix, _ := kv.NextSubtree(beginPrefix)
	return &nativeRwCursor{
		it: tx.NewNativeIterator(beginPrefix, endPrefix),
		tx: tx,

		table: table,
	}
}

////////////// implement RwCursor //////////////

func (c *nativeRwCursor) First() ([]byte, []byte, error) {
	c.it.SeekToFirst()
	if !c.it.Valid() {
		return nil, nil, common.ErrInvalidIter
	}

	_, key := common.SplitKey(c.it.NativeKey())
	return key, c.it.NativeValue(), nil
}

func (c *nativeRwCursor) Seek(seek []byte) ([]byte, []byte, error) {
	c.it.NativeSeek(common.MergeKey(c.table, seek))
	if !c.it.Valid() {
		return nil, nil, common.ErrInvalidIter
	}

	_, key := common.SplitKey(c.it.NativeKey())
	return key, c.it.NativeValue(), nil
}

func (c *nativeRwCursor) SeekExact(seekK []byte) ([]byte, []byte, error) {
	seekK = common.MergeKey(c.table, seekK)
	c.it.NativeSeek(seekK)
	if !c.it.Valid() {
		return nil, nil, common.ErrInvalidIter
	}

	curKey := c.it.NativeKey()
	if !bytes.Equal(curKey, seekK) {
		return nil, nil, common.ErrInvalidIter
	}

	_, curKey = common.SplitKey(curKey)
	return curKey, c.it.NativeValue(), nil
}

func (c *nativeRwCursor) Next() ([]byte, []byte, error) {
	c.it.Next()
	if !c.it.Valid() {
		return nil, nil, common.ErrInvalidIter
	}

	_, key := common.SplitKey(c.it.NativeKey())
	return key, c.it.NativeValue(), nil
}

func (c *nativeRwCursor) Prev() ([]byte, []byte, error) {
	c.it.Prev()
	if !c.it.Valid() {
		return nil, nil, common.ErrInvalidIter
	}

	_, key := common.SplitKey(c.it.NativeKey())
	return key, c.it.NativeValue(), nil
}

func (c *nativeRwCursor) Last() ([]byte, []byte, error) {
	c.it.SeekToLast()
	if !c.it.Valid() {
		return nil, nil, common.ErrInvalidIter
	}

	_, key := common.SplitKey(c.it.NativeKey())
	return key, c.it.NativeValue(), nil
}

func (c *nativeRwCursor) Current() ([]byte, []byte, error) {
	if !c.it.Valid() {
		return nil, nil, common.ErrInvalidIter
	}

	_, key := common.SplitKey(c.it.NativeKey())
	return key, c.it.NativeValue(), nil
}

func (c *nativeRwCursor) Count() (uint64, error) {
	panic("nativeRwCursor.Count is not supported")
}

func (c *nativeRwCursor) Close() {
	c.it.Close()
}

func (c *nativeRwCursor) Put(k, v []byte) error {
	return c.tx.NativePut(common.MergeKey(c.table, k), v)
}

func (c *nativeRwCursor) Append(k []byte, v []byte) error {
	panic("nativeRwCursor.Append is not supported")
}

func (c *nativeRwCursor) Delete(k []byte) error {
	return c.tx.NativeDelete(common.MergeKey(c.table, k))
}

func (c *nativeRwCursor) DeleteCurrent() error {
	panic("nativeRwCursor.DeleteCurrent is not supported")
}
