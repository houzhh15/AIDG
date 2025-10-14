package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15-hub/AIDG/cmd/server/internal/domain/meetings"
)

const (
	MaxFileSize = 500 * 1024 * 1024 // 500MB
)

// HandleAudioUpload 处理音频分片上传
// POST /api/v1/meetings/:meeting_id/audio/upload
func HandleAudioUpload(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		meetingID := c.Param("meeting_id")

		// 检查任务是否存在
		task := reg.Get(meetingID)
		if task == nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "任务不存在",
			})
			return
		}

		// 解析 multipart form
		file, err := c.FormFile("audio")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": fmt.Sprintf("获取文件失败: %v", err),
			})
			return
		}

		// 检查文件大小
		if file.Size > MaxFileSize {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"success": false,
				"message": "文件大小超过500MB限制",
			})
			return
		}

		// 获取表单参数
		chunkIndexStr := c.PostForm("chunk_index")
		format := c.PostForm("format")
		if format == "" {
			format = "webm" // 默认格式
		}

		// 转换 chunk_index 为整数
		chunkIndex, err := strconv.Atoi(chunkIndexStr)
		if err != nil {
			chunkIndex = 0
		}

		// 创建音频目录
		audioDir := filepath.Join(task.Cfg.OutputDir, "audio")
		if err := os.MkdirAll(audioDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("创建目录失败: %v", err),
			})
			return
		}

		// 保存文件
		filename := fmt.Sprintf("chunk_%d.%s", chunkIndex, format)
		savePath := filepath.Join(audioDir, filename)

		if err := c.SaveUploadedFile(file, savePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("保存文件失败: %v", err),
			})
			return
		}

		// 检查是否启用音频转换（支持轻量级部署）
		enableConversion := strings.ToLower(os.Getenv("ENABLE_AUDIO_CONVERSION"))
		if enableConversion == "false" || enableConversion == "0" {
			fmt.Printf("[AUDIO] Audio conversion disabled (ENABLE_AUDIO_CONVERSION=%s), skipping conversion\n", enableConversion)
			// 音频转换已禁用，仅保存文件即可
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": gin.H{
					"chunk_id":          fmt.Sprintf("chunk_%04d", chunkIndex),
					"file_path":         savePath,
					"processing_status": "saved",
					"note":              "音频转换已禁用（轻量级模式）",
				},
			})
			return
		}

		// 转换 webm 为 wav 格式
		wavFilename := fmt.Sprintf("chunk_%04d.wav", chunkIndex)
		wavPath := filepath.Join(task.Cfg.OutputDir, wavFilename)

		// 打印日志
		fmt.Printf("[AUDIO] Converting audio: chunk_index=%d, webm=%s, wav=%s\n",
			chunkIndex, savePath, wavPath)

		// Check dependency mode - use remote deps-service if available
		dependencyMode := strings.ToLower(os.Getenv("DEPENDENCY_MODE"))
		depsServiceURL := os.Getenv("DEPS_SERVICE_URL")

		if (dependencyMode == "fallback" || depsServiceURL != "") && depsServiceURL != "" {
			// Use remote dependency service for conversion
			fmt.Printf("[AUDIO] Using remote deps-service for conversion (url=%s)\n", depsServiceURL)

			// Construct container paths (deps-service mounts host data/ to /data)
			containerWebmPath := strings.Replace(savePath, task.Cfg.OutputDir, "/data/meetings/"+meetingID, 1)
			containerWavPath := strings.Replace(wavPath, task.Cfg.OutputDir, "/data/meetings/"+meetingID, 1)

			// Call deps-service to convert webm to wav
			requestBody := map[string]interface{}{
				"command": "ffmpeg",
				"args": []string{
					"-y",
					"-i", containerWebmPath,
					"-ar", "16000",
					"-ac", "1",
					"-c:a", "pcm_s16le",
					containerWavPath,
				},
			}

			jsonData, err := json.Marshal(requestBody)
			if err != nil {
				log.Printf("[AUDIO] Failed to marshal deps-service request: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to prepare conversion request"})
				return
			}

			// Make HTTP POST request to deps-service
			client := &http.Client{Timeout: 30 * time.Second}
			resp, err := client.Post(
				depsServiceURL+"/api/v1/execute",
				"application/json",
				bytes.NewBuffer(jsonData),
			)
			if err != nil {
				log.Printf("[AUDIO] Failed to call deps-service: %v", err)
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Audio conversion service unavailable"})
				return
			}
			defer resp.Body.Close()

			respBody, _ := io.ReadAll(resp.Body)
			var result map[string]interface{}
			if err := json.Unmarshal(respBody, &result); err != nil {
				log.Printf("[AUDIO] Failed to parse deps-service response: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid conversion service response"})
				return
			}

			if resp.StatusCode != http.StatusOK || !getBoolValue(result, "success") {
				log.Printf("[AUDIO] Conversion failed: %s", respBody)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error":   "Audio conversion failed",
					"details": result,
				})
				return
			}

			fmt.Printf("[AUDIO] Remote conversion successful: chunk_%04d.webm -> chunk_%04d.wav (took %vms)\n",
				chunkIndex, chunkIndex, getFloatValue(result, "duration_ms"))

			// 触发 ASR 转录处理（添加到队列）
			fmt.Printf("[AUDIO] Checking Orchestrator: task.Orch=%v, task.State=%v\n", task.Orch != nil, task.State)
			if task.Orch != nil {
				fmt.Printf("[AUDIO] Triggering ASR transcription for chunk %d after remote conversion\n", chunkIndex)
				go task.Orch.EnqueueAudioChunk(chunkIndex, wavPath)
			} else {
				fmt.Printf("[AUDIO] Warning: Orchestrator is nil, cannot trigger transcription\n")
			}

			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": gin.H{
					"chunk_id":          fmt.Sprintf("chunk_%04d", chunkIndex),
					"webm_path":         savePath,
					"wav_path":          wavPath,
					"processing_status": "converted",
					"duration_ms":       getFloatValue(result, "duration_ms"),
				},
			})
			return
		} // Fallback to local FFmpeg if no remote service available
		task.Cfg.ApplyRuntimeDefaults()
		ffmpegBin := strings.TrimSpace(task.Cfg.FFmpegBinaryPath)
		if ffmpegBin == "" {
			ffmpegBin = "ffmpeg"
		}
		if _, err := exec.LookPath(ffmpegBin); err != nil {
			hint := "检测到轻量级镜像未包含 FFmpeg，请在宿主机安装后通过 docker-compose 卷挂载到容器，或设置 FFMPEG_PATH 指向可执行文件。也可以切换至完整版镜像以获得内置依赖。"
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"success": false,
				"message": fmt.Sprintf("FFmpeg 未配置或不可用: %v", err),
				"hint":    hint,
			})
			return
		}

		// 使用 FFmpeg 转换音频格式（16kHz 单声道 PCM）
		cmd := exec.Command(ffmpegBin,
			"-y",           // 覆盖输出文件
			"-i", savePath, // 输入文件
			"-ar", "16000", // 采样率 16kHz
			"-ac", "1", // 单声道
			"-c:a", "pcm_s16le", // PCM 编码
			wavPath, // 输出文件
		)

		// 捕获错误输出
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("[AUDIO] FFmpeg conversion failed: %v\nOutput: %s\n", err, string(output))
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("音频格式转换失败: %v", err),
				"details": string(output),
			})
			return
		}

		fmt.Printf("[AUDIO] Conversion successful: %s\n", wavPath)

		// 触发 ASR 转录处理
		if task.Orch != nil {
			fmt.Printf("[AUDIO] Triggering ASR transcription for chunk %d\n", chunkIndex)
			go task.Orch.EnqueueAudioChunk(chunkIndex, wavPath)
		} else {
			fmt.Printf("[AUDIO] Warning: Orchestrator is nil, cannot trigger transcription\n")
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"chunk_id":          fmt.Sprintf("chunk_%04d", chunkIndex),
				"file_path":         savePath,
				"wav_path":          wavPath,
				"processing_status": "queued",
			},
		})
	}
}

// HandleAudioFileUpload 处理完整音频文件上传
// POST /api/v1/meetings/:meeting_id/audio/upload-file
func HandleAudioFileUpload(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		meetingID := c.Param("meeting_id")

		// 检查任务是否存在
		task := reg.Get(meetingID)
		if task == nil {
			c.JSON(http.StatusNotFound, gin.H{
				"success": false,
				"message": "任务不存在",
			})
			return
		}

		// 解析 multipart form
		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": fmt.Sprintf("获取文件失败: %v", err),
			})
			return
		}

		// 检查文件大小
		if file.Size > MaxFileSize {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"success": false,
				"message": "文件大小超过500MB限制",
			})
			return
		}

		// 验证文件格式
		ext := filepath.Ext(file.Filename)
		allowedFormats := map[string]bool{
			".wav":  true,
			".mp3":  true,
			".m4a":  true,
			".flac": true,
			".ogg":  true,
			".webm": true,
		}
		if !allowedFormats[ext] {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": fmt.Sprintf("不支持的文件格式: %s", ext),
			})
			return
		}

		// 创建音频目录
		audioDir := filepath.Join(task.Cfg.OutputDir, "audio")
		if err := os.MkdirAll(audioDir, 0755); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("创建目录失败: %v", err),
			})
			return
		}

		// 生成唯一文件名（使用时间戳）
		timestamp := time.Now().Unix()
		filename := fmt.Sprintf("uploaded_%d%s", timestamp, ext)
		savePath := filepath.Join(audioDir, filename)

		// 保存文件
		if err := c.SaveUploadedFile(file, savePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("保存文件失败: %v", err),
			})
			return
		}

		// 获取文件信息
		fileInfo, err := os.Stat(savePath)
		var sizeBytes int64 = 0
		if err == nil {
			sizeBytes = fileInfo.Size()
		}

		// TODO: 推入 asrQ 队列进行转录处理
		// TODO: 使用 ffprobe 检测音频时长
		// 目前仅保存文件，后续迭代添加队列集成和时长检测

		fileID := fmt.Sprintf("file_%d", timestamp)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"file_id":           fileID,
				"file_path":         savePath,
				"duration_ms":       0, // TODO: 实际检测
				"size_bytes":        sizeBytes,
				"processing_status": "queued",
			},
		})
	}
}

// Helper functions for parsing JSON responses
func getBoolValue(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func getFloatValue(m map[string]interface{}, key string) float64 {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return val
		case int:
			return float64(val)
		}
	}
	return 0
}
