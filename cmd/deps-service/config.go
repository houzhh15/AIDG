package main

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// LoadConfig loads configuration from a YAML file and validates it.
// Returns an error if the file cannot be read, parsed, or validation fails.
func LoadConfig(path string) (*Config, error) {
	// Read the configuration file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML content
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// GetCommandConfig returns the configuration for a specific command by name.
// Returns an error if the command is not found in the whitelist.
func (c *Config) GetCommandConfig(name string) (*CommandConfig, error) {
	for i := range c.Commands {
		if c.Commands[i].Name == name {
			return &c.Commands[i], nil
		}
	}
	return nil, fmt.Errorf("command %s not found in whitelist", name)
}

// validateConfig performs comprehensive validation of the configuration.
// Returns an error if any required field is missing or invalid.
func validateConfig(config *Config) error {
	// Validate Commands array
	if len(config.Commands) == 0 {
		return fmt.Errorf("commands array cannot be empty")
	}

	for i, cmd := range config.Commands {
		// Validate Name
		if cmd.Name == "" {
			return fmt.Errorf("command[%d]: name cannot be empty", i)
		}

		// Validate BinaryPath
		if cmd.BinaryPath == "" {
			return fmt.Errorf("command[%d] (%s): binary_path cannot be empty", i, cmd.Name)
		}

		// Validate AllowedArgsPatterns
		if len(cmd.AllowedArgsPatterns) == 0 {
			return fmt.Errorf("command[%d] (%s): allowed_args_patterns cannot be empty", i, cmd.Name)
		}

		// Validate Timeout format
		if cmd.Timeout == "" {
			return fmt.Errorf("command[%d] (%s): timeout cannot be empty", i, cmd.Name)
		}
		if _, err := time.ParseDuration(cmd.Timeout); err != nil {
			return fmt.Errorf("command[%d] (%s): invalid timeout format: %w", i, cmd.Name, err)
		}

		// Validate MaxConcurrent
		if cmd.MaxConcurrent <= 0 {
			return fmt.Errorf("command[%d] (%s): max_concurrent must be greater than 0", i, cmd.Name)
		}
	}

	// Validate Security configuration
	if config.Security.SharedVolumePath == "" {
		return fmt.Errorf("security.shared_volume_path cannot be empty")
	}

	if len(config.Security.ForbiddenPaths) == 0 {
		return fmt.Errorf("security.forbidden_paths cannot be empty")
	}

	if config.Security.MaxCommandLength <= 0 {
		return fmt.Errorf("security.max_command_length must be greater than 0")
	}

	return nil
}
