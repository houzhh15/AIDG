package orchestrator

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/houzhh15/AIDG/cmd/server/internal/metrics"
	"github.com/houzhh15/AIDG/cmd/server/internal/orchestrator/degradation"
	"github.com/houzhh15/AIDG/cmd/server/internal/orchestrator/dependency"
	"github.com/houzhh15/AIDG/cmd/server/internal/orchestrator/health"
	"github.com/houzhh15/AIDG/cmd/server/internal/orchestrator/whisper"
	"github.com/houzhh15/AIDG/pkg/logger"
)

// init logging configuration if needed
func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
}

// removeBlankLines rewrites a text file without empty/whitespace-only lines.
func removeBlankLines(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	original := string(data)
	lines := strings.Split(original, "\n")
	kept := make([]string, 0, len(lines))
	for _, l := range lines {
		if strings.TrimSpace(l) == "" { // 纯空行丢弃
			continue
		}
		kept = append(kept, l)
	}
	// 若全部是空行或被判定为空，则保持原文件不动，避免 0 字节
	if len(kept) == 0 {
		return
	}
	// 写入（末尾补换行）
	out := strings.Join(kept, "\n") + "\n"
	// 仅当有变化时才写，减少 IO
	if out != original {
		_ = os.WriteFile(path, []byte(out), 0o644)
	}
}

// copyFile copies a file from src to dst, preserving contents. Caller is
// responsible for handling any cleanup of the source file.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}

	return out.Sync()
}

// Config holds runtime adjustable parameters.
type Config struct {
	OutputDir             string
	RecordChunkDuration   time.Duration
	RecordOverlap         time.Duration
	UseContinuous         bool // true: 单进程持续捕获, 按墙钟切片
	FFmpegDeviceName      string
	FFmpegBinaryPath      string `json:"ffmpeg_path,omitempty"`
	PythonBinaryPath      string `json:"python_path,omitempty"`
	WhisperMode           string // "http" (default) or "cli"
	WhisperAPIURL         string // Whisper HTTP API endpoint (default: "http://whisper:8082")
	WhisperModel          string
	WhisperTemperature    float64 // Temperature for sampling (0.0-1.0, default: 0.0 for deterministic output)
	WhisperSegments       string  // e.g. "20s"; empty or "0" -> do not pass --segments
	DeviceDefault         string
	DiarizationBackend    string // "pyannote" (default) or "speechbrain"
	DiarizationScriptPath string // PyAnnote diarization script path (default: "/app/scripts/pyannote_diarize.py")
	// SpeechBrain diarization tunables
	SBNumSpeakers          int // >0 时强制指定说话人数量 (--num_speakers)
	SBMinSpeakers          int // (--min_speakers), 默认 1
	SBMaxSpeakers          int // (--max_speakers), 默认 8
	SBOverclusterFactor    float64
	SBMergeThreshold       float64
	SBMinSegmentMerge      float64
	SBReassignAfterMerge   bool
	SBEnergyVAD            bool
	SBEnergyVADThr         float64
	EmbeddingScriptPath    string // Speaker embedding generation script path (default: "/app/scripts/generate_speaker_embeddings.py")
	EmbeddingDeviceDefault string
	EmbeddingThreshold     string
	EmbeddingAutoLowerMin  string
	EmbeddingAutoLowerStep string
	InitialEmbeddingsPath  string
	HFTokenValue           string
	EnableOffline          bool
	// Whisper degradation and health check configuration
	EnableDegradation        bool          // Enable automatic degradation (default: true)
	HealthCheckInterval      time.Duration // Health check interval (default: 5 minutes)
	HealthCheckFailThreshold int           // Consecutive failures before degradation (default: 3)
	// Task metadata fields
	ProductLine string    `json:"product_line"` // 产品线
	MeetingTime time.Time `json:"meeting_time"` // 会议时间

	// ============ Dependency Execution Configuration ============
	// DependencyMode specifies how to execute external commands (FFmpeg, PyAnnote).
	// Valid values: "local" (direct exec), "remote" (HTTP service), "fallback" (auto-degrade).
	// Default: "local" (backward compatible).
	DependencyMode string `json:"dependency_mode,omitempty" yaml:"dependency_mode,omitempty"`

	// DependencyServiceURL is the HTTP endpoint of the optional dependency service.
	// Example: "http://deps-service:8080"
	// Required when DependencyMode is "remote" or "fallback".
	DependencyServiceURL string `json:"dependency_service_url,omitempty" yaml:"dependency_service_url,omitempty"`

	// DependencySharedVolume is the base path of the shared volume between
	// main service and dependency service (e.g., "/data").
	// All input/output files must reside within this path.
	// Default: "/app/data" (matches current OutputDir default).
	DependencySharedVolume string `json:"dependency_shared_volume,omitempty" yaml:"dependency_shared_volume,omitempty"`

	// DependencyTimeout is the default timeout for all command executions.
	// Can be overridden per command. Default: 5 minutes.
	DependencyTimeout time.Duration `json:"dependency_timeout,omitempty" yaml:"dependency_timeout,omitempty"`

	// Speaker mapping configuration
	SpeakerMapThreshold   float64 `json:"speaker_map_threshold"`
	GlobalSpeakersMapPath string  `json:"global_speakers_map_path"`
}

// DependencyError conveys missing runtime dependencies and friendly recovery guidance.
type DependencyError struct {
	Missing    []string // 不可用依赖列表（如 ["FFmpeg", "PyAnnote"]）
	LiteMode   bool     // 系统是否运行在轻量模式
	Details    []string // 可操作的配置指导
	TriedModes []string // 尝试的执行模式（如 ["remote", "local"]）
}

func (e DependencyError) Error() string {
	core := fmt.Sprintf("缺少必需依赖: %s", strings.Join(e.Missing, ", "))

	// 添加尝试模式信息
	if len(e.TriedModes) > 0 {
		core += fmt.Sprintf("（已尝试: %s）", strings.Join(e.TriedModes, " → "))
	}

	if len(e.Details) == 0 {
		return core
	}
	return core + "。\n" + strings.Join(e.Details, "\n")
}

// ToHTTPResponse 将错误转换为结构化 HTTP 响应（用于 API 错误返回）
func (e DependencyError) ToHTTPResponse() map[string]interface{} {
	return map[string]interface{}{
		"error":       "dependency_unavailable",
		"missing":     e.Missing,
		"lite_mode":   e.LiteMode,
		"details":     e.Details,
		"tried_modes": e.TriedModes,
	}
}

// ApplyRuntimeDefaults ensures backwards compatibility for persisted configs by
// hydrating binary paths from the current environment when missing.
func (cfg *Config) ApplyRuntimeDefaults() {
	if cfg == nil {
		return
	}
	if strings.TrimSpace(cfg.FFmpegBinaryPath) == "" {
		if env := strings.TrimSpace(os.Getenv("FFMPEG_PATH")); env != "" {
			cfg.FFmpegBinaryPath = env
		} else {
			cfg.FFmpegBinaryPath = "ffmpeg"
		}
	}
	if strings.TrimSpace(cfg.PythonBinaryPath) == "" {
		if env := strings.TrimSpace(os.Getenv("PYTHON_PATH")); env != "" {
			cfg.PythonBinaryPath = env
		} else {
			cfg.PythonBinaryPath = "python3"
		}
	}
	// Always check environment variables for script paths, regardless of current value
	if env := strings.TrimSpace(os.Getenv("DIARIZATION_SCRIPT_PATH")); env != "" {
		cfg.DiarizationScriptPath = env
	} else if strings.TrimSpace(cfg.DiarizationScriptPath) == "" {
		cfg.DiarizationScriptPath = "/app/scripts/pyannote_diarize.py"
	}
	if env := strings.TrimSpace(os.Getenv("EMBEDDING_SCRIPT_PATH")); env != "" {
		cfg.EmbeddingScriptPath = env
	} else if strings.TrimSpace(cfg.EmbeddingScriptPath) == "" {
		cfg.EmbeddingScriptPath = "/app/scripts/generate_speaker_embeddings.py"
	}
	// Always check environment variables for Whisper mode
	if env := strings.TrimSpace(os.Getenv("WHISPER_MODE")); env != "" {
		cfg.WhisperMode = env
	}
}

// ValidateCriticalDependencies verifies external binaries required for the
// configured processing pipeline. It returns an error describing all missing
// dependencies so the caller can surface actionable feedback to the user.
func (cfg Config) ValidateCriticalDependencies() error {
	// Check if audio processing is disabled (lightweight mode)
	enableAudioConversion := strings.ToLower(os.Getenv("ENABLE_AUDIO_CONVERSION"))
	enableDiarization := strings.ToLower(os.Getenv("ENABLE_SPEAKER_DIARIZATION"))

	// If audio processing is explicitly disabled, skip validation
	if (enableAudioConversion == "false" || enableAudioConversion == "0") &&
		(enableDiarization == "false" || enableDiarization == "0") {
		log.Println("[ValidateCriticalDependencies] Audio processing disabled, skipping dependency checks")
		return nil
	}

	// Check if using remote dependency service (fallback mode)
	dependencyMode := strings.ToLower(os.Getenv("DEPENDENCY_MODE"))
	depsServiceURL := os.Getenv("DEPS_SERVICE_URL")

	// If using remote deps-service, skip local dependency checks
	if dependencyMode == "fallback" || depsServiceURL != "" {
		log.Printf("[ValidateCriticalDependencies] Using remote dependency service (mode=%s, url=%s), skipping local dependency checks", dependencyMode, depsServiceURL)
		return nil
	}

	var (
		missing  []string
		details  []string
		liteMode bool
	)

	// Continuous recording relies on FFmpeg.
	if cfg.UseContinuous {
		ffmpegBin := strings.TrimSpace(cfg.FFmpegBinaryPath)
		if ffmpegBin == "" {
			ffmpegBin = "ffmpeg"
		}
		if _, err := exec.LookPath(ffmpegBin); err != nil {
			missing = append(missing, fmt.Sprintf("FFmpeg (%s)", ffmpegBin))
			liteMode = true
			details = append(details, "当前容器内未找到 FFmpeg，可在宿主机安装后通过卷挂载，或切换至完整版镜像")
		}
	}

	// PyAnnote diarization requires Python executable and diarization scripts.
	if strings.EqualFold(cfg.DiarizationBackend, "pyannote") {
		pythonBin := strings.TrimSpace(cfg.PythonBinaryPath)
		if pythonBin == "" {
			pythonBin = "python3"
		}
		if _, err := exec.LookPath(pythonBin); err != nil {
			missing = append(missing, fmt.Sprintf("Python (%s)", pythonBin))
			liteMode = true
			details = append(details, "未检测到 Python 运行时，请通过卷挂载提供或在宿主机安装后映射进容器")
		}
		if strings.TrimSpace(cfg.DiarizationScriptPath) == "" {
			missing = append(missing, "PyAnnote diarization script (未配置)")
			liteMode = true
			details = append(details, "请将 pyannote_diarize.py 挂载到 /app/scripts/pyannote_diarize.py 或更新配置")
		} else if _, err := os.Stat(cfg.DiarizationScriptPath); err != nil {
			missing = append(missing, fmt.Sprintf("PyAnnote diarization script %s", cfg.DiarizationScriptPath))
			liteMode = true
			details = append(details, "挂载的 pyannote 脚本不存在，请确认宿主机路径与 docker-compose 映射")
		}
		if strings.TrimSpace(cfg.EmbeddingScriptPath) == "" {
			missing = append(missing, "PyAnnote embedding script (未配置)")
			liteMode = true
			details = append(details, "请将 generate_speaker_embeddings.py 挂载到 /app/scripts/generate_speaker_embeddings.py 或更新配置")
		} else if _, err := os.Stat(cfg.EmbeddingScriptPath); err != nil {
			missing = append(missing, fmt.Sprintf("PyAnnote embedding script %s", cfg.EmbeddingScriptPath))
			liteMode = true
			details = append(details, "挂载的 embedding 脚本不存在，请检查路径映射")
		}
	}

	if len(missing) == 0 {
		return nil
	}

	if liteMode {
		details = append([]string{"检测到轻量级镜像（lite）未包含音频处理依赖"}, details...)
	}

	return DependencyError{Missing: missing, LiteMode: liteMode, Details: details}
}

// DefaultConfig returns sensible defaults matching original main.go constants.
func DefaultConfig() Config {
	// 从环境变量读取 Whisper API URL，如果未设置则使用默认值
	whisperURL := os.Getenv("WHISPER_API_URL")
	if whisperURL == "" {
		whisperURL = "http://whisper:80"
	}

	// Parse ENABLE_DEGRADATION (default: true)
	enableDegradation := true
	if envDeg := os.Getenv("ENABLE_DEGRADATION"); envDeg != "" {
		enableDegradation = strings.ToLower(envDeg) == "true"
	}

	// Parse HEALTH_CHECK_INTERVAL (default: 5 minutes)
	healthCheckInterval := 5 * time.Minute
	if envInterval := os.Getenv("HEALTH_CHECK_INTERVAL"); envInterval != "" {
		if parsed, err := time.ParseDuration(envInterval); err == nil {
			healthCheckInterval = parsed
		} else {
			log.Printf("[Config] Invalid HEALTH_CHECK_INTERVAL '%s', using default 5m", envInterval)
		}
	}

	// Parse HEALTH_CHECK_FAIL_THRESHOLD (default: 3)
	healthCheckFailThreshold := 3
	if envThreshold := os.Getenv("HEALTH_CHECK_FAIL_THRESHOLD"); envThreshold != "" {
		if parsed, err := strconv.Atoi(envThreshold); err == nil && parsed > 0 {
			healthCheckFailThreshold = parsed
		} else {
			log.Printf("[Config] Invalid HEALTH_CHECK_FAIL_THRESHOLD '%s', using default 3", envThreshold)
		}
	}

	ffmpegPath := strings.TrimSpace(os.Getenv("FFMPEG_PATH"))
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}

	pythonPath := strings.TrimSpace(os.Getenv("PYTHON_PATH"))
	if pythonPath == "" {
		pythonPath = "python3"
	}

	// 从环境变量读取依赖模式和服务URL
	dependencyMode := strings.TrimSpace(os.Getenv("DEPENDENCY_MODE"))
	if dependencyMode == "" {
		dependencyMode = "local" // 默认本地模式
	}

	dependencyServiceURL := strings.TrimSpace(os.Getenv("DEPS_SERVICE_URL"))

	// 从环境变量读取依赖共享卷路径
	dependencySharedVolume := strings.TrimSpace(os.Getenv("DEPENDENCY_SHARED_VOLUME"))

	// 从环境变量读取离线模式配置
	enableOffline := true // 默认启用离线模式，禁止下载模型
	if offlineEnv := strings.ToLower(strings.TrimSpace(os.Getenv("ENABLE_OFFLINE"))); offlineEnv != "" {
		enableOffline = offlineEnv == "true" || offlineEnv == "1"
	}

	return Config{
		OutputDir:                "meeting_output",
		RecordChunkDuration:      5 * time.Minute,
		RecordOverlap:            5 * time.Second,
		UseContinuous:            false, // 禁用自动录音，使用文件上传模式
		FFmpegDeviceName:         "",    // Docker 环境中不使用音频设备
		FFmpegBinaryPath:         ffmpegPath,
		PythonBinaryPath:         pythonPath,
		DependencyMode:           dependencyMode,
		DependencyServiceURL:     dependencyServiceURL,
		DependencySharedVolume:   dependencySharedVolume,
		WhisperMode:              "",              // 留空，让ApplyRuntimeDefaults从环境变量读取
		WhisperAPIURL:            whisperURL,      // 从环境变量读取或使用默认值
		WhisperModel:             "ggml-large-v3", // 默认使用 large-v3 模型
		WhisperSegments:          "15s",           // 修改为 15s
		DeviceDefault:            "auto",          // 使用 CPU (Linux 容器中 mps 不可用)
		DiarizationBackend:       "pyannote",
		DiarizationScriptPath:    "/app/scripts/pyannote_diarize.py",
		SBMinSpeakers:            1,
		SBMaxSpeakers:            8,
		SBOverclusterFactor:      1.4,
		SBMergeThreshold:         0.86,
		SBMinSegmentMerge:        0.8,
		SBReassignAfterMerge:     true,
		SBEnergyVAD:              true,
		SBEnergyVADThr:           0.5,
		EmbeddingScriptPath:      "/app/scripts/generate_speaker_embeddings.py",
		EmbeddingDeviceDefault:   "auto",
		EmbeddingThreshold:       "0.55",
		EmbeddingAutoLowerMin:    "0.35", // 修改为 0.35
		EmbeddingAutoLowerStep:   "0.02",
		InitialEmbeddingsPath:    "",
		HFTokenValue:             os.Getenv("HUGGINGFACE_TOKEN"), // 从环境变量读取，不硬编码
		EnableOffline:            enableOffline,
		EnableDegradation:        enableDegradation,
		HealthCheckInterval:      healthCheckInterval,
		HealthCheckFailThreshold: healthCheckFailThreshold,
		SpeakerMapThreshold:      0.5,
	}
}

// State values
type State string

const (
	StateCreated   State = "created"
	StateRunning   State = "running"
	StateStopping  State = "stopping"
	StateDraining  State = "draining"
	StateStopped   State = "stopped"
	StateCompleted State = "completed"
)

// ProgressInfo snapshot
type ProgressInfo struct {
	State          State          `json:"state"`
	CurrentChunk   int            `json:"current_chunk"`
	Files          map[string]int `json:"files"`
	LastEmbeddings string         `json:"last_embeddings"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

// internal queue generic
type SafeQueue[T any] struct {
	ch     chan T
	once   sync.Once
	closed atomic.Bool
}

func NewSafeQueue[T any](size int) *SafeQueue[T] { return &SafeQueue[T]{ch: make(chan T, size)} }
func (q *SafeQueue[T]) Push(v T) {
	if q.closed.Load() {
		return
	}
	q.ch <- v
}
func (q *SafeQueue[T]) Pop() (T, bool) { v, ok := <-q.ch; return v, ok }
func (q *SafeQueue[T]) Close()         { q.once.Do(func() { q.closed.Store(true); close(q.ch) }) }

type AudioChunk struct {
	ID        int
	Path      string
	StartTime time.Time
	EndTime   time.Time
}
type ASRResult struct {
	Chunk   AudioChunk
	SegJSON string
}
type SDResult struct {
	Chunk                 AudioChunk
	SegJSON, SpeakersJSON string
}
type EmbeddingResult struct {
	Chunk                                 AudioChunk
	SegJSON, SpeakersJSON, EmbeddingsJSON string
}

type Orchestrator struct {
	cfg   Config
	state State
	mutex sync.Mutex

	recorder *Recorder
	asrQ     *SafeQueue[AudioChunk]
	sdQ      *SafeQueue[ASRResult]
	embQ     *SafeQueue[SDResult]
	mergeQ   *SafeQueue[EmbeddingResult]
	wg       sync.WaitGroup

	voicePrint   *VoicePrintState
	startChunkID int
	nextChunkID  int  // 下一个要使用的 chunk ID (用于停止后恢复)
	reprocess    bool // reprocess mode flag (skip recorder/asr, feed existing wav+segments)
	procCtx      context.Context
	procCancel   context.CancelFunc

	// Whisper服务健康检查和降级控制
	healthChecker         *health.HealthChecker
	degradationController *degradation.DegradationController

	// DependencyClient for external command execution (FFmpeg, PyAnnote)
	dependencyClient *dependency.DependencyClient

	// PathManager for consistent file path construction
	pathManager *dependency.PathManager

	// healthTicker for periodic dependency availability checks (fallback mode only)
	healthTicker *time.Ticker
}

// RunSingleASR runs whisper once on an existing chunk wav with a provided model, segment length, and temperature
// It does not mutate orchestrator config. Returns path to generated segments json.
func (o *Orchestrator) RunSingleASR(ctx context.Context, chunkWav string, model string, segLen string, temperature float64) (string, error) {
	if model == "" {
		model = o.cfg.WhisperModel
	}

	// Extract chunk ID from filename
	base := filepath.Base(chunkWav)
	m := regexp.MustCompile(`^chunk_([0-9]{4})\.wav$`).FindStringSubmatch(base)
	if m == nil {
		return "", fmt.Errorf("invalid chunk wav name: %s", base)
	}
	idStr := m[1]
	chunkID, _ := strconv.Atoi(idStr) // Safe: regex ensures 4-digit number

	// Use PathManager to construct output path
	meetingID := filepath.Base(o.cfg.OutputDir)
	out := o.pathManager.GetChunkSegmentsPath(meetingID, chunkID)

	// Defensive check: ensure degradationController is initialized
	if o.degradationController == nil {
		log.Printf("[RunSingleASR] DegradationController not initialized, initializing now...")
		// Directly initialize the degradation controller without changing state
		if err := o.ensureDegradationController(); err != nil {
			return "", fmt.Errorf("failed to initialize degradation controller: %w", err)
		}
		log.Printf("[RunSingleASR] DegradationController initialized successfully")
	}

	// Use DegradationController to get active transcriber
	transcriber := o.degradationController.GetTranscriber()

	// For all transcribers, use normal flow with proper JSON parsing
	// Prepare transcription options
	opts := &whisper.TranscribeOptions{
		Model:       model,
		Language:    "",          // Auto-detect
		Temperature: temperature, // Use provided temperature instead of config
	}

	// Call transcriber
	result, err := transcriber.Transcribe(ctx, chunkWav, opts)
	if err != nil {
		log.Printf("[RunSingleASR] Transcription failed: %v", err)
		return "", fmt.Errorf("transcription failed: %w", err)
	}
	log.Printf("[RunSingleASR] Transcription successful, segments count: %d", len(result.Segments))

	// Convert TranscriptionResult to JSON and save
	jsonData, err := json.Marshal(result)
	if err != nil {
		log.Printf("[RunSingleASR] Failed to marshal result: %v", err)
		return "", fmt.Errorf("failed to marshal transcription result: %w", err)
	}
	log.Printf("[RunSingleASR] JSON marshaled, size: %d bytes", len(jsonData))

	if err := os.WriteFile(out, jsonData, 0644); err != nil {
		log.Printf("[RunSingleASR] Failed to write file %s: %v", out, err)
		return "", fmt.Errorf("failed to write segments file: %w", err)
	}
	log.Printf("[RunSingleASR] Successfully wrote segments file: %s", out)

	return out, nil
}

// runLocalWhisperDirectOutput runs local whisper and writes output directly to file
func (o *Orchestrator) runLocalWhisperDirectOutput(ctx context.Context, chunkWav, model, outputPath string, temperature float64) (string, error) {
	// Get whisper program path
	programPath := os.Getenv("WHISPER_PROGRAM_PATH")
	if programPath == "" {
		programPath = "./bin/whisper/whisper"
	}

	// Process model name - keep ggml- prefix, just remove .bin suffix if present
	model = strings.TrimSuffix(model, ".bin")
	// Ensure it has ggml- prefix
	if !strings.HasPrefix(model, "ggml-") {
		model = "ggml-" + model
	}

	// Build CLI arguments
	args := []string{"transcribe", model, chunkWav, "--format", "json"}
	// Add temperature parameter (default 0.0 if not specified)
	args = append(args, "--temperature", fmt.Sprintf("%.1f", temperature))

	// Execute command
	cmd := exec.CommandContext(ctx, programPath, args...)
	log.Printf("[LocalWhisperDirect] Executing: %s %s", programPath, strings.Join(args, " "))

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("[LocalWhisperDirect] ERROR: %v", err)
		log.Printf("[LocalWhisperDirect] Output: %s", string(output))
		return "", fmt.Errorf("CLI execution failed: %w, output: %s", err, string(output))
	}

	log.Printf("[LocalWhisperDirect] OK - Output length: %d bytes", len(output))

	// Write raw output directly to file
	if err := os.WriteFile(outputPath, output, 0644); err != nil {
		log.Printf("[LocalWhisperDirect] Failed to write file %s: %v", outputPath, err)
		return "", fmt.Errorf("failed to write segments file: %w", err)
	}

	log.Printf("[LocalWhisperDirect] Successfully wrote segments file: %s", outputPath)
	return outputPath, nil
}

// transcribeViaHTTP 通过 HTTP POST 调用 Whisper API 进行转录
func (o *Orchestrator) transcribeViaHTTP(ctx context.Context, chunk AudioChunk) (string, error) {
	whisperURL := o.cfg.WhisperAPIURL
	if whisperURL == "" {
		whisperURL = "http://whisper:8082"
	}
	endpoint := fmt.Sprintf("%s/transcribe", whisperURL)

	audioFile, err := os.Open(chunk.Path)
	if err != nil {
		return "", fmt.Errorf("open audio file: %w", err)
	}
	defer audioFile.Close()

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	// 使用 "file" 作为字段名，这是OpenAI Whisper API的标准字段名
	part, err := writer.CreateFormFile("file", filepath.Base(chunk.Path))
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(part, audioFile); err != nil {
		return "", fmt.Errorf("copy audio data: %w", err)
	}

	// 添加其他表单字段
	// 使用配置的模型名称，如果为空则使用 ggml-base
	modelName := "ggml-base"
	if o.cfg.WhisperModel != "" {
		modelName = o.cfg.WhisperModel
	}
	if err := writer.WriteField("model", modelName); err != nil {
		return "", fmt.Errorf("write model field: %w", err)
	}
	if err := writer.WriteField("response_format", "json"); err != nil {
		return "", fmt.Errorf("write format field: %w", err)
	}
	if o.cfg.WhisperSegments != "" && o.cfg.WhisperSegments != "0" && o.cfg.WhisperSegments != "0s" {
		if err := writer.WriteField("segments", o.cfg.WhisperSegments); err != nil {
			return "", fmt.Errorf("write segments field: %w", err)
		}
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("close writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &buf)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 10 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("whisper API error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("whisper API returned %d: %s", resp.StatusCode, string(body))
	}

	// Use PathManager to construct output path
	meetingID := filepath.Base(o.cfg.OutputDir)
	segPath := o.pathManager.GetChunkSegmentsPath(meetingID, chunk.ID)
	outFile, err := os.Create(segPath)
	if err != nil {
		return "", fmt.Errorf("create output file: %w", err)
	}
	defer outFile.Close()

	if _, err := io.Copy(outFile, resp.Body); err != nil {
		return "", fmt.Errorf("save response: %w", err)
	}
	return segPath, nil
}

// transcribeViaCLI 通过命令行调用 Whisper CLI 进行转录
func (o *Orchestrator) transcribeViaCLI(ctx context.Context, chunk AudioChunk) (string, error) {
	// Use PathManager to construct output path
	meetingID := filepath.Base(o.cfg.OutputDir)
	segPath := o.pathManager.GetChunkSegmentsPath(meetingID, chunk.ID)
	args := []string{"go-whisper/whisper", "transcribe", o.cfg.WhisperModel, chunk.Path, "--format", "json"}
	ws := strings.TrimSpace(strings.ToLower(o.cfg.WhisperSegments))
	if ws != "" && ws != "0" && ws != "0s" {
		args = append(args, "--segments", o.cfg.WhisperSegments)
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("pipe error: %w", err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start error: %w", err)
	}
	f, err := os.Create(segPath)
	if err != nil {
		return "", fmt.Errorf("create output: %w", err)
	}
	w := bufio.NewWriter(f)
	_, _ = io.Copy(w, stdout)
	w.Flush()
	f.Close()
	if err := cmd.Wait(); err != nil {
		return "", fmt.Errorf("wait error: %w", err)
	}
	return segPath, nil
}

type VoicePrintState struct {
	CurrentEmbPath string
	Mutex          sync.Mutex
}

// Recorder logic (simplified copy)
type Recorder struct {
	cfg           Config
	chunkID       int
	ctx           context.Context
	cancel        context.CancelFunc
	asrQueue      *SafeQueue[AudioChunk]
	wg            *sync.WaitGroup
	stopFlag      atomic.Bool
	forceFinalize atomic.Bool // true: keep partial chunk when stopping
	pathManager   *dependency.PathManager
}

func NewRecorder(cfg Config, asrQueue *SafeQueue[AudioChunk], wg *sync.WaitGroup, pathManager *dependency.PathManager) *Recorder {
	ctx, cancel := context.WithCancel(context.Background())
	return &Recorder{cfg: cfg, ctx: ctx, cancel: cancel, asrQueue: asrQueue, wg: wg, pathManager: pathManager}
}

func (r *Recorder) Start()       { r.wg.Add(1); go r.loop() }
func (r *Recorder) RequestStop() { r.stopFlag.Store(true) }

// FinalizeAndStop: stop early but keep (partial) current wav and enqueue
func (r *Recorder) FinalizeAndStop() {
	r.stopFlag.Store(true)
	r.forceFinalize.Store(true)
	if r.cancel != nil {
		r.cancel()
	}
}

func (r *Recorder) loop() {
	// 如果启用持续录制模式，切换到 continuousLoop
	if r.cfg.UseContinuous {
		r.continuousLoop()
		return
	}
	defer r.wg.Done()
	for {
		curID := r.chunkID
		// Use PathManager to construct audio file path
		meetingID := filepath.Base(r.cfg.OutputDir)
		audioFile := r.pathManager.GetChunkAudioPath(meetingID, curID, "wav")
		tmpFile := audioFile + ".partial"
		start := time.Now()
		endPlanned := start.Add(r.cfg.RecordChunkDuration)
		// 方案A: 使用 -t 强制确保录制时长不被意外提前截断
		durSec := int(r.cfg.RecordChunkDuration.Seconds())
		ffmpegBin := strings.TrimSpace(r.cfg.FFmpegBinaryPath)
		if ffmpegBin == "" {
			ffmpegBin = "ffmpeg"
		}
		cmd := exec.CommandContext(r.ctx, ffmpegBin, "-y", "-f", "avfoundation", "-i", fmt.Sprintf(":%s", r.cfg.FFmpegDeviceName), "-t", strconv.Itoa(durSec), "-ac", "1", "-ar", "16000", tmpFile)
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := cmd.Start(); err != nil {
			log.Printf("[REC] start err: %v", err)
			break
		}
		// 方案C: 保留 killTimer 作为保险，同时监听进程提前退出
		killTimer := time.NewTimer(r.cfg.RecordChunkDuration + 5*time.Second) // 余量 5s
		processDone := make(chan struct{})
		var waitErr error
		go func() {
			waitErr = cmd.Wait()
			close(processDone)
		}()
		enqueue := true
		endActual := endPlanned
		endedEarly := false
		for {
			select {
			case <-r.ctx.Done():
				// context 取消 -> 尝试优雅中断
				_ = cmd.Process.Signal(os.Interrupt)
				select {
				case <-processDone:
				case <-time.After(1 * time.Second):
				}
				endActual = time.Now()
				if !r.forceFinalize.Load() {
					enqueue = false
				}
				killTimer.Stop()
				goto RECORD_END
			case <-killTimer.C:
				// 超时兜底: 进程还没退出则发送 interrupt
				_ = cmd.Process.Signal(os.Interrupt)
				select {
				case <-processDone:
				case <-time.After(2 * time.Second):
					_ = cmd.Process.Kill()
				}
				endActual = time.Now()
				goto RECORD_END
			case <-processDone:
				// 正常 / 提前退出
				endActual = time.Now()
				if endActual.Before(endPlanned.Add(-2 * time.Second)) { // 比计划提前超过2s
					endedEarly = true
				}
				killTimer.Stop()
				goto RECORD_END
			}
		}
	RECORD_END:
		if waitErr != nil && !r.stopFlag.Load() {
			log.Printf("[REC] ffmpeg exit err chunk=%04d err=%v", curID, waitErr)
		}
		if endedEarly && !r.stopFlag.Load() {
			log.Printf("[REC] early termination chunk=%04d actual=%.2fs planned=%.2fs", curID, endActual.Sub(start).Seconds(), r.cfg.RecordChunkDuration.Seconds())
		}
		// 确保进程已退出，防御性 Kill (无害)
		_ = cmd.Process.Kill()
		if !r.stopFlag.Load() {
			r.chunkID++
		}
		finalPath := audioFile
		if _, err := os.Stat(tmpFile); err == nil {
			if enqueue {
				if err := os.Rename(tmpFile, audioFile); err != nil {
					log.Printf("[REC] rename partial chunk failed: %v", err)
					if errCopy := copyFile(tmpFile, audioFile); errCopy != nil {
						log.Printf("[REC] copy partial chunk failed: %v", errCopy)
						finalPath = tmpFile
					} else {
						finalPath = audioFile
						_ = os.Remove(tmpFile)
					}
				}
			} else {
				_ = os.Remove(tmpFile)
			}
		} else if !enqueue {
			_ = os.Remove(audioFile)
		}
		if enqueue {
			log.Printf("[ASR Queue] Pushing chunk to queue: ID=%d, Path=%s", curID, finalPath)
			r.asrQueue.Push(AudioChunk{ID: curID, Path: finalPath, StartTime: start, EndTime: endActual})
			log.Printf("[ASR Queue] Chunk pushed successfully")
		}
		recDur := endActual.Sub(start)
		planned := r.cfg.RecordChunkDuration
		if diff := recDur - planned; diff < -2*time.Second || diff > 2*time.Second {
			log.Printf("[REC] duration drift chunk=%04d actual=%.2fs planned=%.2fs diff=%.2fs", curID, recDur.Seconds(), planned.Seconds(), diff.Seconds())
		}
		if r.stopFlag.Load() {
			break
		}
	}
	r.asrQueue.Close()
}

// continuousLoop: 单次启动 ffmpeg 持续输出 raw PCM (s16le), 按采样数切片 (无补零)
func (r *Recorder) continuousLoop() {
	defer r.wg.Done()
	log.Printf("[REC] continuous mode start (chunk=%04d dur=%s slice-by-samples)", r.chunkID, r.cfg.RecordChunkDuration)
	// 准备 ffmpeg 命令: 输出原始 PCM 到 stdout
	ffmpegBin := strings.TrimSpace(r.cfg.FFmpegBinaryPath)
	if ffmpegBin == "" {
		ffmpegBin = "ffmpeg"
	}
	cmd := exec.CommandContext(r.ctx, ffmpegBin, "-hide_banner", "-loglevel", "error", "-f", "avfoundation", "-i", fmt.Sprintf(":%s", r.cfg.FFmpegDeviceName), "-ac", "1", "-ar", "16000", "-f", "s16le", "-use_wallclock_as_timestamps", "1", "-")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("[REC] continuous stdout pipe err: %v", err)
		return
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Printf("[REC] continuous start ffmpeg err: %v", err)
		return
	}

	const (
		sampleRate     = 16000
		channels       = 1
		bitsPerSample  = 16
		bytesPerSample = bitsPerSample / 8
	)
	expectedPerChunkSamples := int(r.cfg.RecordChunkDuration.Seconds() * sampleRate)
	if expectedPerChunkSamples <= 0 {
		expectedPerChunkSamples = sampleRate * 60
	}

	var (
		curFile    *os.File
		curSamples int
		chunkStart time.Time
		buf        = make([]byte, 4096)
	)

	openChunk := func(id int) error {
		meetingID := filepath.Base(r.cfg.OutputDir)
		name := r.pathManager.GetChunkAudioPath(meetingID, id, "wav")
		f, err := os.Create(name)
		if err != nil {
			return err
		}
		// 预写 44 字节占位
		_, _ = f.Write(make([]byte, 44))
		curFile = f
		curSamples = 0
		chunkStart = time.Now()
		return nil
	}

	finalizeChunk := func(id int, start time.Time) {
		if curFile == nil {
			return
		}
		actualSamples := curSamples // 不补零
		dataSize := actualSamples * bytesPerSample * channels
		writeWavHeader(curFile, sampleRate, channels, bitsPerSample, dataSize)
		curFile.Close()
		endTime := start.Add(time.Duration(actualSamples) * time.Second / sampleRate)
		ratio := float64(actualSamples) / float64(expectedPerChunkSamples)
		log.Printf("[REC] continuous finalize chunk=%04d samples=%d expected=%d ratio=%.4f", id, actualSamples, expectedPerChunkSamples, ratio)
		// 推入队列
		meetingID := filepath.Base(r.cfg.OutputDir)
		name := r.pathManager.GetChunkAudioPath(meetingID, id, "wav")
		log.Printf("[ASR Queue] Pushing finalized chunk to queue: ID=%04d, Path=%s", id, name)
		r.asrQueue.Push(AudioChunk{ID: id, Path: name, StartTime: start, EndTime: endTime})
		log.Printf("[ASR Queue] Finalized chunk pushed successfully")
		curFile = nil
	}

	if err := openChunk(r.chunkID); err != nil {
		log.Printf("[REC] open first chunk err: %v", err)
		_ = cmd.Process.Kill()
		return
	}

	for {
		if r.stopFlag.Load() {
			finalizeChunk(r.chunkID, chunkStart)
			_ = cmd.Process.Kill()
			break
		}
		n, readErr := stdout.Read(buf)
		if n > 0 && curFile != nil {
			if _, err := curFile.Write(buf[:n]); err != nil {
				log.Printf("[REC] write err: %v", err)
				_ = cmd.Process.Kill()
				break
			}
			curSamples += n / bytesPerSample / channels
			if curSamples >= expectedPerChunkSamples { // 达到设定样本数 -> 切片
				finalizeChunk(r.chunkID, chunkStart)
				if r.stopFlag.Load() {
					_ = cmd.Process.Kill()
					break
				}
				r.chunkID++
				if err := openChunk(r.chunkID); err != nil {
					log.Printf("[REC] open next chunk err: %v", err)
					_ = cmd.Process.Kill()
					break
				}
			}
		}
		if readErr != nil { // 设备结束
			if readErr == io.EOF {
				log.Printf("[REC] continuous EOF")
			} else {
				log.Printf("[REC] continuous read err: %v", readErr)
			}
			finalizeChunk(r.chunkID, chunkStart)
			break
		}
	}
	if err := cmd.Wait(); err != nil {
		log.Printf("[REC] continuous ffmpeg wait err: %v", err)
	}
	log.Printf("[REC] continuous mode end")
	r.asrQueue.Close()
}

// writeWavHeader rewrites WAV header at file beginning.
func writeWavHeader(f *os.File, sampleRate, channels, bitsPerSample, dataSize int) error {
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}
	byteRate := sampleRate * channels * bitsPerSample / 8
	blockAlign := channels * bitsPerSample / 8
	chunkSize := 36 + dataSize
	// RIFF header
	f.Write([]byte("RIFF"))
	binary.Write(f, binary.LittleEndian, uint32(chunkSize))
	f.Write([]byte("WAVE"))
	// fmt chunk
	f.Write([]byte("fmt "))
	binary.Write(f, binary.LittleEndian, uint32(16))            // Subchunk1Size
	binary.Write(f, binary.LittleEndian, uint16(1))             // PCM
	binary.Write(f, binary.LittleEndian, uint16(channels))      // NumChannels
	binary.Write(f, binary.LittleEndian, uint32(sampleRate))    // SampleRate
	binary.Write(f, binary.LittleEndian, uint32(byteRate))      // ByteRate
	binary.Write(f, binary.LittleEndian, uint16(blockAlign))    // BlockAlign
	binary.Write(f, binary.LittleEndian, uint16(bitsPerSample)) // BitsPerSample
	// data chunk
	f.Write([]byte("data"))
	binary.Write(f, binary.LittleEndian, uint32(dataSize))
	return nil
}

// worker helpers
func (o *Orchestrator) asrWorker(ctx context.Context) {
	log.Printf("[ASR Worker] Starting ASR worker goroutine")
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		log.Printf("[ASR Worker] ASR worker goroutine started, waiting for chunks...")
		for {
			log.Printf("[ASR Worker] Attempting to pop from queue...")
			chunk, ok := o.asrQ.Pop()
			if !ok {
				log.Printf("[ASR Worker] Queue closed, exiting worker")
				break
			}
			log.Printf("[ASR Worker] Got chunk from queue: ID=%d, Path=%s", chunk.ID, chunk.Path)

			startTime := time.Now()

			// 日志:开始处理
			logger.LogAudioProcessing(slog.Default(), "asr", "start", chunk.ID, 0, "")

			// 【核心变更】通过降级控制器获取当前Transcriber
			transcriber := o.degradationController.GetTranscriber()
			transcriberName := transcriber.Name()

			// 构造TranscribeOptions
			opts := &whisper.TranscribeOptions{
				Model:       o.cfg.WhisperModel,
				Language:    "", // 自动检测
				Temperature: o.cfg.WhisperTemperature,
				Prompt:      "",
				Timeout:     10 * time.Minute,
			}

			// 调用Transcriber.Transcribe()
			result, err := transcriber.Transcribe(ctx, chunk.Path, opts)
			duration := time.Since(startTime)
			durationMs := duration.Milliseconds()

			if err != nil {
				// 记录错误
				log.Printf("[ASR] transcribe error (transcriber=%s): %v", transcriberName, err)

				// 日志:错误
				errorCode := fmt.Sprintf("WHISPER_ERROR_%s", strings.ToUpper(transcriberName))
				logger.LogAudioProcessing(slog.Default(), "asr", "error", chunk.ID, durationMs, errorCode)

				// 指标:失败
				metrics.RecordChunkProcessed("asr", false)
				metrics.RecordError("asr", "whisper_"+transcriberName)
				continue
			}

			// 将结果写入segments.json文件 (保持原有格式兼容)
			meetingID := filepath.Base(o.cfg.OutputDir)
			segPath := o.pathManager.GetChunkSegmentsPath(meetingID, chunk.ID)
			if err := writeSegmentsJSON(segPath, result); err != nil {
				log.Printf("[ASR] write segments error: %v", err)
				logger.LogAudioProcessing(slog.Default(), "asr", "error", chunk.ID, durationMs, "WRITE_SEGMENTS_ERROR")
				metrics.RecordChunkProcessed("asr", false)
				continue
			}

			// 日志:成功 (带transcriber名称和降级状态)
			log.Printf("[ASR] transcribe success (transcriber=%s, degraded=%v)", transcriberName, o.degradationController.IsDegraded())
			logger.LogAudioProcessing(slog.Default(), "asr", "success", chunk.ID, durationMs, "")

			// 指标:成功
			metrics.RecordChunkProcessed("asr", true)
			metrics.RecordDuration("asr", duration.Seconds())

			// 推送到 SD 队列进行说话人分离
			log.Printf("[ASR Worker] Pushing chunk %d to SD queue for speaker diarization", chunk.ID)
			o.sdQ.Push(ASRResult{Chunk: chunk, SegJSON: segPath})
		}
		o.sdQ.Close()
	}()
}

// writeSegmentsJSON 将TranscriptionResult写入JSON文件 (保持原有格式兼容)
func writeSegmentsJSON(path string, result *whisper.TranscriptionResult) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func (o *Orchestrator) sdWorker(ctx context.Context) {
	log.Printf("[SD Worker] Starting Speaker Diarization worker goroutine")
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		log.Printf("[SD Worker] SD worker goroutine started, waiting for items...")
		for {
			log.Printf("[SD Worker] Attempting to pop from queue...")
			item, ok := o.sdQ.Pop()
			if !ok {
				log.Printf("[SD Worker] Queue closed, exiting worker")
				break
			}
			log.Printf("[SD Worker] Got item from queue: Chunk ID=%d", item.Chunk.ID)
			meetingID := filepath.Base(o.cfg.OutputDir)
			speakersPath := o.pathManager.GetChunkSpeakersPath(meetingID, item.Chunk.ID)

			// Use DependencyClient for diarization (supports local/remote/fallback modes)
			if o.dependencyClient != nil {
				// ========== File Sharing: Ensure audio file is in shared volume ==========
				audioPath := item.Chunk.Path
				sharedVolume := o.cfg.DependencySharedVolume

				// Check if audio file is within shared volume
				if sharedVolume != "" && !strings.HasPrefix(audioPath, sharedVolume) {
					// File is outside shared volume, need to copy it
					pm := o.dependencyClient.PathManager()
					meetingID := filepath.Base(filepath.Dir(audioPath)) // Extract meeting ID from path
					if meetingID == "." || meetingID == "/" {
						meetingID = fmt.Sprintf("meeting_%d", item.Chunk.ID/100) // Fallback: group by 100 chunks
					}

					// Construct target path in shared volume
					chunkFilename := filepath.Base(audioPath)
					targetPath := pm.GetAudioPath(meetingID, chunkFilename)

					// Ensure meeting directory exists
					if _, err := pm.EnsureMeetingDir(meetingID); err != nil {
						slog.Error("[SD] failed to create shared volume directory",
							"chunk_id", item.Chunk.ID,
							"meeting_id", meetingID,
							"error", err.Error(),
						)
						continue
					}

					// Copy file to shared volume
					if err := copyFile(audioPath, targetPath); err != nil {
						slog.Error("[SD] failed to copy audio file to shared volume",
							"chunk_id", item.Chunk.ID,
							"src", audioPath,
							"dst", targetPath,
							"error", err.Error(),
						)
						continue
					}

					slog.Info("[SD] copied audio file to shared volume",
						"chunk_id", item.Chunk.ID,
						"src", audioPath,
						"dst", targetPath,
					)

					// Update audio path to point to shared volume
					audioPath = targetPath
				}
				// ========== End File Sharing ==========

				// Transform path for deps-service container if needed
				// Note: After volume mount fix, both containers use /app/data, so no transformation needed
				audioPathForDeps := audioPath
				// Previously: unified: /app/data/meetings/... -> deps-service: /data/meetings/...
				// Now: Both use /app/data/meetings/... (transformation not needed)

				// Execute diarization via DependencyClient high-level API
				// Use 30 minutes timeout to allow model download on first run
				diarizationCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
				defer cancel() // Prepare diarization options
				opts := &dependency.DiarizationOptions{
					Device:        o.cfg.DeviceDefault,
					EnableOffline: o.cfg.EnableOffline,
					// Note: HFToken will be read from environment in deps-service
					// No need to pass it here for security reasons
				}

				// Use RunDiarization high-level API which handles script path internally
				err := o.dependencyClient.RunDiarization(diarizationCtx, audioPathForDeps, speakersPath, opts)
				if err != nil {
					slog.Error("[SD] diarization failed via DependencyClient",
						"chunk_id", item.Chunk.ID,
						"error", err.Error(),
					)
					continue
				}

				slog.Info("[SD] diarization completed successfully",
					"chunk_id", item.Chunk.ID,
					"audio", audioPathForDeps,
					"output", speakersPath,
				)
			} else {
				// Fallback: direct Python script execution (legacy behavior)
				scriptPath := o.cfg.DiarizationScriptPath
				if scriptPath == "" {
					scriptPath = "/app/scripts/pyannote_diarize.py"
				}

				args := []string{"python3", scriptPath, "--input", item.Chunk.Path, "--device", o.cfg.DeviceDefault}
				if o.cfg.EnableOffline {
					args = append(args, "--offline")
				}

				cmd := exec.CommandContext(ctx, args[0], args[1:]...)
				env := os.Environ()
				// Note: HUGGINGFACE_ACCESS_TOKEN should be set in environment
				// No need to append from config - let Python read from env
				if o.cfg.EnableOffline {
					env = append(env, "HF_HUB_OFFLINE=1")
				}
				cmd.Env = env
				cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
				stdout, err := cmd.StdoutPipe()
				if err != nil {
					log.Printf("[SD] pipe err: %v", err)
					continue
				}
				cmd.Stderr = os.Stderr
				if err := cmd.Start(); err != nil {
					log.Printf("[SD] start err: %v", err)
					continue
				}
				f, _ := os.Create(speakersPath)
				w := bufio.NewWriter(f)
				_, _ = io.Copy(w, stdout)
				w.Flush()
				f.Close()
				if err := cmd.Wait(); err != nil {
					log.Printf("[SD] wait err: %v", err)
					continue
				}
			}

			// Post-process: clamp any segment ends beyond wav duration
			if err := sanitizeSpeakersJSON(speakersPath, item.Chunk.Path); err != nil {
				log.Printf("[SD][sanitize] error: %v", err)
			}

			// 推送到 EMB 队列进行嵌入处理
			log.Printf("[SD Worker] Pushing chunk %d to EMB queue for embedding extraction", item.Chunk.ID)
			o.embQ.Push(SDResult{Chunk: item.Chunk, SegJSON: item.SegJSON, SpeakersJSON: speakersPath})
		}
		o.embQ.Close()
	}()
}

func (o *Orchestrator) embeddingWorker(ctx context.Context) {
	log.Printf("[EMB Worker] Starting Embedding worker goroutine")
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		log.Printf("[EMB Worker] EMB worker goroutine started, waiting for items...")
		for {
			log.Printf("[EMB Worker] Attempting to pop from queue...")
			item, ok := o.embQ.Pop()
			if !ok {
				log.Printf("[EMB Worker] Queue closed, exiting worker")
				break
			}
			log.Printf("[EMB Worker] Got item from queue: Chunk ID=%d", item.Chunk.ID)
			meetingID := filepath.Base(o.cfg.OutputDir)
			embPath := o.pathManager.GetChunkEmbeddingsPath(meetingID, item.Chunk.ID)

			// Get existing embeddings path for speaker continuity
			o.voicePrint.Mutex.Lock()
			existing := o.voicePrint.CurrentEmbPath
			o.voicePrint.Mutex.Unlock()

			// ========== File Sharing: Ensure audio and speakers files are in shared volume ==========
			audioPath := item.Chunk.Path
			speakersPath := item.SpeakersJSON
			sharedVolume := o.cfg.DependencySharedVolume

			// Check if audio file needs to be copied to shared volume
			if sharedVolume != "" && !strings.HasPrefix(audioPath, sharedVolume) {
				pm := o.dependencyClient.PathManager()
				meetingID := filepath.Base(filepath.Dir(audioPath))
				if meetingID == "." || meetingID == "/" {
					meetingID = fmt.Sprintf("meeting_%d", item.Chunk.ID/100)
				}

				chunkFilename := filepath.Base(audioPath)
				targetPath := pm.GetAudioPath(meetingID, chunkFilename)

				if _, err := pm.EnsureMeetingDir(meetingID); err != nil {
					slog.Error("[EMB] failed to create shared volume directory",
						"chunk_id", item.Chunk.ID,
						"meeting_id", meetingID,
						"error", err.Error(),
					)
					continue
				}

				if err := copyFile(audioPath, targetPath); err != nil {
					slog.Error("[EMB] failed to copy audio file to shared volume",
						"chunk_id", item.Chunk.ID,
						"src", audioPath,
						"dst", targetPath,
						"error", err.Error(),
					)
					continue
				}

				slog.Info("[EMB] copied audio file to shared volume",
					"chunk_id", item.Chunk.ID,
					"src", audioPath,
					"dst", targetPath,
				)

				audioPath = targetPath
			}

			// Check if speakers file needs to be copied to shared volume
			if sharedVolume != "" && !strings.HasPrefix(speakersPath, sharedVolume) {
				pm := o.dependencyClient.PathManager()
				meetingID := filepath.Base(filepath.Dir(speakersPath))
				if meetingID == "." || meetingID == "/" {
					meetingID = fmt.Sprintf("meeting_%d", item.Chunk.ID/100)
				}

				speakersFilename := filepath.Base(speakersPath)
				targetPath := pm.GetDiarizationPath(meetingID, speakersFilename)

				if _, err := pm.EnsureMeetingDir(meetingID); err != nil {
					slog.Error("[EMB] failed to create shared volume directory for speakers",
						"chunk_id", item.Chunk.ID,
						"meeting_id", meetingID,
						"error", err.Error(),
					)
					continue
				}

				if err := copyFile(speakersPath, targetPath); err != nil {
					slog.Error("[EMB] failed to copy speakers file to shared volume",
						"chunk_id", item.Chunk.ID,
						"src", speakersPath,
						"dst", targetPath,
						"error", err.Error(),
					)
					continue
				}

				slog.Info("[EMB] copied speakers file to shared volume",
					"chunk_id", item.Chunk.ID,
					"src", speakersPath,
					"dst", targetPath,
				)

				speakersPath = targetPath
			}
			// ========== End File Sharing ==========

			// Transform paths for deps-service container if needed
			// Note: After volume mount fix, both containers use /app/data, so no transformation needed
			audioPathForDeps := audioPath
			speakersPathForDeps := speakersPath
			embPathForDeps := embPath
			existingForDeps := existing
			// Previously: unified: /app/data/meetings/... -> deps-service: /data/meetings/...
			// Now: Both use /app/data/meetings/... (transformation not needed)

			slog.Info("[EMB] using paths for deps-service",
				"chunk_id", item.Chunk.ID,
				"audio", audioPathForDeps,
				"speakers", speakersPathForDeps,
				"output", embPathForDeps,
				"existing", existingForDeps,
			)

			// Construct EmbeddingOptions from config
			opts := &dependency.EmbeddingOptions{
				Device:             o.cfg.EmbeddingDeviceDefault,
				EnableOffline:      o.cfg.EnableOffline,
				HFToken:            o.cfg.HFTokenValue,
				Threshold:          o.cfg.EmbeddingThreshold,
				AutoLowerThreshold: true, // Always enable auto-lowering
				AutoLowerMin:       o.cfg.EmbeddingAutoLowerMin,
				AutoLowerStep:      o.cfg.EmbeddingAutoLowerStep,
				ExistingEmbeddings: existingForDeps, // Use path (no transformation needed)
			}

			log.Printf("[EMB] Calling DependencyClient.GenerateEmbeddings: audio=%s, speakers=%s, output=%s, device=%s, threshold=%s",
				audioPathForDeps, speakersPathForDeps, embPathForDeps, opts.Device, opts.Threshold) // Use DependencyClient to generate embeddings (use transformed paths)
			if err := o.dependencyClient.GenerateEmbeddings(ctx, audioPathForDeps, speakersPathForDeps, embPathForDeps, opts); err != nil {
				log.Printf("[EMB] error: %v", err)
				continue
			}

			// Update current embeddings path for next chunk (use original path for local file access)
			o.voicePrint.Mutex.Lock()
			o.voicePrint.CurrentEmbPath = embPath
			o.voicePrint.Mutex.Unlock() // 推送到 MERGE 队列进行最终合并
			log.Printf("[EMB Worker] Pushing chunk %d to MERGE queue for final merging", item.Chunk.ID)
			o.mergeQ.Push(EmbeddingResult{Chunk: item.Chunk, SegJSON: item.SegJSON, SpeakersJSON: item.SpeakersJSON, EmbeddingsJSON: embPath})
		}
		o.mergeQ.Close()
	}()
}

// mapping helpers (simplified; only local mapping for progress not required full functions) same as original
type SpeakersFile struct {
	Segments []struct {
		Start, End float64
		Speaker    string
	} `json:"segments"`
}

// sanitizeSpeakersJSON 读取 speakers json，基于 WAV 头部（仅解析固定 44 字节 PCM header）计算时长，裁剪超过音频时长的 end。
func sanitizeSpeakersJSON(jsonPath, wavPath string) error {
	f, err := os.Open(jsonPath)
	if err != nil {
		return err
	}
	defer f.Close()
	var data SpeakersFile
	if err := json.NewDecoder(f).Decode(&data); err != nil {
		return err
	}

	wf, err := os.Open(wavPath)
	if err != nil {
		return err
	}
	defer wf.Close()
	header := make([]byte, 44)
	if _, err := io.ReadFull(wf, header); err != nil {
		return err
	}
	if string(header[0:4]) != "RIFF" || string(header[8:12]) != "WAVE" {
		return fmt.Errorf("not wav")
	}
	sampleRate := int(binary.LittleEndian.Uint32(header[24:28]))
	bitsPerSample := int(binary.LittleEndian.Uint16(header[34:36]))
	if sampleRate <= 0 || bitsPerSample <= 0 {
		return fmt.Errorf("bad wav header")
	}
	fi, err := wf.Stat()
	if err != nil {
		return err
	}
	dataBytes := fi.Size() - 44
	bytesPerSample := bitsPerSample / 8
	if bytesPerSample <= 0 {
		return fmt.Errorf("bps <=0")
	}
	totalSamples := float64(dataBytes) / float64(bytesPerSample)
	dur := totalSamples / float64(sampleRate)
	tol := 0.05
	clipped := 0
	for i := range data.Segments {
		if data.Segments[i].End > dur+tol {
			data.Segments[i].End = dur
			if data.Segments[i].Start > data.Segments[i].End {
				data.Segments[i].Start = data.Segments[i].End - 0.01
			}
			clipped++
		}
	}
	if clipped > 0 {
		log.Printf("[SD][sanitize] clipped %d segment ends (>%.2fs) in %s", clipped, dur, filepath.Base(jsonPath))
		if err := writeSpeakersFile(jsonPath, &data); err != nil {
			return err
		}
	}
	return nil
}

func writeSpeakersFile(path string, sf *SpeakersFile) error {
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(sf); err != nil {
		f.Close()
		return err
	}
	f.Close()
	return os.Rename(tmp, path)
}

func applyLocalMapping(speakersPath, embeddingsPath string) (string, error) {
	f, err := os.Open(embeddingsPath)
	if err != nil {
		return speakersPath, err
	}
	defer f.Close()
	var raw map[string]any
	if err := json.NewDecoder(f).Decode(&raw); err != nil {
		return speakersPath, err
	}
	lom, ok := raw["local_original_mapping"].(map[string]any)
	if !ok {
		return speakersPath, nil
	}
	spFile, err := os.Open(speakersPath)
	if err != nil {
		return "", err
	}
	defer spFile.Close()
	var spData SpeakersFile
	if err := json.NewDecoder(spFile).Decode(&spData); err != nil {
		return "", err
	}
	repl := map[string]string{}
	for k, v := range lom {
		if vs, ok := v.(string); ok {
			repl[k] = vs
		}
	}
	for i := range spData.Segments {
		if newSpk, ok := repl[spData.Segments[i].Speaker]; ok {
			spData.Segments[i].Speaker = newSpk
		}
	}
	outPath := strings.TrimSuffix(speakersPath, ".json") + "_mapped.json"
	of, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer of.Close()
	enc := json.NewEncoder(of)
	enc.SetIndent("", "  ")
	if err := enc.Encode(spData); err != nil {
		return "", err
	}
	return outPath, nil
}

func applyGlobalMapping(speakersPath, embeddingsPath string) (string, error) {
	f, err := os.Open(embeddingsPath)
	if err != nil {
		return speakersPath, err
	}
	defer f.Close()
	var raw map[string]any
	if err := json.NewDecoder(f).Decode(&raw); err != nil {
		return speakersPath, err
	}
	mp, ok := raw["mapping"].(map[string]any)
	if !ok || len(mp) == 0 {
		return speakersPath, nil
	}
	spFile, err := os.Open(speakersPath)
	if err != nil {
		return speakersPath, err
	}
	defer spFile.Close()
	var spData SpeakersFile
	if err := json.NewDecoder(spFile).Decode(&spData); err != nil {
		return speakersPath, err
	}
	repl := map[string]string{}
	for k, v := range mp {
		if vs, ok := v.(string); ok {
			repl[k] = vs
		}
	}
	changed := false
	for i := range spData.Segments {
		if newSpk, ok := repl[spData.Segments[i].Speaker]; ok && newSpk != spData.Segments[i].Speaker {
			spData.Segments[i].Speaker = newSpk
			changed = true
		}
	}
	if !changed {
		return speakersPath, nil
	}
	outPath := strings.TrimSuffix(speakersPath, ".json") + "_global.json"
	of, err := os.Create(outPath)
	if err != nil {
		return speakersPath, err
	}
	defer of.Close()
	enc := json.NewEncoder(of)
	enc.SetIndent("", "  ")
	if err := enc.Encode(spData); err != nil {
		return speakersPath, err
	}
	return outPath, nil
}

func (o *Orchestrator) mergeWorker(ctx context.Context) {
	log.Printf("[MERGE Worker] Starting Merge worker goroutine")
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		log.Printf("[MERGE Worker] MERGE worker goroutine started, waiting for items...")
		for {
			log.Printf("[MERGE Worker] Attempting to pop from queue...")
			item, ok := o.mergeQ.Pop()
			if !ok {
				log.Printf("[MERGE Worker] Queue closed, exiting worker")
				break
			}
			log.Printf("[MERGE] chunk %04d merging (seg=%s spk=%s emb=%s)", item.Chunk.ID, filepath.Base(item.SegJSON), filepath.Base(item.SpeakersJSON), filepath.Base(item.EmbeddingsJSON))
			mapped := item.SpeakersJSON
			if p, err := applyLocalMapping(item.SpeakersJSON, item.EmbeddingsJSON); err == nil && p != "" {
				mapped = p
				log.Printf("[MERGE] chunk %04d local mapping -> %s", item.Chunk.ID, filepath.Base(mapped))
			}
			if p, err := applyGlobalMapping(mapped, item.EmbeddingsJSON); err == nil && p != "" {
				mapped = p
				log.Printf("[MERGE] chunk %04d global mapping -> %s", item.Chunk.ID, filepath.Base(mapped))
			}
			// Note: No dedicated PathManager method for merged.txt, using custom path
			meetingID := filepath.Base(o.cfg.OutputDir)
			mergedTxt := filepath.Join(o.pathManager.GetMeetingDir(meetingID), fmt.Sprintf("chunk_%04d_merged.txt", item.Chunk.ID))

			// Determine merge-segments binary path: prefer local bin/merge-segments for development
			mergeCmd := "merge-segments"
			if _, err := os.Stat("./bin/merge-segments"); err == nil {
				mergeCmd = "./bin/merge-segments"
			}

			args := []string{mergeCmd, "--segments-file", item.SegJSON, "--speaker-file", mapped}
			cmd := exec.CommandContext(ctx, args[0], args[1:]...)
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			outFile, _ := os.Create(mergedTxt)
			cmd.Stdout = outFile
			cmd.Stderr = os.Stderr
			_ = cmd.Run()
			outFile.Close()
			removeBlankLines(mergedTxt)
			log.Printf("[MERGE] chunk %04d merged -> %s", item.Chunk.ID, filepath.Base(mergedTxt))
		}
	}()
}

// New creates orchestrator (not started)
func New(cfg Config) *Orchestrator {
	os.MkdirAll(cfg.OutputDir, 0o755)
	cfg.ApplyRuntimeDefaults()

	// Initialize dependency client for external code
	depClient, err := initDependencyClient(cfg)
	if err != nil {
		log.Printf("[WARN] Failed to initialize dependency client: %v", err)
	}

	return &Orchestrator{
		cfg:              cfg,
		state:            StateCreated,
		asrQ:             NewSafeQueue[AudioChunk](8),
		sdQ:              NewSafeQueue[ASRResult](8),
		embQ:             NewSafeQueue[SDResult](8),
		mergeQ:           NewSafeQueue[EmbeddingResult](8),
		voicePrint:       &VoicePrintState{CurrentEmbPath: cfg.InitialEmbeddingsPath},
		startChunkID:     0,
		reprocess:        false,
		dependencyClient: depClient,
		pathManager:      depClient.PathManager(),
	}
}

// initDependencyClient initializes the dependency client based on configuration.
// It maps legacy config fields (FFmpegBinaryPath, PythonBinaryPath) to the new
// ExecutorConfig for backward compatibility.
func initDependencyClient(cfg Config) (*dependency.DependencyClient, error) {
	// Set defaults for backward compatibility
	mode := cfg.DependencyMode
	if mode == "" {
		mode = "local" // Default to local mode (backward compatible)
	}

	sharedVolume := cfg.DependencySharedVolume
	if sharedVolume == "" {
		// 首先尝试从环境变量获取
		sharedVolume = os.Getenv("DEPENDENCY_SHARED_VOLUME")
	}
	if sharedVolume == "" {
		// 【修复】PathManager 需要 baseDir (如 "/app/data"),而不是 meeting 目录
		// OutputDir 格式可能是:
		// - /app/data/meetings/{meeting_id}  (Docker)
		// - data/meetings/{meeting_id}        (本地相对路径)
		// - /path/to/AIDG/data/meetings/{meeting_id} (本地绝对路径)
		// 策略: 寻找包含 "meetings" 的父目录，如果找不到则使用 OutputDir 的父目录
		absOutputDir, err := filepath.Abs(cfg.OutputDir)
		if err != nil {
			absOutputDir = cfg.OutputDir
		}
		
		// 向上查找，直到找到包含 "meetings" 子目录的目录
		dir := absOutputDir
		for {
			parent := filepath.Dir(dir)
			if parent == dir || parent == "." || parent == "/" {
				// 到达文件系统根目录或无法继续，使用 data 的父目录
				// 假设: OutputDir 是 .../data/meetings/xxx
				sharedVolume = filepath.Dir(filepath.Dir(cfg.OutputDir))
				break
			}
			// 检查当前目录是否包含 meetings 子目录
			if filepath.Base(dir) == "meetings" {
				// 找到 meetings 目录，父目录就是我们要的 baseDir (例如 /app/data)
				sharedVolume = filepath.Dir(dir)
				break
			}
			dir = parent
		}
	}

	timeout := cfg.DependencyTimeout
	if timeout == 0 {
		timeout = 5 * time.Minute // Default 5 minutes
	}

	// Map legacy binary paths to new LocalBinaryPaths
	localBinaryPaths := make(map[string]string)
	if cfg.FFmpegBinaryPath != "" {
		localBinaryPaths["ffmpeg"] = cfg.FFmpegBinaryPath
	}
	if cfg.PythonBinaryPath != "" {
		localBinaryPaths["python"] = cfg.PythonBinaryPath
	}

	// Create executor config
	execConfig := dependency.ExecutorConfig{
		Mode:                  dependency.ExecutionMode(mode),
		ServiceURL:            cfg.DependencyServiceURL,
		SharedVolumePath:      sharedVolume,
		LocalBinaryPaths:      localBinaryPaths,
		DiarizationScriptPath: cfg.DiarizationScriptPath,
		EmbeddingScriptPath:   cfg.EmbeddingScriptPath,
		DefaultTimeout:        timeout,
		AllowedCommands:       []string{"ffmpeg", "ffprobe", "pyannote", "python"},
	}

	// Create dependency client
	client, err := dependency.NewClient(execConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dependency client: %w", err)
	}

	// Perform startup health check (non-blocking)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.HealthCheck(ctx); err != nil {
		slog.Warn("dependency availability check failed (service may degrade)",
			"error", err.Error(),
			"mode", mode)
		// Don't fail - just log warning and continue
	} else {
		slog.Info("dependency availability check passed",
			"mode", mode)
	}

	return client, nil
}

// ReprocessFromSegments enumerates existing chunk_XXXX.wav + chunk_XXXX_segments.json and pushes them to sd -> emb -> merge pipeline.
func (o *Orchestrator) ReprocessFromSegments() error {
	o.mutex.Lock()
	if o.state != StateCreated && o.state != StateStopped {
		o.mutex.Unlock()
		return fmt.Errorf("invalid state: %s", o.state)
	}
	o.reprocess = true
	o.state = StateRunning
	o.mutex.Unlock()

	// start downstream workers with cancellable context (skip recorder + asr worker)
	o.procCtx, o.procCancel = context.WithCancel(context.Background())
	o.sdWorker(o.procCtx)
	o.embeddingWorker(o.procCtx)
	o.mergeWorker(o.procCtx)

	// scan existing wav + segments, feed queue in order
	entries, err := os.ReadDir(o.cfg.OutputDir)
	if err != nil {
		return err
	}
	// remove old per-chunk merged outputs to avoid confusion with new run
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "chunk_") && strings.HasSuffix(e.Name(), "_merged.txt") {
			_ = os.Remove(filepath.Join(o.cfg.OutputDir, e.Name()))
		}
	}
	// NOTE: raw string literals: use single backslash for regex escape; previous version had `\\.` which matched a literal backslash + dot, so no files matched.
	reWav := regexp.MustCompile(`^chunk_([0-9]{4})\.wav$`)
	reSeg := regexp.MustCompile(`^chunk_([0-9]{4})_segments\.json$`)
	type pair struct {
		id       int
		wav, seg string
	}
	pm := map[int]*pair{}
	for _, e := range entries {
		name := e.Name()
		if m := reWav.FindStringSubmatch(name); m != nil {
			id, _ := strconv.Atoi(m[1])
			p := pm[id]
			if p == nil {
				p = &pair{id: id}
				pm[id] = p
			}
			p.wav = filepath.Join(o.cfg.OutputDir, name)
		}
		if m := reSeg.FindStringSubmatch(name); m != nil {
			id, _ := strconv.Atoi(m[1])
			p := pm[id]
			if p == nil {
				p = &pair{id: id}
				pm[id] = p
			}
			p.seg = filepath.Join(o.cfg.OutputDir, name)
		}
	}
	ids := []int{}
	for id, p := range pm {
		if p.wav != "" && p.seg != "" {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	log.Printf("[REPROCESS] found %d chunks with wav+segments", len(ids))
	go func() {
		for _, id := range ids {
			p := pm[id]
			chunk := AudioChunk{ID: id, Path: p.wav}
			log.Printf("[REPROCESS] enqueue chunk %04d", id)
			// Push directly into sdQ (simulate post-ASR output)
			o.sdQ.Push(ASRResult{Chunk: chunk, SegJSON: p.seg})
		}
		o.sdQ.Close()
	}()
	return nil
}

// PrepareResume scans existing output directory to set startChunkID and last embeddings path.
func (o *Orchestrator) PrepareResume() error {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	if o.state != StateCreated && o.state != StateStopped {
		return fmt.Errorf("cannot resume from state %s", o.state)
	}
	entries, err := os.ReadDir(o.cfg.OutputDir)
	if err != nil {
		return err
	}
	reWav := regexp.MustCompile(`^chunk_([0-9]{4})\.wav$`)
	reEmb := regexp.MustCompile(`^chunk_([0-9]{4})_embeddings\.json$`)
	maxID := -1
	lastEmb := ""
	lastEmbID := -1
	for _, e := range entries {
		name := e.Name()
		if m := reWav.FindStringSubmatch(name); m != nil {
			if v, _ := strconv.Atoi(m[1]); v > maxID {
				maxID = v
			}
		}
		if m := reEmb.FindStringSubmatch(name); m != nil {
			if v, _ := strconv.Atoi(m[1]); v > lastEmbID {
				lastEmbID = v
				lastEmb = filepath.Join(o.cfg.OutputDir, name)
			}
		}
	}
	o.startChunkID = maxID + 1
	if lastEmb != "" {
		o.voicePrint.Mutex.Lock()
		o.voicePrint.CurrentEmbPath = lastEmb
		o.voicePrint.Mutex.Unlock()
	}
	return nil
}

// InitForSingleASR initializes an ephemeral orchestrator for single ASR operations.
// This is a lightweight initialization that sets up transcribers and health checking
// without starting background workers.
func (o *Orchestrator) InitForSingleASR() error {
	log.Printf("[InitForSingleASR] START - current state: %s", o.state)
	o.mutex.Lock()
	defer o.mutex.Unlock()
	if o.state != StateCreated {
		log.Printf("[InitForSingleASR] Invalid state: %s (expected StateCreated)", o.state)
		return fmt.Errorf("invalid state for single ASR init: %s", o.state)
	}

	// Ensure runtime defaults are set and dependencies are available
	o.cfg.ApplyRuntimeDefaults()
	if err := o.cfg.ValidateCriticalDependencies(); err != nil {
		log.Printf("[InitForSingleASR] Dependency validation failed: %v", err)
		return err
	}
	log.Printf("[InitForSingleASR] Dependencies validated, creating transcribers...")

	// 1. 创建Primary Transcriber (same logic as Start method)
	var primaryTranscriber whisper.WhisperTranscriber
	whisperMode := strings.ToLower(o.cfg.WhisperMode)
	if whisperMode == "" {
		whisperMode = "http" // 默认HTTP模式
	}

	switch whisperMode {
	case "http", "go-whisper":
		apiURL := o.cfg.WhisperAPIURL
		if apiURL == "" {
			apiURL = "http://whisper:8082"
		}
		primaryTranscriber = whisper.NewGoWhisperImpl(apiURL)
		log.Printf("[Orchestrator] Using GoWhisper HTTP mode (API=%s)", apiURL)
	case "faster-whisper":
		// FasterWhisper也使用HTTP API
		apiURL := o.cfg.WhisperAPIURL
		if apiURL == "" {
			apiURL = "http://whisper:8082"
		}
		primaryTranscriber = whisper.NewGoWhisperImpl(apiURL)
		log.Printf("[Orchestrator] Using FasterWhisper HTTP mode (API=%s)", apiURL)
	case "cli", "local-whisper":
		// 使用本地CLI模式
		programPath := os.Getenv("WHISPER_PROGRAM_PATH")
		if programPath == "" {
			programPath = "/app/bin/whisper/whisper" // 默认值
		}
		// modelPath应该是模型文件目录，而不是模型文件名
		modelDir := "./models/whisper" // 模型文件所在目录
		var err error
		primaryTranscriber, err = whisper.NewLocalWhisperImpl(programPath, modelDir)
		if err != nil {
			return fmt.Errorf("failed to create LocalWhisperImpl: %w", err)
		}
		log.Printf("[Orchestrator] Using LocalWhisper CLI mode (program=%s, modelDir=%s)", programPath, modelDir)
	default:
		// 默认使用HTTP模式
		apiURL := o.cfg.WhisperAPIURL
		if apiURL == "" {
			apiURL = "http://whisper:8082"
		}
		primaryTranscriber = whisper.NewGoWhisperImpl(apiURL)
		log.Printf("[Orchestrator] WhisperMode unspecified, defaulting to GoWhisper HTTP (API=%s)", apiURL)
	}

	// 2. 创建Fallback Transcriber (MockTranscriber)
	fallbackTranscriber := whisper.NewMockTranscriber()
	log.Printf("[Orchestrator] Fallback transcriber: MockTranscriber (graceful degradation)")

	// 3. 创建HealthChecker
	checkInterval := o.cfg.HealthCheckInterval
	if checkInterval == 0 {
		checkInterval = 5 * time.Minute // 回退到默认值
	}
	failThreshold := o.cfg.HealthCheckFailThreshold
	if failThreshold == 0 {
		failThreshold = 3 // 回退到默认值
	}
	o.healthChecker = health.NewHealthChecker(primaryTranscriber, checkInterval, failThreshold)
	log.Printf("[Orchestrator] HealthChecker created (interval=%s, failThreshold=%d)", checkInterval, failThreshold)

	// 4. 创建DegradationController
	o.degradationController = degradation.NewDegradationController(
		primaryTranscriber,
		fallbackTranscriber,
		o.healthChecker,
	)
	log.Printf("[Orchestrator] DegradationController created for single ASR")

	// Note: We don't start the health checker or workers for single ASR operations
	o.state = StateRunning
	log.Printf("[InitForSingleASR] COMPLETE - state set to Running, degradationController: %v", o.degradationController != nil)
	return nil
}

// ensureDegradationController initializes the degradation controller if not already initialized.
// This is a helper method for RunSingleASR when working with existing orchestrators.
func (o *Orchestrator) ensureDegradationController() error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	if o.degradationController != nil {
		return nil // Already initialized
	}

	log.Printf("[ensureDegradationController] Initializing degradation controller...")

	// Ensure runtime defaults are set
	o.cfg.ApplyRuntimeDefaults()

	// 1. Create Primary Transcriber
	var primaryTranscriber whisper.WhisperTranscriber
	whisperMode := strings.ToLower(o.cfg.WhisperMode)
	if whisperMode == "" {
		whisperMode = "http"
	}

	switch whisperMode {
	case "http", "go-whisper":
		apiURL := o.cfg.WhisperAPIURL
		if apiURL == "" {
			apiURL = "http://whisper:8082"
		}
		primaryTranscriber = whisper.NewGoWhisperImpl(apiURL)
		log.Printf("[ensureDegradationController] Using GoWhisper HTTP mode (API=%s)", apiURL)
	case "faster-whisper":
		apiURL := o.cfg.WhisperAPIURL
		if apiURL == "" {
			apiURL = "http://whisper:8082"
		}
		primaryTranscriber = whisper.NewGoWhisperImpl(apiURL)
		log.Printf("[ensureDegradationController] Using FasterWhisper HTTP mode (API=%s)", apiURL)
	case "cli", "local-whisper":
		programPath := os.Getenv("WHISPER_PROGRAM_PATH")
		if programPath == "" {
			programPath = "/app/bin/whisper/whisper"
		}
		modelDir := "./models/whisper"
		var err error
		primaryTranscriber, err = whisper.NewLocalWhisperImpl(programPath, modelDir)
		if err != nil {
			return fmt.Errorf("failed to create LocalWhisperImpl: %w", err)
		}
		log.Printf("[ensureDegradationController] Using LocalWhisper CLI mode (program=%s, modelDir=%s)", programPath, modelDir)
	default:
		apiURL := o.cfg.WhisperAPIURL
		if apiURL == "" {
			apiURL = "http://whisper:8082"
		}
		primaryTranscriber = whisper.NewGoWhisperImpl(apiURL)
		log.Printf("[ensureDegradationController] WhisperMode unspecified, defaulting to GoWhisper HTTP (API=%s)", apiURL)
	}

	// 2. Create Fallback Transcriber
	fallbackTranscriber := whisper.NewMockTranscriber()

	// 3. Create HealthChecker
	checkInterval := o.cfg.HealthCheckInterval
	if checkInterval == 0 {
		checkInterval = 5 * time.Minute
	}
	failThreshold := o.cfg.HealthCheckFailThreshold
	if failThreshold == 0 {
		failThreshold = 3
	}
	o.healthChecker = health.NewHealthChecker(primaryTranscriber, checkInterval, failThreshold)

	// 4. Create DegradationController
	o.degradationController = degradation.NewDegradationController(
		primaryTranscriber,
		fallbackTranscriber,
		o.healthChecker,
	)
	log.Printf("[ensureDegradationController] DegradationController created successfully")

	return nil
}

func (o *Orchestrator) Start() error {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	if o.state != StateCreated && o.state != StateStopped {
		return fmt.Errorf("invalid state: %s", o.state)
	}

	// Ensure runtime defaults are set and dependencies are available before
	// launching any external processes.
	o.cfg.ApplyRuntimeDefaults()
	if err := o.cfg.ValidateCriticalDependencies(); err != nil {
		return err
	}

	// 1. 创建Primary Transcriber
	var primaryTranscriber whisper.WhisperTranscriber
	whisperMode := strings.ToLower(o.cfg.WhisperMode)
	if whisperMode == "" {
		whisperMode = "http" // 默认HTTP模式
	}

	switch whisperMode {
	case "http", "go-whisper":
		apiURL := o.cfg.WhisperAPIURL
		if apiURL == "" {
			apiURL = "http://whisper:8082"
		}
		primaryTranscriber = whisper.NewGoWhisperImpl(apiURL)
		log.Printf("[Orchestrator] Using GoWhisper HTTP mode (API=%s)", apiURL)
	case "faster-whisper":
		// FasterWhisper也使用HTTP API
		apiURL := o.cfg.WhisperAPIURL
		if apiURL == "" {
			apiURL = "http://whisper:8082"
		}
		primaryTranscriber = whisper.NewGoWhisperImpl(apiURL)
		log.Printf("[Orchestrator] Using FasterWhisper HTTP mode (API=%s)", apiURL)
	case "cli", "local-whisper":
		// 使用本地CLI模式
		programPath := os.Getenv("WHISPER_PROGRAM_PATH")
		if programPath == "" {
			programPath = "/app/bin/whisper/whisper" // 默认值
		}
		modelPath := o.cfg.WhisperModel
		if modelPath == "" {
			modelPath = "ggml-base.bin"
		}
		var err error
		primaryTranscriber, err = whisper.NewLocalWhisperImpl(programPath, modelPath)
		if err != nil {
			return fmt.Errorf("failed to create LocalWhisperImpl: %w", err)
		}
		log.Printf("[Orchestrator] Using LocalWhisper CLI mode (program=%s, model=%s)", programPath, modelPath)
	default:
		// 默认使用HTTP模式
		apiURL := o.cfg.WhisperAPIURL
		if apiURL == "" {
			apiURL = "http://whisper:8082"
		}
		primaryTranscriber = whisper.NewGoWhisperImpl(apiURL)
		log.Printf("[Orchestrator] WhisperMode unspecified, defaulting to GoWhisper HTTP (API=%s)", apiURL)
	}

	// 2. 创建Fallback Transcriber (MockTranscriber)
	fallbackTranscriber := whisper.NewMockTranscriber()
	log.Printf("[Orchestrator] Fallback transcriber: MockTranscriber (graceful degradation)")

	// 3. 创建HealthChecker (使用Config中的值)
	checkInterval := o.cfg.HealthCheckInterval
	if checkInterval == 0 {
		checkInterval = 5 * time.Minute // 回退到默认值
	}
	failThreshold := o.cfg.HealthCheckFailThreshold
	if failThreshold == 0 {
		failThreshold = 3 // 回退到默认值
	}
	o.healthChecker = health.NewHealthChecker(primaryTranscriber, checkInterval, failThreshold)
	log.Printf("[Orchestrator] HealthChecker created (interval=%s, failThreshold=%d)", checkInterval, failThreshold)

	// 仅在启用持续录制模式时启动 FFmpeg 录音
	if o.cfg.UseContinuous {
		o.recorder = NewRecorder(o.cfg, o.asrQ, &o.wg, o.pathManager)

		// 确定起始 chunk ID
		startID := 0
		if o.startChunkID > 0 {
			// 用户明确指定了起始 ID (reprocess 模式)
			startID = o.startChunkID
		} else if o.nextChunkID > 0 {
			// 从上次停止的位置恢复
			startID = o.nextChunkID
			log.Printf("[Orchestrator] Resuming from previous session, nextChunkID=%d", startID)
		} else {
			// 检测目录中已有的最大 chunk ID
			maxID := o.detectMaxChunkID()
			if maxID >= 0 {
				startID = maxID + 1
				log.Printf("[Orchestrator] Detected existing chunks, starting from chunkID=%d", startID)
			}
		}

		o.recorder.chunkID = startID
		log.Printf("[Orchestrator] Recorder initialized with chunkID=%d", startID)
		o.recorder.Start()
	}

	o.procCtx, o.procCancel = context.WithCancel(context.Background())

	// 4. 启动HealthChecker (使用procCtx) - 在goroutine中运行以避免阻塞
	go o.healthChecker.Start(o.procCtx)
	log.Printf("[Orchestrator] HealthChecker started")

	// 5. 创建DegradationController
	o.degradationController = degradation.NewDegradationController(
		primaryTranscriber,
		fallbackTranscriber,
		o.healthChecker,
	)
	log.Printf("[Orchestrator] DegradationController created")

	// 启动所有处理队列的 worker
	log.Printf("[Orchestrator] Starting all workers...")
	o.asrWorker(o.procCtx)
	o.sdWorker(o.procCtx)
	o.embeddingWorker(o.procCtx)
	o.mergeWorker(o.procCtx)
	log.Printf("[Orchestrator] All workers started (ASR -> SD -> EMB -> MERGE)")

	// 6. 启动 DependencyClient 定期健康检查 (仅 fallback 模式)
	if o.cfg.DependencyMode == "fallback" && o.dependencyClient != nil {
		o.healthTicker = time.NewTicker(5 * time.Minute)
		go o.healthCheckWorker()
		log.Printf("[Orchestrator] Dependency health check worker started (interval=5m)")
	}

	o.state = StateRunning
	return nil
}

func (o *Orchestrator) Stop() {
	o.mutex.Lock()
	if o.state != StateRunning {
		o.mutex.Unlock()
		return
	}
	o.state = StateStopping
	reproc := o.reprocess
	recorder := o.recorder
	cancel := o.procCancel
	healthChecker := o.healthChecker // 保存引用
	healthTicker := o.healthTicker   // 保存引用
	o.mutex.Unlock()

	// 停止健康检查器 (在停止其他组件之前)
	if healthChecker != nil {
		log.Printf("[Orchestrator] Stopping HealthChecker")
		healthChecker.Stop()
	}

	// 停止 DependencyClient 健康检查 Ticker
	if healthTicker != nil {
		log.Printf("[Orchestrator] Stopping dependency health check ticker")
		healthTicker.Stop()
	}

	if reproc { // 重处理：立即终止所有外部进程并放弃剩余队列
		if cancel != nil {
			cancel()
		}
		// 直接关闭所有队列，worker 会尽快退出
		o.asrQ.Close()
		o.sdQ.Close()
		o.embQ.Close()
		o.mergeQ.Close()
		go func() {
			o.wg.Wait()
			o.mutex.Lock()
			o.state = StateStopped
			o.mutex.Unlock()
		}()
		return
	}
	// 录制模式：停止当前 ffmpeg，保留部分 wav 并继续处理队列
	if recorder != nil {
		// 保存下一个 chunk ID 供下次启动使用
		o.nextChunkID = recorder.chunkID
		log.Printf("[Orchestrator] Saved nextChunkID=%d for next start", o.nextChunkID)
		recorder.FinalizeAndStop()
	}
	go func() {
		o.wg.Wait()
		o.mutex.Lock()
		_, _ = o.ConcatAllMerged()

		o.mutex.Lock()
		o.state = StateStopped
		o.mutex.Unlock()
	}()
}

// detectMaxChunkID 扫描输出目录,返回已存在的最大 chunk ID
// 如果没有找到任何 chunk 文件,返回 -1
func (o *Orchestrator) detectMaxChunkID() int {
	maxID := -1

	// 检查输出目录是否存在
	if _, err := os.Stat(o.cfg.OutputDir); os.IsNotExist(err) {
		return maxID
	}

	// 扫描目录中的所有文件
	entries, err := os.ReadDir(o.cfg.OutputDir)
	if err != nil {
		log.Printf("[Orchestrator] Failed to read output directory: %v", err)
		return maxID
	}

	// 匹配 chunk_XXXX.wav 格式的文件
	chunkPattern := regexp.MustCompile(`^chunk_(\d{4})\.wav$`)

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		matches := chunkPattern.FindStringSubmatch(entry.Name())
		if matches != nil {
			chunkID, err := strconv.Atoi(matches[1])
			if err == nil && chunkID > maxID {
				maxID = chunkID
			}
		}
	}

	return maxID
}

// EnqueueAudioChunk 将外部上传的音频文件推入转录队列
// 用于浏览器录音或文件上传场景
func (o *Orchestrator) EnqueueAudioChunk(chunkID int, wavPath string) {
	o.mutex.Lock()
	state := o.state
	o.mutex.Unlock()

	// 【修复】如果 Orchestrator 从未启动(StateCreated)，自动调用 Start()
	// 这样支持纯文件上传模式（无需手动调用 Start 按钮）
	if state == StateCreated {
		fmt.Printf("[AUDIO] Orchestrator not started, auto-starting for file upload mode...\n")
		if err := o.Start(); err != nil {
			fmt.Printf("[AUDIO] Failed to auto-start orchestrator: %v\n", err)
			return
		}
		// 重新获取状态
		o.mutex.Lock()
		state = o.state
		o.mutex.Unlock()
		fmt.Printf("[AUDIO] Orchestrator auto-started, state=%s\n", state)
	}

	// 允许在运行、停止中、排空中状态下接受音频chunk
	// 这样可以处理停止按钮触发后仍在上传的最后几个chunk
	if state != StateRunning && state != StateStopping && state != StateDraining {
		fmt.Printf("[AUDIO] Cannot enqueue chunk %d: orchestrator state=%s\n", chunkID, state)
		return
	}

	// 获取文件信息
	fileInfo, err := os.Stat(wavPath)
	if err != nil {
		fmt.Printf("[AUDIO] Cannot stat file %s: %v\n", wavPath, err)
		return
	}

	// 创建 AudioChunk 并推入队列
	chunk := AudioChunk{
		ID:        chunkID,
		Path:      wavPath,
		StartTime: fileInfo.ModTime(), // 使用文件修改时间作为开始时间
		EndTime:   time.Now(),         // 当前时间作为结束时间
	}

	fmt.Printf("[AUDIO] Enqueuing chunk %d for transcription: %s\n", chunkID, wavPath)
	o.asrQ.Push(chunk)
}

// GetDegradationController 返回降级控制器实例 (用于API访问)
func (o *Orchestrator) GetDegradationController() *degradation.DegradationController {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	return o.degradationController
}

func (o *Orchestrator) GetDependencyClient() *dependency.DependencyClient {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	return o.dependencyClient
}

// GetState returns the current state of the orchestrator
func (o *Orchestrator) GetState() State {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	return o.state
}

// EnqueueExistingChunks 扫描并推送指定范围的 chunk 文件到 ASR 队列
// 用于音频文件上传后的批量处理
func (o *Orchestrator) EnqueueExistingChunks(startChunkID, endChunkID int) error {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	if o.state != StateRunning {
		return fmt.Errorf("orchestrator is not running (state=%s)", o.state)
	}

	meetingID := filepath.Base(o.cfg.OutputDir)
	log.Printf("[Orchestrator] Enqueuing existing chunks: %d to %d", startChunkID, endChunkID-1)

	for chunkID := startChunkID; chunkID < endChunkID; chunkID++ {
		wavPath := o.pathManager.GetChunkAudioPath(meetingID, chunkID, "wav")

		// 检查文件是否存在
		fileInfo, err := os.Stat(wavPath)
		if err != nil {
			log.Printf("[Orchestrator] Chunk file not found, skipping: %s (error: %v)", wavPath, err)
			continue
		}

		chunk := AudioChunk{
			ID:        chunkID,
			Path:      wavPath,
			StartTime: fileInfo.ModTime(),
			EndTime:   time.Now(),
		}

		log.Printf("[Orchestrator] Enqueuing chunk %d: %s", chunkID, wavPath)
		o.asrQ.Push(chunk)
	}

	log.Printf("[Orchestrator] Enqueued %d chunks for processing", endChunkID-startChunkID)
	return nil
}

// GetHealthChecker 返回健康检查器实例 (用于API访问)
func (o *Orchestrator) GetHealthChecker() *health.HealthChecker {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	return o.healthChecker
}

// healthCheckWorker periodically checks dependency client availability (fallback mode only).
// Runs in background goroutine, exits when procCtx is cancelled.
func (o *Orchestrator) healthCheckWorker() {
	log.Printf("[HealthCheckWorker] Starting periodic dependency availability check")
	defer log.Printf("[HealthCheckWorker] Exiting")

	for {
		select {
		case <-o.healthTicker.C:
			// Perform health check with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := o.dependencyClient.HealthCheck(ctx)
			cancel()

			if err != nil {
				slog.Warn("dependency client health check failed (degradation may occur)",
					"error", err.Error())
				// Note: Do not trigger alerts here - let FallbackExecutor handle degradation
				// automatically on next command execution.
			} else {
				slog.Info("dependency client health check passed",
					"mode", o.cfg.DependencyMode)
			}

		case <-o.procCtx.Done():
			// Orchestrator is stopping, exit health check loop
			return
		}
	}
}

// Scan progress
func (o *Orchestrator) Progress() ProgressInfo {
	o.mutex.Lock()
	st := o.state
	o.mutex.Unlock()
	counts := map[string]int{"wav": 0, "segments": 0, "speakers": 0, "speakers_mapped": 0, "merged": 0}
	entries, _ := os.ReadDir(o.cfg.OutputDir)
	reSeg := regexp.MustCompile(`^chunk_([0-9]{4})_segments\.json$`)
	reWav := regexp.MustCompile(`^chunk_([0-9]{4})\.wav$`)
	reSpk := regexp.MustCompile(`^chunk_([0-9]{4})_speakers\.json$`)
	reSpkM := regexp.MustCompile(`^chunk_([0-9]{4})_speakers_mapped\.json$`)
	reMerged := regexp.MustCompile(`^chunk_([0-9]{4})_merged\.txt$`)
	maxChunk := -1
	lastEmb := ""
	for _, e := range entries {
		name := e.Name()
		if m := reWav.FindStringSubmatch(name); m != nil {
			counts["wav"]++
			if v, _ := strconv.Atoi(m[1]); v > maxChunk {
				maxChunk = v
			}
		}
		if reSeg.MatchString(name) {
			counts["segments"]++
		}
		if reSpk.MatchString(name) {
			counts["speakers"]++
		}
		if reSpkM.MatchString(name) {
			counts["speakers_mapped"]++
		}
		if reMerged.MatchString(name) {
			counts["merged"]++
		}
		if strings.HasSuffix(name, "_embeddings.json") {
			lastEmb = name
		}
	}
	return ProgressInfo{State: st, CurrentChunk: maxChunk, Files: counts, LastEmbeddings: lastEmb, UpdatedAt: time.Now()}
}

// ConcatAllMerged concatenates all chunk_*_merged.txt into merged_all.txt with headers.
func (o *Orchestrator) ConcatAllMerged() (string, error) {
	outPath := filepath.Join(o.cfg.OutputDir, "merged_all.txt")
	pattern := regexp.MustCompile(`^chunk_([0-9]{4})_merged\.txt$`)
	entries, err := os.ReadDir(o.cfg.OutputDir)
	if err != nil {
		return "", err
	}
	type item struct {
		id   int
		name string
	}
	arr := []item{}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if m := pattern.FindStringSubmatch(name); m != nil {
			v, _ := strconv.Atoi(m[1])
			arr = append(arr, item{v, name})
		}
	}
	if len(arr) == 0 {
		return outPath, nil
	}
	// sort
	sort.Slice(arr, func(i, j int) bool { return arr[i].id < arr[j].id })
	f, err := os.Create(outPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	writer := bufio.NewWriter(f)
	startTime := time.Time{}
	for _, it := range arr {
		chunkID := it.id
		// attempt to find original chunk wav for relative time; not storing time so just show chunk index
		header := fmt.Sprintf("===== Chunk %04d =====\n", chunkID)
		writer.WriteString(header)
		mf, err := os.Open(filepath.Join(o.cfg.OutputDir, it.name))
		if err == nil {
			io.Copy(writer, mf)
			mf.Close()
		}
		writer.WriteString("\n")
		if startTime.IsZero() {
			startTime = time.Now()
		}
	}
	writer.Flush()
	return outPath, nil
}

// MergeOnly generates per-chunk merged files if missing and then concatenates.
func (o *Orchestrator) MergeOnly() (string, error) {
	// generate chunk_*_merged.txt for each chunk having segments & speaker json
	entries, err := os.ReadDir(o.cfg.OutputDir)
	if err != nil {
		return "", err
	}
	segRe := regexp.MustCompile(`^chunk_([0-9]{4})_segments\.json$`)
	spkPrefer := func(base string) string {
		// prefer mapped/global if available
		mapped := strings.Replace(base, "_speakers.json", "_speakers_mapped.json", 1)
		global := strings.Replace(base, "_speakers.json", "_speakers_mapped_global.json", 1)
		if _, err := os.Stat(filepath.Join(o.cfg.OutputDir, mapped)); err == nil {
			return mapped
		}
		if _, err := os.Stat(filepath.Join(o.cfg.OutputDir, global)); err == nil {
			return global
		}
		return base
	}
	haveSeg := map[int]string{}
	haveSpk := map[int]string{}
	haveMerged := map[int]bool{}
	mergedRe := regexp.MustCompile(`^chunk_([0-9]{4})_merged\.txt$`)
	spkRe := regexp.MustCompile(`^chunk_([0-9]{4})_speakers\.json$`)
	for _, e := range entries {
		name := e.Name()
		if m := segRe.FindStringSubmatch(name); m != nil {
			v, _ := strconv.Atoi(m[1])
			haveSeg[v] = name
		}
		if m := spkRe.FindStringSubmatch(name); m != nil {
			v, _ := strconv.Atoi(m[1])
			haveSpk[v] = spkPrefer(name)
		}
		if m := mergedRe.FindStringSubmatch(name); m != nil {
			v, _ := strconv.Atoi(m[1])
			haveMerged[v] = true
		}
	}
	for id, seg := range haveSeg {
		spk, ok := haveSpk[id]
		if !ok {
			continue
		}
		if haveMerged[id] {
			continue
		}
		outName := fmt.Sprintf("chunk_%04d_merged.txt", id)

		// Determine merge-segments binary path: prefer local bin/merge-segments for development
		mergeCmd := "merge-segments"
		if _, err := os.Stat("./bin/merge-segments"); err == nil {
			mergeCmd = "./bin/merge-segments"
		}

		args := []string{mergeCmd, "--segments-file", filepath.Join(o.cfg.OutputDir, seg), "--speaker-file", filepath.Join(o.cfg.OutputDir, spk)}
		cmd := exec.Command(args[0], args[1:]...)
		mf, _ := os.Create(filepath.Join(o.cfg.OutputDir, outName))
		cmd.Stdout = mf
		cmd.Stderr = os.Stderr
		_ = cmd.Run()
		mf.Close()
		removeBlankLines(filepath.Join(o.cfg.OutputDir, outName))
	}
	return o.ConcatAllMerged()
}
