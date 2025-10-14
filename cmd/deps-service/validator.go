package main

import (
	"fmt"
	"regexp"
	"strings"
)

// Validator performs security validation on command execution requests.
type Validator struct {
	config *Config
}

// NewValidator creates a new Validator instance with the provided configuration.
func NewValidator(config *Config) *Validator {
	return &Validator{config: config}
}

// ValidateRequest performs comprehensive security validation on a command request.
// It checks command whitelist, argument patterns, path security, and environment variables.
// Returns an error if any validation check fails.
func (v *Validator) ValidateRequest(req CommandRequest) error {
	// Layer 1: Check command whitelist
	cmdConfig, err := v.config.GetCommandConfig(req.Command)
	if err != nil {
		return fmt.Errorf("command %s is not in whitelist", req.Command)
	}

	// Layer 2: Check command length limit
	cmdLength := len(req.Command) + len(strings.Join(req.Args, " "))
	if cmdLength > v.config.Security.MaxCommandLength {
		return fmt.Errorf("command length (%d) exceeds maximum allowed (%d)", cmdLength, v.config.Security.MaxCommandLength)
	}

	// Layer 3: Validate arguments against allowed patterns
	if err := v.validateArgs(req.Args, cmdConfig.AllowedArgsPatterns); err != nil {
		return err
	}

	// Layer 4: Validate path security in arguments
	if err := v.validatePaths(req.Args); err != nil {
		return err
	}

	// Layer 5: Validate working directory if specified
	if req.WorkingDir != "" {
		if err := v.validatePath(req.WorkingDir); err != nil {
			return fmt.Errorf("invalid working directory: %w", err)
		}
	}

	// Layer 6: Validate environment variables if specified
	if len(req.Env) > 0 && len(cmdConfig.EnvWhitelist) > 0 {
		if err := v.validateEnv(req.Env, cmdConfig.EnvWhitelist); err != nil {
			return err
		}
	}

	return nil
}

// validateArgs checks if all arguments match at least one allowed pattern.
func (v *Validator) validateArgs(args []string, patterns []string) error {
	for _, arg := range args {
		matched := false
		for _, pattern := range patterns {
			if match, _ := regexp.MatchString(pattern, arg); match {
				matched = true
				break
			}
		}
		if !matched {
			return fmt.Errorf("argument '%s' does not match any allowed pattern", arg)
		}
	}
	return nil
}

// validatePaths checks all path-like arguments for security issues.
func (v *Validator) validatePaths(args []string) error {
	for _, arg := range args {
		// Check if argument looks like a path (starts with / or contains ..)
		if strings.HasPrefix(arg, "/") || strings.Contains(arg, "..") {
			if err := v.validatePath(arg); err != nil {
				return err
			}
		}
	}
	return nil
}

// validatePath performs security checks on a single path.
// It prohibits path traversal and access to forbidden directories.
func (v *Validator) validatePath(path string) error {
	// Check 1: Prohibit path traversal (..)
	if strings.Contains(path, "..") {
		return fmt.Errorf("path contains dangerous characters '..' (path traversal attempt): %s", path)
	}

	// Check 2: Prohibit access to forbidden directories
	for _, forbiddenPath := range v.config.Security.ForbiddenPaths {
		if strings.HasPrefix(path, forbiddenPath) {
			return fmt.Errorf("path attempts to access forbidden directory %s: %s", forbiddenPath, path)
		}
	}

	// Check 3: Allow specific whitelisted paths (scripts, binaries)
	allowedPrefixes := []string{
		v.config.Security.SharedVolumePath, // /data (shared volume)
		"/app/scripts/",                    // Application scripts
		"/usr/bin/",                        // System binaries
		"/usr/local/bin/",                  // Local binaries
	}

	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return nil // Path is allowed
		}
	}

	return fmt.Errorf("path must be within allowed directories (e.g., %s, /app/scripts): %s", v.config.Security.SharedVolumePath, path)
}

// validateEnv checks if all environment variables are in the whitelist.
func (v *Validator) validateEnv(env map[string]string, whitelist []string) error {
	for key := range env {
		allowed := false
		for _, whitelistedKey := range whitelist {
			if key == whitelistedKey {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("environment variable '%s' is not in whitelist", key)
		}
	}
	return nil
}
