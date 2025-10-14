package dependency

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

// LocalExecutor executes commands directly on the local system using exec.Command.
// It is suitable for development environments or when tools are installed on the host.
type LocalExecutor struct {
	config ExecutorConfig
}

// NewLocalExecutor creates a new LocalExecutor with the given configuration.
func NewLocalExecutor(config ExecutorConfig) *LocalExecutor {
	return &LocalExecutor{config: config}
}

// ExecuteCommand executes a command locally and returns the result.
func (e *LocalExecutor) ExecuteCommand(ctx context.Context, req CommandRequest) (CommandResponse, error) {
	// 1. Resolve binary path (from config or PATH)
	binaryPath, err := e.resolveBinaryPath(req.Command)
	if err != nil {
		return CommandResponse{}, fmt.Errorf("failed to resolve binary path for %s: %w", req.Command, err)
	}

	// 2. Create timeout context
	timeout := req.Timeout
	if timeout == 0 {
		timeout = e.config.DefaultTimeout
	}
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// 3. Build command
	cmd := exec.CommandContext(ctx, binaryPath, req.Args...)

	// 4. Set environment variables
	cmd.Env = append(os.Environ(), e.buildEnvSlice(req.Env)...)

	// 5. Set working directory
	if req.WorkingDir != "" {
		cmd.Dir = req.WorkingDir
	}

	// 6. Set process group (for killing entire process tree on timeout)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	// 7. Execute command and capture output
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err = cmd.Run()
	duration := time.Since(start)

	// 8. Build response
	resp := CommandResponse{
		Success:  err == nil,
		ExitCode: e.getExitCode(err),
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	// 9. Handle timeout error
	if ctx.Err() == context.DeadlineExceeded {
		// Kill process group
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		return resp, fmt.Errorf("command execution timeout (%v): %s", timeout, req.Command)
	}

	return resp, err
}

// HealthCheck verifies that all configured local binaries are available.
func (e *LocalExecutor) HealthCheck(ctx context.Context) error {
	for cmd, path := range e.config.LocalBinaryPaths {
		if _, err := exec.LookPath(path); err != nil {
			return fmt.Errorf("local command %s not available at %s: %w", cmd, path, err)
		}
	}
	return nil
}

// resolveBinaryPath resolves the binary path from config or PATH environment.
func (e *LocalExecutor) resolveBinaryPath(command string) (string, error) {
	// Priority 1: Use configured path
	if path, ok := e.config.LocalBinaryPaths[command]; ok {
		return path, nil
	}
	// Priority 2: Fall back to PATH lookup
	return exec.LookPath(command)
}

// buildEnvSlice converts environment map to slice format.
func (e *LocalExecutor) buildEnvSlice(envMap map[string]string) []string {
	var result []string
	for k, v := range envMap {
		result = append(result, fmt.Sprintf("%s=%s", k, v))
	}
	return result
}

// getExitCode extracts exit code from error.
func (e *LocalExecutor) getExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1 // Unable to determine exit code
}
