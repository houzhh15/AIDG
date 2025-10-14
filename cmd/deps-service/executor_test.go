package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestExecuteCommand tests command execution with various scenarios.
func TestExecuteCommand(t *testing.T) {
	// Setup test configuration
	config := &Config{
		Commands: []CommandConfig{
			{
				Name:                "echo",
				BinaryPath:          "/bin/echo",
				AllowedArgsPatterns: []string{`.*`},
				EnvWhitelist:        []string{"TEST_VAR"},
				Timeout:             "5s",
				MaxConcurrent:       2,
			},
			{
				Name:                "sleep",
				BinaryPath:          "/bin/sleep",
				AllowedArgsPatterns: []string{`^\d+$`},
				EnvWhitelist:        []string{},
				Timeout:             "2s",
				MaxConcurrent:       1,
			},
			{
				Name:                "ls",
				BinaryPath:          "/bin/ls",
				AllowedArgsPatterns: []string{`^-[a-z]+$`, `^/.*$`},
				EnvWhitelist:        []string{},
				Timeout:             "3s",
				MaxConcurrent:       3,
			},
		},
		Security: SecurityConfig{
			SharedVolumePath: "/data",
			ForbiddenPaths:   []string{"/etc"},
			MaxCommandLength: 1024,
		},
	}

	executor := NewExecutor(config)
	ctx := context.Background()

	tests := []struct {
		name         string
		req          CommandRequest
		wantSuccess  bool
		wantExitCode int
		checkStdout  string
		checkStderr  bool
		expectError  bool
		errorMsg     string
	}{
		{
			name: "successful echo command",
			req: CommandRequest{
				Command: "echo",
				Args:    []string{"hello", "world"},
				Timeout: 5 * time.Second,
			},
			wantSuccess:  true,
			wantExitCode: 0,
			checkStdout:  "hello world",
			expectError:  false,
		},
		{
			name: "command with environment variable",
			req: CommandRequest{
				Command: "echo",
				Args:    []string{"$TEST_VAR"},
				Env: map[string]string{
					"TEST_VAR": "test_value",
				},
				Timeout: 5 * time.Second,
			},
			wantSuccess:  true,
			wantExitCode: 0,
			expectError:  false,
		},
		{
			name: "command timeout",
			req: CommandRequest{
				Command: "sleep",
				Args:    []string{"10"},
				Timeout: 1 * time.Second,
			},
			wantSuccess:  false,
			wantExitCode: -1, // Killed by timeout
			expectError:  true,
			errorMsg:     "timeout",
		},
		{
			name: "nonexistent command",
			req: CommandRequest{
				Command: "nonexistent",
				Args:    []string{},
			},
			wantSuccess: false,
			expectError: true,
			errorMsg:    "command config",
		},
		{
			name: "command with working directory",
			req: CommandRequest{
				Command:    "ls",
				Args:       []string{"-la", "."},
				WorkingDir: "/tmp",
				Timeout:    3 * time.Second,
			},
			wantSuccess:  true,
			wantExitCode: 0,
			expectError:  false,
		},
		{
			name: "command with zero exit code",
			req: CommandRequest{
				Command: "ls",
				Args:    []string{"/tmp"},
				Timeout: 3 * time.Second,
			},
			wantSuccess:  true,
			wantExitCode: 0,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := executor.ExecuteCommand(ctx, tt.req)

			// Check error expectation
			if tt.expectError {
				if err == nil {
					t.Errorf("ExecuteCommand() expected error containing '%s', got nil", tt.errorMsg)
					return
				}
				if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("ExecuteCommand() error = %v, want error containing '%s'", err, tt.errorMsg)
				}
				return
			}

			// No error expected beyond this point
			if err != nil && !tt.expectError {
				t.Errorf("ExecuteCommand() unexpected error = %v", err)
				return
			}

			// Check success flag
			if resp.Success != tt.wantSuccess {
				t.Errorf("ExecuteCommand() Success = %v, want %v", resp.Success, tt.wantSuccess)
			}

			// Check exit code
			if tt.wantExitCode >= 0 && resp.ExitCode != tt.wantExitCode {
				t.Errorf("ExecuteCommand() ExitCode = %v, want %v", resp.ExitCode, tt.wantExitCode)
			}

			// Check stdout content
			if tt.checkStdout != "" && !strings.Contains(resp.Stdout, tt.checkStdout) {
				t.Errorf("ExecuteCommand() Stdout = %v, want to contain '%s'", resp.Stdout, tt.checkStdout)
			}

			// Check duration is recorded
			if resp.DurationMs < 0 {
				t.Errorf("ExecuteCommand() DurationMs = %v, should be >= 0", resp.DurationMs)
			}
		})
	}
}

// TestExecuteCommandTimeout tests timeout behavior in detail.
func TestExecuteCommandTimeout(t *testing.T) {
	config := &Config{
		Commands: []CommandConfig{
			{
				Name:                "sleep",
				BinaryPath:          "/bin/sleep",
				AllowedArgsPatterns: []string{`^\d+$`},
				Timeout:             "1s",
				MaxConcurrent:       1,
			},
		},
		Security: SecurityConfig{
			SharedVolumePath: "/data",
			ForbiddenPaths:   []string{},
			MaxCommandLength: 1024,
		},
	}

	executor := NewExecutor(config)
	ctx := context.Background()

	// Test with request timeout override
	req := CommandRequest{
		Command: "sleep",
		Args:    []string{"5"},
		Timeout: 500 * time.Millisecond,
	}

	start := time.Now()
	_, err := executor.ExecuteCommand(ctx, req)
	duration := time.Since(start)

	// Should timeout
	if err == nil {
		t.Error("ExecuteCommand() expected timeout error, got nil")
	}

	// Should timeout within ~500ms (allow some overhead)
	if duration > 2*time.Second {
		t.Errorf("ExecuteCommand() took %v, expected ~500ms timeout", duration)
	}
}

// TestExecuteCommandWithContext tests context cancellation.
func TestExecuteCommandWithContext(t *testing.T) {
	config := &Config{
		Commands: []CommandConfig{
			{
				Name:                "sleep",
				BinaryPath:          "/bin/sleep",
				AllowedArgsPatterns: []string{`^\d+$`},
				Timeout:             "10s",
				MaxConcurrent:       1,
			},
		},
		Security: SecurityConfig{
			SharedVolumePath: "/data",
			ForbiddenPaths:   []string{},
			MaxCommandLength: 1024,
		},
	}

	executor := NewExecutor(config)

	// Create cancellable context
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after 500ms
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	req := CommandRequest{
		Command: "sleep",
		Args:    []string{"10"},
		Timeout: 10 * time.Second,
	}

	start := time.Now()
	_, err := executor.ExecuteCommand(ctx, req)
	duration := time.Since(start)

	// Should be cancelled
	if err == nil {
		t.Error("ExecuteCommand() expected cancellation error, got nil")
	}

	// Should cancel within ~500ms
	if duration > 2*time.Second {
		t.Errorf("ExecuteCommand() took %v, expected ~500ms cancellation", duration)
	}
}

// TestExecuteCommandWorkingDirectory tests working directory setting.
func TestExecuteCommandWorkingDirectory(t *testing.T) {
	config := &Config{
		Commands: []CommandConfig{
			{
				Name:                "pwd",
				BinaryPath:          "/bin/pwd",
				AllowedArgsPatterns: []string{},
				Timeout:             "3s",
				MaxConcurrent:       1,
			},
		},
		Security: SecurityConfig{
			SharedVolumePath: "/data",
			ForbiddenPaths:   []string{},
			MaxCommandLength: 1024,
		},
	}

	executor := NewExecutor(config)
	ctx := context.Background()

	// Create a temp directory for testing
	tempDir := t.TempDir()

	req := CommandRequest{
		Command:    "pwd",
		Args:       []string{},
		WorkingDir: tempDir,
		Timeout:    3 * time.Second,
	}

	resp, err := executor.ExecuteCommand(ctx, req)
	if err != nil {
		t.Fatalf("ExecuteCommand() unexpected error = %v", err)
	}

	// Check that output contains the temp directory path
	if !strings.Contains(resp.Stdout, tempDir) {
		t.Errorf("ExecuteCommand() Stdout = %v, want to contain '%s'", resp.Stdout, tempDir)
	}
}

// TestExecuteCommandEnvironment tests environment variable passing.
func TestExecuteCommandEnvironment(t *testing.T) {
	// Skip on systems without /usr/bin/env
	if _, err := os.Stat("/usr/bin/env"); os.IsNotExist(err) {
		t.Skip("Skipping test: /usr/bin/env not available")
	}

	config := &Config{
		Commands: []CommandConfig{
			{
				Name:                "env",
				BinaryPath:          "/usr/bin/env",
				AllowedArgsPatterns: []string{`.*`},
				EnvWhitelist:        []string{"MY_TEST_VAR"},
				Timeout:             "3s",
				MaxConcurrent:       1,
			},
		},
		Security: SecurityConfig{
			SharedVolumePath: "/data",
			ForbiddenPaths:   []string{},
			MaxCommandLength: 1024,
		},
	}

	executor := NewExecutor(config)
	ctx := context.Background()

	req := CommandRequest{
		Command: "env",
		Args:    []string{},
		Env: map[string]string{
			"MY_TEST_VAR": "test_value_12345",
		},
		Timeout: 3 * time.Second,
	}

	resp, err := executor.ExecuteCommand(ctx, req)
	if err != nil {
		t.Fatalf("ExecuteCommand() unexpected error = %v", err)
	}

	// Check that the custom env var appears in output
	if !strings.Contains(resp.Stdout, "MY_TEST_VAR=test_value_12345") {
		t.Errorf("ExecuteCommand() Stdout does not contain custom environment variable")
	}
}
