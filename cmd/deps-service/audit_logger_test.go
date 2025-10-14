package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestAuditLogger tests audit log recording functionality.
func TestAuditLogger(t *testing.T) {
	// Create a temporary log file
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "audit.log")

	logger := NewAuditLogger(logPath)

	t.Run("log successful execution", func(t *testing.T) {
		req := CommandRequest{
			Command: "ffmpeg",
			Args:    []string{"-i", "input.wav", "-y", "output.mp3"},
		}

		resp := CommandResponse{
			Success:    true,
			ExitCode:   0,
			Stdout:     "conversion successful",
			Stderr:     "",
			DurationMs: 1234,
		}

		logger.LogExecution(req, resp, nil, "192.168.1.100")

		// Read the log file
		entries := readLogEntries(t, logPath)
		if len(entries) != 1 {
			t.Fatalf("Expected 1 log entry, got %d", len(entries))
		}

		entry := entries[0]

		// Verify fields
		if entry["command"] != "ffmpeg" {
			t.Errorf("command = %v, want 'ffmpeg'", entry["command"])
		}

		if entry["result"] != "success" {
			t.Errorf("result = %v, want 'success'", entry["result"])
		}

		if exitCode, ok := entry["exit_code"].(float64); !ok || exitCode != 0 {
			t.Errorf("exit_code = %v, want 0", entry["exit_code"])
		}

		if durationMs, ok := entry["duration_ms"].(float64); !ok || durationMs != 1234 {
			t.Errorf("duration_ms = %v, want 1234", entry["duration_ms"])
		}

		if entry["source_ip"] != "192.168.1.100" {
			t.Errorf("source_ip = %v, want '192.168.1.100'", entry["source_ip"])
		}

		// Verify timestamp format
		if _, ok := entry["timestamp"].(string); !ok {
			t.Error("timestamp should be a string")
		}

		// Verify args array
		if args, ok := entry["args"].([]interface{}); !ok || len(args) != 4 {
			t.Errorf("args = %v, want array of 4 elements", entry["args"])
		}
	})

	t.Run("log failed execution with error", func(t *testing.T) {
		req := CommandRequest{
			Command: "python",
			Args:    []string{"script.py"},
		}

		resp := CommandResponse{
			Success:    false,
			ExitCode:   1,
			Stdout:     "",
			Stderr:     "script error",
			DurationMs: 567,
		}

		execError := errors.New("command execution failed")

		logger.LogExecution(req, resp, execError, "10.0.0.5")

		// Read all log entries
		entries := readLogEntries(t, logPath)
		if len(entries) < 2 {
			t.Fatalf("Expected at least 2 log entries, got %d", len(entries))
		}

		// Get the last entry
		entry := entries[len(entries)-1]

		// Verify fields
		if entry["command"] != "python" {
			t.Errorf("command = %v, want 'python'", entry["command"])
		}

		if entry["result"] != "failed" {
			t.Errorf("result = %v, want 'failed'", entry["result"])
		}

		if exitCode, ok := entry["exit_code"].(float64); !ok || exitCode != 1 {
			t.Errorf("exit_code = %v, want 1", entry["exit_code"])
		}

		if entry["error_message"] != "command execution failed" {
			t.Errorf("error_message = %v, want 'command execution failed'", entry["error_message"])
		}

		if entry["source_ip"] != "10.0.0.5" {
			t.Errorf("source_ip = %v, want '10.0.0.5'", entry["source_ip"])
		}
	})

	t.Run("log failed execution with non-zero exit code", func(t *testing.T) {
		req := CommandRequest{
			Command: "ls",
			Args:    []string{"/nonexistent"},
		}

		resp := CommandResponse{
			Success:    false,
			ExitCode:   2,
			Stdout:     "",
			Stderr:     "no such file or directory",
			DurationMs: 10,
		}

		logger.LogExecution(req, resp, nil, "127.0.0.1")

		entries := readLogEntries(t, logPath)
		entry := entries[len(entries)-1]

		// Should be marked as failed due to non-zero exit code
		if entry["result"] != "failed" {
			t.Errorf("result = %v, want 'failed'", entry["result"])
		}

		if exitCode, ok := entry["exit_code"].(float64); !ok || exitCode != 2 {
			t.Errorf("exit_code = %v, want 2", entry["exit_code"])
		}
	})

	t.Run("log rejected request", func(t *testing.T) {
		req := CommandRequest{
			Command: "rm",
			Args:    []string{"-rf", "/"},
		}

		logger.LogRejection(req, "command not in whitelist", "1.2.3.4")

		entries := readLogEntries(t, logPath)
		entry := entries[len(entries)-1]

		// Verify fields
		if entry["command"] != "rm" {
			t.Errorf("command = %v, want 'rm'", entry["command"])
		}

		if entry["result"] != "rejected" {
			t.Errorf("result = %v, want 'rejected'", entry["result"])
		}

		if entry["rejection_reason"] != "command not in whitelist" {
			t.Errorf("rejection_reason = %v, want 'command not in whitelist'", entry["rejection_reason"])
		}

		if entry["source_ip"] != "1.2.3.4" {
			t.Errorf("source_ip = %v, want '1.2.3.4'", entry["source_ip"])
		}

		// Should have timestamp
		if _, ok := entry["timestamp"].(string); !ok {
			t.Error("timestamp should be a string")
		}

		// Should have args
		if args, ok := entry["args"].([]interface{}); !ok || len(args) != 2 {
			t.Errorf("args = %v, want array of 2 elements", entry["args"])
		}
	})

	t.Run("log multiple entries", func(t *testing.T) {
		// Log several entries
		for i := 0; i < 5; i++ {
			req := CommandRequest{
				Command: "echo",
				Args:    []string{"test", string(rune('a' + i))},
			}
			resp := CommandResponse{
				Success:    true,
				ExitCode:   0,
				DurationMs: int64(100 + i),
			}
			logger.LogExecution(req, resp, nil, "192.168.1.1")
		}

		entries := readLogEntries(t, logPath)

		// Should have multiple entries (at least the 5 we just added)
		if len(entries) < 5 {
			t.Errorf("Expected at least 5 log entries, got %d", len(entries))
		}

		// Verify the last 5 entries are our echo commands
		lastEntries := entries[len(entries)-5:]
		for i, entry := range lastEntries {
			if entry["command"] != "echo" {
				t.Errorf("Entry %d: command = %v, want 'echo'", i, entry["command"])
			}
			if entry["result"] != "success" {
				t.Errorf("Entry %d: result = %v, want 'success'", i, entry["result"])
			}
		}
	})
}

// TestAuditLoggerTimestampFormat tests that timestamps are in UTC RFC3339 format.
func TestAuditLoggerTimestampFormat(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "audit_timestamp.log")

	logger := NewAuditLogger(logPath)

	req := CommandRequest{
		Command: "test",
		Args:    []string{},
	}

	resp := CommandResponse{
		Success:    true,
		ExitCode:   0,
		DurationMs: 100,
	}

	logger.LogExecution(req, resp, nil, "127.0.0.1")

	entries := readLogEntries(t, logPath)
	if len(entries) == 0 {
		t.Fatal("No log entries found")
	}

	entry := entries[0]
	timestampStr, ok := entry["timestamp"].(string)
	if !ok {
		t.Fatal("timestamp is not a string")
	}

	// Parse timestamp to verify it's valid RFC3339
	_, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		t.Errorf("timestamp '%s' is not valid RFC3339 format: %v", timestampStr, err)
	}
}

// TestAuditLoggerConcurrent tests concurrent logging from multiple goroutines.
func TestAuditLoggerConcurrent(t *testing.T) {
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "audit_concurrent.log")

	logger := NewAuditLogger(logPath)

	const numGoroutines = 10
	const entriesPerGoroutine = 10

	// Log from multiple goroutines concurrently
	done := make(chan bool, numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			for i := 0; i < entriesPerGoroutine; i++ {
				req := CommandRequest{
					Command: "concurrent_test",
					Args:    []string{string(rune('a' + goroutineID))},
				}
				resp := CommandResponse{
					Success:    true,
					ExitCode:   0,
					DurationMs: int64(goroutineID*100 + i),
				}
				logger.LogExecution(req, resp, nil, "127.0.0.1")
			}
			done <- true
		}(g)
	}

	// Wait for all goroutines to finish
	for g := 0; g < numGoroutines; g++ {
		<-done
	}

	// Verify we have the expected number of entries
	entries := readLogEntries(t, logPath)
	expectedCount := numGoroutines * entriesPerGoroutine

	if len(entries) != expectedCount {
		t.Errorf("Expected %d log entries, got %d", expectedCount, len(entries))
	}

	// Verify all entries are valid JSON with expected command
	for i, entry := range entries {
		if entry["command"] != "concurrent_test" {
			t.Errorf("Entry %d: command = %v, want 'concurrent_test'", i, entry["command"])
		}
	}
}

// readLogEntries reads and parses all JSON log entries from a file.
func readLogEntries(t *testing.T, logPath string) []map[string]interface{} {
	t.Helper()

	file, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer file.Close()

	var entries []map[string]interface{}
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var entry map[string]interface{}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("Failed to parse log entry: %v\nLine: %s", err, line)
		}

		entries = append(entries, entry)
	}

	if err := scanner.Err(); err != nil {
		t.Fatalf("Error reading log file: %v", err)
	}

	return entries
}
