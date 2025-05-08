package utils

import (
	"fmt"
	stdlog "log"
	"os"
)

const traceLogFilename = "/home/erigon/data/logs/trace.log"

func WriteToTraceLog(format string, v ...interface{}) {

	logFilePath := traceLogFilename

	f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		stdlog.Printf("Error: Failed to open trace log file %s: %v\n", logFilePath, err)
		return
	}
	defer f.Close()

	message := fmt.Sprintf(format, v...)

	if len(message) == 0 || message[len(message)-1] != '\n' {
		message += "\n"
	}

	if _, err := f.WriteString(message); err != nil {
		stdlog.Printf("Error: Failed to write to trace log file %s: %v\n", logFilePath, err)
	}
}
