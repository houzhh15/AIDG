package api

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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

		// 转换 webm 为 wav 格式
		wavFilename := fmt.Sprintf("chunk_%04d.wav", chunkIndex)
		wavPath := filepath.Join(task.Cfg.OutputDir, wavFilename)
		
		// 使用 FFmpeg 转换音频格式（16kHz 单声道 PCM）
		convertCmd := fmt.Sprintf("ffmpeg -y -i %s -ar 16000 -ac 1 -c:a pcm_s16le %s", 
			savePath, wavPath)
		
		if err := exec.Command("sh", "-c", convertCmd).Run(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": fmt.Sprintf("音频格式转换失败: %v", err),
			})
			return
		}

		// 触发 ASR 转录处理
		go func() {
			// 创建 ASR 任务并推入队列
			// 这里需要调用 orchestrator 的转录方法
			if task.Orch != nil {
				// 假设有转录方法，需要实现
				// task.Orch.TranscribeChunk(wavPath, chunkIndex)
			}
		}()

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
