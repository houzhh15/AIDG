package orchestrator

import (
	"fmt"
	"time"
)

// ErrorCode 表示音频处理错误类型代码
type ErrorCode string

const (
	// ENV_NOT_READY 环境未就绪（Token、模型、Whisper 服务等）
	ENV_NOT_READY ErrorCode = "ENV_NOT_READY"

	// WHISPER_UNAVAILABLE Whisper 服务不可用（网络错误、服务未启动）
	WHISPER_UNAVAILABLE ErrorCode = "WHISPER_UNAVAILABLE"

	// WHISPER_HTTP_ERROR Whisper HTTP API 错误（非 200 响应）
	WHISPER_HTTP_ERROR ErrorCode = "WHISPER_HTTP_ERROR"

	// WHISPER_CLI_ERROR Whisper CLI 命令执行错误
	WHISPER_CLI_ERROR ErrorCode = "WHISPER_CLI_ERROR"

	// PYANNOTE_FAILED PyAnnote 说话人识别失败
	PYANNOTE_FAILED ErrorCode = "PYANNOTE_FAILED"

	// FFMPEG_FAILED FFmpeg 音频录制失败
	FFMPEG_FAILED ErrorCode = "FFMPEG_FAILED"

	// MERGE_FAILED merge-segments 合并失败
	MERGE_FAILED ErrorCode = "MERGE_FAILED"

	// DISK_FULL 磁盘空间不足
	DISK_FULL ErrorCode = "DISK_FULL"
)

// OrchError 表示 Orchestrator 音频处理错误
type OrchError struct {
	Code      ErrorCode `json:"code"`
	Message   string    `json:"message"`
	Cause     error     `json:"-"`
	Timestamp time.Time `json:"timestamp"`
}

// Error 实现 error 接口
func (e *OrchError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap 实现错误链支持
func (e *OrchError) Unwrap() error {
	return e.Cause
}

// NewOrchError 创建新的 Orchestrator 错误
func NewOrchError(code ErrorCode, message string, cause error) *OrchError {
	return &OrchError{
		Code:      code,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
	}
}

// NewEnvNotReadyError 创建环境未就绪错误
func NewEnvNotReadyError(message string) *OrchError {
	return NewOrchError(ENV_NOT_READY, message, nil)
}

// NewWhisperUnavailableError 创建 Whisper 服务不可用错误
func NewWhisperUnavailableError(cause error) *OrchError {
	return NewOrchError(WHISPER_UNAVAILABLE, "Whisper 服务不可达", cause)
}

// NewWhisperHTTPError 创建 Whisper HTTP 错误
func NewWhisperHTTPError(statusCode int, body string) *OrchError {
	msg := fmt.Sprintf("Whisper API 返回错误 HTTP %d: %s", statusCode, body)
	return NewOrchError(WHISPER_HTTP_ERROR, msg, nil)
}

// NewWhisperCLIError 创建 Whisper CLI 错误
func NewWhisperCLIError(cause error) *OrchError {
	return NewOrchError(WHISPER_CLI_ERROR, "Whisper CLI 执行失败", cause)
}

// NewPyAnnoteError 创建 PyAnnote 错误
func NewPyAnnoteError(cause error) *OrchError {
	return NewOrchError(PYANNOTE_FAILED, "PyAnnote 说话人识别失败", cause)
}

// NewFFmpegError 创建 FFmpeg 错误
func NewFFmpegError(cause error) *OrchError {
	return NewOrchError(FFMPEG_FAILED, "FFmpeg 音频录制失败", cause)
}

// NewMergeError 创建 merge-segments 错误
func NewMergeError(cause error) *OrchError {
	return NewOrchError(MERGE_FAILED, "merge-segments 合并失败", cause)
}

// NewDiskFullError 创建磁盘空间不足错误
func NewDiskFullError(path string) *OrchError {
	msg := fmt.Sprintf("磁盘空间不足: %s", path)
	return NewOrchError(DISK_FULL, msg, nil)
}
