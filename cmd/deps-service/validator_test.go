package main

import (
	"testing"
	"time"
)

// TestValidateRequest tests the comprehensive security validation logic.
func TestValidateRequest(t *testing.T) {
	// Setup test configuration
	config := &Config{
		Commands: []CommandConfig{
			{
				Name:       "ffmpeg",
				BinaryPath: "/usr/bin/ffmpeg",
				AllowedArgsPatterns: []string{
					`^-i$`,
					`^/data/.*\.wav$`,
					`^/data/.*\.mp3$`,
					`^-ar$`,
					`^\d+$`,
					`^-ac$`,
					`^-y$`,
				},
				EnvWhitelist:  []string{"LOG_LEVEL", "TEMP_DIR"},
				Timeout:       "300s",
				MaxConcurrent: 3,
			},
			{
				Name:       "python",
				BinaryPath: "/usr/bin/python3",
				AllowedArgsPatterns: []string{
					`^/data/.*\.py$`,
					`^--.*$`,
				},
				EnvWhitelist:  []string{"PYTHONPATH"},
				Timeout:       "600s",
				MaxConcurrent: 2,
			},
		},
		Security: SecurityConfig{
			SharedVolumePath: "/data",
			ForbiddenPaths:   []string{"/etc", "/sys", "/proc", "/root"},
			MaxCommandLength: 1024,
			EnableAuditLog:   true,
		},
	}

	validator := NewValidator(config)

	// Table-driven tests
	tests := []struct {
		name    string
		req     CommandRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid ffmpeg command",
			req: CommandRequest{
				Command: "ffmpeg",
				Args: []string{
					"-i", "/data/meetings/001/input.wav",
					"-ar", "16000",
					"-ac", "1",
					"-y", "/data/meetings/001/output.mp3",
				},
				WorkingDir: "/data/meetings/001",
				Timeout:    300 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "command not in whitelist",
			req: CommandRequest{
				Command: "rm",
				Args:    []string{"-rf", "/data/test"},
			},
			wantErr: true,
			errMsg:  "not in whitelist",
		},
		{
			name: "path traversal attack with ..",
			req: CommandRequest{
				Command: "ffmpeg",
				Args: []string{
					"-i", "/data/meetings/../../../etc/passwd",
					"-y", "/data/output.mp3",
				},
			},
			wantErr: true,
			errMsg:  "..", // Will fail on argument pattern first, then path validation
		},
		{
			name: "access to forbidden path /etc",
			req: CommandRequest{
				Command: "ffmpeg",
				Args:    []string{"-i", "/data/input.wav", "-y", "/etc/shadow"},
			},
			wantErr: true,
			errMsg:  "", // Any error is acceptable - pattern or path validation
		},
		{
			name: "access to forbidden path /sys",
			req: CommandRequest{
				Command: "ffmpeg",
				Args:    []string{"-i", "/data/input.wav", "-y", "/sys/kernel/debug"},
			},
			wantErr: true,
			errMsg:  "", // Any error is acceptable
		},
		{
			name: "access to forbidden path /proc",
			req: CommandRequest{
				Command: "ffmpeg",
				Args:    []string{"-i", "/data/input.wav", "-y", "/proc/self/environ"},
			},
			wantErr: true,
			errMsg:  "", // Any error is acceptable
		},
		{
			name: "command exceeds max length",
			req: CommandRequest{
				Command: "ffmpeg",
				Args:    []string{"-i", string(make([]byte, 2000))}, // Very long argument
			},
			wantErr: true,
			errMsg:  "exceeds maximum allowed",
		},
		{
			name: "argument does not match allowed pattern",
			req: CommandRequest{
				Command: "ffmpeg",
				Args:    []string{"-i", "/data/input.wav", "--dangerous-option"},
			},
			wantErr: true,
			errMsg:  "does not match any allowed pattern",
		},
		{
			name: "working directory outside shared volume",
			req: CommandRequest{
				Command:    "ffmpeg",
				Args:       []string{"-i", "/data/input.wav"},
				WorkingDir: "/tmp/unsafe",
			},
			wantErr: true,
			errMsg:  "within allowed directories",
		},
		{
			name: "unauthorized environment variable",
			req: CommandRequest{
				Command: "ffmpeg",
				Args:    []string{"-i", "/data/input.wav"},
				Env: map[string]string{
					"LOG_LEVEL":      "debug", // Allowed
					"DANGEROUS_PATH": "/etc",  // Not in whitelist
				},
			},
			wantErr: true,
			errMsg:  "not in whitelist",
		},
		{
			name: "valid environment variables",
			req: CommandRequest{
				Command: "ffmpeg",
				Args:    []string{"-i", "/data/input.wav", "-y", "/data/output.mp3"},
				Env: map[string]string{
					"LOG_LEVEL": "info",
					"TEMP_DIR":  "/data/temp",
				},
			},
			wantErr: false,
		},
		{
			name: "valid python command with allowed patterns",
			req: CommandRequest{
				Command: "python",
				Args:    []string{"/data/scripts/process.py", "--verbose", "--output=/data/result.txt"},
			},
			wantErr: false,
		},
		{
			name: "path not starting with shared volume",
			req: CommandRequest{
				Command: "ffmpeg",
				Args:    []string{"-i", "/data/input.wav", "-y", "/tmp/output.mp3"},
			},
			wantErr: true,
			errMsg:  "", // Any error is acceptable - could be pattern or path validation
		},
	}

	// Run all test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateRequest(tt.req)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateRequest() expected error containing '%s', but got nil", tt.errMsg)
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateRequest() error = %v, want error containing '%s'", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateRequest() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestValidateArgs tests argument pattern validation.
func TestValidateArgs(t *testing.T) {
	config := &Config{
		Commands: []CommandConfig{
			{
				Name: "test",
				AllowedArgsPatterns: []string{
					`^-[a-z]$`,   // Single letter flags
					`^\d+$`,      // Numbers only
					`^/data/.*$`, // Paths under /data
				},
			},
		},
		Security: SecurityConfig{
			SharedVolumePath: "/data",
			ForbiddenPaths:   []string{"/etc"},
			MaxCommandLength: 1000,
		},
	}

	validator := NewValidator(config)

	tests := []struct {
		name     string
		args     []string
		patterns []string
		wantErr  bool
	}{
		{
			name:     "all args match patterns",
			args:     []string{"-i", "123", "/data/file.txt"},
			patterns: config.Commands[0].AllowedArgsPatterns,
			wantErr:  false,
		},
		{
			name:     "one arg does not match",
			args:     []string{"-i", "abc", "/data/file.txt"},
			patterns: config.Commands[0].AllowedArgsPatterns,
			wantErr:  true,
		},
		{
			name:     "empty args",
			args:     []string{},
			patterns: config.Commands[0].AllowedArgsPatterns,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateArgs(tt.args, tt.patterns)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateArgs() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestValidatePath tests path security validation.
func TestValidatePath(t *testing.T) {
	config := &Config{
		Security: SecurityConfig{
			SharedVolumePath: "/data",
			ForbiddenPaths:   []string{"/etc", "/sys", "/proc"},
			MaxCommandLength: 1000,
		},
	}

	validator := NewValidator(config)

	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid path under shared volume",
			path:    "/data/meetings/001/audio.wav",
			wantErr: false,
		},
		{
			name:    "path traversal with ..",
			path:    "/data/../etc/passwd",
			wantErr: true,
			errMsg:  "path traversal",
		},
		{
			name:    "forbidden path /etc",
			path:    "/etc/shadow",
			wantErr: true,
			errMsg:  "forbidden directory",
		},
		{
			name:    "forbidden path /sys",
			path:    "/sys/class/net",
			wantErr: true,
			errMsg:  "forbidden directory",
		},
		{
			name:    "path outside shared volume",
			path:    "/tmp/data",
			wantErr: true,
			errMsg:  "within allowed directories",
		},
		{
			name:    "shared volume root",
			path:    "/data",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validatePath(tt.path)

			if tt.wantErr {
				if err == nil {
					t.Errorf("validatePath() expected error, got nil")
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("validatePath() error = %v, want error containing '%s'", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validatePath() unexpected error = %v", err)
				}
			}
		})
	}
}

// TestValidateEnv tests environment variable whitelist validation.
func TestValidateEnv(t *testing.T) {
	validator := NewValidator(&Config{})

	tests := []struct {
		name      string
		env       map[string]string
		whitelist []string
		wantErr   bool
	}{
		{
			name: "all env vars in whitelist",
			env: map[string]string{
				"LOG_LEVEL": "debug",
				"TEMP_DIR":  "/data/temp",
			},
			whitelist: []string{"LOG_LEVEL", "TEMP_DIR", "DEBUG"},
			wantErr:   false,
		},
		{
			name: "one env var not in whitelist",
			env: map[string]string{
				"LOG_LEVEL": "debug",
				"EVIL_PATH": "/etc",
			},
			whitelist: []string{"LOG_LEVEL"},
			wantErr:   true,
		},
		{
			name:      "empty env map",
			env:       map[string]string{},
			whitelist: []string{"LOG_LEVEL"},
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.validateEnv(tt.env, tt.whitelist)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateEnv() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Helper function to check if a string contains a substring.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
