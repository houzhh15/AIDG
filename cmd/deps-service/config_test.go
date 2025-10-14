package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadConfig tests configuration file loading and parsing.
func TestLoadConfig(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("load valid config", func(t *testing.T) {
		validConfig := `
commands:
  - name: ffmpeg
    binary_path: /usr/bin/ffmpeg
    allowed_args_patterns:
      - "^-i$"
      - "^/data/.*\\.wav$"
      - "^-ar$"
      - "^\\d+$"
    env_whitelist:
      - LOG_LEVEL
    timeout: 300s
    max_concurrent: 3

  - name: python
    binary_path: /usr/bin/python3
    allowed_args_patterns:
      - "^/data/.*\\.py$"
      - "^--.*$"
    env_whitelist:
      - PYTHONPATH
    timeout: 600s
    max_concurrent: 2

security:
  shared_volume_path: /data
  forbidden_paths:
    - /etc
    - /sys
    - /proc
  max_command_length: 1024
  enable_audit_log: true
`

		configPath := filepath.Join(tempDir, "valid_config.yaml")
		err := os.WriteFile(configPath, []byte(validConfig), 0644)
		if err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		config, err := LoadConfig(configPath)
		if err != nil {
			t.Fatalf("LoadConfig() failed: %v", err)
		}

		// Verify Commands
		if len(config.Commands) != 2 {
			t.Errorf("len(Commands) = %d, want 2", len(config.Commands))
		}

		// Check ffmpeg command
		ffmpegCmd, err := config.GetCommandConfig("ffmpeg")
		if err != nil {
			t.Errorf("GetCommandConfig('ffmpeg') failed: %v", err)
		}
		if ffmpegCmd.BinaryPath != "/usr/bin/ffmpeg" {
			t.Errorf("ffmpeg binary_path = %s, want '/usr/bin/ffmpeg'", ffmpegCmd.BinaryPath)
		}
		if ffmpegCmd.MaxConcurrent != 3 {
			t.Errorf("ffmpeg max_concurrent = %d, want 3", ffmpegCmd.MaxConcurrent)
		}

		// Check python command
		pythonCmd, err := config.GetCommandConfig("python")
		if err != nil {
			t.Errorf("GetCommandConfig('python') failed: %v", err)
		}
		if pythonCmd.Timeout != "600s" {
			t.Errorf("python timeout = %s, want '600s'", pythonCmd.Timeout)
		}

		// Verify Security config
		if config.Security.SharedVolumePath != "/data" {
			t.Errorf("shared_volume_path = %s, want '/data'", config.Security.SharedVolumePath)
		}
		if len(config.Security.ForbiddenPaths) != 3 {
			t.Errorf("len(ForbiddenPaths) = %d, want 3", len(config.Security.ForbiddenPaths))
		}
		if config.Security.MaxCommandLength != 1024 {
			t.Errorf("max_command_length = %d, want 1024", config.Security.MaxCommandLength)
		}
		if !config.Security.EnableAuditLog {
			t.Error("enable_audit_log should be true")
		}
	})

	t.Run("load config with missing file", func(t *testing.T) {
		_, err := LoadConfig("/nonexistent/config.yaml")
		if err == nil {
			t.Error("LoadConfig() should fail for missing file")
		}
	})

	t.Run("load config with invalid YAML", func(t *testing.T) {
		invalidYAML := `
commands:
  - name: test
    invalid yaml syntax here {{{
`

		configPath := filepath.Join(tempDir, "invalid.yaml")
		err := os.WriteFile(configPath, []byte(invalidYAML), 0644)
		if err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		_, err = LoadConfig(configPath)
		if err == nil {
			t.Error("LoadConfig() should fail for invalid YAML")
		}
	})

	t.Run("load config with empty commands", func(t *testing.T) {
		emptyConfig := `
commands: []
security:
  shared_volume_path: /data
  forbidden_paths:
    - /etc
  max_command_length: 1024
`

		configPath := filepath.Join(tempDir, "empty_commands.yaml")
		err := os.WriteFile(configPath, []byte(emptyConfig), 0644)
		if err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		_, err = LoadConfig(configPath)
		if err == nil {
			t.Error("LoadConfig() should fail for empty commands array")
		}
	})

	t.Run("load config with missing required fields", func(t *testing.T) {
		missingFields := `
commands:
  - name: test
    # binary_path is missing
    allowed_args_patterns:
      - ".*"
    timeout: 10s
    max_concurrent: 1
security:
  shared_volume_path: /data
  forbidden_paths:
    - /etc
  max_command_length: 1024
`

		configPath := filepath.Join(tempDir, "missing_fields.yaml")
		err := os.WriteFile(configPath, []byte(missingFields), 0644)
		if err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		_, err = LoadConfig(configPath)
		if err == nil {
			t.Error("LoadConfig() should fail when binary_path is missing")
		}
	})

	t.Run("load config with invalid timeout format", func(t *testing.T) {
		invalidTimeout := `
commands:
  - name: test
    binary_path: /usr/bin/test
    allowed_args_patterns:
      - ".*"
    timeout: invalid_duration
    max_concurrent: 1
security:
  shared_volume_path: /data
  forbidden_paths:
    - /etc
  max_command_length: 1024
`

		configPath := filepath.Join(tempDir, "invalid_timeout.yaml")
		err := os.WriteFile(configPath, []byte(invalidTimeout), 0644)
		if err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		_, err = LoadConfig(configPath)
		if err == nil {
			t.Error("LoadConfig() should fail for invalid timeout format")
		}
	})

	t.Run("load config with zero max_concurrent", func(t *testing.T) {
		zeroMaxConcurrent := `
commands:
  - name: test
    binary_path: /usr/bin/test
    allowed_args_patterns:
      - ".*"
    timeout: 10s
    max_concurrent: 0
security:
  shared_volume_path: /data
  forbidden_paths:
    - /etc
  max_command_length: 1024
`

		configPath := filepath.Join(tempDir, "zero_max_concurrent.yaml")
		err := os.WriteFile(configPath, []byte(zeroMaxConcurrent), 0644)
		if err != nil {
			t.Fatalf("Failed to write config file: %v", err)
		}

		_, err = LoadConfig(configPath)
		if err == nil {
			t.Error("LoadConfig() should fail when max_concurrent is 0")
		}
	})
}

// TestGetCommandConfig tests retrieving command configurations.
func TestGetCommandConfig(t *testing.T) {
	config := &Config{
		Commands: []CommandConfig{
			{
				Name:       "ffmpeg",
				BinaryPath: "/usr/bin/ffmpeg",
			},
			{
				Name:       "python",
				BinaryPath: "/usr/bin/python3",
			},
		},
	}

	tests := []struct {
		name        string
		commandName string
		wantErr     bool
		wantPath    string
	}{
		{
			name:        "existing command ffmpeg",
			commandName: "ffmpeg",
			wantErr:     false,
			wantPath:    "/usr/bin/ffmpeg",
		},
		{
			name:        "existing command python",
			commandName: "python",
			wantErr:     false,
			wantPath:    "/usr/bin/python3",
		},
		{
			name:        "non-existent command",
			commandName: "nonexistent",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmdConfig, err := config.GetCommandConfig(tt.commandName)

			if tt.wantErr {
				if err == nil {
					t.Error("GetCommandConfig() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("GetCommandConfig() unexpected error: %v", err)
				return
			}

			if cmdConfig.BinaryPath != tt.wantPath {
				t.Errorf("BinaryPath = %s, want %s", cmdConfig.BinaryPath, tt.wantPath)
			}
		})
	}
}

// TestValidateConfig tests configuration validation logic.
func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: &Config{
				Commands: []CommandConfig{
					{
						Name:                "test",
						BinaryPath:          "/usr/bin/test",
						AllowedArgsPatterns: []string{".*"},
						Timeout:             "10s",
						MaxConcurrent:       1,
					},
				},
				Security: SecurityConfig{
					SharedVolumePath: "/data",
					ForbiddenPaths:   []string{"/etc"},
					MaxCommandLength: 1024,
				},
			},
			wantErr: false,
		},
		{
			name: "empty commands array",
			config: &Config{
				Commands: []CommandConfig{},
				Security: SecurityConfig{
					SharedVolumePath: "/data",
					ForbiddenPaths:   []string{"/etc"},
					MaxCommandLength: 1024,
				},
			},
			wantErr: true,
			errMsg:  "commands array cannot be empty",
		},
		{
			name: "command with empty name",
			config: &Config{
				Commands: []CommandConfig{
					{
						Name:                "",
						BinaryPath:          "/usr/bin/test",
						AllowedArgsPatterns: []string{".*"},
						Timeout:             "10s",
						MaxConcurrent:       1,
					},
				},
				Security: SecurityConfig{
					SharedVolumePath: "/data",
					ForbiddenPaths:   []string{"/etc"},
					MaxCommandLength: 1024,
				},
			},
			wantErr: true,
			errMsg:  "name cannot be empty",
		},
		{
			name: "command with empty binary path",
			config: &Config{
				Commands: []CommandConfig{
					{
						Name:                "test",
						BinaryPath:          "",
						AllowedArgsPatterns: []string{".*"},
						Timeout:             "10s",
						MaxConcurrent:       1,
					},
				},
				Security: SecurityConfig{
					SharedVolumePath: "/data",
					ForbiddenPaths:   []string{"/etc"},
					MaxCommandLength: 1024,
				},
			},
			wantErr: true,
			errMsg:  "binary_path cannot be empty",
		},
		{
			name: "security with empty shared volume path",
			config: &Config{
				Commands: []CommandConfig{
					{
						Name:                "test",
						BinaryPath:          "/usr/bin/test",
						AllowedArgsPatterns: []string{".*"},
						Timeout:             "10s",
						MaxConcurrent:       1,
					},
				},
				Security: SecurityConfig{
					SharedVolumePath: "",
					ForbiddenPaths:   []string{"/etc"},
					MaxCommandLength: 1024,
				},
			},
			wantErr: true,
			errMsg:  "shared_volume_path cannot be empty",
		},
		{
			name: "security with empty forbidden paths",
			config: &Config{
				Commands: []CommandConfig{
					{
						Name:                "test",
						BinaryPath:          "/usr/bin/test",
						AllowedArgsPatterns: []string{".*"},
						Timeout:             "10s",
						MaxConcurrent:       1,
					},
				},
				Security: SecurityConfig{
					SharedVolumePath: "/data",
					ForbiddenPaths:   []string{},
					MaxCommandLength: 1024,
				},
			},
			wantErr: true,
			errMsg:  "forbidden_paths cannot be empty",
		},
		{
			name: "security with zero max command length",
			config: &Config{
				Commands: []CommandConfig{
					{
						Name:                "test",
						BinaryPath:          "/usr/bin/test",
						AllowedArgsPatterns: []string{".*"},
						Timeout:             "10s",
						MaxConcurrent:       1,
					},
				},
				Security: SecurityConfig{
					SharedVolumePath: "/data",
					ForbiddenPaths:   []string{"/etc"},
					MaxCommandLength: 0,
				},
			},
			wantErr: true,
			errMsg:  "max_command_length must be greater than 0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)

			if tt.wantErr {
				if err == nil {
					t.Errorf("validateConfig() expected error containing '%s', got nil", tt.errMsg)
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("validateConfig() error = %v, want error containing '%s'", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateConfig() unexpected error = %v", err)
				}
			}
		})
	}
}
