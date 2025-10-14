// Package dependency provides an abstraction layer for executing external commands
// (FFmpeg, PyAnnote, etc.) in different execution modes (local, remote, fallback).
package dependency

import "time"

// ExecutionMode specifies how commands should be executed.
type ExecutionMode string

const (
	// ModeLocal executes commands directly on the local system using exec.Command.
	ModeLocal ExecutionMode = "local"

	// ModeRemote executes commands by calling a remote dependency service via HTTP.
	ModeRemote ExecutionMode = "remote"

	// ModeFallback tries remote execution first, then falls back to local on failure.
	ModeFallback ExecutionMode = "fallback"
)

// CommandRequest encapsulates all information needed to execute a command.
type CommandRequest struct {
	// Command is the binary name or alias (e.g., "ffmpeg", "pyannote").
	Command string `json:"command" yaml:"command"`

	// Args are the command-line arguments (e.g., ["-i", "input.wav", "output.mp3"]).
	Args []string `json:"args" yaml:"args"`

	// Env contains environment variables to set (e.g., {"LOG_LEVEL": "debug"}).
	Env map[string]string `json:"env,omitempty" yaml:"env,omitempty"`

	// WorkingDir is the directory to execute the command in (default: current dir).
	WorkingDir string `json:"working_dir,omitempty" yaml:"working_dir,omitempty"`

	// Timeout is the maximum execution duration (0 means no timeout).
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

// CommandResponse contains the result of a command execution.
type CommandResponse struct {
	// Success indicates if the command completed without errors.
	Success bool `json:"success" yaml:"success"`

	// ExitCode is the process exit code (0 typically means success).
	ExitCode int `json:"exit_code" yaml:"exit_code"`

	// Stdout contains the standard output of the command.
	Stdout string `json:"stdout" yaml:"stdout"`

	// Stderr contains the standard error output (useful for debugging).
	Stderr string `json:"stderr" yaml:"stderr"`

	// Duration is the actual execution time.
	Duration time.Duration `json:"duration_ms" yaml:"duration_ms"`

	// OutputFiles lists the paths of generated files (relative to shared volume).
	OutputFiles []string `json:"output_files,omitempty" yaml:"output_files,omitempty"`
}

// ExecutorConfig defines the configuration for dependency execution.
type ExecutorConfig struct {
	// Mode specifies the execution strategy: "local", "remote", or "fallback".
	Mode ExecutionMode `json:"mode" yaml:"mode"`

	// ServiceURL is the HTTP endpoint of the remote dependency service
	// (e.g., "http://deps-service:8080"). Required for "remote" and "fallback" modes.
	ServiceURL string `json:"service_url" yaml:"service_url"`

	// SharedVolumePath is the base path of the shared volume
	// (e.g., "/data"). All file operations must be within this directory.
	SharedVolumePath string `json:"shared_volume_path" yaml:"shared_volume_path"`

	// LocalBinaryPaths maps command names to local binary paths
	// (e.g., {"ffmpeg": "/usr/local/bin/ffmpeg"}). Used in "local" and "fallback" modes.
	LocalBinaryPaths map[string]string `json:"local_binary_paths" yaml:"local_binary_paths"`

	// DefaultTimeout is the default execution timeout for all commands.
	DefaultTimeout time.Duration `json:"default_timeout" yaml:"default_timeout"`

	// AllowedCommands lists the commands that are permitted to execute
	// (security: whitelist approach). Empty list means allow all.
	AllowedCommands []string `json:"allowed_commands" yaml:"allowed_commands"`
}
