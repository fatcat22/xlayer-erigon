package common

import (
	"bytes"
	"context"
	"fmt"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/order"
)

type Cursor2Iter struct {
	c  kv.Cursor
	tx kv.Tx

	fromPrefix, toPrefix, nextK, nextV []byte
	orderAscend                        order.By
	limit                              int64
	ctx                                context.Context
}

func NewCursor2Iter(ctx context.Context, table string, tx kv.Tx, fromPrefix, toPrefix []byte, orderAscend order.By, limit int) (*Cursor2Iter, error) {
	s := &Cursor2Iter{ctx: ctx, tx: tx, fromPrefix: fromPrefix, toPrefix: toPrefix, orderAscend: orderAscend, limit: int64(limit)}
	return s.init(table, tx)

}

func (s *Cursor2Iter) init(table string, tx kv.Tx) (*Cursor2Iter, error) {
	if s.orderAscend && s.fromPrefix != nil && s.toPrefix != nil && bytes.Compare(s.fromPrefix, s.toPrefix) >= 0 {
		return s, fmt.Errorf("tx.Dual: %x must be lexicographicaly before %x", s.fromPrefix, s.toPrefix)
	}
	if !s.orderAscend && s.fromPrefix != nil && s.toPrefix != nil && bytes.Compare(s.fromPrefix, s.toPrefix) <= 0 {
		return s, fmt.Errorf("tx.Dual: %x must be lexicographicaly before %x", s.toPrefix, s.fromPrefix)
	}
	c, err := tx.Cursor(table)
	if err != nil {
		return s, err
	}
	s.c = c

	if s.fromPrefix == nil { // no initial position
		if s.orderAscend {
			s.nextK, s.nextV, err = s.c.First()
		} else {
			s.nextK, s.nextV, err = s.c.Last()
		}
		return s, err
	}

	if s.orderAscend {
		s.nextK, s.nextV, err = s.c.Seek(s.fromPrefix)
		return s, err
	} else {
		// seek exactly to given key or previous one
		s.nextK, s.nextV, err = s.c.SeekExact(s.fromPrefix)
		if err != nil {
			return s, err
		}
		if s.nextK != nil { // go to last value of this key
			if casted, ok := s.c.(kv.CursorDupSort); ok {
				s.nextV, err = casted.LastDup()
			}
		} else { // key not found, go to prev one
			s.nextK, s.nextV, err = s.c.Prev()
		}
		return s, err
	}
}

func (s *Cursor2Iter) advance() (err error) {
	if s.orderAscend {
		s.nextK, s.nextV, err = s.c.Next()
	} else {
		s.nextK, s.nextV, err = s.c.Prev()
	}
	return err
}

func (s *Cursor2Iter) HasNext() bool {
	if s.limit == 0 { // limit reached
		return false
	}
	if s.nextK == nil { // EndOfTable
		return false
	}
	if s.toPrefix == nil { // s.nextK == nil check is above
		return true
	}

	//Asc:  [from, to) AND from > to
	//Desc: [from, to) AND from < to
	cmp := bytes.Compare(s.nextK, s.toPrefix)
	return (bool(s.orderAscend) && cmp < 0) || (!bool(s.orderAscend) && cmp > 0)
}

func (s *Cursor2Iter) Next() (k, v []byte, err error) {
	select {
	case <-s.ctx.Done():
		return nil, nil, s.ctx.Err()
	default:
	}
	s.limit--
	k, v = s.nextK, s.nextV
	if err = s.advance(); err != nil {
		return nil, nil, err
	}
	return k, v, nil
}

func (s *Cursor2Iter) Close() {
	if s.c != nil {
		s.c.Close()
		s.c = nil
	}
}
