package main

import (
	"encoding/json"
	"log"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

// AuditLogger records all command execution attempts for security auditing.
type AuditLogger struct {
	logger *log.Logger
}

// NewAuditLogger creates a new AuditLogger with automatic log rotation.
// The logger uses lumberjack for rotating log files based on size and age.
func NewAuditLogger(logPath string) *AuditLogger {
	// Create lumberjack logger for log rotation
	writer := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    100, // MB
		MaxBackups: 10,  // Keep 10 old files
		MaxAge:     30,  // Keep for 30 days
		Compress:   true,
	}

	return &AuditLogger{
		logger: log.New(writer, "", 0), // No prefix, no timestamp (we add our own)
	}
}

// LogExecution records a command execution attempt (successful or failed).
func (a *AuditLogger) LogExecution(req CommandRequest, resp CommandResponse, err error, sourceIP string) {
	record := map[string]interface{}{
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
		"command":     req.Command,
		"args":        req.Args,
		"result":      "success",
		"exit_code":   resp.ExitCode,
		"duration_ms": resp.DurationMs,
		"source_ip":   sourceIP,
	}

	// Mark as failed if there was an error or non-zero exit code
	if err != nil || resp.ExitCode != 0 {
		record["result"] = "failed"
		if err != nil {
			record["error_message"] = err.Error()
		}
	}

	// Serialize to JSON and log
	data, _ := json.Marshal(record)
	a.logger.Println(string(data))
}

// LogRejection records a request that was rejected during validation.
func (a *AuditLogger) LogRejection(req CommandRequest, reason string, sourceIP string) {
	record := map[string]interface{}{
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
		"command":          req.Command,
		"args":             req.Args,
		"result":           "rejected",
		"rejection_reason": reason,
		"source_ip":        sourceIP,
	}

	// Serialize to JSON and log
	data, _ := json.Marshal(record)
	a.logger.Println(string(data))
}
