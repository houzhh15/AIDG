package whisper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// GoWhisperImpl implements WhisperTranscriber interface for go-whisper HTTP service.
// It wraps the go-whisper REST API (ghcr.io/mutablelogic/go-whisper container) to provide
// audio transcription via HTTP multipart/form-data requests.
//
// This implementation is optimized for Linux production environments where the go-whisper
// container can run without CPU instruction set incompatibility issues.
type GoWhisperImpl struct {
	apiURL     string       // Base URL of the go-whisper service (e.g., "http://whisper:80")
	httpClient *http.Client // Reusable HTTP client with configured timeout
}

// NewGoWhisperImpl creates a new GoWhisperImpl instance with the specified API URL.
//
// Parameters:
//   - apiURL: Base URL of the go-whisper service (e.g., "http://whisper:80" or "http://localhost:8082")
//
// Returns:
//   - *GoWhisperImpl: Configured instance ready to perform transcription
//
// The HTTP client is configured with a 10-minute timeout to accommodate large audio files.
// Since audio chunks can be up to 5 minutes long, and transcription time is roughly equal
// to audio duration, we need at least 5+ minutes timeout. Setting to 10 minutes for safety.
func NewGoWhisperImpl(apiURL string) *GoWhisperImpl {
	return &GoWhisperImpl{
		apiURL: apiURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute, // 10-minute timeout for long audio chunk transcription
		},
	}
}

// Transcribe performs audio transcription by sending a multipart/form-data request to go-whisper API.
//
// Implementation details:
//   - Opens the audio file from the provided path
//   - Constructs a multipart request with audio, model fields
//   - Sends POST request to /api/whisper/transcribe endpoint
//   - Parses JSON response into TranscriptionResult
//   - Handles errors with context wrapping for better debugging
//
// Supported audio formats: WAV (16kHz, mono, PCM recommended)
// API endpoint: POST {apiURL}/api/whisper/transcribe
// Reference: https://github.com/mutablelogic/go-whisper/blob/main/doc/API.md#transcription
func (g *GoWhisperImpl) Transcribe(ctx context.Context, audioPath string, options *TranscribeOptions) (*TranscriptionResult, error) {
	// Open audio file
	file, err := os.Open(audioPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open audio file: %w", err)
	}
	defer file.Close()

	// Prepare multipart form data
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Add audio field (go-whisper API uses 'audio' field name)
	part, err := writer.CreateFormFile("audio", filepath.Base(audioPath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("failed to copy file data: %w", err)
	}

	// Add model field (default: "ggml-base" for go-whisper)
	model := "ggml-base"
	if options != nil && options.Model != "" {
		model = options.Model
	}
	if err := writer.WriteField("model", model); err != nil {
		return nil, fmt.Errorf("failed to write model field: %w", err)
	}

	// Add response format field (always JSON for parsing)
	if err := writer.WriteField("response_format", "json"); err != nil {
		return nil, fmt.Errorf("failed to write response_format field: %w", err)
	}

	// Add optional language field
	if options != nil && options.Language != "" {
		if err := writer.WriteField("language", options.Language); err != nil {
			return nil, fmt.Errorf("failed to write language field: %w", err)
		}
	}

	// Add temperature field (default 0.0 to reduce hallucinations/repetitions)
	temperature := 0.0
	if options != nil && options.Temperature > 0 {
		temperature = options.Temperature
	}
	if err := writer.WriteField("temperature", fmt.Sprintf("%.1f", temperature)); err != nil {
		return nil, fmt.Errorf("failed to write temperature field: %w", err)
	}

	// Add optional prompt field to provide context
	if options != nil && options.Prompt != "" {
		if err := writer.WriteField("prompt", options.Prompt); err != nil {
			return nil, fmt.Errorf("failed to write prompt field: %w", err)
		}
	}

	// Close writer to finalize multipart data
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Send HTTP POST request
	endpoint := fmt.Sprintf("%s/api/whisper/transcribe", g.apiURL)
	fmt.Printf("[GO-WHISPER] Sending transcription request to: %s\n", endpoint)
	fmt.Printf("[GO-WHISPER] Audio file: %s, Model: %s\n", audioPath, model)
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check HTTP status code
	fmt.Printf("[GO-WHISPER] Response status: %d\n", resp.StatusCode)
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		fmt.Printf("[GO-WHISPER] Error response body: %s\n", string(bodyBytes))
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse JSON response
	var result TranscriptionResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return &result, nil
}

// HealthCheck verifies that the go-whisper service is operational.
//
// Implementation:
//   - Sends GET request to /api/whisper/model endpoint (go-whisper standard)
//   - Returns true if service responds with 200 OK
//   - Returns false only for network/connection errors
//
// Reference: https://github.com/mutablelogic/go-whisper/blob/main/doc/API.md#models
func (g *GoWhisperImpl) HealthCheck(ctx context.Context) (bool, error) {
	endpoint := fmt.Sprintf("%s/api/whisper/model", g.apiURL)
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("health check request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		return true, nil
	}

	return false, fmt.Errorf("health check failed: status %d", resp.StatusCode)
}

// Name returns the identifier of this transcriber implementation.
// Used for logging, monitoring, and debugging to distinguish between different implementations.
func (g *GoWhisperImpl) Name() string {
	return "go-whisper"
}
