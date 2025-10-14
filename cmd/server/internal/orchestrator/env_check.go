package orchestrator

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// EnvironmentStatus 表示整体环境状态
type EnvironmentStatus struct {
	Ready    bool               `json:"ready"`
	Issues   []string           `json:"issues"`
	Warnings []string           `json:"warnings"`
	Details  EnvironmentDetails `json:"details"`
}

// EnvironmentDetails 包含各组件的详细状态
type EnvironmentDetails struct {
	HuggingFaceToken TokenStatus   `json:"huggingface_token"`
	PyAnnoteModel    ModelStatus   `json:"pyannote_model"`
	WhisperService   ServiceStatus `json:"whisper_service"`
	FFmpeg           ToolStatus    `json:"ffmpeg"`
}

// TokenStatus 表示 HuggingFace Token 配置状态
type TokenStatus struct {
	Configured bool   `json:"configured"`
	Masked     string `json:"masked,omitempty"`
}

// ModelStatus 表示 AI 模型状态
type ModelStatus struct {
	Exists bool   `json:"exists"`
	Path   string `json:"path"`
	Size   string `json:"size,omitempty"`
}

// ServiceStatus 表示外部服务状态
type ServiceStatus struct {
	Reachable bool   `json:"reachable"`
	URL       string `json:"url"`
	Latency   string `json:"latency,omitempty"`
	Error     string `json:"error,omitempty"`
}

// ToolStatus 表示命令行工具状态
type ToolStatus struct {
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
	Error     string `json:"error,omitempty"`
}

// CheckEnvironment 执行完整的环境检查
func CheckEnvironment() *EnvironmentStatus {
	status := &EnvironmentStatus{
		Ready:    true,
		Issues:   []string{},
		Warnings: []string{},
		Details:  EnvironmentDetails{},
	}

	// 1. 检查 HuggingFace Token
	token := os.Getenv("HUGGINGFACE_TOKEN")
	if token == "" {
		status.Ready = false
		status.Issues = append(status.Issues, "HUGGINGFACE_TOKEN 环境变量未配置")
		status.Details.HuggingFaceToken = TokenStatus{Configured: false}
	} else {
		status.Details.HuggingFaceToken = TokenStatus{
			Configured: true,
			Masked:     maskToken(token),
		}
	}

	// 2. 检查 PyAnnote 模型
	modelPath := "/models/huggingface/pyannote/speaker-diarization-3.1"
	if fi, err := os.Stat(modelPath); err != nil {
		status.Ready = false
		status.Issues = append(status.Issues, fmt.Sprintf("PyAnnote 模型不存在: %s", modelPath))
		status.Details.PyAnnoteModel = ModelStatus{
			Exists: false,
			Path:   modelPath,
		}
	} else if fi.IsDir() {
		size := dirSize(modelPath)
		status.Details.PyAnnoteModel = ModelStatus{
			Exists: true,
			Path:   modelPath,
			Size:   fmt.Sprintf("%.2f MB", float64(size)/(1024*1024)),
		}
		if size == 0 {
			status.Warnings = append(status.Warnings, "PyAnnote 模型目录为空")
		}
	} else {
		status.Warnings = append(status.Warnings, "PyAnnote 模型路径不是目录")
	}

	// 3. 检查 Whisper 服务
	whisperURL := os.Getenv("WHISPER_API_URL")
	if whisperURL == "" {
		whisperURL = "http://whisper:8082"
	}
	if svcStatus := checkWhisperConnection(whisperURL); !svcStatus.Reachable {
		status.Ready = false
		status.Issues = append(status.Issues, fmt.Sprintf("Whisper 服务不可达: %s", svcStatus.Error))
		status.Details.WhisperService = svcStatus
	} else {
		status.Details.WhisperService = svcStatus
	}

	// 4. 检查 FFmpeg
	if toolStatus := checkFFmpeg(); !toolStatus.Available {
		status.Ready = false
		status.Issues = append(status.Issues, fmt.Sprintf("FFmpeg 不可用: %s", toolStatus.Error))
		status.Details.FFmpeg = toolStatus
	} else {
		status.Details.FFmpeg = toolStatus
	}

	return status
}

// CheckPyAnnoteEnv 简化版：仅检查脚本文件是否存在
func CheckPyAnnoteEnv() error {
	scripts := []string{
		"/app/scripts/pyannote_diarize.py",
		"/app/scripts/generate_speaker_embeddings.py",
	}

	var missing []string
	for _, script := range scripts {
		if _, err := os.Stat(script); err != nil {
			missing = append(missing, script)
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("PyAnnote 脚本缺失: %s", strings.Join(missing, ", "))
	}
	return nil
}

// CheckDiarizationEnv 简化版：直接调用 CheckPyAnnoteEnv
func CheckDiarizationEnv(backend string) error {
	if backend != "pyannote" && backend != "" {
		return fmt.Errorf("不支持的 diarization backend: %s（仅支持 pyannote）", backend)
	}
	return CheckPyAnnoteEnv()
}

// maskToken 遮蔽 Token 的中间部分
func maskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

// checkWhisperConnection 检查 Whisper 服务健康状态
func checkWhisperConnection(baseURL string) ServiceStatus {
	healthURL := strings.TrimSuffix(baseURL, "/") + "/health"

	start := time.Now()
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(healthURL)
	latency := time.Since(start)

	if err != nil {
		return ServiceStatus{
			Reachable: false,
			URL:       baseURL,
			Error:     err.Error(),
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ServiceStatus{
			Reachable: false,
			URL:       baseURL,
			Error:     fmt.Sprintf("HTTP %d", resp.StatusCode),
		}
	}

	return ServiceStatus{
		Reachable: true,
		URL:       baseURL,
		Latency:   fmt.Sprintf("%dms", latency.Milliseconds()),
	}
}

func resolveFFmpegBinary() string {
	if path := strings.TrimSpace(os.Getenv("FFMPEG_PATH")); path != "" {
		return path
	}
	return "ffmpeg"
}

// checkFFmpeg 检查 FFmpeg 可用性
func checkFFmpeg() ToolStatus {
	bin := resolveFFmpegBinary()
	if _, err := exec.LookPath(bin); err != nil {
		return ToolStatus{
			Available: false,
			Error:     err.Error(),
		}
	}

	cmd := exec.Command(bin, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return ToolStatus{
			Available: false,
			Error:     err.Error(),
		}
	}

	// 尝试解析版本号（第一行通常包含版本信息）
	lines := strings.Split(string(output), "\n")
	version := "unknown"
	if len(lines) > 0 {
		parts := strings.Fields(lines[0])
		if len(parts) >= 3 {
			version = parts[2]
		}
	}

	return ToolStatus{
		Available: true,
		Version:   version,
	}
}

// dirSize 计算目录大小（递归）
func dirSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}
