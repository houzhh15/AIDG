package whisper

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestGoWhisperImpl tests the go-whisper HTTP client implementation.
func TestGoWhisperImpl(t *testing.T) {
	t.Run("successful transcription", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/whisper/transcribe" {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(map[string]interface{}{
					"text": "Hello world",
					"segments": []map[string]interface{}{
						{"text": "Hello", "start": 0.0, "end": 1.2},
						{"text": "world", "start": 1.2, "end": 2.8},
					},
					"language": "en",
					"duration": 2.8,
				})
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		defer server.Close()

		impl := NewGoWhisperImpl(server.URL)

		// Create a temporary test audio file
		tempDir := t.TempDir()
		audioPath := filepath.Join(tempDir, "test.wav")
		if err := os.WriteFile(audioPath, []byte("RIFF....WAVE"), 0644); err != nil {
			t.Fatalf("Failed to create test audio file: %v", err)
		}

		ctx := context.Background()
		result, err := impl.Transcribe(ctx, audioPath, &TranscribeOptions{
			Model:    "base",
			Language: "en",
		})

		if err != nil {
			t.Fatalf("Transcribe() error = %v", err)
		}

		if result.Text != "Hello world" {
			t.Errorf("Text = %q, want %q", result.Text, "Hello world")
		}

		if len(result.Segments) != 2 {
			t.Errorf("len(Segments) = %d, want 2", len(result.Segments))
		}
	})

	t.Run("server returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`{"error": "internal server error"}`))
		}))
		defer server.Close()

		impl := NewGoWhisperImpl(server.URL)

		tempDir := t.TempDir()
		audioPath := filepath.Join(tempDir, "test.wav")
		os.WriteFile(audioPath, []byte("RIFF....WAVE"), 0644)

		ctx := context.Background()
		_, err := impl.Transcribe(ctx, audioPath, nil)

		if err == nil {
			t.Error("Expected error from server, got nil")
		}
	})

	t.Run("health check success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		impl := NewGoWhisperImpl(server.URL)

		ctx := context.Background()
		healthy, err := impl.HealthCheck(ctx)

		if err != nil {
			t.Errorf("HealthCheck() error = %v", err)
		}

		if !healthy {
			t.Error("Expected healthy status")
		}
	})

	t.Run("health check failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		impl := NewGoWhisperImpl(server.URL)

		ctx := context.Background()
		healthy, err := impl.HealthCheck(ctx)

		if healthy {
			t.Error("Expected unhealthy status")
		}

		if err == nil {
			t.Error("Expected error, got nil")
		}
	})

	t.Run("name method", func(t *testing.T) {
		impl := NewGoWhisperImpl("http://localhost:8082")

		name := impl.Name()
		if name != "go-whisper" {
			t.Errorf("Name() = %q, want %q", name, "go-whisper")
		}
	})
}

// TestLocalWhisperImpl tests the local whisper program implementation.
func TestLocalWhisperImpl(t *testing.T) {
	t.Run("creation with invalid program path", func(t *testing.T) {
		_, err := NewLocalWhisperImpl("/nonexistent/whisper", "/models")

		if err == nil {
			t.Error("Expected error for nonexistent program, got nil")
		}
	})

	t.Run("name method", func(t *testing.T) {
		// Create a temporary executable file for testing
		tempDir := t.TempDir()
		programPath := filepath.Join(tempDir, "whisper")
		os.WriteFile(programPath, []byte("#!/bin/sh\necho test"), 0755)

		impl, err := NewLocalWhisperImpl(programPath, tempDir)
		if err != nil {
			t.Fatalf("NewLocalWhisperImpl() error = %v", err)
		}

		name := impl.Name()
		if name != "local-whisper" {
			t.Errorf("Name() = %q, want %q", name, "local-whisper")
		}
	})
}

// TestMockTranscriber tests the mock fallback implementation.
func TestMockTranscriber(t *testing.T) {
	t.Run("transcribe returns empty result", func(t *testing.T) {
		mock := NewMockTranscriber()

		ctx := context.Background()
		result, err := mock.Transcribe(ctx, "/test/audio.wav", nil)

		if err != nil {
			t.Errorf("Transcribe() error = %v", err)
		}

		if result.Text != "" {
			t.Errorf("Expected empty text, got %q", result.Text)
		}

		if len(result.Segments) != 0 {
			t.Errorf("Expected 0 segments, got %d", len(result.Segments))
		}

		if result.Language != "unknown" {
			t.Errorf("Language = %q, want %q", result.Language, "unknown")
		}
	})

	t.Run("health check always returns unhealthy", func(t *testing.T) {
		mock := NewMockTranscriber()

		ctx := context.Background()
		healthy, err := mock.HealthCheck(ctx)

		if err != nil {
			t.Errorf("HealthCheck() error = %v", err)
		}

		if healthy {
			t.Error("MockTranscriber should always be unhealthy")
		}
	})

	t.Run("name method", func(t *testing.T) {
		mock := NewMockTranscriber()

		name := mock.Name()
		if name != "mock-degraded" {
			t.Errorf("Name() = %q, want %q", name, "mock-degraded")
		}
	})
}
