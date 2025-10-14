// Package health provides health checking functionality for Whisper transcription services.
// It implements periodic health probes with configurable intervals and failure thresholds.
package health

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/houzhh15-hub/AIDG/cmd/server/internal/orchestrator/whisper"
)

// ServiceStatus represents the current health state of a Whisper transcription service.
// All fields are safe for JSON serialization and can be exposed via API endpoints.
type ServiceStatus struct {
	// IsHealthy indicates whether the service passed recent health checks
	IsHealthy bool `json:"is_healthy"`

	// LastCheckTime records when the most recent health check was performed
	LastCheckTime time.Time `json:"last_check_time"`

	// ConsecutiveFails counts how many health checks have failed in a row
	// Reset to 0 when a check succeeds
	ConsecutiveFails int `json:"consecutive_fails"`

	// ErrorMessage contains the last error message if health check failed
	// Empty string if healthy
	ErrorMessage string `json:"error_message"`
}

// HealthChecker performs periodic health checks on a WhisperTranscriber implementation.
// It monitors service health and tracks consecutive failures to trigger degradation.
//
// Thread-safety: All public methods are thread-safe via sync.RWMutex.
type HealthChecker struct {
	transcriber   whisper.WhisperTranscriber // The transcriber instance to monitor
	status        *ServiceStatus             // Current health status (protected by mu)
	mu            sync.RWMutex               // Protects status reads/writes
	checkInterval time.Duration              // Interval between health checks (e.g., 5 minutes)
	failThreshold int                        // Number of consecutive failures before marking unhealthy
	stopChan      chan struct{}              // Signal channel to stop the health check loop
}

// NewHealthChecker creates a new HealthChecker with the specified configuration.
//
// Parameters:
//   - transcriber: The WhisperTranscriber implementation to monitor
//   - checkInterval: Duration between health checks (e.g., 5*time.Minute)
//   - failThreshold: Number of consecutive failures before marking unhealthy (e.g., 3)
//
// Returns:
//   - *HealthChecker: Configured instance ready to start monitoring
//
// The health checker starts in a healthy state (optimistic assumption).
// Call Start() to begin periodic health checks.
func NewHealthChecker(transcriber whisper.WhisperTranscriber, checkInterval time.Duration, failThreshold int) *HealthChecker {
	return &HealthChecker{
		transcriber:   transcriber,
		checkInterval: checkInterval,
		failThreshold: failThreshold,
		stopChan:      make(chan struct{}),
		status: &ServiceStatus{
			IsHealthy:        true, // Start optimistic
			LastCheckTime:    time.Now(),
			ConsecutiveFails: 0,
			ErrorMessage:     "",
		},
	}
}

// Start begins periodic health checking in a background goroutine.
// It performs an immediate check, then checks at regular intervals.
//
// The goroutine stops when:
//   - Stop() is called (stopChan closed)
//   - Context is cancelled
//
// This method does not block. Call Stop() to gracefully terminate.
func (hc *HealthChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(hc.checkInterval)
	defer ticker.Stop()

	// Perform immediate check on startup
	hc.performCheck(ctx)

	for {
		select {
		case <-ticker.C:
			hc.performCheck(ctx)
		case <-hc.stopChan:
			log.Printf("[INFO] HealthChecker: Stopped for %s", hc.transcriber.Name())
			return
		case <-ctx.Done():
			log.Printf("[INFO] HealthChecker: Context cancelled for %s", hc.transcriber.Name())
			return
		}
	}
}

// performCheck executes a single health check and updates the status.
// It manages the consecutive failure counter and logging.
func (hc *HealthChecker) performCheck(ctx context.Context) {
	// Create timeout context for health check (10 seconds)
	checkCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	isHealthy, err := hc.transcriber.HealthCheck(checkCtx)

	hc.mu.Lock()
	defer hc.mu.Unlock()

	hc.status.LastCheckTime = time.Now()

	if isHealthy {
		// Health check passed - reset failure counter
		hc.status.IsHealthy = true
		hc.status.ConsecutiveFails = 0
		hc.status.ErrorMessage = ""
		log.Printf("[INFO] HealthChecker: Health check passed for %s", hc.transcriber.Name())
	} else {
		// Health check failed - increment counter
		hc.status.ConsecutiveFails++
		errMsg := "unknown error"
		if err != nil {
			errMsg = err.Error()
		}
		hc.status.ErrorMessage = fmt.Sprintf("Health check failed: %s", errMsg)

		// Check if threshold reached
		if hc.status.ConsecutiveFails >= hc.failThreshold {
			hc.status.IsHealthy = false
			log.Printf("[ERROR] HealthChecker: Health check failed %d times for %s, marking as unhealthy",
				hc.status.ConsecutiveFails, hc.transcriber.Name())
		} else {
			log.Printf("[WARN] HealthChecker: Health check failed (%d/%d) for %s: %s",
				hc.status.ConsecutiveFails, hc.failThreshold, hc.transcriber.Name(), errMsg)
		}
	}
}

// GetStatus returns a copy of the current health status.
// Thread-safe for concurrent access.
//
// Returns:
//   - ServiceStatus: Copy of the current status (not a pointer to avoid external mutation)
func (hc *HealthChecker) GetStatus() ServiceStatus {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	return *hc.status // Return copy, not pointer
}

// Stop gracefully terminates the health checking goroutine.
// It is safe to call Stop multiple times (subsequent calls are no-ops).
//
// After calling Stop, the HealthChecker should not be reused.
func (hc *HealthChecker) Stop() {
	select {
	case <-hc.stopChan:
		// Already closed, do nothing
	default:
		close(hc.stopChan)
	}
}
