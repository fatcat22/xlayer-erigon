package rocksdb

import (
	"fmt"
	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/dbutils"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/common"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/compatible_rocksdb"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/compatible_rocksdb/backend/backend_type"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/native_rocksdb"
	// "github.com/ledgerwatch/erigon-lib/kv/rocksdb/native_rocksdb/mock_rocksdb"
	"github.com/ledgerwatch/log/v3"
	"golang.org/x/sync/semaphore"
)

type RocksDBType struct {
	major          rdbType
	compatibleType backend_type.CompatibleBackendType
}

type rdbType int

const (
	_ rdbType = iota
	rdbTypeCompatible
	rdbTypeNative
)

func ToRocksDBType(s string) RocksDBType {
	var dbType RocksDBType

	major, others := dbutils.SplitAtFirst(s, ".")
	switch major {
	case "compatible":
		dbType.major = rdbTypeCompatible
		dbType.compatibleType = backend_type.ToCompatibleBackendType(others)
	case "native":
		dbType.major = rdbTypeNative
	//case "mock":
	//	dbType.major = rdbTypeTypeMock
	default:
		panic(fmt.Sprintf("unknown rocksdb type: %s", s))
	}

	return dbType
}

func (rt RocksDBType) NewRocksDB(dbPath string, logger log.Logger, tablesCfg kv.TableCfg, label kv.Label, readTxLimiter *semaphore.Weighted, readOnly bool, options *common.RocksDBOptions) (kv.RwDB, error) {
	switch rt.major {
	//case rdbTypeTypeMock:
	//	return mock_rocksdb.NewMemoryRDB(), nil
	case rdbTypeNative:
		return native_rocksdb.NewNativeRocksDB(dbPath, readOnly, options)
	case rdbTypeCompatible:
		return compatible_rocksdb.NewCompatibleRocksDB(dbPath, logger, tablesCfg, label, readTxLimiter, readOnly, rt.compatibleType, options)
	default:
		panic(fmt.Sprintf("unknown NativeRocksDB type: %v", rt))
	}
}
