package health

import (
	"context"
	"testing"
	"time"

	"github.com/houzhh15-hub/AIDG/cmd/server/internal/orchestrator/whisper"
)

// MockTranscriberForHealth is a simple mock transcriber for health testing.
type MockTranscriberForHealth struct {
	healthy bool
}

func (m *MockTranscriberForHealth) Transcribe(ctx context.Context, audioPath string, options *whisper.TranscribeOptions) (*whisper.TranscriptionResult, error) {
	return &whisper.TranscriptionResult{}, nil
}

func (m *MockTranscriberForHealth) HealthCheck(ctx context.Context) (bool, error) {
	return m.healthy, nil
}

func (m *MockTranscriberForHealth) Name() string {
	return "mock-health-test"
}

// TestHealthChecker tests the health checking functionality.
func TestHealthChecker(t *testing.T) {
	t.Run("initial state is healthy", func(t *testing.T) {
		mock := &MockTranscriberForHealth{healthy: true}
		checker := NewHealthChecker(mock, 1*time.Second, 3)

		status := checker.GetStatus()

		if !status.IsHealthy {
			t.Error("Initial state should be healthy")
		}

		if status.ConsecutiveFails != 0 {
			t.Errorf("ConsecutiveFails = %d, want 0", status.ConsecutiveFails)
		}
	})

	t.Run("get status returns copy", func(t *testing.T) {
		mock := &MockTranscriberForHealth{healthy: true}
		checker := NewHealthChecker(mock, 1*time.Second, 3)

		status1 := checker.GetStatus()
		status2 := checker.GetStatus()

		// Addresses are different for copies
		if status1.IsHealthy != status2.IsHealthy {
			t.Error("Status fields should match")
		}
	})

	t.Run("stop can be called multiple times", func(t *testing.T) {
		mock := &MockTranscriberForHealth{healthy: true}
		checker := NewHealthChecker(mock, 1*time.Second, 3)

		checker.Stop()
		checker.Stop()
		checker.Stop()
	})

	t.Run("service status fields", func(t *testing.T) {
		status := ServiceStatus{
			IsHealthy:        true,
			LastCheckTime:    time.Now(),
			ConsecutiveFails: 0,
			ErrorMessage:     "",
		}

		if !status.IsHealthy {
			t.Error("IsHealthy should be true")
		}

		if status.ConsecutiveFails != 0 {
			t.Errorf("ConsecutiveFails = %d, want 0", status.ConsecutiveFails)
		}

		if status.ErrorMessage != "" {
			t.Errorf("ErrorMessage = %q, want empty", status.ErrorMessage)
		}
	})
}

// TestNewHealthChecker tests constructor.
func TestNewHealthChecker(t *testing.T) {
	mock := &MockTranscriberForHealth{healthy: true}
	checker := NewHealthChecker(mock, 5*time.Minute, 3)

	if checker == nil {
		t.Fatal("NewHealthChecker returned nil")
	}

	status := checker.GetStatus()
	if !status.IsHealthy {
		t.Error("Initial status should be healthy")
	}
}
