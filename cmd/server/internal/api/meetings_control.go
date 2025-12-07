package api

// meetings_control.go - Meeting task lifecycle control operations
// Handles: Start, Stop, Reprocess, Resume, MergeOnly, GetStatus, ASROnce

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/meetings"
	orchestrator "github.com/houzhh15/AIDG/cmd/server/internal/orchestrator"
)

// ============================================================================
// Task Control Handlers (step-13-02)
// ============================================================================

// HandleStartTask POST /api/v1/tasks/:id/start
// 启动任务 (初始化 orchestrator,调用 Start(),更新状态)
func HandleStartTask(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		t := reg.Get(id)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		// Hydrate runtime defaults and validate external dependencies before starting.
		t.Cfg.ApplyRuntimeDefaults()
		if err := t.Cfg.ValidateCriticalDependencies(); err != nil {
			if depErr, ok := err.(orchestrator.DependencyError); ok {
				// Mark task as stopped so UI reflects that startup did not proceed.
				t.State = orchestrator.StateStopped
				meetings.SaveTasks(reg)
				c.JSON(http.StatusServiceUnavailable, gin.H{
					"error":   depErr.Error(),
					"missing": depErr.Missing,
					"details": depErr.Details,
				})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Always rebuild orchestrator using the latest hydrated configuration to
		// ensure runtime defaults are honored even for legacy tasks.
		t.Orch = orchestrator.New(t.Cfg)
		// Auto resume preparation if directory already has chunks but state is Created
		if err := t.Orch.PrepareResume(); err != nil {
			log.Println("PrepareResume warning:", err)
		}

		// Start the task
		if err := t.Orch.Start(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Update state and persist
		t.State = orchestrator.StateRunning
		meetings.SaveTasks(reg)

		c.JSON(http.StatusOK, gin.H{"status": "started"})
	}
}

// HandleStopTask POST /api/v1/tasks/:id/stop
// 停止任务 (调用 Stop(),设置 StateStopping)
func HandleStopTask(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		t := reg.Get(id)
		if t == nil || t.Orch == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not running"})
			return
		}

		// Stop the orchestrator
		t.Orch.Stop()

		// Update state and persist
		t.State = orchestrator.StateStopping
		meetings.SaveTasks(reg)

		c.JSON(http.StatusOK, gin.H{"status": "stopping"})
	}
}

// HandleReprocessTask POST /api/v1/tasks/:id/reprocess
// 从已有 segments 重新处理 (调用 ReprocessFromSegments())
func HandleReprocessTask(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		t := reg.Get(id)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		// Initialize orchestrator if not exists
		if t.Orch == nil {
			t.Orch = orchestrator.New(t.Cfg)
			_ = t.Orch.PrepareResume()
		}

		// Reprocess from existing segments
		if err := t.Orch.ReprocessFromSegments(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// Update state and persist
		t.State = orchestrator.StateRunning
		meetings.SaveTasks(reg)

		c.JSON(http.StatusOK, gin.H{"status": "reprocessing"})
	}
}

// HandleResumeTask POST /api/v1/tasks/:id/resume
// 恢复准备 (扫描现有 chunks,调用 PrepareResume())
func HandleResumeTask(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		t := reg.Get(id)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		// Initialize orchestrator if not exists
		if t.Orch == nil {
			t.Orch = orchestrator.New(t.Cfg)
		}

		// Prepare resume (scan existing chunks)
		if err := t.Orch.PrepareResume(); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "prepared"})
	}
}

// HandleMergeOnlyTask POST /api/v1/tasks/:id/merge_only
// 仅合并现有 chunks (调用 MergeOnly(),返回合并文件路径)
func HandleMergeOnlyTask(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		t := reg.Get(id)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}

		// Initialize orchestrator if not exists
		if t.Orch == nil {
			t.Orch = orchestrator.New(t.Cfg)
		}

		// Merge only (no recording)
		path, err := t.Orch.MergeOnly()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"merged_all": path})
	}
}

// HandleGetTaskStatus GET /api/v1/tasks/:id/status
// 获取任务状态和进度
func HandleGetTaskStatus(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		t := reg.Get(id)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if t.Orch == nil {
			c.JSON(http.StatusOK, gin.H{"state": t.State, "ffmpeg_device": t.Cfg.FFmpegDeviceName, "diarization_backend": t.Cfg.DiarizationBackend})
			return
		}
		p := t.Orch.Progress()
		// update cached state
		t.State = p.State

		// Check if task should be marked as completed
		if p.State == orchestrator.StateStopped {
			mergedAllPath := filepath.Join(t.Cfg.OutputDir, "merged_all.txt")
			if _, err := os.Stat(mergedAllPath); err == nil {
				t.State = orchestrator.StateCompleted
				p.State = orchestrator.StateCompleted
			}
		}

		meetings.SaveTasks(reg)
		c.JSON(http.StatusOK, p)
	}
}

// HandleASROnce POST /api/v1/tasks/:id/chunks/:cid/asr_once
// 对指定 chunk 执行一次性 ASR(不修改任务配置)
func HandleASROnce(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		cid := c.Param("cid")
		t := reg.Get(id)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		var body struct {
			Model    string `json:"model"`
			Segments string `json:"segments"`
		}
		_ = c.ShouldBindJSON(&body)
		wavPath := filepath.Join(t.Cfg.OutputDir, fmt.Sprintf("chunk_%s.wav", cid))
		if _, err := os.Stat(wavPath); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "chunk wav not found"})
			return
		}
		if body.Segments != "" && !strings.HasSuffix(body.Segments, "s") && regexp.MustCompile(`^\d+$`).MatchString(body.Segments) {
			body.Segments = body.Segments + "s"
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		orch := t.Orch
		if orch == nil { // ephemeral orchestrator (no recording workers)
			log.Printf("[HandleASROnce] Creating ephemeral orchestrator for task %s", id)
			orch = orchestrator.New(t.Cfg)
			// Initialize ephemeral orchestrator for single ASR operation
			if err := orch.InitForSingleASR(); err != nil {
				log.Printf("[HandleASROnce] Failed to initialize orchestrator: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to initialize orchestrator: %v", err)})
				return
			}
			log.Printf("[HandleASROnce] Orchestrator initialized successfully")
		}
		segPath, err := orch.RunSingleASR(ctx, wavPath, body.Model, body.Segments)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"segments_json": filepath.Base(segPath)})
	}
}
