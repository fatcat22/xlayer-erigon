package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"time"

	"github.com/ledgerwatch/erigon-lib/kv"
	"github.com/ledgerwatch/erigon-lib/kv/dbbuilder"
	"github.com/ledgerwatch/erigon-lib/kv/mdbx"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/common"
	"github.com/ledgerwatch/erigon-lib/kv/rocksdb/compatible_rocksdb"
	"github.com/ledgerwatch/log/v3"
	"github.com/linxGnu/grocksdb"
	"golang.org/x/sync/semaphore"
)

type dataPair struct {
	K []byte
	V []byte
}

func main() {
	go func() {
		_ = http.ListenAndServe(":6060", nil)
	}()
	// Command-line argument parsing
	mdbxPath := flag.String("mdbx", "", "Path to the source MDBX database")
	rocksdbPath := flag.String("rocksdb", "", "Path to the target RocksDB database")
	verbose := flag.Bool("verbose", false, "Whether to output detailed logs")
	labelStr := flag.String("label", "", "database label")
	dbTypeStr := flag.String("dbtype", "", "database type")
	flag.Parse()

	if *mdbxPath == "" || *rocksdbPath == "" {
		fmt.Println("Usage: mdbx2rocksdb --mdbx /mdbx/db/path --rocksdb /rocksdb/path [--verbose]")
		os.Exit(1)
	}

	// Configure logging
	logger := log.New()
	if *verbose {
		logger.SetHandler(log.LvlFilterHandler(log.LvlInfo, log.StderrHandler))
	} else {
		logger.SetHandler(log.LvlFilterHandler(log.LvlWarn, log.StderrHandler))
	}
	logger.Info("Starting mdbx2rocksdb", "mdbxPath", *mdbxPath, "rocksdbPath", *rocksdbPath, "label", *labelStr, "dbType", *dbTypeStr)

	// Configure logging
	if err := os.MkdirAll(*rocksdbPath, 0755); err != nil {
		logger.Error("Failed to create target directory", "path", *rocksdbPath, "error", err)
		os.Exit(1)
	}

	label := kv.UnmarshalLabel(*labelStr)
	dbType := dbbuilder.ToDatabaseType(*dbTypeStr)

	// Open the source MDBX database
	srcDB := openMDBX(*mdbxPath, label, logger)
	defer srcDB.Close()

	logger.Info("start querying all tables")
	normalTables, specialTables := queryAllTables(srcDB, map[string]struct{}{
		kv.HashedStorage: {},
		kv.PlainState:    {},
	})
	logger.Info("all normal tables", "special tables", specialTables, "normal tables", normalTables)

	var totalRecords uint64
	startTime := time.Now()
	logger.Info("Starting database conversion")

	// handle special tables
	if label == kv.ChainDB {
		memDstDB := openCompatibleMockDB(label, logger)
		defer memDstDB.Close()
		totalRecords += convertTables(specialTables, srcDB, memDstDB, logger, func() {
			memDatas := memDstDB.(*compatible_rocksdb.CompatibleRocksDB).GetMemStorage()
			writeDeduplicatedDirect(memDatas, *rocksdbPath, logger)
		})
		memDstDB.Close()
	}

	// handle normal tables
	dstDB := openDestinationDB(*rocksdbPath, label, dbType, logger)
	defer dstDB.Close()
	totalRecords += convertTables(normalTables, srcDB, dstDB, logger, func() {})

	elapsed := time.Since(startTime)
	logger.Info("Conversion completed successfully",
		"total_records", totalRecords,
		"total_time", elapsed,
		"avg_speed", fmt.Sprintf("%.0f records/sec", float64(totalRecords)/elapsed.Seconds()))
}

func queryAllTables(db kv.RwDB, specialTablesMap map[string]struct{}) (normalTables []string, specialTables []string) {
	if err := db.View(context.Background(), func(tx kv.Tx) error {
		tables, err := tx.ListBuckets()
		if err != nil {
			return err
		}

		for _, table := range tables {
			if _, ok := specialTablesMap[table]; ok {
				specialTables = append(specialTables, table)
			} else {
				normalTables = append(normalTables, table)
			}
		}
		return nil
	}); err != nil {
		panic(fmt.Sprintf("Failed to list buckets. err=%v", err))
	}

	return
}

func buildMdbxOpts(path string, label kv.Label, logger log.Logger) mdbx.MdbxOpts {
	roTxLimit := int64(32)
	roTxsLimiter := semaphore.NewWeighted(roTxLimit)

	return mdbx.NewMDBX(logger).
		Path(path).
		RoTxsLimiter(roTxsLimiter).
		Readonly().
		Label(label)
}

func openMDBX(path string, label kv.Label, logger log.Logger) kv.RwDB {
	logger.Info("Opening MDBX database", "path", path, "label", label)
	return openDB(dbbuilder.ToDatabaseType("mdbx"), buildMdbxOpts(path, label, logger), kv.ChaindataTablesCfg)
}

func openCompatibleMockDB(label kv.Label, logger log.Logger) kv.RwDB {
	return openDB(dbbuilder.ToDatabaseType("rocksdb.compatible.mock"), buildMdbxOpts("", label, logger), kv.ChaindataTablesCfg)
}

func openDestinationDB(path string, label kv.Label, dbType dbbuilder.DatabaseType, logger log.Logger) kv.RwDB {
	return openDB(dbType, buildMdbxOpts(path, label, logger), kv.ChaindataTablesCfg)
}

func openDB(rdbType dbbuilder.DatabaseType, opts mdbx.MdbxOpts, tableCfg kv.TableCfg) kv.RwDB {
	db, err := rdbType.NewDB(context.Background(), opts, tableCfg, false)
	if err != nil {
		panic(fmt.Sprintf("Failed to open rocksdb. path=%s. err=%v", opts.GetPath(), err))
	}
	return db
}

func writeDeduplicatedDirect(data map[string]*common.DBValue, dbPath string, logger log.Logger) {
	opts := grocksdb.NewDefaultOptions()
	opts.SetCreateIfMissing(true)
	defer opts.Destroy()
	rdb, err := grocksdb.OpenTransactionDb(opts, grocksdb.NewDefaultTransactionDBOptions(), dbPath)
	if err != nil {
		panic(err)
	}
	defer rdb.Close()

	wopts := grocksdb.NewDefaultWriteOptions()
	defer wopts.Destroy()
	txopts := grocksdb.NewDefaultTransactionOptions()
	defer txopts.Destroy()
	tx := rdb.TransactionBegin(wopts, txopts, nil)

	logger.Info("total deduplicated data", "total", len(data))
	batchCount := 0
	for key, value := range data {
		if err := tx.Put([]byte(key), value.Serialize()); err != nil {
			panic(err)
		}

		batchCount++
		if batchCount >= 10000 {
			if err := tx.Commit(); err != nil {
				panic(err)
			}
			tx.Destroy()
			logger.Info("committed deduplicated", "count", batchCount)

			tx = rdb.TransactionBegin(wopts, txopts, nil)
		}
	}
	if err := tx.Commit(); err != nil {
		panic(err)
	}
	tx.Destroy()
}

func convertTables(tables []string, srcDB, dstDB kv.RwDB, logger log.Logger, afterConvertF func()) uint64 {
	var totalRecords uint64

	for _, table := range tables {
		logger.Info("Converting table", "name", table)

		start := time.Now()
		keysStat, recordCount := convertTable(table, srcDB, dstDB, logger)
		elapsed := time.Since(start)

		duplicateKeyCnt := 0
		for _, v := range keysStat {
			if v > 1 {
				duplicateKeyCnt++
			}
		}

		logger.Info("Table conversion completed",
			"table", table,
			"records", recordCount,
			"duplicate key count", duplicateKeyCnt,
			"time", elapsed,
			"speed", fmt.Sprintf("%.0f records/sec", float64(recordCount)/elapsed.Seconds()))
		totalRecords += recordCount
	}

	afterConvertF()

	return totalRecords
}

func convertTable(table string, srcDB, dstDB kv.RwDB, logger log.Logger) (map[string]int, uint64) {
	keysStat := make(map[string]int)
	recordCount := uint64(0)

	// Process records in batches to avoid a single large transaction
	batchSize := 10000
	batch := make([]dataPair, 0, batchSize)
	batchCount := 0

	// Start data migration transaction
	if err := srcDB.View(context.Background(), func(srcTx kv.Tx) error {
		srcCursor, err := srcTx.Cursor(table)
		if err != nil {
			panic(fmt.Sprintf("Failed to create source cursor. table=%s. err=%v", table, err))
		}
		defer srcCursor.Close()

		// Iterate over records in the source database
		for k, v, err := srcCursor.First(); k != nil; k, v, err = srcCursor.Next() {
			if err != nil {
				panic(fmt.Sprintf("Failed to read record. table=%s, err=%v", table, err))
			}

			batch = append(batch, dataPair{K: k, V: v})
			sk := string(k)
			if cnt, ok := keysStat[sk]; ok {
				keysStat[sk] = cnt + 1
			} else {
				keysStat[sk] = 1
			}
			recordCount++

			if len(batch) >= batchSize {
				start := time.Now()
				putBatch(table, dstDB, batch)

				logger.Info("Put batch", "table", table, "index", batchCount, "batch size", len(batch), "cost", time.Since(start))

				batchCount++
				batch = batch[:0]
			}
		}
		return nil
	}); err != nil {
		panic(fmt.Sprintf("source view error. table=%s. err=%v", table, err))
	}
	putBatch(table, dstDB, batch)

	return keysStat, recordCount
}

func putBatch(table string, dstDB kv.RwDB, batch []dataPair) {
	if len(batch) == 0 {
		return
	}

	err := dstDB.Update(context.Background(), func(dstTx kv.RwTx) error {
		dstCursor, err := dstTx.RwCursor(table)
		if err != nil {
			panic(fmt.Sprintf("Failed to create rw cursor. table=%s. err=%v", table, err))
		}
		defer dstCursor.Close()

		for _, item := range batch {
			if err := dstCursor.Put(item.K, item.V); err != nil {
				panic(fmt.Sprintf("Failed to write record. table=%s. err=%v", table, err))
			}
		}

		return nil
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to update batch. table=%s. err=%v", table, err))
	}
}
