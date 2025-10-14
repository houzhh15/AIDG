package degradation

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/houzhh15-hub/AIDG/cmd/server/internal/orchestrator/health"
	"github.com/houzhh15-hub/AIDG/cmd/server/internal/orchestrator/whisper"
)

// MockTranscriberForDegradation is a thread-safe mock transcriber for degradation testing.
type MockTranscriberForDegradation struct {
	name    string
	healthy bool
	mu      sync.RWMutex
}

func (m *MockTranscriberForDegradation) Transcribe(ctx context.Context, audioPath string, options *whisper.TranscribeOptions) (*whisper.TranscriptionResult, error) {
	return &whisper.TranscriptionResult{
		Text:     "transcribed by " + m.name,
		Segments: []whisper.TranscriptionSegment{},
		Language: "en",
		Duration: 1.0,
	}, nil
}

func (m *MockTranscriberForDegradation) HealthCheck(ctx context.Context) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.healthy, nil
}

func (m *MockTranscriberForDegradation) Name() string {
	return m.name
}

func (m *MockTranscriberForDegradation) SetHealthy(healthy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthy = healthy
}

// TestDegradationController tests automatic degradation logic.
func TestDegradationController(t *testing.T) {
	t.Run("initial state uses primary transcriber", func(t *testing.T) {
		primary := &MockTranscriberForDegradation{name: "primary", healthy: true}
		fallback := &MockTranscriberForDegradation{name: "fallback", healthy: true}

		hc := health.NewHealthChecker(primary, 1*time.Hour, 3)
		controller := NewDegradationController(primary, fallback, hc)

		transcriber := controller.GetTranscriber()

		if transcriber.Name() != "primary" {
			t.Errorf("Initial transcriber = %q, want %q", transcriber.Name(), "primary")
		}

		if controller.IsDegraded() {
			t.Error("Initial state should not be degraded")
		}
	})

	t.Run("degrades to fallback when primary is unhealthy", func(t *testing.T) {
		primary := &MockTranscriberForDegradation{name: "primary", healthy: false}
		fallback := &MockTranscriberForDegradation{name: "fallback", healthy: true}

		hc := health.NewHealthChecker(primary, 10*time.Millisecond, 1)
		controller := NewDegradationController(primary, fallback, hc)

		ctx := context.Background()

		// Start health checker to trigger checks
		go hc.Start(ctx)
		defer hc.Stop()

		// Wait for health check to complete
		time.Sleep(100 * time.Millisecond)

		transcriber := controller.GetTranscriber()

		if transcriber.Name() != "fallback" {
			t.Errorf("After degradation: transcriber = %q, want %q", transcriber.Name(), "fallback")
		}

		if !controller.IsDegraded() {
			t.Error("Should be in degraded state")
		}
	})

	t.Run("recovers to primary when health is restored", func(t *testing.T) {
		primary := &MockTranscriberForDegradation{name: "primary", healthy: false}
		fallback := &MockTranscriberForDegradation{name: "fallback", healthy: true}

		hc := health.NewHealthChecker(primary, 10*time.Millisecond, 1)
		controller := NewDegradationController(primary, fallback, hc)

		ctx := context.Background()

		// Start health checker
		go hc.Start(ctx)
		defer hc.Stop()

		// Wait for degradation
		time.Sleep(100 * time.Millisecond)

		// Verify degraded
		if controller.GetTranscriber().Name() != "fallback" {
			t.Error("Should be degraded to fallback")
		}

		// Recover primary
		primary.SetHealthy(true)
		time.Sleep(100 * time.Millisecond)

		// Get transcriber should now return primary
		transcriber := controller.GetTranscriber()

		if transcriber.Name() != "primary" {
			t.Errorf("After recovery: transcriber = %q, want %q", transcriber.Name(), "primary")
		}

		if controller.IsDegraded() {
			t.Error("Should not be degraded after recovery")
		}
	})

	t.Run("transcribe uses correct implementation", func(t *testing.T) {
		primary := &MockTranscriberForDegradation{name: "primary-impl", healthy: true}
		fallback := &MockTranscriberForDegradation{name: "fallback-impl", healthy: true}

		hc := health.NewHealthChecker(primary, 1*time.Hour, 3)
		controller := NewDegradationController(primary, fallback, hc)

		ctx := context.Background()

		// Transcribe using primary
		result1, err := controller.GetTranscriber().Transcribe(ctx, "/test/audio.wav", nil)
		if err != nil {
			t.Fatalf("Transcribe error: %v", err)
		}

		if result1.Text != "transcribed by primary-impl" {
			t.Errorf("Primary transcription text = %q", result1.Text)
		}
	})

	t.Run("multiple degradations and recoveries", func(t *testing.T) {
		primary := &MockTranscriberForDegradation{name: "primary", healthy: true}
		fallback := &MockTranscriberForDegradation{name: "fallback", healthy: true}

		hc := health.NewHealthChecker(primary, 10*time.Millisecond, 1)
		controller := NewDegradationController(primary, fallback, hc)

		ctx := context.Background()

		// Start health checker
		go hc.Start(ctx)
		defer hc.Stop()

		// Cycle through degradation and recovery
		for cycle := 0; cycle < 2; cycle++ {
			// Degrade
			primary.SetHealthy(false)
			time.Sleep(50 * time.Millisecond)

			if controller.GetTranscriber().Name() != "fallback" {
				t.Errorf("Cycle %d: Should be degraded", cycle)
			}

			// Recover
			primary.SetHealthy(true)
			time.Sleep(50 * time.Millisecond)

			if controller.GetTranscriber().Name() != "primary" {
				t.Errorf("Cycle %d: Should be recovered", cycle)
			}
		}
	})
}

// TestDegradationControllerConstructor tests controller creation.
func TestDegradationControllerConstructor(t *testing.T) {
	primary := &MockTranscriberForDegradation{name: "primary", healthy: true}
	fallback := &MockTranscriberForDegradation{name: "fallback", healthy: true}
	hc := health.NewHealthChecker(primary, 1*time.Hour, 3)

	controller := NewDegradationController(primary, fallback, hc)

	if controller == nil {
		t.Fatal("NewDegradationController returned nil")
	}

	transcriber := controller.GetTranscriber()
	if transcriber == nil {
		t.Error("GetTranscriber returned nil")
	}

	if transcriber.Name() != "primary" {
		t.Errorf("Initial transcriber = %q, want %q", transcriber.Name(), "primary")
	}
}
