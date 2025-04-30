package backend_type

import (
	"fmt"
	rdbcommon "github.com/ledgerwatch/erigon-lib/kv/rocksdb/common"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/compatible_rocksdb/backend"
	backendmock "github.com/ledgerwatch/erigon-lib/kv/rocksdb/compatible_rocksdb/backend/mock"
	backendrocksdb "github.com/ledgerwatch/erigon-lib/kv/rocksdb/compatible_rocksdb/backend/rocksdb"
)

type CompatibleBackendType int

const (
	_ CompatibleBackendType = iota
	compatibleBackendRocksDB
	compatibleBackendMock
)

func ToCompatibleBackendType(s string) CompatibleBackendType {
	switch s {
	case "rocksdb":
		return compatibleBackendRocksDB
	case "mock":
		return compatibleBackendMock
	default:
		panic(fmt.Sprintf("unknown compatible type: %s", s))
	}
}

func (ct CompatibleBackendType) NewBackend(dbPath string, readOnly bool, options *rdbcommon.RocksDBOptions) (backend.CompatibleBackend, error) {
	switch ct {
	case compatibleBackendRocksDB:
		return backendrocksdb.NewCompatibleBackendRocksDB(dbPath, readOnly, options)
	case compatibleBackendMock:
		options.Destroy()
		return backendmock.NewMemoryRDB(), nil
	default:
		panic(fmt.Sprintf("unknown compatible type: %d", ct))
	}
}
