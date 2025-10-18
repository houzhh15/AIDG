package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// Executor executes commands in a controlled and secure manner.
type Executor struct {
	config *Config
}

// NewExecutor creates a new Executor instance with the provided configuration.
func NewExecutor(config *Config) *Executor {
	return &Executor{config: config}
}

// ExecuteCommand executes a command with the specified parameters and returns the result.
// It implements timeout control, captures stdout/stderr, and tracks execution duration.
func (e *Executor) ExecuteCommand(ctx context.Context, req CommandRequest) (CommandResponse, error) {
	// Get command configuration
	cmdConfig, err := e.config.GetCommandConfig(req.Command)
	if err != nil {
		return CommandResponse{}, fmt.Errorf("failed to get command config: %w", err)
	}

	// Determine timeout
	timeout := req.Timeout
	if timeout == 0 {
		parsedTimeout, err := time.ParseDuration(cmdConfig.Timeout)
		if err != nil {
			return CommandResponse{}, fmt.Errorf("invalid timeout format in config: %w", err)
		}
		timeout = parsedTimeout
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build command
	cmd := exec.CommandContext(ctx, cmdConfig.BinaryPath, req.Args...)

	// Log the command being executed
	fmt.Printf("[EXECUTOR] Executing command: %s %v (timeout=%v)\n", cmdConfig.BinaryPath, req.Args, timeout)

	// Set working directory if specified
	if req.WorkingDir != "" {
		cmd.Dir = req.WorkingDir
		fmt.Printf("[EXECUTOR] Working directory: %s\n", req.WorkingDir)
	}

	// Set environment variables if specified
	if len(req.Env) > 0 {
		// Inherit system environment variables
		cmd.Env = os.Environ()
		// Add custom environment variables
		for key, value := range req.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
		}
	}

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Record start time and execute command
	start := time.Now()
	err = cmd.Run()
	duration := time.Since(start)

	// Build response
	resp := CommandResponse{
		Success:     err == nil,
		ExitCode:    0,
		Stdout:      stdout.String(),
		Stderr:      stderr.String(),
		DurationMs:  duration.Milliseconds(),
		OutputFiles: []string{}, // Not auto-detected, caller should infer
	}

	// Get exit code if command ran
	if cmd.ProcessState != nil {
		resp.ExitCode = cmd.ProcessState.ExitCode()
	}

	// Log the result
	if err != nil {
		fmt.Printf("[EXECUTOR] Command failed: exit_code=%d, error=%v\n", resp.ExitCode, err)
		if stdout.Len() > 0 {
			fmt.Printf("[EXECUTOR] Stdout: %s\n", stdout.String())
		}
		if stderr.Len() > 0 {
			fmt.Printf("[EXECUTOR] Stderr: %s\n", stderr.String())
		}
	} else {
		fmt.Printf("[EXECUTOR] Command succeeded: exit_code=%d, duration=%dms\n", resp.ExitCode, resp.DurationMs)
		if stdout.Len() > 0 {
			fmt.Printf("[EXECUTOR] Stdout: %s\n", stdout.String())
		}
	}

	// Check for timeout error
	if ctx.Err() == context.DeadlineExceeded {
		return resp, fmt.Errorf("command timeout after %v", timeout)
	}

	return resp, err
}
