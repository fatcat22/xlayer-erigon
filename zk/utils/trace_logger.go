// zk/utils/trace_logger.go
package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	log "github.com/ledgerwatch/erigon/zkevm/log"
)

var (
	traceLogPath string
)

// Internal logging function with hardcoded format string
func writeTraceLogInternal(v ...interface{}) {
	// Format string defining 22 fields
	format := "%s,%s,%s,%s,%s,%s,%d,%d,%s,%s,%s,%d,%s,%s,%d,%s,%d,%s,%s,%s,%s,%s"

	logFilePath := traceLogPath

	f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Errorf("Error: Failed to open trace log file %s: %v\n", logFilePath, err)
		return
	}
	defer f.Close()

	message := fmt.Sprintf(format, v...)

	if len(message) == 0 || message[len(message)-1] != '\n' {
		message += "\n"
	}

	if _, err := f.WriteString(message); err != nil {
		log.Errorf("Error: Failed to write to trace log file %s: %v\n", logFilePath, err)
	}
}

// Public logging function with standardized parameters
func LogTrace(
	txhash string,
	serviceName string,
	processId uint64,
	processWord string,
	blockHeight uint64,
	blockHash string,
	blockTime uint64,
	transactionType string,
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

// Set the path for trace logs, creating directories if needed
func SetTraceLogPath(newPath string) {
	if newPath != "" {
		logDir := filepath.Dir(newPath)
		if _, err := os.Stat(logDir); os.IsNotExist(err) {
			errMkdir := os.MkdirAll(logDir, 0755)
			if errMkdir != nil {
				log.Errorf("Failed to create trace log directory %s: %v", logDir, errMkdir)
			}
		}
		traceLogPath = newPath
		log.Infof("Trace log path set to: %s", traceLogPath)
	} else {
		log.Warnf("Attempted to set an empty trace log path. Current path remains: %s", traceLogPath)
	}
}
