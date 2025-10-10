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

	"github.com/houzhh15-hub/AIDG/cmd/server/internal/metrics"
	"github.com/houzhh15-hub/AIDG/pkg/logger"
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

// Config holds runtime adjustable parameters.
type Config struct {
	OutputDir             string
	RecordChunkDuration   time.Duration
	RecordOverlap         time.Duration
	UseContinuous         bool // true: 单进程持续捕获, 按墙钟切片
	FFmpegDeviceName      string
	WhisperMode           string // "http" (default) or "cli"
	WhisperAPIURL         string // Whisper HTTP API endpoint (default: "http://whisper:8082")
	WhisperModel          string
	WhisperSegments       string // e.g. "20s"; empty or "0" -> do not pass --segments
	DeviceDefault         string
	DiarizationBackend    string // "pyannote" (default) or "speechbrain"
	DiarizationScriptPath string // PyAnnote diarization script path (default: "/app/audio/diarization/pyannote_diarize.py")
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
	EmbeddingScriptPath    string // Speaker embedding generation script path (default: "/app/audio/diarization/generate_speaker_embeddings.py")
	EmbeddingDeviceDefault string
	EmbeddingThreshold     string
	EmbeddingAutoLowerMin  string
	EmbeddingAutoLowerStep string
	InitialEmbeddingsPath  string
	HFTokenValue           string
	EnableOffline          bool
	// Task metadata fields
	ProductLine string    `json:"product_line"` // 产品线
	MeetingTime time.Time `json:"meeting_time"` // 会议时间
}

// DefaultConfig returns sensible defaults matching original main.go constants.
func DefaultConfig() Config {
	// 从环境变量读取 Whisper API URL，如果未设置则使用默认值
	whisperURL := os.Getenv("WHISPER_API_URL")
	if whisperURL == "" {
		whisperURL = "http://whisper:80"
	}

	return Config{
		OutputDir:              "meeting_output",
		RecordChunkDuration:    5 * time.Minute,
		RecordOverlap:          5 * time.Second,
		UseContinuous:          false, // 禁用自动录音，使用文件上传模式
		FFmpegDeviceName:       "",    // Docker 环境中不使用音频设备
		WhisperMode:            "http",
		WhisperAPIURL:          whisperURL, // 从环境变量读取或使用默认值
		WhisperModel:           "ggml-large-v3",
		WhisperSegments:        "20s",
		DeviceDefault:          "mps",
		DiarizationBackend:     "pyannote",
		DiarizationScriptPath:  "/app/audio/diarization/pyannote_diarize.py",
		SBMinSpeakers:          1,
		SBMaxSpeakers:          8,
		SBOverclusterFactor:    1.4,
		SBMergeThreshold:       0.86,
		SBMinSegmentMerge:      0.8,
		SBReassignAfterMerge:   true,
		SBEnergyVAD:            true,
		SBEnergyVADThr:         0.5,
		EmbeddingScriptPath:    "/app/audio/diarization/generate_speaker_embeddings.py",
		EmbeddingDeviceDefault: "auto",
		EmbeddingThreshold:     "0.55",
		EmbeddingAutoLowerMin:  "0.45",
		EmbeddingAutoLowerStep: "0.02",
		InitialEmbeddingsPath:  "",
		HFTokenValue:           "hf_REPLACE_WITH_YOUR_TOKEN_HERE",
		EnableOffline:          true,
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
	reprocess    bool // reprocess mode flag (skip recorder/asr, feed existing wav+segments)
	procCtx      context.Context
	procCancel   context.CancelFunc
}

// RunSingleASR runs whisper once on an existing chunk wav with a provided model and segment length (in seconds string like "20s")
// It does not mutate orchestrator config. Returns path to generated segments json.
func (o *Orchestrator) RunSingleASR(ctx context.Context, chunkWav string, model string, segLen string) (string, error) {
	if model == "" {
		model = o.cfg.WhisperModel
	}
	// segLen logic: empty -> default 20s; "0" or "0s" -> disable segment flag
	disableSeg := false
	if segLen == "" {
		segLen = "20s"
	}
	sl := strings.ToLower(segLen)
	if sl == "0" || sl == "0s" {
		disableSeg = true
	}
	base := filepath.Base(chunkWav)
	m := regexp.MustCompile(`^chunk_([0-9]{4})\.wav$`).FindStringSubmatch(base)
	if m == nil {
		return "", fmt.Errorf("invalid chunk wav name: %s", base)
	}
	id := m[1]
	out := filepath.Join(o.cfg.OutputDir, fmt.Sprintf("chunk_%s_segments.json", id))
	args := []string{"go-whisper/whisper", "transcribe", model, chunkWav, "--format", "json"}
	if !disableSeg {
		args = append(args, "--segments", segLen)
	}
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return "", err
	}
	f, err := os.Create(out)
	if err != nil {
		return "", err
	}
	w := bufio.NewWriter(f)
	_, _ = io.Copy(w, stdout)
	w.Flush()
	f.Close()
	if err := cmd.Wait(); err != nil {
		return "", err
	}
	return out, nil
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
	// 使用base模型（对应容器中的 ggml-base.bin）
	modelName := "base"
	if o.cfg.WhisperModel != "" && o.cfg.WhisperModel != "ggml-large-v3" {
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

	segPath := filepath.Join(o.cfg.OutputDir, fmt.Sprintf("chunk_%04d_segments.json", chunk.ID))
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
	segPath := filepath.Join(o.cfg.OutputDir, fmt.Sprintf("chunk_%04d_segments.json", chunk.ID))
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
}

func NewRecorder(cfg Config, asrQueue *SafeQueue[AudioChunk], wg *sync.WaitGroup) *Recorder {
	ctx, cancel := context.WithCancel(context.Background())
	return &Recorder{cfg: cfg, ctx: ctx, cancel: cancel, asrQueue: asrQueue, wg: wg}
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
		audioFile := filepath.Join(r.cfg.OutputDir, fmt.Sprintf("chunk_%04d.wav", curID))
		start := time.Now()
		endPlanned := start.Add(r.cfg.RecordChunkDuration)
		// 方案A: 使用 -t 强制确保录制时长不被意外提前截断
		durSec := int(r.cfg.RecordChunkDuration.Seconds())
		cmd := exec.CommandContext(r.ctx, "ffmpeg", "-y", "-f", "avfoundation", "-i", fmt.Sprintf(":%s", r.cfg.FFmpegDeviceName), "-t", strconv.Itoa(durSec), "-ac", "1", "-ar", "16000", audioFile)
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
		if enqueue {
			r.asrQueue.Push(AudioChunk{ID: curID, Path: audioFile, StartTime: start, EndTime: endActual})
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
	cmd := exec.CommandContext(r.ctx, "ffmpeg", "-hide_banner", "-loglevel", "error", "-f", "avfoundation", "-i", fmt.Sprintf(":%s", r.cfg.FFmpegDeviceName), "-ac", "1", "-ar", "16000", "-f", "s16le", "-use_wallclock_as_timestamps", "1", "-")
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
		name := filepath.Join(r.cfg.OutputDir, fmt.Sprintf("chunk_%04d.wav", id))
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
		name := filepath.Join(r.cfg.OutputDir, fmt.Sprintf("chunk_%04d.wav", id))
		r.asrQueue.Push(AudioChunk{ID: id, Path: name, StartTime: start, EndTime: endTime})
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
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		for {
			chunk, ok := o.asrQ.Pop()
			if !ok {
				break
			}

			startTime := time.Now()
			var segPath string
			var err error

			// 日志：开始处理
			logger.LogAudioProcessing(slog.Default(), "asr", "start", chunk.ID, 0, "")

			// 根据 WhisperMode 选择转录方式
			if o.cfg.WhisperMode == "http" {
				segPath, err = o.transcribeViaHTTP(ctx, chunk)
			} else {
				segPath, err = o.transcribeViaCLI(ctx, chunk)
			}

			duration := time.Since(startTime)
			durationMs := duration.Milliseconds()

			if err != nil {
				// 记录错误
				log.Printf("[ASR] transcribe error (mode=%s): %v", o.cfg.WhisperMode, err)

				// 日志：错误
				errorCode := "WHISPER_HTTP_ERROR"
				if o.cfg.WhisperMode != "http" {
					errorCode = "WHISPER_CLI_ERROR"
				}
				logger.LogAudioProcessing(slog.Default(), "asr", "error", chunk.ID, durationMs, errorCode)

				// 指标：失败
				metrics.RecordChunkProcessed("asr", false)
				metrics.RecordError("asr", "whisper_"+o.cfg.WhisperMode)
				continue
			}

			// 日志：成功
			logger.LogAudioProcessing(slog.Default(), "asr", "success", chunk.ID, durationMs, "")

			// 指标：成功
			metrics.RecordChunkProcessed("asr", true)
			metrics.RecordDuration("asr", duration.Seconds())

			o.sdQ.Push(ASRResult{Chunk: chunk, SegJSON: segPath})
		}
		o.sdQ.Close()
	}()
}

func (o *Orchestrator) sdWorker(ctx context.Context) {
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		for {
			item, ok := o.sdQ.Pop()
			if !ok {
				break
			}
			speakersPath := filepath.Join(o.cfg.OutputDir, fmt.Sprintf("chunk_%04d_speakers.json", item.Chunk.ID))

			// 使用配置中的 DiarizationScriptPath，默认 /app/audio/diarization/pyannote_diarize.py
			scriptPath := o.cfg.DiarizationScriptPath
			if scriptPath == "" {
				scriptPath = "/app/audio/diarization/pyannote_diarize.py"
			}

			args := []string{"python3", scriptPath, "--input", item.Chunk.Path, "--device", o.cfg.DeviceDefault}
			if o.cfg.EnableOffline {
				args = append(args, "--offline")
			}

			cmd := exec.CommandContext(ctx, args[0], args[1:]...)
			env := os.Environ()
			env = append(env, "HUGGINGFACE_TOKEN="+o.cfg.HFTokenValue)
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
			// Post-process: clamp any segment ends beyond wav duration
			if err := sanitizeSpeakersJSON(speakersPath, item.Chunk.Path); err != nil {
				log.Printf("[SD][sanitize] error: %v", err)
			}
			o.embQ.Push(SDResult{Chunk: item.Chunk, SegJSON: item.SegJSON, SpeakersJSON: speakersPath})
		}
		o.embQ.Close()
	}()
}

func (o *Orchestrator) embeddingWorker(ctx context.Context) {
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		for {
			item, ok := o.embQ.Pop()
			if !ok {
				break
			}
			embPath := filepath.Join(o.cfg.OutputDir, fmt.Sprintf("chunk_%04d_embeddings.json", item.Chunk.ID))
			o.voicePrint.Mutex.Lock()
			existing := o.voicePrint.CurrentEmbPath
			o.voicePrint.Mutex.Unlock()
			args := []string{"python3", o.cfg.EmbeddingScriptPath, "--audio", item.Chunk.Path, "--speakers-json", item.SpeakersJSON, "--output", embPath, "--device", o.cfg.EmbeddingDeviceDefault, "--threshold", o.cfg.EmbeddingThreshold, "--auto-lower-threshold", "--auto-lower-min", o.cfg.EmbeddingAutoLowerMin, "--auto-lower-step", o.cfg.EmbeddingAutoLowerStep, "--hf_token", o.cfg.HFTokenValue}
			// 仅 pyannote 版本支持 --offline
			if o.cfg.EnableOffline && strings.Contains(o.cfg.EmbeddingScriptPath, "pyannote/") {
				args = append(args, "--offline")
			}
			if existing != "" {
				args = append(args, "--existing-embeddings", existing)
			}
			log.Printf("[EMB] run: %v (output=%s)", args, embPath)
			cmd := exec.CommandContext(ctx, args[0], args[1:]...)
			env := os.Environ()
			env = append(env, "HUGGINGFACE_TOKEN="+o.cfg.HFTokenValue)
			cmd.Env = env
			cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				log.Printf("[EMB] error: %v", err)
				continue
			}
			// 验证文件是否生成
			if fi, err := os.Stat(embPath); err != nil {
				log.Printf("[EMB] output missing: %s err=%v", embPath, err)
			} else if fi.Size() == 0 {
				log.Printf("[EMB] output empty: %s", embPath)
			}
			o.voicePrint.Mutex.Lock()
			o.voicePrint.CurrentEmbPath = embPath
			o.voicePrint.Mutex.Unlock()
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
	o.wg.Add(1)
	go func() {
		defer o.wg.Done()
		for {
			item, ok := o.mergeQ.Pop()
			if !ok {
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
			mergedTxt := filepath.Join(o.cfg.OutputDir, fmt.Sprintf("chunk_%04d_merged.txt", item.Chunk.ID))
			args := []string{"merge-segments", "--segments-file", item.SegJSON, "--speaker-file", mapped}
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
	return &Orchestrator{cfg: cfg, state: StateCreated, asrQ: NewSafeQueue[AudioChunk](8), sdQ: NewSafeQueue[ASRResult](8), embQ: NewSafeQueue[SDResult](8), mergeQ: NewSafeQueue[EmbeddingResult](8), voicePrint: &VoicePrintState{CurrentEmbPath: cfg.InitialEmbeddingsPath}, startChunkID: 0, reprocess: false}
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

func (o *Orchestrator) Start() error {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	if o.state != StateCreated && o.state != StateStopped {
		return fmt.Errorf("invalid state: %s", o.state)
	}
	
	// 仅在启用持续录制模式时启动 FFmpeg 录音
	if o.cfg.UseContinuous {
		o.recorder = NewRecorder(o.cfg, o.asrQ, &o.wg)
		if o.startChunkID > 0 {
			o.recorder.chunkID = o.startChunkID
		}
		o.recorder.Start()
	}
	
	o.procCtx, o.procCancel = context.WithCancel(context.Background())
	o.asrWorker(o.procCtx)
	o.sdWorker(o.procCtx)
	o.embeddingWorker(o.procCtx)
	o.mergeWorker(o.procCtx)
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
	o.mutex.Unlock()

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
		recorder.FinalizeAndStop()
	}
	go func() {
		o.wg.Wait()
		o.mutex.Lock()
		o.state = StateDraining
		o.mutex.Unlock()
		_, _ = o.ConcatAllMerged()

		o.mutex.Lock()
		o.state = StateStopped
		o.mutex.Unlock()
	}()
}

// EnqueueAudioChunk 将外部上传的音频文件推入转录队列
// 用于浏览器录音或文件上传场景
func (o *Orchestrator) EnqueueAudioChunk(chunkID int, wavPath string) {
	o.mutex.Lock()
	state := o.state
	o.mutex.Unlock()
	
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
		EndTime:   time.Now(),          // 当前时间作为结束时间
	}
	
	fmt.Printf("[AUDIO] Enqueuing chunk %d for transcription: %s\n", chunkID, wavPath)
	o.asrQ.Push(chunk)
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
		args := []string{"go-whisper/merge-segments", "--segments-file", filepath.Join(o.cfg.OutputDir, seg), "--speaker-file", filepath.Join(o.cfg.OutputDir, spk)}
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
