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
	"github.com/houzhh15-hub/AIDG/cmd/server/internal/orchestrator"
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
			log.Printf("[AUDIO] Failed to parse chunk_index '%s': %v, using default 0", chunkIndexStr, err)
			chunkIndex = 0
		}

		// 记录日志便于调试
		log.Printf("[AUDIO] Uploading chunk: meeting_id=%s, chunk_index=%d, format=%s, size=%d bytes",
			meetingID, chunkIndex, format, file.Size) // 创建音频目录
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

		// 检查 webm 文件是否已存在（避免重复上传覆盖正在转换的文件）
		if fileInfo, err := os.Stat(savePath); err == nil {
			// 文件已存在，检查大小是否一致
			if fileInfo.Size() == file.Size {
				log.Printf("[AUDIO] WebM file already exists with same size: %s (size: %d bytes), skipping save", savePath, fileInfo.Size())
				// 不保存，直接继续后续流程
			} else {
				log.Printf("[AUDIO] WebM file exists but size differs (old: %d, new: %d), will overwrite", fileInfo.Size(), file.Size)
				// 大小不同，覆盖文件
				if err := c.SaveUploadedFile(file, savePath); err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{
						"success": false,
						"message": fmt.Sprintf("保存文件失败: %v", err),
					})
					return
				}
				log.Printf("[AUDIO] File saved successfully (overwritten): %s (size: %d bytes)", savePath, file.Size)
			}
		} else {
			// 文件不存在，正常保存
			log.Printf("[AUDIO] Saving file: chunk_index=%d, filename=%s, savePath=%s", chunkIndex, filename, savePath)
			if err := c.SaveUploadedFile(file, savePath); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": fmt.Sprintf("保存文件失败: %v", err),
				})
				return
			}
			log.Printf("[AUDIO] File saved successfully: %s (size: %d bytes)", savePath, file.Size)
		}

		// 检查是否已经转换过（避免重复处理失败后的重试）
		wavFilename := fmt.Sprintf("chunk_%04d.wav", chunkIndex)
		wavPath := filepath.Join(task.Cfg.OutputDir, wavFilename)
		if _, err := os.Stat(wavPath); err == nil {
			log.Printf("[AUDIO] WAV file already exists: %s, skipping conversion", wavPath)
			c.JSON(http.StatusOK, gin.H{
				"success": true,
				"data": gin.H{
					"chunk_id":          fmt.Sprintf("chunk_%04d", chunkIndex),
					"file_path":         savePath,
					"wav_path":          wavPath,
					"processing_status": "already_processed",
				},
			})
			return
		}

		// 立即返回成功响应，避免转换失败导致前端重试
		// 转换和处理在后台异步进行
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"chunk_id":          fmt.Sprintf("chunk_%04d", chunkIndex),
				"file_path":         savePath,
				"processing_status": "uploaded",
			},
		})

		// 后台异步处理转换，避免阻塞上传响应
		go func() {
			// 检查是否启用音频转换（支持轻量级部署）
			enableConversion := strings.ToLower(os.Getenv("ENABLE_AUDIO_CONVERSION"))
			if enableConversion == "false" || enableConversion == "0" {
				log.Printf("[AUDIO] Audio conversion disabled (ENABLE_AUDIO_CONVERSION=%s), skipping conversion", enableConversion)
				return
			}

			// 转换 webm 为 wav 格式
			// wavFilename 和 wavPath 已在上面定义

			// 打印日志
			fmt.Printf("[AUDIO] Converting audio: chunk_index=%d, webm=%s, wav=%s\n",
				chunkIndex, savePath, wavPath)

			// Check dependency mode - use remote deps-service if available
			dependencyMode := strings.ToLower(os.Getenv("DEPENDENCY_MODE"))
			depsServiceURL := os.Getenv("DEPS_SERVICE_URL")

			if (dependencyMode == "fallback" || depsServiceURL != "") && depsServiceURL != "" {
				// Use remote dependency service for conversion
				fmt.Printf("[AUDIO] Using remote deps-service for conversion (url=%s)\n", depsServiceURL)

				// Construct container paths (deps-service mounts host data/ to /app/data)
				containerWebmPath := strings.Replace(savePath, task.Cfg.OutputDir, "/app/data/meetings/"+meetingID, 1)
				containerWavPath := strings.Replace(wavPath, task.Cfg.OutputDir, "/app/data/meetings/"+meetingID, 1)

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
					return
				}
				defer resp.Body.Close()

				respBody, _ := io.ReadAll(resp.Body)
				var result map[string]interface{}
				if err := json.Unmarshal(respBody, &result); err != nil {
					log.Printf("[AUDIO] Failed to parse deps-service response: %v", err)
					return
				}

				if resp.StatusCode != http.StatusOK || !getBoolValue(result, "success") {
					log.Printf("[AUDIO] Conversion failed: %s", respBody)
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
				return
			}

			// Fallback to local FFmpeg if no remote service available
			task.Cfg.ApplyRuntimeDefaults()
			ffmpegBin := strings.TrimSpace(task.Cfg.FFmpegBinaryPath)
			if ffmpegBin == "" {
				ffmpegBin = "ffmpeg"
			}
			if _, err := exec.LookPath(ffmpegBin); err != nil {
				log.Printf("[AUDIO] FFmpeg not found: %v", err)
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
				log.Printf("[AUDIO] FFmpeg conversion failed: %v\nOutput: %s", err, string(output))
				return
			}

			log.Printf("[AUDIO] Conversion successful: %s", wavPath)

			// 触发 ASR 转录处理
			if task.Orch != nil {
				log.Printf("[AUDIO] Triggering ASR transcription for chunk %d", chunkIndex)
				go task.Orch.EnqueueAudioChunk(chunkIndex, wavPath)
			} else {
				log.Printf("[AUDIO] Warning: Orchestrator is nil, cannot trigger transcription")
			}
		}() // 结束 goroutine
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

		// 获取 orchestrator 的 dependency client
		orch := task.Orch
		if orch == nil {
			// Orchestrator 未初始化，需要先创建
			log.Printf("[AudioUpload] Orchestrator not initialized, creating new orchestrator for task %s", meetingID)

			// 应用运行时默认值和验证依赖
			task.Cfg.ApplyRuntimeDefaults()
			if err := task.Cfg.ValidateCriticalDependencies(); err != nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"success": false,
					"message": fmt.Sprintf("依赖验证失败: %v", err),
				})
				return
			}

			// 创建新的 orchestrator
			newOrch := orchestrator.New(task.Cfg)

			// 更新任务的 orchestrator
			task.Orch = newOrch
			orch = newOrch

			// 启动 orchestrator（不启动录音，只初始化处理队列）
			log.Printf("[AudioUpload] Starting orchestrator...")
			if err := orch.Start(); err != nil {
				log.Printf("[AudioUpload] ERROR: Failed to start orchestrator: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": fmt.Sprintf("启动 orchestrator 失败: %v", err),
				})
				return
			}
			log.Printf("[AudioUpload] Orchestrator started successfully")

			// 更新任务状态
			task.State = orchestrator.StateRunning
			meetings.SaveTasks(reg)
			log.Printf("[AudioUpload] Orchestrator created and started for task %s", meetingID)
		} else {
			// Orchestrator 已存在，检查状态
			currentState := orch.GetState()
			log.Printf("[AudioUpload] Using existing orchestrator for task %s (state=%s)", meetingID, currentState)

			// 如果 orchestrator 未运行，需要启动它
			if currentState != orchestrator.StateRunning {
				log.Printf("[AudioUpload] Orchestrator is not running, starting it now...")
				if err := orch.Start(); err != nil {
					log.Printf("[AudioUpload] ERROR: Failed to start orchestrator: %v", err)
					c.JSON(http.StatusInternalServerError, gin.H{
						"success": false,
						"message": fmt.Sprintf("启动 orchestrator 失败: %v", err),
					})
					return
				}
				log.Printf("[AudioUpload] Orchestrator started successfully")

				// 更新任务状态
				task.State = orchestrator.StateRunning
				meetings.SaveTasks(reg)
			}
		}

		depClient := orch.GetDependencyClient()
		if depClient == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "依赖服务客户端未初始化",
			})
			return
		}

		// 根据执行模式选择正确的路径
		// 本地模式：使用宿主机路径（直接访问文件系统）
		// 远程模式：使用容器路径（deps-service 容器内的路径）
		var audioPath string
		execMode := depClient.GetMode()

		log.Printf("[AudioUpload] Execution mode: %s", execMode)

		if execMode == "local" || execMode == "fallback" {
			// 本地模式：使用宿主机路径
			audioPath = savePath
		} else {
			// 远程模式：转换为容器路径
			// deps-service 容器内挂载的是 ./data:/app/data
			// 宿主机路径格式：data/meetings/{meetingID}/...
			// 容器内路径格式：/app/data/meetings/{meetingID}/...

			if strings.HasPrefix(savePath, "data/") {
				// 如果是相对路径，直接转换
				relPath := strings.TrimPrefix(savePath, "data/")
				audioPath = filepath.Join("/app/data", relPath)
			} else if strings.Contains(savePath, "/data/") {
				// 如果是绝对路径，提取 data/ 之后的部分
				parts := strings.Split(savePath, "/data/")
				if len(parts) >= 2 {
					audioPath = filepath.Join("/app/data", parts[len(parts)-1])
				} else {
					// 回退到原有逻辑
					audioPath = strings.Replace(savePath, task.Cfg.OutputDir, "/app/data/meetings/"+meetingID, 1)
				}
			} else {
				// 回退到原有逻辑
				audioPath = strings.Replace(savePath, task.Cfg.OutputDir, "/app/data/meetings/"+meetingID, 1)
			}
		}

		log.Printf("[AudioUpload] Path selection: mode=%s, host=%s, audio=%s", execMode, savePath, audioPath)

		// 先检查宿主机上文件是否存在并获取文件信息
		hostFileInfo, statErr := os.Stat(savePath)
		if statErr != nil {
			log.Printf("[AudioUpload] ERROR: File does not exist on host: %s, error: %v", savePath, statErr)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("音频文件不存在: %v", statErr),
			})
			return
		}

		// 输出文件信息用于调试
		log.Printf("[AudioUpload] File info: size=%d bytes, mode=%v", hostFileInfo.Size(), hostFileInfo.Mode())

		// 使用 ffprobe 检测音频时长
		ctx := c.Request.Context()
		duration, err := depClient.GetAudioDuration(ctx, audioPath)
		if err != nil {
			log.Printf("[AudioUpload] ERROR: Failed to get audio duration: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("检测音频时长失败: %v", err),
			})
			return
		}

		durationMs := int64(duration * 1000)
		log.Printf("[AudioUpload] Audio duration: %.2f seconds (%.2f minutes)", duration, duration/60)

		// 按 5 分钟拆分音频文件
		chunkDurationSec := 300 // 5 minutes

		// 根据执行模式构建输出路径模式
		var outputPattern string
		if execMode == "local" || execMode == "fallback" {
			// 本地模式：使用宿主机路径
			outputPattern = filepath.Join(task.Cfg.OutputDir, "chunk_%04d.wav")
		} else {
			// 远程模式：使用容器路径
			outputPattern = "/app/data/meetings/" + meetingID + "/chunk_%04d.wav"
		}

		numChunks, err := depClient.SplitAudioIntoChunks(ctx, audioPath, outputPattern, chunkDurationSec)
		if err != nil {
			log.Printf("[AudioUpload] Failed to split audio: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("拆分音频失败: %v", err),
			})
			return
		}

		log.Printf("[AudioUpload] Split audio into %d chunks", numChunks)

		// 将每个 chunk 推入 ASR 队列进行处理
		// 注意：需要通过 orchestrator 的公开方法来推送到队列
		// 这里我们触发 orchestrator 扫描新生成的 chunk 文件
		if err := orch.EnqueueExistingChunks(0, numChunks); err != nil {
			log.Printf("[AudioUpload] Failed to enqueue chunks: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("推入处理队列失败: %v", err),
			})
			return
		}

		fileID := fmt.Sprintf("file_%d", timestamp)
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"file_id":            fileID,
				"file_path":          savePath,
				"duration_ms":        durationMs,
				"size_bytes":         sizeBytes,
				"num_chunks":         numChunks,
				"chunk_duration_sec": chunkDurationSec,
				"processing_status":  "queued",
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
