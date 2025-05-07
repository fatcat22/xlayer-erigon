package native_rocksdb

import (
	"encoding/binary"
	"errors"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/iter"
	"github.com/ledgerwatch/erigon-lib/kv/order"
	rdbcommon "github.com/ledgerwatch/erigon-lib/kv/rocksdb/common"
	"github.com/linxGnu/grocksdb"
	"golang.org/x/net/context"
	"unsafe"
)

type nativeTransaction struct {
	ctx           context.Context
	tx            *grocksdb.Transaction
	defaultRdopts *grocksdb.ReadOptions

	cursors  map[uint64]kv.Closer
	cursorID uint64
	streams  map[int]kv.Closer
	streamID int

	statelessCursors map[string]kv.RwCursor
}

func newNativeTransaction(ctx context.Context, tx *grocksdb.Transaction) *nativeTransaction {
	defaultRdopts := grocksdb.NewDefaultReadOptions()
	return &nativeTransaction{
		ctx:           ctx,
		tx:            tx,
		defaultRdopts: defaultRdopts,
	}
}

////////////// implement NativeTransactionBase //////////////

func (rtx *nativeTransaction) Commit() error {
	return rtx.tx.Commit()
}

func (rtx *nativeTransaction) Rollback() {
	_ = rtx.tx.Rollback() // ignore rollback error
}

func (rtx *nativeTransaction) Destroy() {
	rtx.tx.Destroy()
	rtx.defaultRdopts.Destroy()
}

////////////// implement NativeTransaction //////////////

func (rtx *nativeTransaction) NativeGet(opts *grocksdb.ReadOptions, key []byte) ([]byte, error) {
	s, err := rtx.tx.Get(opts, key)
	if err != nil {
		return nil, err
	}
	if !s.Exists() {
		return nil, rdbcommon.ErrKeyNotExist
	}

	return rdbcommon.MoveSliceToBytes(s), nil
}

func (rtx *nativeTransaction) NativePut(key, value []byte) error {
	return rtx.tx.Put(key, value)
}

func (rtx *nativeTransaction) NativeDelete(key []byte) error {
	return rtx.tx.Delete(key)
}

func (rtx *nativeTransaction) NewNativeIterator(beginPrefix, endPrefix []byte) NativeIterator {
	return newNativeIterator(beginPrefix, endPrefix, rtx.tx)
}

////////////// implement kv.RwTx //////////////

func (rtx *nativeTransaction) Has(table string, key []byte) (bool, error) {
	// todo: what if use table as ColumnFamily
	_, err := rtx.NativeGet(rtx.defaultRdopts, rdbcommon.MergeKey(table, key))
	if err != nil {
		if errors.Is(err, rdbcommon.ErrKeyNotExist) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (rtx *nativeTransaction) GetOne(table string, key []byte) (val []byte, err error) {
	val, err = rtx.NativeGet(rtx.defaultRdopts, rdbcommon.MergeKey(table, key))
	if err != nil {
		if errors.Is(err, rdbcommon.ErrKeyNotExist) {
			// keep same as mdbx.GetOne
			return nil, nil
		}
	}
	return val, err
}

func (rtx *nativeTransaction) ForEach(table string, fromPrefix []byte, walker func(k, v []byte) error) error {
	return rtx.foreach(rdbcommon.MergeKey(table, fromPrefix), nil, walker, nil)
}

func (rtx *nativeTransaction) ForPrefix(table string, prefix []byte, walker func(k, v []byte) error) error {
	beginPrefix := rdbcommon.MergeKey(table, prefix)
	endPrefix, _ := kv.NextSubtree(beginPrefix)
	return rtx.foreach(beginPrefix, endPrefix, walker, nil)
}

func (rtx *nativeTransaction) ForAmount(table string, fromPrefix []byte, amount uint32, walker func(k, v []byte) error) error {
	if amount == 0 {
		return nil
	}

	return rtx.foreach(rdbcommon.MergeKey(table, fromPrefix), nil, walker, func() bool {
		amount--
		return amount == 0
	})
}

func (rtx *nativeTransaction) Put(table string, k, v []byte) error {
	return rtx.NativePut(rdbcommon.MergeKey(table, k), v)
}

func (rtx *nativeTransaction) Delete(table string, k []byte) error {
	return rtx.NativeDelete(rdbcommon.MergeKey(table, k))
}

func (rtx *nativeTransaction) ReadSequence(table string) (uint64, error) {
	v, err := rtx.GetOne(kv.Sequence, []byte(table))
	if err != nil && !errors.Is(err, rdbcommon.ErrKeyNotExist) {
		return 0, err
	}

	var currentV uint64
	if len(v) > 0 {
		currentV = binary.BigEndian.Uint64(v)
	}

	return currentV, nil
}

func (rtx *nativeTransaction) ViewID() uint64 {
	panic("nativeTransaction.ViewID is not supported")
}

func (rtx *nativeTransaction) Cursor(table string) (kv.Cursor, error) {
	return newNativeRwCursor(table, rtx), nil
}

func (rtx *nativeTransaction) CursorDupSort(table string) (kv.CursorDupSort, error) {
	return newNativeRwCursor(table, rtx), nil
}

func (rtx *nativeTransaction) DBSize() (uint64, error) {
	panic("nativeTransaction.DBSize is not supported")
}

func (rtx *nativeTransaction) Range(table string, fromPrefix, toPrefix []byte) (iter.KV, error) {
	return rtx.RangeAscend(table, fromPrefix, toPrefix, -1)
}

func (rtx *nativeTransaction) RangeAscend(table string, fromPrefix, toPrefix []byte, limit int) (iter.KV, error) {
	return rtx.rangeOrderLimit(table, fromPrefix, toPrefix, order.Asc, limit)
}

func (rtx *nativeTransaction) RangeDescend(table string, fromPrefix, toPrefix []byte, limit int) (iter.KV, error) {
	return rtx.rangeOrderLimit(table, fromPrefix, toPrefix, order.Desc, limit)
}

func (rtx *nativeTransaction) Prefix(table string, prefix []byte) (iter.KV, error) {
	nextPrefix, ok := kv.NextSubtree(prefix)
	if !ok {
		return rtx.Range(table, prefix, nil)
	}
	return rtx.Range(table, prefix, nextPrefix)
}

func (rtx *nativeTransaction) RangeDupSort(table string, key []byte, fromPrefix, toPrefix []byte, asc order.By, limit int) (iter.KV, error) {
	panic("nativeTransaction.RangeDupSort is not supported")
}

func (rtx *nativeTransaction) CHandle() unsafe.Pointer {
	panic("nativeTransaction.CHandle is not supported")
}

func (rtx *nativeTransaction) BucketSize(table string) (uint64, error) {
	panic("nativeTransaction.BucketSize is not supported")
}

func (rtx *nativeTransaction) IncrementSequence(table string, amount uint64) (uint64, error) {
	currentV, err := rtx.ReadSequence(table)
	if err != nil {
		return 0, err
	}

	newVBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(newVBytes, currentV+amount)
	if err := rtx.Put(kv.Sequence, []byte(table), newVBytes); err != nil {
		return 0, err
	}

	return currentV, nil
}

func (rtx *nativeTransaction) Append(table string, k, v []byte) error {
	return rtx.Put(table, k, v)
}

func (rtx *nativeTransaction) AppendDup(table string, k, v []byte) error {
	panic("nativeTransaction.AppendDup is not supported")
}

func (rtx *nativeTransaction) ListBuckets() ([]string, error) {
	panic("nativeTransaction.ListBuckets is not supported")
}

func (rtx *nativeTransaction) DropBucket(string) error {
	panic("nativeTransaction.DropBucket is not supported")
}

func (rtx *nativeTransaction) CreateBucket(string) error {
	// do nothing
	return nil
}

func (rtx *nativeTransaction) ExistsBucket(string) (bool, error) {
	panic("nativeTransaction.ExistsBucket is not supported")
}

func (rtx *nativeTransaction) ClearBucket(string) error {
	panic("nativeTransaction.ClearBucket is not supported")
}

func (rtx *nativeTransaction) RwCursor(table string) (kv.RwCursor, error) {
	return newNativeRwCursor(table, rtx), nil
}

func (rtx *nativeTransaction) RwCursorDupSort(table string) (kv.RwCursorDupSort, error) {
	panic("nativeTransaction.RwCursorDupSort is not supported")
}

func (rtx *nativeTransaction) CollectMetrics() {
	// todo: not implemented yet
}

func (rtx *nativeTransaction) SpaceDirty() (uint64, uint64, error) {
	panic("nativeTransaction.SpaceDirty is not supported")
}

////////////// internal methods //////////////

func (rtx *nativeTransaction) foreach(beginPrefix, endPrefix []byte, walker func(k, v []byte) error, isStop func() bool) error {
	it := rtx.NewNativeIterator(beginPrefix, endPrefix)
	defer it.Close()

	for it.SeekToFirst(); it.Valid(); it.Next() {
		if err := walker(it.NativeKey(), it.NativeValue()); err != nil {
			return err
		}
		if isStop != nil {
			if isStop() {
				break
			}
		}
	}

	return nil
}

func (rtx *nativeTransaction) rangeOrderLimit(table string, fromPrefix, toPrefix []byte, orderAscend order.By, limit int) (*rdbcommon.Cursor2Iter, error) {
	s, err := rdbcommon.NewCursor2Iter(rtx.ctx, table, rtx, fromPrefix, toPrefix, orderAscend, limit)
	if err != nil {
		return nil, err
	}

	rtx.streamID++
	if rtx.streams == nil {
		rtx.streams = map[int]kv.Closer{}
	}
	rtx.streams[rtx.streamID] = s

	return s, nil
}
