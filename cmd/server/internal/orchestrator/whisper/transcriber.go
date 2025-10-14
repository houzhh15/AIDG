// Package whisper provides an abstraction layer for Whisper audio transcription services.
// It defines standard interfaces and data structures to support multiple implementations
// (go-whisper, faster-whisper, local whisper programs, and mock fallback).
package whisper

import (
	"context"
	"time"
)

// TranscriptionSegment represents a single segment of transcribed audio with timing information.
// Each segment corresponds to a continuous speech interval in the audio.
type TranscriptionSegment struct {
	// ID is the sequential identifier of this segment within the transcription
	ID int `json:"id"`

	// Start is the beginning time of this segment in seconds from the audio start
	Start float64 `json:"start"`

	// End is the ending time of this segment in seconds from the audio start
	End float64 `json:"end"`

	// Text is the transcribed text content of this segment
	Text string `json:"text"`
}

// TranscriptionResult represents the complete result of an audio transcription operation.
// It includes all segments, the full text, detected language, and audio duration.
type TranscriptionResult struct {
	// Segments is the list of all transcribed segments with timing information
	Segments []TranscriptionSegment `json:"segments"`

	// Text is the complete transcribed text (concatenation of all segment texts)
	Text string `json:"text"`

	// Language is the detected or specified language code (e.g., "en", "zh")
	Language string `json:"language"`

	// Duration is the total duration of the audio in seconds
	Duration float64 `json:"duration"`
}

// WhisperTranscriber defines the standard interface for audio transcription services.
// All concrete implementations (GoWhisperImpl, LocalWhisperImpl, MockTranscriber, etc.)
// must implement this interface to be used by the orchestrator's degradation controller.
//
// The interface supports health checking and multiple transcription options to ensure
// flexibility across different Whisper service backends.
type WhisperTranscriber interface {
	// Transcribe performs audio transcription on the given WAV file.
	//
	// Parameters:
	//   - ctx: Context for timeout control and cancellation
	//   - audioPath: Absolute path to the WAV audio file (16kHz, mono, PCM recommended)
	//   - options: Optional transcription parameters (model, language, prompt, timeout)
	//
	// Returns:
	//   - *TranscriptionResult: Complete transcription with segments and metadata
	//   - error: Non-nil if transcription fails (network error, parsing error, service unavailable)
	//
	// Implementation notes:
	//   - Must respect context timeout and cancellation
	//   - Should wrap external errors with context: fmt.Errorf("transcription failed: %w", err)
	//   - Empty segments should return valid TranscriptionResult with empty Segments slice, not error
	Transcribe(ctx context.Context, audioPath string, options *TranscribeOptions) (*TranscriptionResult, error)

	// HealthCheck verifies that the transcription service is operational.
	//
	// Parameters:
	//   - ctx: Context for timeout control (typically 10 seconds)
	//
	// Returns:
	//   - bool: true if service is healthy and ready to transcribe, false otherwise
	//   - error: Non-nil if health check encounters an error (network error, service down)
	//
	// Implementation notes:
	//   - Should be lightweight and fast (< 10 seconds)
	//   - For HTTP-based services: check /health or /api/v1/models endpoint
	//   - For local programs: execute --version command
	//   - MockTranscriber always returns (false, nil)
	HealthCheck(ctx context.Context) (bool, error)

	// Name returns the human-readable identifier of this transcriber implementation.
	//
	// Returns:
	//   - string: Implementation name (e.g., "go-whisper", "local-whisper", "mock-degraded")
	//
	// This name is used for logging, monitoring, and debugging purposes to identify
	// which implementation is currently active in the degradation controller.
	Name() string
}

// TranscribeOptions defines optional parameters for the Transcribe operation.
// All fields are optional; implementations should provide sensible defaults.
type TranscribeOptions struct {
	// Model specifies the Whisper model to use (e.g., "base", "small", "large-v3").
	// Default: "base" (fastest, reasonable accuracy)
	Model string

	// Language forces transcription in a specific language (ISO 639-1 code, e.g., "en", "zh").
	// Empty string means auto-detection.
	// Default: "" (auto-detect)
	Language string

	// Prompt provides context to improve transcription accuracy (optional).
	// Useful for domain-specific terminology or acronyms.
	// Default: ""
	Prompt string

	// Timeout overrides the default transcription timeout.
	// Default: 120 seconds (2 minutes)
	Timeout time.Duration
}
