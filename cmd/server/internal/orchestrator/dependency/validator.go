package dependency

import (
	"fmt"
	"strings"
)

// ValidateCommandRequest performs security checks before command execution.
// It validates:
//  1. Command whitelist (if configured)
//  2. Argument safety (no path traversal, no system directory access)
//  3. Working directory validation (must be within shared volume)
//
// This function should be called by DependencyClient high-level methods
// (e.g., ConvertAudio) after constructing CommandRequest but before passing
// it to the executor.
func ValidateCommandRequest(req CommandRequest, config ExecutorConfig) error {
	// 1. Check command whitelist
	if len(config.AllowedCommands) > 0 {
		allowed := false
		for _, cmd := range config.AllowedCommands {
			if req.Command == cmd {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("command %s is not in whitelist (allowed: %v)", req.Command, config.AllowedCommands)
		}
	}

	// 2. Check argument safety
	for _, arg := range req.Args {
		// 2.1 Prohibit path traversal
		if strings.Contains(arg, "..") {
			return fmt.Errorf("argument contains dangerous characters '..' (path traversal attempt): %s", arg)
		}

		// 2.2 Prohibit access to system directories
		dangerousPrefixes := []string{"/etc", "/sys", "/proc", "/dev"}
		for _, prefix := range dangerousPrefixes {
			if strings.HasPrefix(arg, prefix) {
				return fmt.Errorf("argument attempts to access forbidden system directory %s: %s", prefix, arg)
			}
		}
	}

	// 3. Check working directory (if specified)
	if req.WorkingDir != "" {
		pm := NewPathManager(config.SharedVolumePath)
		if err := pm.ValidatePath(req.WorkingDir); err != nil {
			return fmt.Errorf("invalid working directory: %w", err)
		}
	}

	return nil
}
