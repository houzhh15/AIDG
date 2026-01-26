package api

// meetings_crud.go - Meeting task CRUD operations
// Handles: List, Get, Create, Delete, Rename, UpdateConfig, GetConfig

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/meetings"
	orchestrator "github.com/houzhh15/AIDG/cmd/server/internal/orchestrator"
)

func tasksRoot() string {
	meetings.InitPaths()
	return meetings.TasksRoot()
}

// HandleListTasks GET /api/v1/tasks
// 获取任务列表,支持按产品线和会议时间过滤
func HandleListTasks(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get filter parameters
		filterProductLine := c.Query("product_line")
		filterMeetingTimeStart := c.Query("meeting_time_start") // YYYY-MM-DD format
		filterMeetingTimeEnd := c.Query("meeting_time_end")     // YYYY-MM-DD format

		tasks := reg.List()

		// 更新活跃任务的状态（从 Orchestrator 获取最新状态）
		stateUpdated := false
		for _, t := range tasks {
			if t.Orch != nil {
				p := t.Orch.Progress()
				if t.State != p.State {
					t.State = p.State
					stateUpdated = true
				}
			}
		}
		if stateUpdated {
			meetings.SaveTasks(reg)
		}

		list := []gin.H{}
		for _, t := range tasks {
			// Apply filters
			if filterProductLine != "" && t.Cfg.ProductLine != filterProductLine {
				continue
			}

			// Meeting time range filter
			if filterMeetingTimeStart != "" || filterMeetingTimeEnd != "" {
				if t.Cfg.MeetingTime.IsZero() {
					// If task has no meeting time but filter is specified, skip
					continue
				}

				taskTime := t.Cfg.MeetingTime

				// Check start date filter
				if filterMeetingTimeStart != "" {
					if startDate, err := time.Parse("2006-01-02", filterMeetingTimeStart); err == nil {
						// Task meeting time should be >= start date (00:00:00)
						if taskTime.Before(startDate) {
							continue
						}
					}
				}

				// Check end date filter
				if filterMeetingTimeEnd != "" {
					if endDate, err := time.Parse("2006-01-02", filterMeetingTimeEnd); err == nil {
						// Task meeting time should be <= end date (23:59:59)
						endOfDay := endDate.Add(24*time.Hour - time.Nanosecond)
						if taskTime.After(endOfDay) {
							continue
						}
					}
				}
			}

			var dirSize int64
			var fileCount int
			var lastMod time.Time

			// ensure SB defaults present for legacy tasks
			if meetings.BackfillSBDefaults(&t.Cfg) {
				meetings.SaveTasks(reg)
			}

			entries, err := os.ReadDir(t.Cfg.OutputDir)
			if err == nil {
				for _, e := range entries {
					if e.IsDir() {
						continue
					}
					info, ierr := e.Info()
					if ierr != nil {
						continue
					}
					dirSize += info.Size()
					fileCount++
					if info.ModTime().After(lastMod) {
						lastMod = info.ModTime()
					}
				}
			}

			list = append(list, gin.H{
				"id":                      t.ID,
				"state":                   t.State,
				"output_dir":              t.Cfg.OutputDir,
				"created_at":              t.CreatedAt,
				"ffmpeg_device":           t.Cfg.FFmpegDeviceName,
				"diarization_backend":     t.Cfg.DiarizationBackend,
				"initial_embeddings_path": t.Cfg.InitialEmbeddingsPath,
				"dir_size":                dirSize,
				"file_count":              fileCount,
				"last_modified":           lastMod,
				"product_line":            t.Cfg.ProductLine,
				"meeting_time":            t.Cfg.MeetingTime,
			})
		}

		// sort by created time
		sort.Slice(list, func(i, j int) bool {
			return list[i]["created_at"].(time.Time).Before(list[j]["created_at"].(time.Time))
		})

		c.JSON(http.StatusOK, gin.H{"tasks": list})
	}
}

// HandleGetTask GET /api/v1/tasks/:id
// 获取单个任务信息
func HandleGetTask(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		t := reg.Get(id)

		if t == nil {
			notFoundResponse(c, "task")
			return
		}

		// Calculate directory size and file count
		var dirSize int64
		var fileCount int
		var lastMod time.Time

		// ensure SB defaults present for legacy tasks
		if meetings.BackfillSBDefaults(&t.Cfg) {
			meetings.SaveTasks(reg)
		}

		entries, err := os.ReadDir(t.Cfg.OutputDir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() {
					continue
				}
				info, ierr := e.Info()
				if ierr != nil {
					continue
				}
				dirSize += info.Size()
				fileCount++
				if info.ModTime().After(lastMod) {
					lastMod = info.ModTime()
				}
			}
		}

		taskInfo := gin.H{
			"id":                      t.ID,
			"state":                   t.State,
			"output_dir":              t.Cfg.OutputDir,
			"created_at":              t.CreatedAt,
			"ffmpeg_device":           t.Cfg.FFmpegDeviceName,
			"diarization_backend":     t.Cfg.DiarizationBackend,
			"initial_embeddings_path": t.Cfg.InitialEmbeddingsPath,
			"dir_size":                dirSize,
			"file_count":              fileCount,
			"last_modified":           lastMod,
			"product_line":            t.Cfg.ProductLine,
			"meeting_time":            t.Cfg.MeetingTime,
		}

		c.JSON(http.StatusOK, taskInfo)
	}
}

// HandleCreateTask POST /api/v1/tasks
// 创建新任务
func HandleCreateTask(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		tasksRootDir := tasksRoot()
		var body struct {
			ID                    *string  `json:"id"`
			OutputDir             *string  `json:"output_dir"`
			FFmpegDevice          *string  `json:"ffmpeg_device"`
			RecordChunkSeconds    *int     `json:"record_chunk_seconds"`
			InitialEmbeddingsPath *string  `json:"initial_embeddings_path"`
			DiarizationBackend    *string  `json:"diarization_backend"`
			SBOverclusterFactor   *float64 `json:"sb_overcluster_factor"`
			SBMergeThreshold      *float64 `json:"sb_merge_threshold"`
			SBMinSegmentMerge     *float64 `json:"sb_min_segment_merge"`
			SBReassignAfterMerge  *bool    `json:"sb_reassign_after_merge"`
			SBEnergyVAD           *bool    `json:"sb_energy_vad"`
			SBEnergyVADThr        *float64 `json:"sb_energy_vad_thr"`
			ProductLine           *string  `json:"product_line"`
			MeetingTime           *string  `json:"meeting_time"` // RFC3339 format
		}

		if err := c.ShouldBindJSON(&body); err != nil {
			badRequestResponse(c, err.Error())
			return
		}

		id := ""
		if body.ID != nil && *body.ID != "" {
			id = *body.ID
		} else {
			id = uuid.New().String()
		}

		cfg := orchestrator.DefaultConfig()

		// Output directory: if caller did not specify, create under tasks/<id>
		if body.OutputDir != nil && *body.OutputDir != "" {
			cfg.OutputDir = *body.OutputDir
		} else {
			cfg.OutputDir = filepath.Join(tasksRootDir, id)
		}

		if body.FFmpegDevice != nil {
			cfg.FFmpegDeviceName = *body.FFmpegDevice
		}
		if body.RecordChunkSeconds != nil && *body.RecordChunkSeconds > 0 {
			cfg.RecordChunkDuration = time.Duration(*body.RecordChunkSeconds) * time.Second
		}
		if body.InitialEmbeddingsPath != nil {
			cfg.InitialEmbeddingsPath = strings.TrimSpace(*body.InitialEmbeddingsPath)
		}
		if body.DiarizationBackend != nil && *body.DiarizationBackend != "" {
			cfg.DiarizationBackend = *body.DiarizationBackend
		}
		if body.SBOverclusterFactor != nil {
			cfg.SBOverclusterFactor = *body.SBOverclusterFactor
		}
		if body.SBMergeThreshold != nil {
			cfg.SBMergeThreshold = *body.SBMergeThreshold
		}
		if body.SBMinSegmentMerge != nil {
			cfg.SBMinSegmentMerge = *body.SBMinSegmentMerge
		}
		if body.SBReassignAfterMerge != nil {
			cfg.SBReassignAfterMerge = *body.SBReassignAfterMerge
		}
		if body.SBEnergyVAD != nil {
			cfg.SBEnergyVAD = *body.SBEnergyVAD
		}
		if body.SBEnergyVADThr != nil {
			cfg.SBEnergyVADThr = *body.SBEnergyVADThr
		}
		if body.ProductLine != nil {
			cfg.ProductLine = *body.ProductLine
		}
		if body.MeetingTime != nil && *body.MeetingTime != "" {
			if t, err := time.Parse(time.RFC3339, *body.MeetingTime); err == nil {
				cfg.MeetingTime = t
			}
		}

		os.MkdirAll(cfg.OutputDir, 0o755)

		// 【修复】创建 Orchestrator 实例（但不启动）
		// 这样在文件上传模式下可以立即使用 EnqueueAudioChunk
		orch := orchestrator.New(cfg)

		t := &meetings.Task{
			ID:        id,
			Cfg:       cfg,
			Orch:      orch, // 关联 Orchestrator
			State:     orchestrator.StateCreated,
			CreatedAt: time.Now(),
		}

		reg.Set(t)
		meetings.SaveTasks(reg)

		maskCfg := t.Cfg
		// new task should already have defaults, but ensure
		meetings.BackfillSBDefaults(&maskCfg)
		if maskCfg.HFTokenValue != "" {
			maskCfg.HFTokenValue = "***"
		}

		c.JSON(http.StatusOK, gin.H{
			"id":                      t.ID,
			"config":                  maskCfg,
			"state":                   t.State,
			"created_at":              t.CreatedAt,
			"record_chunk_seconds":    int(maskCfg.RecordChunkDuration.Seconds()),
			"sb_overcluster_factor":   maskCfg.SBOverclusterFactor,
			"sb_merge_threshold":      maskCfg.SBMergeThreshold,
			"sb_min_segment_merge":    maskCfg.SBMinSegmentMerge,
			"sb_reassign_after_merge": maskCfg.SBReassignAfterMerge,
			"sb_energy_vad":           maskCfg.SBEnergyVAD,
			"sb_energy_vad_thr":       maskCfg.SBEnergyVADThr,
			"product_line":            maskCfg.ProductLine,
			"meeting_time":            maskCfg.MeetingTime,
		})
	}
}

// HandleDeleteTask DELETE /api/v1/tasks/:id
// 删除任务
func HandleDeleteTask(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		t := reg.Get(id)
		if t != nil && t.State == orchestrator.StateRunning {
			badRequestResponse(c, "stop task first")
			return
		}

		// capture output dir before removal
		var outDir string
		if t != nil {
			outDir = t.Cfg.OutputDir
		}

		t = reg.Delete(id)
		if t == nil {
			notFoundResponse(c, "task")
			return
		}

		meetings.SaveTasks(reg)

		removedDir := false
		if outDir != "" {
			// 仅移除位于 tasksRoot 下的目录，避免误删自定义路径
			absOut, _ := filepath.Abs(outDir)
			absRoot, _ := filepath.Abs(tasksRoot())
			if strings.HasPrefix(absOut, absRoot+string(os.PathSeparator)) {
				if err := os.RemoveAll(outDir); err == nil {
					removedDir = true
				} else {
					log.Printf("[WARN] remove dir %s failed: %v", outDir, err)
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{"deleted": id, "removed_dir": removedDir})
	}
}

// HandleRenameTask PATCH /api/v1/tasks/:id/rename
// 重命名任务
func HandleRenameTask(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		oldID := c.Param("id")

		t := reg.Get(oldID)
		if t == nil {
			notFoundResponse(c, "task")
			return
		}

		if t.State == orchestrator.StateRunning || t.State == orchestrator.StateStopping || t.State == orchestrator.StateDraining {
			badRequestResponse(c, "cannot rename running task")
			return
		}

		var body struct {
			NewID string `json:"new_id"`
		}

		if err := c.ShouldBindJSON(&body); err != nil || strings.TrimSpace(body.NewID) == "" {
			badRequestResponse(c, "new_id required")
			return
		}

		newID := strings.TrimSpace(body.NewID)
		if newID == oldID {
			c.JSON(http.StatusOK, gin.H{"id": t.ID})
			return
		}

		t, err := reg.Rename(oldID, newID)
		if err != nil {
			badRequestResponse(c, err.Error())
			return
		}

		// Directory rename if matches tasksRoot/oldID exactly
		oldDirAbs, _ := filepath.Abs(t.Cfg.OutputDir)
		tasksRootDir := tasksRoot()
		expectedOldAbs, _ := filepath.Abs(filepath.Join(tasksRootDir, oldID))
		if oldDirAbs == expectedOldAbs {
			newDir := filepath.Join(tasksRootDir, newID)
			if err := os.Rename(t.Cfg.OutputDir, newDir); err == nil {
				t.Cfg.OutputDir = newDir
			} else {
				log.Printf("[WARN] rename dir failed %s -> %s: %v", t.Cfg.OutputDir, newDir, err)
			}
		}

		meetings.SaveTasks(reg)
		c.JSON(http.StatusOK, gin.H{"id": newID})
	}
}

// HandleUpdateTaskConfig PATCH /api/v1/tasks/:id/config
// 更新任务配置
func HandleUpdateTaskConfig(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		t := reg.Get(id)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if t.Orch != nil && t.State == orchestrator.StateRunning {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot modify running task"})
			return
		}
		var body map[string]any
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cfg := t.Cfg
		if v, ok := body["output_dir"].(string); ok && v != "" {
			cfg.OutputDir = v
			os.MkdirAll(v, 0o755)
		}
		if v, ok := body["record_chunk_seconds"].(float64); ok && v > 0 {
			cfg.RecordChunkDuration = time.Duration(int64(v)) * time.Second
		}
		if v, ok := body["record_overlap_seconds"].(float64); ok && v >= 0 {
			cfg.RecordOverlap = time.Duration(int64(v)) * time.Second
		}
		if v, ok := body["ffmpeg_device"].(string); ok && v != "" {
			cfg.FFmpegDeviceName = v
		}
		if v, ok := body["whisper_model"].(string); ok && v != "" {
			cfg.WhisperModel = v
		}
		if v, ok := body["whisper_segments"].(string); ok { // allow empty to clear
			cfg.WhisperSegments = strings.TrimSpace(v)
		}
		if v, ok := body["device_default"].(string); ok && v != "" {
			cfg.DeviceDefault = v
		}
		if v, ok := body["embedding_script"].(string); ok && v != "" {
			cfg.EmbeddingScriptPath = v
		}
		if v, ok := body["embedding_device"].(string); ok && v != "" {
			cfg.EmbeddingDeviceDefault = v
		}
		if v, ok := body["embedding_threshold"].(string); ok && v != "" {
			cfg.EmbeddingThreshold = v
		}
		if v, ok := body["embedding_auto_lower_min"].(string); ok && v != "" {
			cfg.EmbeddingAutoLowerMin = v
		}
		if v, ok := body["embedding_auto_lower_step"].(string); ok && v != "" {
			cfg.EmbeddingAutoLowerStep = v
		}
		if v, ok := body["initial_embeddings_path"].(string); ok {
			// 允许置空
			cfg.InitialEmbeddingsPath = strings.TrimSpace(v)
		}
		if v, ok := body["hf_token"].(string); ok && v != "" {
			cfg.HFTokenValue = v
		}
		if v, ok := body["diarization_backend"].(string); ok && v != "" {
			cfg.DiarizationBackend = v
		}
		if v, ok := body["offline"].(bool); ok {
			cfg.EnableOffline = v
		}
		// SpeechBrain diarization parameters
		if v, ok := body["sb_overcluster_factor"].(float64); ok && v > 0 {
			cfg.SBOverclusterFactor = v
		}
		if v, ok := body["sb_num_speakers"].(float64); ok && v > 0 { // explicit override
			cfg.SBNumSpeakers = int(v)
		} else if _, exists := body["sb_num_speakers"]; exists { // allow clearing
			cfg.SBNumSpeakers = 0
		}
		if v, ok := body["sb_min_speakers"].(float64); ok && v > 0 {
			cfg.SBMinSpeakers = int(v)
		}
		if v, ok := body["sb_max_speakers"].(float64); ok && v > 0 {
			cfg.SBMaxSpeakers = int(v)
		}
		if v, ok := body["sb_merge_threshold"].(float64); ok && v > 0 {
			cfg.SBMergeThreshold = v
		}
		if v, ok := body["sb_min_segment_merge"].(float64); ok && v >= 0 {
			cfg.SBMinSegmentMerge = v
		}
		if v, ok := body["sb_reassign_after_merge"].(bool); ok {
			cfg.SBReassignAfterMerge = v
		}
		if v, ok := body["sb_energy_vad"].(bool); ok {
			cfg.SBEnergyVAD = v
		}
		if v, ok := body["sb_energy_vad_thr"].(float64); ok && v > 0 {
			cfg.SBEnergyVADThr = v
		}
		// Task metadata fields
		if v, ok := body["product_line"].(string); ok {
			cfg.ProductLine = strings.TrimSpace(v)
		}
		if v, ok := body["meeting_time"].(string); ok && v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				cfg.MeetingTime = t
			}
		} else if _, exists := body["meeting_time"]; exists { // allow clearing
			cfg.MeetingTime = time.Time{}
		}
		t.Cfg = cfg
		if t.Orch != nil { // recreate orchestrator with new config for future start
			t.Orch = orchestrator.New(cfg)
		}
		meetings.SaveTasks(reg)
		maskCfg := cfg
		meetings.BackfillSBDefaults(&maskCfg)
		if maskCfg.HFTokenValue != "" {
			maskCfg.HFTokenValue = "***"
		}
		c.JSON(http.StatusOK, gin.H{"config": maskCfg, "record_chunk_seconds": int(maskCfg.RecordChunkDuration.Seconds()),
			"whisper_segments":        maskCfg.WhisperSegments,
			"sb_num_speakers":         maskCfg.SBNumSpeakers,
			"sb_min_speakers":         maskCfg.SBMinSpeakers,
			"sb_max_speakers":         maskCfg.SBMaxSpeakers,
			"sb_overcluster_factor":   maskCfg.SBOverclusterFactor,
			"sb_merge_threshold":      maskCfg.SBMergeThreshold,
			"sb_min_segment_merge":    maskCfg.SBMinSegmentMerge,
			"sb_reassign_after_merge": maskCfg.SBReassignAfterMerge,
			"sb_energy_vad":           maskCfg.SBEnergyVAD,
			"sb_energy_vad_thr":       maskCfg.SBEnergyVADThr,
			"product_line":            maskCfg.ProductLine,
			"meeting_time":            maskCfg.MeetingTime,
		})
	}
}

// HandleGetTaskConfig GET /api/v1/tasks/:id/config
// 获取任务配置(用于UI预填充)
func HandleGetTaskConfig(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		t := reg.Get(id)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		cfg := t.Cfg
		maskCfg := cfg
		meetings.BackfillSBDefaults(&maskCfg)
		if maskCfg.HFTokenValue != "" {
			maskCfg.HFTokenValue = "***"
		}
		c.JSON(http.StatusOK, gin.H{
			"id":                        t.ID,
			"state":                     t.State,
			"config":                    maskCfg,
			"record_chunk_seconds":      int(maskCfg.RecordChunkDuration.Seconds()),
			"ffmpeg_device":             maskCfg.FFmpegDeviceName,
			"diarization_backend":       maskCfg.DiarizationBackend,
			"whisper_model":             maskCfg.WhisperModel,
			"whisper_segments":          maskCfg.WhisperSegments,
			"sb_num_speakers":           maskCfg.SBNumSpeakers,
			"sb_min_speakers":           maskCfg.SBMinSpeakers,
			"sb_max_speakers":           maskCfg.SBMaxSpeakers,
			"initial_embeddings_path":   maskCfg.InitialEmbeddingsPath,
			"embedding_threshold":       maskCfg.EmbeddingThreshold,
			"embedding_auto_lower_min":  maskCfg.EmbeddingAutoLowerMin,
			"embedding_auto_lower_step": maskCfg.EmbeddingAutoLowerStep,
			"sb_overcluster_factor":     maskCfg.SBOverclusterFactor,
			"sb_merge_threshold":        maskCfg.SBMergeThreshold,
			"sb_min_segment_merge":      maskCfg.SBMinSegmentMerge,
			"sb_reassign_after_merge":   maskCfg.SBReassignAfterMerge,
			"sb_energy_vad":             maskCfg.SBEnergyVAD,
			"sb_energy_vad_thr":         maskCfg.SBEnergyVADThr,
			"product_line":              maskCfg.ProductLine,
			"meeting_time":              maskCfg.MeetingTime,
		})
	}
}
