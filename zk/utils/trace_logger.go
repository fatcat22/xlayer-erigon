// zk/utils/trace_logger.go
package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	erigonlog "github.com/ledgerwatch/erigon/zkevm/log"
)

var (
	traceLogEnabled bool
	traceLogPath    string
	traceLogFile    *os.File
	traceLogger     *log.Logger
)

// Write a trace log line
func writeTraceLogInternal(v ...interface{}) {
	format := "%s,%s,%s,%s,%s,%s,%d,%d,%s,%s,%s,%d,%s,%s,%d,%s,%d,%s,%s,%s,%s,%d"
	message := fmt.Sprintf(format, v...)
	traceLogger.Print(message)
}

// Public logging function
func LogTrace(
	txhash string,
	serviceName string,
	processId uint64,
	processWord string,
	blockHeight uint64,
	blockHash string,
	blockTime uint64,
	transactionType int8,
) {
	if !traceLogEnabled || traceLogger == nil {
		return
	}
	allArgs := []interface{}{
		Chain,
		txhash,
		Status,
		serviceName,
		Business,
		Client,
		ChainID,
		processId,
		processWord,
		Index,
		innerIndex,
		time.Now().UnixMilli(),
		ReferId,
		ContractAddress,
		blockHeight,
		blockHash,
		blockTime,
		DepositConfirmHeight,
		TokenID,
		MevSupplier,
		BusinessHash,
		transactionType,
	}
	writeTraceLogInternal(allArgs...)
}

func SetTraceLogConfig(enabled bool, path string) {
	if !enabled {
		traceLogEnabled = false
		erigonlog.Info("Trace logging is disabled.")
		return
	}

	if path == "" {
		traceLogEnabled = false
		erigonlog.Warn("Trace logging enabled in config, but no path provided. Logger will not be initialized.")
		return
	}

	logDir := filepath.Dir(path)
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		if mkErr := os.MkdirAll(logDir, 0755); mkErr != nil {
			erigonlog.Errorf("Failed to create trace log directory %s: %v. Trace logging will be off.", logDir, mkErr)
			traceLogEnabled = false
			return
		}
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		erigonlog.Errorf("Failed to open trace log file %s: %v. Trace logging will be off.", path, err)
		traceLogEnabled = false
		return
	}

	traceLogFile = f
	traceLogger = log.New(traceLogFile, "", 0)
	traceLogPath = path
	traceLogEnabled = true
	erigonlog.Infof("Trace logging enabled. Path set to: %s", traceLogPath)
}
