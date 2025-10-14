package dependency

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/houzhh15-hub/AIDG/pkg/metrics"
)

// FallbackExecutor tries remote execution first, then falls back to local on failure.
// It implements intelligent degradation with automatic mode switching.
type FallbackExecutor struct {
	config         ExecutorConfig
	remoteExecutor *RemoteExecutor
	localExecutor  *LocalExecutor
	primaryMode    ExecutionMode // Current active mode ("remote" or "local")
	mu             sync.RWMutex  // Protects primaryMode
}

// NewFallbackExecutor creates a new FallbackExecutor with remote as the initial primary mode.
func NewFallbackExecutor(config ExecutorConfig) *FallbackExecutor {
	return &FallbackExecutor{
		config:         config,
		remoteExecutor: NewRemoteExecutor(config),
		localExecutor:  NewLocalExecutor(config),
		primaryMode:    ModeRemote, // Start with remote by default
	}
}

// ExecuteCommand executes a command using the current primary mode, with automatic fallback.
func (e *FallbackExecutor) ExecuteCommand(ctx context.Context, req CommandRequest) (CommandResponse, error) {
	// Record start time for metrics
	start := time.Now()

	e.mu.RLock()
	mode := e.primaryMode
	e.mu.RUnlock()

	var resp CommandResponse
	var err error

	// 1. Try primary mode
	if mode == ModeRemote {
		resp, err = e.remoteExecutor.ExecuteCommand(ctx, req)
		if err != nil && e.isNetworkError(err) {
			// Network error detected, attempt fallback to local
			slog.Warn("remote execution failed, attempting local fallback",
				"command", req.Command,
				"error", err.Error())

			// Record failed remote execution before fallback
			metrics.RecordCommandExecution(req.Command, string(ModeRemote), "failed")
			metrics.RecordCommandDuration(req.Command, string(ModeRemote), time.Since(start).Seconds())

			// Attempt fallback (will record its own metrics)
			return e.fallbackToLocal(ctx, req)
		}
	} else {
		// Already in local mode, execute directly
		resp, err = e.localExecutor.ExecuteCommand(ctx, req)
	}

	// Record metrics for successful or non-network-error cases
	status := determineExecutionStatus(resp, err)
	metrics.RecordCommandExecution(req.Command, string(mode), status)
	metrics.RecordCommandDuration(req.Command, string(mode), time.Since(start).Seconds())

	return resp, err
}

// determineExecutionStatus categorizes execution result as "success", "timeout", or "failed".
func determineExecutionStatus(resp CommandResponse, err error) string {
	if err == nil && resp.Success {
		return "success"
	}
	if err != nil && strings.Contains(err.Error(), "timeout") {
		return "timeout"
	}
	return "failed"
}

// HealthCheck probes both remote and local executors to determine availability.
// Prioritizes remote service, falls back to local if remote is unavailable.
func (e *FallbackExecutor) HealthCheck(ctx context.Context) error {
	slog.Info("performing fallback executor health check...")

	// Try remote executor first
	if err := e.remoteExecutor.HealthCheck(ctx); err == nil {
		e.setPrimaryMode(ModeRemote)
		slog.Info("dependency service health check passed, using remote mode")
		return nil
	} else {
		slog.Warn("remote dependency service unavailable, trying local fallback",
			"error", err.Error())
	}

	// Fallback to local executor
	if err := e.localExecutor.HealthCheck(ctx); err == nil {
		e.setPrimaryMode(ModeLocal)
		slog.Info("local dependencies available, using local mode (degraded)")
		return nil
	} else {
		// Both failed - return comprehensive error with tried modes
		return fmt.Errorf("both remote and local dependencies unavailable: remote service unreachable, local tools not installed (check dependency configuration) [tried modes: remote → local]")
	}
}

// fallbackToLocal attempts to execute the command locally after remote failure.
func (e *FallbackExecutor) fallbackToLocal(ctx context.Context, req CommandRequest) (CommandResponse, error) {
	// Record start time for local execution metrics
	start := time.Now()

	resp, err := e.localExecutor.ExecuteCommand(ctx, req)

	// Record local execution metrics
	status := determineExecutionStatus(resp, err)
	metrics.RecordCommandExecution(req.Command, string(ModeLocal), status)
	metrics.RecordCommandDuration(req.Command, string(ModeLocal), time.Since(start).Seconds())

	if err == nil && resp.Success {
		// Fallback succeeded, update primary mode to local
		e.setPrimaryMode(ModeLocal)
		slog.Info("local fallback succeeded, updated primary mode to local",
			"command", req.Command)

		// Record degradation event (remote → local)
		metrics.RecordDegradationEvent(string(ModeRemote), string(ModeLocal))
	}

	return resp, err
}

// isNetworkError checks if an error is network-related (connection refused, timeout, etc.).
func (e *FallbackExecutor) isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	errMsg := err.Error()
	return strings.Contains(errMsg, "connection refused") ||
		strings.Contains(errMsg, "connection reset") ||
		strings.Contains(errMsg, "no such host") ||
		strings.Contains(errMsg, "timeout") ||
		strings.Contains(errMsg, "network error")
}

// setPrimaryMode atomically updates the primary execution mode.
func (e *FallbackExecutor) setPrimaryMode(mode ExecutionMode) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.primaryMode = mode
}
