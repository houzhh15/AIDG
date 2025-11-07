// Package degradation provides automatic service degradation and recovery for Whisper transcription.
// It monitors health status and switches between primary and fallback transcriber implementations.
package degradation

import (
	"log"
	"sync"

	"github.com/houzhh15/AIDG/cmd/server/internal/orchestrator/health"
	"github.com/houzhh15/AIDG/cmd/server/internal/orchestrator/whisper"
)

// DegradationController manages the lifecycle of transcriber implementations based on health status.
// It automatically switches between a primary transcriber (e.g., go-whisper, local-whisper) and
// a fallback transcriber (typically MockTranscriber) to ensure continuous operation.
//
// Priority strategy (highest to lowest):
//  1. LocalWhisperImpl (if /app/bin/whisper exists and is executable)
//  2. GoWhisperImpl (if go-whisper container is healthy)
//  3. FasterWhisperImpl (if faster-whisper container is healthy)
//  4. MockTranscriber (always available, returns empty results)
//
// Thread-safety: All public methods are thread-safe via sync.RWMutex.
type DegradationController struct {
	primaryTranscriber  whisper.WhisperTranscriber // Preferred transcriber (e.g., GoWhisperImpl)
	fallbackTranscriber whisper.WhisperTranscriber // Fallback transcriber (e.g., MockTranscriber)
	healthChecker       *health.HealthChecker      // Monitors primary transcriber health
	currentTranscriber  whisper.WhisperTranscriber // Currently active transcriber (protected by mu)
	mu                  sync.RWMutex               // Protects currentTranscriber and isDegraded
	isDegraded          bool                       // True if currently using fallback (protected by mu)
}

// NewDegradationController creates a new DegradationController with the specified transcribers.
//
// Parameters:
//   - primary: The preferred transcriber implementation (must not be nil)
//   - fallback: The fallback transcriber (typically MockTranscriber, must not be nil)
//   - hc: The health checker monitoring the primary transcriber (must not be nil)
//
// Returns:
//   - *DegradationController: Configured instance ready to manage transcriber lifecycle
//
// Initial state: Uses primary transcriber (optimistic assumption of health).
func NewDegradationController(
	primary whisper.WhisperTranscriber,
	fallback whisper.WhisperTranscriber,
	hc *health.HealthChecker,
) *DegradationController {
	return &DegradationController{
		primaryTranscriber:  primary,
		fallbackTranscriber: fallback,
		healthChecker:       hc,
		currentTranscriber:  primary, // Start with primary
		isDegraded:          false,
	}
}

// GetTranscriber returns the current active transcriber, automatically switching between
// primary and fallback based on health status.
//
// Behavior:
//   - Queries health checker for latest status
//   - If unhealthy and not degraded: switches to fallback, logs WARN
//   - If healthy and degraded: switches back to primary, logs INFO
//   - If status unchanged: returns current transcriber without logging
//
// Thread-safe: Uses RWMutex for read and exclusive lock for write.
//
// Returns:
//   - whisper.WhisperTranscriber: The currently active transcriber
func (dc *DegradationController) GetTranscriber() whisper.WhisperTranscriber {
	status := dc.healthChecker.GetStatus()

	dc.mu.Lock()
	defer dc.mu.Unlock()

	// Check if degradation is needed (unhealthy and not yet degraded)
	if !status.IsHealthy && !dc.isDegraded {
		log.Printf("[WARN] DegradationController: Degrading to fallback transcriber (%s) due to unhealthy primary (%s)",
			dc.fallbackTranscriber.Name(), dc.primaryTranscriber.Name())
		dc.currentTranscriber = dc.fallbackTranscriber
		dc.isDegraded = true
	}

	// Check if recovery is possible (healthy and currently degraded)
	if status.IsHealthy && dc.isDegraded {
		log.Printf("[INFO] DegradationController: Recovering to primary transcriber (%s)",
			dc.primaryTranscriber.Name())
		dc.currentTranscriber = dc.primaryTranscriber
		dc.isDegraded = false
	}

	return dc.currentTranscriber
}

// IsDegraded returns whether the system is currently operating in degraded mode.
// Thread-safe for concurrent access.
//
// Returns:
//   - bool: true if using fallback transcriber, false if using primary
func (dc *DegradationController) IsDegraded() bool {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	return dc.isDegraded
}
