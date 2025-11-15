package dependency

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"
)

// DependencyClient is a facade for Orchestrator to interact with
// external dependencies (FFmpeg, PyAnnote) without worrying about
// execution details (local vs remote).
//
// It provides high-level business methods that encapsulate:
//   - Command construction
//   - Security validation
//   - Executor selection and invocation
//   - Error handling and reporting
type DependencyClient struct {
	executor    DependencyExecutor
	config      ExecutorConfig
	pathManager *PathManager
}

// NewClient creates a new DependencyClient based on the provided configuration.
// It selects the appropriate executor (Local, Remote, or Fallback) based on config.Mode.
func NewClient(config ExecutorConfig) (*DependencyClient, error) {
	var executor DependencyExecutor

	// Select executor based on mode
	switch config.Mode {
	case ModeLocal:
		executor = NewLocalExecutor(config)
	case ModeRemote:
		executor = NewRemoteExecutor(config)
	case ModeFallback:
		executor = NewFallbackExecutor(config)
	default:
		return nil, fmt.Errorf("invalid execution mode: %s (must be 'local', 'remote', or 'fallback')", config.Mode)
	}

	// Create path manager
	pathManager := NewPathManager(config.SharedVolumePath)

	return &DependencyClient{
		executor:    executor,
		config:      config,
		pathManager: pathManager,
	}, nil
}

// ConvertAudio converts audio from inputPath to outputPath using FFmpeg.
//
// The conversion applies standard settings:
//   - Sample rate: 16000 Hz (required for Whisper)
//   - Audio channels: 1 (mono)
//   - Format: WAV (uncompressed)
//
// Example:
//
//	err := client.ConvertAudio(ctx, "/data/meetings/123/chunk_0000.webm", "/data/meetings/123/chunk_0000.wav")
func (c *DependencyClient) ConvertAudio(ctx context.Context, inputPath, outputPath string) error {
	// Construct command request
	req := CommandRequest{
		Command: "ffmpeg",
		Args: []string{
			"-i", inputPath, // Input file
			"-ar", "16000", // Sample rate 16kHz
			"-ac", "1", // Mono channel
			outputPath, // Output file
		},
		Timeout: c.config.DefaultTimeout,
	}

	// Validate request security
	if err := ValidateCommandRequest(req, c.config); err != nil {
		return fmt.Errorf("command validation failed: %w", err)
	}

	// Execute command
	resp, err := c.executor.ExecuteCommand(ctx, req)
	if err != nil {
		return fmt.Errorf("audio conversion failed: %w", err)
	}

	// Check execution result
	if !resp.Success || resp.ExitCode != 0 {
		return fmt.Errorf("audio conversion failed (exit code %d): %s", resp.ExitCode, resp.Stderr)
	}

	return nil
}

// GetAudioDuration uses ffprobe to detect the duration of an audio file.
//
// Returns the duration in seconds (as float64).
//
// Example:
//
//	duration, err := client.GetAudioDuration(ctx, "/data/meetings/123/uploaded_audio.mp3")
func (c *DependencyClient) GetAudioDuration(ctx context.Context, audioPath string) (float64, error) {
	// Construct ffprobe command to get duration in JSON format
	req := CommandRequest{
		Command: "ffprobe",
		Args: []string{
			"-v", "error",
			"-show_entries", "format=duration",
			"-of", "json",
			audioPath,
		},
		Timeout: 30 * time.Second,
	}

	// Validate request security
	if err := ValidateCommandRequest(req, c.config); err != nil {
		return 0, fmt.Errorf("command validation failed: %w", err)
	}

	// Execute command
	resp, err := c.executor.ExecuteCommand(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("ffprobe execution failed: %w", err)
	}

	// Check execution result
	if !resp.Success || resp.ExitCode != 0 {
		// Enhanced error message with more details
		return 0, fmt.Errorf("ffprobe failed (exit code %d): stderr=%s, stdout=%s", resp.ExitCode, resp.Stderr, resp.Stdout)
	}

	// Parse JSON output to extract duration
	var result struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}

	if err := json.Unmarshal([]byte(resp.Stdout), &result); err != nil {
		return 0, fmt.Errorf("failed to parse ffprobe output: %w, stdout=%s", err, resp.Stdout)
	}

	// Convert duration string to float64
	duration := 0.0
	if _, err := fmt.Sscanf(result.Format.Duration, "%f", &duration); err != nil {
		return 0, fmt.Errorf("failed to parse duration value: %w, duration_string=%s", err, result.Format.Duration)
	}

	return duration, nil
}

// SplitAudioIntoChunks splits a large audio file into fixed-duration chunks using ffmpeg.
//
// Parameters:
//   - inputPath: Path to the input audio file
//   - outputPattern: Output path pattern with %04d placeholder (e.g., "/data/meetings/123/chunk_%04d.wav")
//   - chunkDurationSec: Duration of each chunk in seconds (e.g., 300 for 5 minutes)
//
// Returns the number of chunks created.
//
// Example:
//
//	numChunks, err := client.SplitAudioIntoChunks(ctx, "/data/meetings/123/uploaded.mp3", "/data/meetings/123/chunk_%04d.wav", 300)
func (c *DependencyClient) SplitAudioIntoChunks(ctx context.Context, inputPath, outputPattern string, chunkDurationSec int) (int, error) {
	// First, get the total duration
	totalDuration, err := c.GetAudioDuration(ctx, inputPath)
	if err != nil {
		return 0, fmt.Errorf("failed to get audio duration: %w", err)
	}

	// Calculate number of chunks (round up)
	numChunks := int((totalDuration / float64(chunkDurationSec)) + 0.999) // ceil without math package

	slog.Info("[DependencyClient] splitting audio into chunks",
		"input_path", inputPath,
		"output_pattern", outputPattern,
		"chunk_duration_sec", chunkDurationSec,
		"total_duration_sec", totalDuration,
		"estimated_chunks", numChunks,
	)

	// Construct ffmpeg command for segmentation
	// Note: We need to re-encode to PCM for WAV format, can't use -c copy with m4a -> wav
	req := CommandRequest{
		Command: "ffmpeg",
		Args: []string{
			"-i", inputPath,
			"-f", "segment",
			"-segment_time", fmt.Sprintf("%d", chunkDurationSec),
			"-ar", "16000", // Sample rate 16kHz (optimal for Whisper)
			"-ac", "1", // Mono audio
			"-c:a", "pcm_s16le", // PCM 16-bit little-endian (WAV format)
			"-reset_timestamps", "1",
			outputPattern,
		},
		Timeout: c.config.DefaultTimeout,
	}

	// Validate request security
	if err := ValidateCommandRequest(req, c.config); err != nil {
		return 0, fmt.Errorf("command validation failed: %w", err)
	}

	// Execute command
	resp, err := c.executor.ExecuteCommand(ctx, req)
	if err != nil {
		return 0, fmt.Errorf("audio splitting failed: %w", err)
	}

	// Check execution result
	if !resp.Success || resp.ExitCode != 0 {
		return 0, fmt.Errorf("audio splitting failed (exit code %d): %s", resp.ExitCode, resp.Stderr)
	}

	slog.Info("[DependencyClient] audio splitting completed",
		"num_chunks", numChunks,
		"duration_ms", resp.Duration.Milliseconds(),
	)

	return numChunks, nil
}

// DiarizationOptions contains optional parameters for RunDiarization.
type DiarizationOptions struct {
	// Device specifies the device to use (e.g., "cuda", "cpu").
	// Default: "cpu"
	Device string

	// EnableOffline indicates whether to run in offline mode (no internet).
	EnableOffline bool

	// HFToken is the Hugging Face access token for downloading models.
	HFToken string

	// NumSpeakers is the expected number of speakers (0 means auto-detect).
	NumSpeakers int
}

// RunDiarization performs speaker diarization using PyAnnote.
//
// It analyzes the audio to identify speaker segments and generates:
//   - Segment timings (start, end)
//   - Speaker labels (SPEAKER_00, SPEAKER_01, ...)
//   - Confidence scores
//
// The output is written to outputPath in JSON format.
//
// NOTE: Script path is fixed at /app/scripts/pyannote_diarize.py in deps-service container.
// Caller does not need to specify the script path.
//
// Example:
//
//	opts := &DiarizationOptions{
//	    Device: "cuda",
//	    HFToken: "hf_...",
//	    NumSpeakers: 2,
//	}
//	err := client.RunDiarization(ctx, "/data/meetings/123/chunk_0000.wav", "/data/meetings/123/chunk_0000_segments.json", opts)
func (c *DependencyClient) RunDiarization(ctx context.Context, audioPath, outputPath string, opts *DiarizationOptions) error {
	// Apply defaults if opts is nil
	if opts == nil {
		opts = &DiarizationOptions{}
	}

	if opts.Device == "" {
		opts.Device = "auto"
	}

	slog.Info("[DependencyClient] starting speaker diarization",
		"audio_path", audioPath,
		"output_path", outputPath,
		"num_speakers", opts.NumSpeakers,
		"device", opts.Device,
		"offline", opts.EnableOffline,
	)

	// Use configured script path, fallback to container path for backward compatibility
	scriptPath := c.config.DiarizationScriptPath
	if scriptPath == "" {
		scriptPath = "/app/scripts/pyannote_diarize.py"
		slog.Warn("[DependencyClient] using default diarization script path (not configured)",
			"path", scriptPath)
	}

	// Build command arguments (python + script + flags)
	args := []string{scriptPath, "--input", audioPath, "--device", opts.Device}
	if opts.EnableOffline {
		args = append(args, "--offline")
		slog.Debug("[DependencyClient] offline mode enabled for diarization")
	}
	if opts.NumSpeakers > 0 {
		args = append(args, "--num-speakers", fmt.Sprintf("%d", opts.NumSpeakers))
	}

	// Construct environment variables (Hugging Face token)
	env := map[string]string{}
	if opts.HFToken != "" {
		env["HUGGINGFACE_TOKEN"] = opts.HFToken
		slog.Debug("[DependencyClient] Hugging Face token configured")
	}
	if opts.EnableOffline {
		env["HF_HUB_OFFLINE"] = "1"
	}

	// Construct command request
	req := CommandRequest{
		Command: "python",
		Args:    args,
		Env:     env,
		Timeout: 30 * time.Minute, // Allow time for model download on first run
	}

	// Validate request security
	if err := ValidateCommandRequest(req, c.config); err != nil {
		slog.Error("[DependencyClient] diarization command validation failed",
			"audio_path", audioPath,
			"error", err.Error(),
		)
		return fmt.Errorf("command validation failed: %w", err)
	}

	// Execute command
	slog.Debug("[DependencyClient] executing diarization command", "command", req.Command, "args", req.Args)
	resp, err := c.executor.ExecuteCommand(ctx, req)
	if err != nil {
		slog.Error("[DependencyClient] speaker diarization execution failed",
			"audio_path", audioPath,
			"error", err.Error(),
			"stderr", resp.Stderr,
			"stdout", resp.Stdout,
		)
		return fmt.Errorf("speaker diarization failed: %w, stderr: %s", err, resp.Stderr)
	}

	// Check execution result
	if !resp.Success || resp.ExitCode != 0 {
		slog.Error("[DependencyClient] speaker diarization failed",
			"audio_path", audioPath,
			"exit_code", resp.ExitCode,
			"stderr", resp.Stderr,
		)
		return fmt.Errorf("speaker diarization failed (exit code %d): %s", resp.ExitCode, resp.Stderr)
	}

	// Write stdout to output file (pyannote outputs JSON to stdout)
	if err := os.WriteFile(outputPath, []byte(resp.Stdout), 0644); err != nil {
		slog.Error("[DependencyClient] failed to write diarization output",
			"output_path", outputPath,
			"error", err.Error(),
		)
		return fmt.Errorf("failed to write diarization output: %w", err)
	}

	slog.Info("[DependencyClient] speaker diarization completed successfully",
		"audio_path", audioPath,
		"output_path", outputPath,
	)
	return nil
}

// EmbeddingOptions contains optional parameters for GenerateEmbeddings.
type EmbeddingOptions struct {
	// Device specifies the device to use (e.g., "cuda", "cpu").
	// Default: "cpu"
	Device string

	// EnableOffline indicates whether to run in offline mode (no internet).
	EnableOffline bool

	// HFToken is the Hugging Face access token for downloading models.
	HFToken string

	// Threshold is the minimum similarity threshold for speaker matching.
	// Default: "0.7"
	Threshold string

	// AutoLowerThreshold enables automatic threshold lowering if no matches found.
	AutoLowerThreshold bool

	// AutoLowerMin is the minimum threshold when auto-lowering (e.g., "0.4").
	AutoLowerMin string

	// AutoLowerStep is the step size for threshold reduction (e.g., "0.05").
	AutoLowerStep string

	// ExistingEmbeddings is the path to existing embeddings JSON for comparison.
	ExistingEmbeddings string
}

// GenerateEmbeddings generates speaker embeddings from audio using PyAnnote.
//
// It extracts voice embeddings for each speaker segment and compares them
// with existing embeddings (if provided) to identify speakers across chunks.
//
// The output is written to outputPath in JSON format.
//
// NOTE: Script path is fixed at /app/scripts/generate_speaker_embeddings.py in deps-service container.
// Caller does not need to specify the script path.
//
// Example:
//
//	opts := &EmbeddingOptions{
//	    Device: "cuda",
//	    HFToken: "hf_...",
//	    Threshold: "0.7",
//	    ExistingEmbeddings: "/data/meetings/123/chunk_0000_embeddings.json",
//	}
//	err := client.GenerateEmbeddings(ctx, "/data/meetings/123/chunk_0001.wav", "/data/meetings/123/chunk_0001_speakers.json", "/data/meetings/123/chunk_0001_embeddings.json", opts)
func (c *DependencyClient) GenerateEmbeddings(ctx context.Context, audioPath, speakersPath, outputPath string, opts *EmbeddingOptions) error {
	// Apply defaults if opts is nil
	if opts == nil {
		opts = &EmbeddingOptions{}
	}

	if opts.Device == "" {
		opts.Device = "auto"
	}
	if opts.Threshold == "" {
		opts.Threshold = "0.7"
	}
	if opts.AutoLowerMin == "" {
		opts.AutoLowerMin = "0.4"
	}
	if opts.AutoLowerStep == "" {
		opts.AutoLowerStep = "0.05"
	}

	slog.Info("[DependencyClient] starting embedding generation",
		"audio_path", audioPath,
		"speakers_path", speakersPath,
		"output_path", outputPath,
		"device", opts.Device,
		"threshold", opts.Threshold,
		"offline", opts.EnableOffline,
		"has_existing", opts.ExistingEmbeddings != "",
	)

	// Use configured script path, fallback to container path for backward compatibility
	scriptPath := c.config.EmbeddingScriptPath
	if scriptPath == "" {
		scriptPath = "/app/scripts/generate_speaker_embeddings.py"
		slog.Warn("[DependencyClient] using default embedding script path (not configured)",
			"path", scriptPath)
	}

	// Build command arguments (python + script + flags)
	// Note: HF token will be read from HUGGINGFACE_ACCESS_TOKEN env var in deps-service
	args := []string{
		scriptPath,
		"--audio", audioPath,
		"--speakers-json", speakersPath,
		"--output", outputPath,
		"--device", opts.Device,
		"--threshold", opts.Threshold,
	}

	// Add optional flags
	if opts.AutoLowerThreshold {
		args = append(args, "--auto-lower-threshold")
		args = append(args, "--auto-lower-min", opts.AutoLowerMin)
		args = append(args, "--auto-lower-step", opts.AutoLowerStep)
		slog.Debug("[DependencyClient] auto threshold lowering enabled",
			"min", opts.AutoLowerMin,
			"step", opts.AutoLowerStep,
		)
	}
	if opts.EnableOffline {
		args = append(args, "--offline")
		slog.Debug("[DependencyClient] offline mode enabled for embedding")
	}
	if opts.ExistingEmbeddings != "" {
		args = append(args, "--existing-embeddings", opts.ExistingEmbeddings)
		slog.Debug("[DependencyClient] using existing embeddings", "path", opts.ExistingEmbeddings)
	}

	// Construct environment variables (Hugging Face token)
	env := map[string]string{}
	if opts.HFToken != "" {
		env["HUGGINGFACE_TOKEN"] = opts.HFToken
		slog.Debug("[DependencyClient] Hugging Face token configured for embedding")
	}

	// Construct command request
	req := CommandRequest{
		Command: "python",
		Args:    args,
		Env:     env,
		Timeout: 15 * time.Minute, // Embedding generation can be slow
	}

	// Validate request security
	if err := ValidateCommandRequest(req, c.config); err != nil {
		slog.Error("[DependencyClient] embedding command validation failed",
			"audio_path", audioPath,
			"error", err.Error(),
		)
		return fmt.Errorf("command validation failed: %w", err)
	}

	// Execute command
	slog.Debug("[DependencyClient] executing embedding command", "command", req.Command, "args", req.Args)
	resp, err := c.executor.ExecuteCommand(ctx, req)
	if err != nil {
		slog.Error("[DependencyClient] embedding generation execution failed",
			"audio_path", audioPath,
			"error", err.Error(),
			"stderr", resp.Stderr,
			"stdout", resp.Stdout,
		)
		return fmt.Errorf("embedding generation failed: %w, stderr: %s", err, resp.Stderr)
	}

	// Check execution result
	if !resp.Success || resp.ExitCode != 0 {
		slog.Error("[DependencyClient] embedding generation failed",
			"audio_path", audioPath,
			"exit_code", resp.ExitCode,
			"stderr", resp.Stderr,
		)
		return fmt.Errorf("embedding generation failed (exit code %d): %s", resp.ExitCode, resp.Stderr)
	}

	// Verify output file was created and is non-empty
	if fi, err := os.Stat(outputPath); err != nil {
		slog.Error("[DependencyClient] embedding output file not found",
			"output_path", outputPath,
			"error", err.Error(),
		)
		return fmt.Errorf("embedding output file not created: %w", err)
	} else if fi.Size() == 0 {
		slog.Warn("[DependencyClient] embedding output file is empty",
			"output_path", outputPath,
		)
		return fmt.Errorf("embedding output file is empty: %s", outputPath)
	}

	slog.Info("[DependencyClient] embedding generation completed successfully",
		"audio_path", audioPath,
		"output_path", outputPath,
	)
	return nil
}

// HealthCheck verifies that the underlying executor is ready to handle requests.
// It delegates to the executor's HealthCheck method.
func (c *DependencyClient) HealthCheck(ctx context.Context) error {
	return c.executor.HealthCheck(ctx)
}

// PathManager returns the path manager for file operations.
// Orchestrator can use this to construct standardized file paths.
func (c *DependencyClient) PathManager() *PathManager {
	return c.pathManager
}

// GetMode returns the current execution mode
func (c *DependencyClient) GetMode() ExecutionMode {
	return ExecutionMode(c.config.Mode)
}

// Config returns the executor configuration (read-only access).
// Useful for validation and debugging.
func (c *DependencyClient) Config() ExecutorConfig {
	return c.config
}

// ExecuteCommand executes a command request directly through the underlying executor.
// This is a lower-level API exposed for advanced use cases where the high-level
// methods (ConvertAudio, RunDiarization) don't provide enough flexibility.
//
// Example:
//
//	req := dependency.CommandRequest{
//	    Command: "python",
//	    Args: []string{"script.py", "--input", "file.wav"},
//	    Env: map[string]string{"TOKEN": "xxx"},
//	    Timeout: 5 * time.Minute,
//	}
//	resp, err := client.ExecuteCommand(ctx, req)
func (c *DependencyClient) ExecuteCommand(ctx context.Context, req CommandRequest) (CommandResponse, error) {
	return c.executor.ExecuteCommand(ctx, req)
}
