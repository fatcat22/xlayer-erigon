package rocksdb

import (
	"fmt"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/native_rocksdb"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/native_rocksdb/mock_rocksdb"
	"github.com/linxGnu/grocksdb"
)

type RDBType int

const (
	MemRDB RDBType = iota
	RealRDB
)

func (rt RDBType) NewRDB(opts *grocksdb.Options, txopts *grocksdb.TransactionDBOptions, dbPath string) (native_rocksdb.RDB, error) {
	switch rt {
	case MemRDB:
		return mock_rocksdb.NewMemoryRDB(), nil
	case RealRDB:
		return native_rocksdb.NewRealRDB(opts, txopts, dbPath)
	default:
		panic(fmt.Sprintf("unknown RDB type: %v", rt))
	}
}
