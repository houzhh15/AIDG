package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15-hub/AIDG/cmd/server/internal/domain/meetings"
)

// HandleDebugEnqueueChunk POST /api/v1/debug/tasks/:id/enqueue/:chunk_id
// 仅用于调试：手动将chunk入队到ASR队列
func HandleDebugEnqueueChunk(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		chunkIDStr := c.Param("chunk_id")

		chunkID, err := strconv.Atoi(chunkIDStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chunk_id"})
			return
		}

		task := reg.Get(id)
		if task == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}

		if task.Orch == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "orchestrator not initialized"})
			return
		}

		// 构建chunk路径
		wavPath := task.Cfg.OutputDir + "/" + fmt.Sprintf("chunk_%04d.wav", chunkID)

		// 入队
		task.Orch.EnqueueAudioChunk(chunkID, wavPath)

		c.JSON(http.StatusOK, gin.H{
			"success":  true,
			"chunk_id": chunkID,
			"path":     wavPath,
			"message":  "chunk enqueued to ASR queue",
		})
	}
}
