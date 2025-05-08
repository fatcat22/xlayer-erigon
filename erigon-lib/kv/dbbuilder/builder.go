package dbbuilder

import (
	"context"
	"fmt"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/combinedb"
	"github.com/ledgerwatch/erigon-lib/kv/dbutils"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/common"
	"path"
)

type majorDBType int

const (
	majorDBTypeMdbx majorDBType = iota // default value
	majorDBTypeRocksdb
	majorDBTypeCombine
)

type DatabaseType struct {
	major       majorDBType
	rocksdbType rocksdb.RocksDBType
	combineType combinedb.CombineDBType
}

// ToDatabaseType convert the string to DatabaseType.
// the string could be:
// - mdbx
// - rocksdb.native
// - rocksdb.compatible.mock
// - rocksdb.compatible.rocksdb
// - combine.native
// - combine.compatible.mock
// - combine.compatible.rocksdb
func ToDatabaseType(s string) DatabaseType {
	var dbType DatabaseType

	major, others := dbutils.SplitAtFirst(s, ".")
	switch major {
	case "mdbx":
		dbType.major = majorDBTypeMdbx
	case "rocksdb":
		dbType.major = majorDBTypeRocksdb
		dbType.rocksdbType = rocksdb.ToRocksDBType(others)
	case "combine":
		dbType.major = majorDBTypeCombine
		dbType.combineType = combinedb.ToCombineDBType(others)
	default:
		panic(fmt.Sprintf("unknown db type: %s", s))
	}

	return dbType
}

func (dt DatabaseType) NewDB(ctx context.Context, opts mdbx.MdbxOpts, tableCfg kv.TableCfg, enableCombineLog bool) (kv.RwDB, error) {
	switch dt.major {
	case majorDBTypeMdbx:
		opts = opts.Path(path.Join(opts.GetPath(), "mdbx"))
		return opts.Open(ctx)
	case majorDBTypeRocksdb:
		options := common.NewRocksDBOptions()
		return dt.rocksdbType.NewRocksDB(opts.GetPath(), opts.GetLogger(), tableCfg, opts.GetLabel(), opts.GetRoTxsLimiter(), opts.IsReadonly(), options)
	case majorDBTypeCombine:
		options := common.NewRocksDBOptions()
		return combinedb.NewCombinDB(ctx, opts, tableCfg, enableCombineLog, options, dt.combineType)
	default:
		panic(fmt.Sprintf("unknown databse type: %v", dt.major))
	}
}
