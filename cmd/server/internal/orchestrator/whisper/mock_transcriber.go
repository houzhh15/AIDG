package whisper

import (
	"context"
	"log"
)

// MockTranscriber implements WhisperTranscriber interface as a fallback "degraded mode" implementation.
// It provides a no-op transcription service that returns empty results without blocking operations.
//
// Purpose:
//   - Ensures core application functionality (meeting management, task management) remains operational
//     even when all actual Whisper services (go-whisper, local-whisper, faster-whisper) are unavailable
//   - Prevents cascading failures by gracefully degrading transcription features
//   - Serves as the lowest-priority fallback in the degradation controller's priority chain
//
// Behavior:
//   - Transcribe: Returns empty TranscriptionResult with nil error (never blocks)
//   - HealthCheck: Always returns false (indicates degraded state)
//   - Logs WARN-level messages for monitoring and alerting
type MockTranscriber struct{}

// NewMockTranscriber creates a new MockTranscriber instance.
// The mock transcriber has no configuration or state, so the returned instance is always valid.
func NewMockTranscriber() *MockTranscriber {
	return &MockTranscriber{}
}

// Transcribe performs a no-op "mock" transcription that returns an empty result.
//
// Behavior:
//   - Logs a WARNING message indicating degraded mode operation
//   - Returns empty TranscriptionResult (zero segments, empty text)
//   - Never returns an error to prevent blocking downstream operations
//
// This allows the orchestrator to continue processing meeting audio files without
// crashing or hanging, even though actual transcription is unavailable.
//
// The frontend should detect empty segments and display a user-friendly message like:
// "Transcription service is currently unavailable. Please check deployment guide."
func (m *MockTranscriber) Transcribe(ctx context.Context, audioPath string, options *TranscribeOptions) (*TranscriptionResult, error) {
	log.Printf("[WARN] MockTranscriber: Transcribe called (degraded mode) for audio file: %s", audioPath)
	log.Printf("[WARN] MockTranscriber: Returning empty transcription result. Whisper service is unavailable.")

	return &TranscriptionResult{
		Segments: []TranscriptionSegment{}, // Empty segments array
		Text:     "",
		Language: "unknown",
		Duration: 0,
	}, nil // Explicitly return nil error to avoid blocking
}

// HealthCheck always returns false to indicate that MockTranscriber represents a degraded state.
//
// Returns:
//   - bool: Always false (mock implementation is "unhealthy" by design)
//   - error: Always nil (no actual health check performed)
//
// This ensures the degradation controller knows that the system is operating in fallback mode
// and can display appropriate warnings to administrators and users.
func (m *MockTranscriber) HealthCheck(ctx context.Context) (bool, error) {
	return false, nil
}

// Name returns the identifier of this transcriber implementation.
// The "mock-degraded" name clearly indicates fallback/degraded mode in logs and monitoring.
func (m *MockTranscriber) Name() string {
	return "mock-degraded"
}
