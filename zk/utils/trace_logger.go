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
	traceLogPath string
	traceLogFile *os.File
	traceLogger  *log.Logger
)

// Write a trace log line
func writeTraceLogInternal(v ...interface{}) {
	format := "%s,%s,%s,%s,%s,%s,%d,%d,%s,%s,%s,%d,%s,%s,%d,%s,%d,%s,%s,%s,%s,%d"
	message := fmt.Sprintf(format, v...)
	if len(message) == 0 || message[len(message)-1] != '\n' {
		message += "\n"
	}
	if traceLogger != nil {
		traceLogger.Print(message)
	} else {
		erigonlog.Warnf("traceLogger is not initialized, log not written")
	}
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

// Set the path for trace logs and initialize logger
func SetTraceLogPath(newPath string) {
	if newPath != "" {
		logDir := filepath.Dir(newPath)
		if _, err := os.Stat(logDir); os.IsNotExist(err) {
			_ = os.MkdirAll(logDir, 0755)
		}
		if traceLogFile != nil {
			traceLogFile.Close()
		}
		f, err := os.OpenFile(newPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			erigonlog.Errorf("Failed to open trace log file %s: %v", newPath, err)
			return
		}
		traceLogFile = f
		traceLogger = log.New(traceLogFile, "", 0)
		traceLogPath = newPath
		erigonlog.Infof("Trace log path set to: %s", traceLogPath)
	} else {
		erigonlog.Warnf("Attempted to set an empty trace log path. Current path remains: %s", traceLogPath)
	}
}
