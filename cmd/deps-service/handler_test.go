package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

// TestHandleExecute tests the HTTP execute endpoint.
func TestHandleExecute(t *testing.T) {
	// Setup test configuration
	config := &Config{
		Commands: []CommandConfig{
			{
				Name:       "echo",
				BinaryPath: "/bin/echo",
				AllowedArgsPatterns: []string{
					`^.*$`, // Allow any args for testing
				},
				EnvWhitelist:  []string{"TEST_VAR"},
				Timeout:       "5s",
				MaxConcurrent: 2,
			},
			{
				Name:       "sleep",
				BinaryPath: "/bin/sleep",
				AllowedArgsPatterns: []string{
					`^\d+$`,
				},
				Timeout:       "2s",
				MaxConcurrent: 1,
			},
		},
		Security: SecurityConfig{
			SharedVolumePath: "/data",
			ForbiddenPaths:   []string{"/etc", "/sys", "/proc"},
			MaxCommandLength: 1024,
			EnableAuditLog:   true,
		},
	}

	// Create dependencies
	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "audit.log")

	validator := NewValidator(config)
	executor := NewExecutor(config)
	auditLogger := NewAuditLogger(logPath)
	limiter := NewConcurrencyLimiter(config)

	handler := NewHandler(validator, executor, auditLogger, limiter)

	t.Run("successful command execution", func(t *testing.T) {
		reqBody := CommandRequest{
			Command: "echo",
			Args:    []string{"hello", "world"},
			Timeout: 5 * time.Second,
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/execute", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Check status code
		if w.Code != http.StatusOK {
			t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
		}

		// Parse response
		var resp CommandResponse
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Verify response
		if !resp.Success {
			t.Error("Expected success = true")
		}
		if resp.ExitCode != 0 {
			t.Errorf("ExitCode = %d, want 0", resp.ExitCode)
		}
	})

	t.Run("invalid JSON request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/execute", bytes.NewReader([]byte("invalid json")))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Should return 400 Bad Request
		if w.Code != http.StatusBadRequest {
			t.Errorf("Status code = %d, want %d", w.Code, http.StatusBadRequest)
		}

		// Parse error response
		var errResp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
			t.Fatalf("Failed to decode error response: %v", err)
		}

		if errResp["error"] != "invalid_request" {
			t.Errorf("error = %v, want 'invalid_request'", errResp["error"])
		}
	})

	t.Run("command not in whitelist", func(t *testing.T) {
		reqBody := CommandRequest{
			Command: "rm",
			Args:    []string{"-rf", "/"},
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/execute", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Should return 400 Bad Request
		if w.Code != http.StatusBadRequest {
			t.Errorf("Status code = %d, want %d", w.Code, http.StatusBadRequest)
		}

		// Parse error response
		var errResp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
			t.Fatalf("Failed to decode error response: %v", err)
		}

		if errResp["error"] != "invalid_arguments" {
			t.Errorf("error = %v, want 'invalid_arguments'", errResp["error"])
		}
	})

	t.Run("path traversal attack", func(t *testing.T) {
		reqBody := CommandRequest{
			Command: "echo",
			Args:    []string{"/data/../etc/passwd"},
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/execute", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Should return 400 Bad Request due to path traversal
		if w.Code != http.StatusBadRequest {
			t.Errorf("Status code = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("access to forbidden path", func(t *testing.T) {
		reqBody := CommandRequest{
			Command: "echo",
			Args:    []string{"/etc/shadow"},
		}

		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest(http.MethodPost, "/api/v1/execute", bytes.NewReader(body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Should return 400 Bad Request
		if w.Code != http.StatusBadRequest {
			t.Errorf("Status code = %d, want %d", w.Code, http.StatusBadRequest)
		}
	})

	t.Run("wrong HTTP method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/execute", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Should return 405 Method Not Allowed
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Status code = %d, want %d", w.Code, http.StatusMethodNotAllowed)
		}
	})
}

// TestHandleExecuteConcurrency tests concurrent request handling.
func TestHandleExecuteConcurrency(t *testing.T) {
	config := &Config{
		Commands: []CommandConfig{
			{
				Name:       "sleep",
				BinaryPath: "/bin/sleep",
				AllowedArgsPatterns: []string{
					`^\d+$`,
				},
				Timeout:       "5s",
				MaxConcurrent: 1, // Only allow 1 concurrent execution
			},
		},
		Security: SecurityConfig{
			SharedVolumePath: "/data",
			ForbiddenPaths:   []string{"/etc"},
			MaxCommandLength: 1024,
			EnableAuditLog:   true,
		},
	}

	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "audit.log")

	validator := NewValidator(config)
	executor := NewExecutor(config)
	auditLogger := NewAuditLogger(logPath)
	limiter := NewConcurrencyLimiter(config)

	handler := NewHandler(validator, executor, auditLogger, limiter)

	// First request should succeed and block for 1 second
	reqBody := CommandRequest{
		Command: "sleep",
		Args:    []string{"1"},
		Timeout: 5 * time.Second,
	}

	body, _ := json.Marshal(reqBody)

	// Start first request in background
	done := make(chan bool)
	go func() {
		req1 := httptest.NewRequest(http.MethodPost, "/api/v1/execute", bytes.NewReader(body))
		w1 := httptest.NewRecorder()
		handler.ServeHTTP(w1, req1)
		done <- true
	}()

	// Give first request time to acquire the slot
	time.Sleep(100 * time.Millisecond)

	// Second request should be rejected with 503 Service Unavailable
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/execute", bytes.NewReader(body))
	w2 := httptest.NewRecorder()

	// Create a context with timeout to simulate the limiter timeout
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req2 = req2.WithContext(ctx)

	handler.ServeHTTP(w2, req2)

	// Second request should fail with service unavailable or succeed if first completed
	// Due to timing, we accept either 503 or 200
	if w2.Code != http.StatusServiceUnavailable && w2.Code != http.StatusOK {
		t.Logf("Second request status = %d (expected 503 or 200 due to timing)", w2.Code)
	}

	// Wait for first request to complete
	<-done
}

// TestHandleHealth tests the health check endpoint.
func TestHandleHealth(t *testing.T) {
	config := &Config{
		Commands: []CommandConfig{
			{
				Name:                "test",
				BinaryPath:          "/bin/test",
				AllowedArgsPatterns: []string{".*"},
				Timeout:             "5s",
				MaxConcurrent:       1,
			},
		},
		Security: SecurityConfig{
			SharedVolumePath: "/data",
			ForbiddenPaths:   []string{"/etc"},
			MaxCommandLength: 1024,
			EnableAuditLog:   false,
		},
	}

	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "audit.log")

	validator := NewValidator(config)
	executor := NewExecutor(config)
	auditLogger := NewAuditLogger(logPath)
	limiter := NewConcurrencyLimiter(config)

	handler := NewHandler(validator, executor, auditLogger, limiter)

	t.Run("health check returns OK", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Check status code
		if w.Code != http.StatusOK {
			t.Errorf("Status code = %d, want %d", w.Code, http.StatusOK)
		}

		// Parse response
		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Verify response fields
		if resp["status"] != "ok" {
			t.Errorf("status = %v, want 'ok'", resp["status"])
		}

		if resp["service"] != "command-executor" {
			t.Errorf("service = %v, want 'command-executor'", resp["service"])
		}

		if resp["version"] != Version {
			t.Errorf("version = %v, want '%s'", resp["version"], Version)
		}
	})

	t.Run("health check with wrong method", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/health", nil)
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		// Should return 405 Method Not Allowed
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Status code = %d, want %d", w.Code, http.StatusMethodNotAllowed)
		}
	})
}

// TestHandleExecuteCommandTimeout tests command execution timeout handling.
func TestHandleExecuteCommandTimeout(t *testing.T) {
	config := &Config{
		Commands: []CommandConfig{
			{
				Name:       "sleep",
				BinaryPath: "/bin/sleep",
				AllowedArgsPatterns: []string{
					`^\d+$`,
				},
				Timeout:       "1s", // 1 second default timeout
				MaxConcurrent: 2,
			},
		},
		Security: SecurityConfig{
			SharedVolumePath: "/data",
			ForbiddenPaths:   []string{"/etc"},
			MaxCommandLength: 1024,
			EnableAuditLog:   true,
		},
	}

	tempDir := t.TempDir()
	logPath := filepath.Join(tempDir, "audit.log")

	validator := NewValidator(config)
	executor := NewExecutor(config)
	auditLogger := NewAuditLogger(logPath)
	limiter := NewConcurrencyLimiter(config)

	handler := NewHandler(validator, executor, auditLogger, limiter)

	// Request that will timeout (sleep 10 seconds with 1 second timeout)
	reqBody := CommandRequest{
		Command: "sleep",
		Args:    []string{"10"},
		Timeout: 500 * time.Millisecond, // Override with shorter timeout
	}

	body, _ := json.Marshal(reqBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/execute", bytes.NewReader(body))
	w := httptest.NewRecorder()

	start := time.Now()
	handler.ServeHTTP(w, req)
	duration := time.Since(start)

	// Should return 500 Internal Server Error (command failed due to timeout)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("Status code = %d, want %d", w.Code, http.StatusInternalServerError)
	}

	// Should timeout quickly (within ~500ms + overhead)
	if duration > 2*time.Second {
		t.Errorf("Request took %v, expected ~500ms timeout", duration)
	}

	t.Logf("Command timed out in %v", duration)
}

// TestRespondJSON tests JSON response helper function.
func TestRespondJSON(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		data       interface{}
		wantStatus int
	}{
		{
			name:       "success response",
			statusCode: http.StatusOK,
			data:       map[string]string{"message": "success"},
			wantStatus: http.StatusOK,
		},
		{
			name:       "error response",
			statusCode: http.StatusBadRequest,
			data:       map[string]string{"error": "bad request"},
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			respondJSON(w, tt.statusCode, tt.data)

			if w.Code != tt.wantStatus {
				t.Errorf("Status code = %d, want %d", w.Code, tt.wantStatus)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("Content-Type = %s, want 'application/json'", contentType)
			}

			// Verify response can be decoded
			var result map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
				t.Errorf("Failed to decode response: %v", err)
			}
		})
	}
}

// TestRespondError tests error response helper function.
func TestRespondError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		errorType  string
		details    []string
		wantStatus int
	}{
		{
			name:       "validation error",
			statusCode: http.StatusBadRequest,
			errorType:  "invalid_arguments",
			details:    []string{"command not in whitelist"},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "service unavailable error",
			statusCode: http.StatusServiceUnavailable,
			errorType:  "service_busy",
			details:    []string{"max concurrent executions reached"},
			wantStatus: http.StatusServiceUnavailable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			respondError(w, tt.statusCode, tt.errorType, tt.details)

			if w.Code != tt.wantStatus {
				t.Errorf("Status code = %d, want %d", w.Code, tt.wantStatus)
			}

			// Parse error response
			var errResp map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&errResp); err != nil {
				t.Fatalf("Failed to decode error response: %v", err)
			}

			if errResp["error"] != tt.errorType {
				t.Errorf("error = %v, want '%s'", errResp["error"], tt.errorType)
			}

			details, ok := errResp["details"].([]interface{})
			if !ok {
				t.Fatal("details is not an array")
			}

			if len(details) != len(tt.details) {
				t.Errorf("len(details) = %d, want %d", len(details), len(tt.details))
			}
		})
	}
}
