package dependency

import "context"

// DependencyExecutor defines the interface for executing external commands
// (FFmpeg, PyAnnote, etc.) in different modes (local, remote, fallback).
//
// Implementations:
//   - LocalExecutor: Executes commands directly using exec.Command
//   - RemoteExecutor: Executes commands via HTTP API calls
//   - FallbackExecutor: Tries remote first, falls back to local on failure
type DependencyExecutor interface {
	// ExecuteCommand executes a command with the given request.
	// It returns the command output and any execution error.
	//
	// The context can be used to cancel or set a deadline for the command execution.
	// If the context is cancelled, the command should be terminated promptly.
	ExecuteCommand(ctx context.Context, req CommandRequest) (CommandResponse, error)

	// HealthCheck verifies that the executor is ready to handle requests.
	// Returns nil if healthy, otherwise an error describing the issue.
	//
	// For LocalExecutor, this checks if required binaries are available in PATH.
	// For RemoteExecutor, this checks if the remote service is reachable.
	// For FallbackExecutor, this checks both remote and local availability.
	HealthCheck(ctx context.Context) error
}
