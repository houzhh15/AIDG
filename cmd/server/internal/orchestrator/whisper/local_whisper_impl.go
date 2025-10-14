package whisper

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// LocalWhisperImpl implements WhisperTranscriber interface for local whisper executable programs.
// It wraps command-line invocations of whisper programs (e.g., whisper.cpp compiled binary)
// that are mounted into the container via Docker volume binding.
//
// This implementation provides the highest priority fallback for environments where
// Docker-based whisper services are unavailable (e.g., macOS ARM with SIGILL errors).
type LocalWhisperImpl struct {
	programPath string // Path to the whisper executable (e.g., /app/bin/whisper)
	modelPath   string // Directory containing whisper model files (e.g., /models/whisper)
}

// NewLocalWhisperImpl creates a new LocalWhisperImpl instance with startup validation.
//
// Parameters:
//   - programPath: Absolute path to the whisper executable (typically /app/bin/whisper)
//   - modelPath: Directory containing GGML model files (e.g., /models/whisper)
//
// Returns:
//   - *LocalWhisperImpl: Configured instance if program exists and is executable
//   - error: Non-nil if program not found, not executable, or validation fails
//
// Startup validation:
//   - Checks program file existence using os.Stat
//   - Verifies executable permission bits (Unix mode 0111: owner/group/other execute)
//   - Returns error immediately if validation fails to prevent runtime surprises
func NewLocalWhisperImpl(programPath, modelPath string) (*LocalWhisperImpl, error) {
	// Check if program exists
	info, err := os.Stat(programPath)
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("whisper program not found: %s", programPath)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat whisper program: %w", err)
	}

	// Check executable permission (at least one execute bit set: owner, group, or other)
	if info.Mode()&0111 == 0 {
		return nil, fmt.Errorf("whisper program is not executable: %s (mode: %s)", programPath, info.Mode())
	}

	return &LocalWhisperImpl{
		programPath: programPath,
		modelPath:   modelPath,
	}, nil
}

// Transcribe performs audio transcription by invoking the local whisper CLI program.
//
// Implementation details:
//   - Constructs CLI command: whisper transcribe <model_file> <audio_file> [options]
//   - Uses exec.CommandContext for timeout control via context
//   - Captures stdout for JSON output parsing
//   - Captures stderr for error diagnostics
//   - Assumes CLI outputs JSON when --output-format json is specified
//
// CLI assumptions (based on whisper.cpp interface):
//   - Subcommand: transcribe
//   - Arguments: model_file_path audio_file_path
//   - Options: --output-format json, --language <lang>
//   - Output: JSON to stdout matching TranscriptionResult structure
func (l *LocalWhisperImpl) Transcribe(ctx context.Context, audioPath string, options *TranscribeOptions) (*TranscriptionResult, error) {
	// Determine model file path
	model := "base"
	if options != nil && options.Model != "" {
		model = options.Model
	}
	modelFile := filepath.Join(l.modelPath, fmt.Sprintf("ggml-%s.bin", model))

	// Build CLI arguments
	args := []string{"transcribe", modelFile, audioPath, "--output-format", "json"}

	// Add optional language parameter
	if options != nil && options.Language != "" {
		args = append(args, "--language", options.Language)
	}

	// Execute command with context timeout
	cmd := exec.CommandContext(ctx, l.programPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("CLI execution failed: %w, output: %s", err, string(output))
	}

	// Parse JSON output from stdout
	var result TranscriptionResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON output: %w, raw output: %s", err, string(output[:min(len(output), 500)]))
	}

	return &result, nil
}

// HealthCheck verifies that the local whisper program is functional.
//
// Implementation:
//   - Executes: whisper --version
//   - Returns true if command succeeds (exit code 0) and produces output
//   - Returns false with error for any failure (program not found, execution error)
//
// This is a lightweight check to ensure the program can be invoked before
// attempting actual transcription operations.
func (l *LocalWhisperImpl) HealthCheck(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, l.programPath, "--version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("version check failed: %w, output: %s", err, string(output))
	}

	// Simple validation: command succeeded and produced some output
	if len(output) > 0 {
		return true, nil
	}

	return false, fmt.Errorf("unexpected empty version output")
}

// Name returns the identifier of this transcriber implementation.
// Used for logging and monitoring to distinguish from other implementations.
func (l *LocalWhisperImpl) Name() string {
	return "local-whisper"
}

// min returns the minimum of two integers (helper function for output truncation)
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
