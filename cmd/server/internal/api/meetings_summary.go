package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15-hub/AIDG/cmd/server/internal/domain/meetings"
	orchestrator "github.com/houzhh15-hub/AIDG/cmd/server/internal/orchestrator"
)

// ============================================================================
// Summary and Document Handlers (step-13-06)
// ============================================================================

// HandleRegenerateMerged POST /api/v1/tasks/:id/regenerate_merged
// 重新生成 merged_all.txt (合并所有 chunks) 并自动生成润色文本
func HandleRegenerateMerged(reg *meetings.Registry) gin.HandlerFunc {
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

		// Concatenate all merged chunks
		path, err := t.Orch.ConcatAllMerged()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"merged_all": path})
	}
}

// HandleGetMerged GET /api/v1/tasks/:id/merged
// 返回 merged_all.txt 文件
func HandleGetMerged(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		t := reg.Get(id)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		path := filepath.Join(t.Cfg.OutputDir, "merged_all.txt")
		if _, err := os.Stat(path); err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not ready"})
			return
		}
		c.File(path)
	}
}

// HandleGetMergedAll GET /api/v1/tasks/:id/merged_all
// 返回 merged_all.txt 内容 (JSON 格式)
func HandleGetMergedAll(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		t := reg.Get(id)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		path := filepath.Join(t.Cfg.OutputDir, "merged_all.txt")
		b, err := os.ReadFile(path)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not ready"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"content": string(b)})
	}
}

// HandleGetPolish GET /api/v1/tasks/:id/polish
// 返回 polish_all.md 内容 (JSON 格式)
func HandleGetPolish(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		t := reg.Get(id)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		path := filepath.Join(t.Cfg.OutputDir, "polish_all.md")
		b, err := os.ReadFile(path)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "not ready"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"content": string(b)})
	}
}

// ============================================================================
// Legacy Document Handlers (task-only format: /api/v1/tasks/:id/...)
// ============================================================================

// HandleGetTaskMeetingSummary handles GET /tasks/{id}/meeting-summary
func HandleGetTaskMeetingSummary(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")

		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "task_id is required",
			})
			return
		}

		content, exists, err := meetings.LoadDocument(reg, taskID, meetings.DocTypeMeetingSummary)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to load meeting-summary document",
			})
			return
		}

		if !exists {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "meeting-summary document not found",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"content": content,
			"exists":  exists,
		})
	}
}

// HandleUpdateTaskMeetingSummary handles PUT /tasks/{id}/meeting-summary
func HandleUpdateTaskMeetingSummary(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")

		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "task_id is required",
			})
			return
		}

		var req struct {
			Content string `json:"content" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "content field is required",
			})
			return
		}

		if err := meetings.SaveDocumentWithHistory(reg, taskID, meetings.DocTypeMeetingSummary, req.Content); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to save meeting-summary document",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "meeting-summary document updated successfully",
		})
	}
}

// HandleGetTaskMeetingContext handles GET /tasks/{id}/meeting-context
func HandleGetTaskMeetingContext(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")

		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "task_id is required",
			})
			return
		}

		content, exists, err := meetings.LoadDocument(reg, taskID, meetings.DocTypeMeetingContext)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to load meeting-context document",
			})
			return
		}

		if !exists {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "meeting-context document not found",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"content": content,
			"exists":  exists,
		})
	}
}

// HandleUpdateTaskMeetingContext handles PUT /tasks/{id}/meeting-context
func HandleUpdateTaskMeetingContext(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")

		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "task_id is required",
			})
			return
		}

		var req struct {
			Content string `json:"content" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "content field is required",
			})
			return
		}

		if err := meetings.SaveDocumentWithHistory(reg, taskID, meetings.DocTypeMeetingContext, req.Content); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to save meeting-context document",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "meeting-context document updated successfully",
		})
	}
}

// HandleGetTaskTopic handles GET /tasks/{id}/topic
func HandleGetTaskTopic(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")

		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "task_id is required",
			})
			return
		}

		content, exists, err := meetings.LoadDocument(reg, taskID, meetings.DocTypeTopic)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to load topic document",
			})
			return
		}

		if !exists {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "topic document not found",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"content": content,
			"exists":  exists,
		})
	}
}

// HandleUpdateTaskTopic handles PUT /tasks/{id}/topic
func HandleUpdateTaskTopic(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")

		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "task_id is required",
			})
			return
		}

		var req struct {
			Content string `json:"content" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "content field is required",
			})
			return
		}

		if err := meetings.SaveDocumentWithHistory(reg, taskID, meetings.DocTypeTopic, req.Content); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to save topic document",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "topic document updated successfully",
		})
	}
}

// HandleGetTaskPolish handles GET /tasks/{id}/polish
func HandleGetTaskPolish(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")

		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "task_id is required",
			})
			return
		}

		t := reg.Get(taskID)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "task not found",
			})
			return
		}

		polishPath := filepath.Join(t.Cfg.OutputDir, "polish_all.md")
		content, err := os.ReadFile(polishPath)
		if err != nil {
			if os.IsNotExist(err) {
				c.JSON(http.StatusOK, gin.H{
					"content": "",
					"exists":  false,
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("failed to read polish file: %v", err),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"content": string(content),
			"exists":  true,
		})
	}
}

// HandleGetTaskPolishAnnotations handles GET /tasks/{id}/polish-annotations
func HandleGetTaskPolishAnnotations(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")

		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "task_id is required",
			})
			return
		}

		t := reg.Get(taskID)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "task not found",
			})
			return
		}

		annotationsPath := filepath.Join(t.Cfg.OutputDir, "polish_annotations.json")
		b, err := os.ReadFile(annotationsPath)
		if err != nil {
			if os.IsNotExist(err) {
				c.JSON(http.StatusOK, gin.H{
					"annotations": map[string]interface{}{},
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to read annotations",
			})
			return
		}

		var annotations map[string]interface{}
		if err := json.Unmarshal(b, &annotations); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to parse annotations",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"annotations": annotations,
		})
	}
}

// HandleUpdateTaskPolishAnnotations handles PUT /tasks/{id}/polish-annotations
func HandleUpdateTaskPolishAnnotations(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")

		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "task_id is required",
			})
			return
		}

		t := reg.Get(taskID)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "task not found",
			})
			return
		}

		var req struct {
			Annotations map[string]interface{} `json:"annotations" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "annotations field is required",
			})
			return
		}

		annotationsPath := filepath.Join(t.Cfg.OutputDir, "polish_annotations.json")
		data, err := json.MarshalIndent(req.Annotations, "", "  ")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to serialize annotations",
			})
			return
		}

		if err := os.WriteFile(annotationsPath, data, 0644); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to save annotations",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "annotations updated successfully",
		})
	}
}

// HandleGetTaskFeatureList handles GET /tasks/{id}/feature-list
func HandleGetTaskFeatureList(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")

		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "task_id is required",
			})
			return
		}

		content, exists, err := meetings.LoadDocument(reg, taskID, meetings.DocTypeFeatureList)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to load feature-list document",
			})
			return
		}

		if !exists {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "feature-list document not found",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"content": content,
			"exists":  exists,
		})
	}
}

// HandleUpdateTaskFeatureList handles PUT /tasks/{id}/feature-list
func HandleUpdateTaskFeatureList(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")

		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "task_id is required",
			})
			return
		}

		var req struct {
			Content string `json:"content" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "content field is required",
			})
			return
		}

		if err := meetings.SaveDocumentWithHistory(reg, taskID, meetings.DocTypeFeatureList, req.Content); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to save feature-list document",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "feature-list document updated successfully",
		})
	}
}

// HandleGetTaskArchitecture handles GET /tasks/{id}/architecture
func HandleGetTaskArchitecture(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")

		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "task_id is required",
			})
			return
		}

		content, exists, err := meetings.LoadDocument(reg, taskID, meetings.DocTypeArchitecture)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to load architecture document",
			})
			return
		}

		if !exists {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "architecture document not found",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"content": content,
			"exists":  exists,
		})
	}
}

// HandleUpdateTaskArchitecture handles PUT /tasks/{id}/architecture
func HandleUpdateTaskArchitecture(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")

		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "task_id is required",
			})
			return
		}

		var req struct {
			Content string `json:"content" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "content field is required",
			})
			return
		}

		if err := meetings.SaveDocumentWithHistory(reg, taskID, meetings.DocTypeArchitecture, req.Content); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to save architecture document",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "architecture document updated successfully",
		})
	}
}

// HandleGetTaskTechDesign handles GET /tasks/{id}/tech-design
func HandleGetTaskTechDesign(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")

		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "task_id is required",
			})
			return
		}

		content, exists, err := meetings.LoadDocument(reg, taskID, meetings.DocTypeTechDesign)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to load tech-design document",
			})
			return
		}

		if !exists {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "tech-design document not found",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"content": content,
			"exists":  exists,
		})
	}
}

// HandleUpdateTaskTechDesign handles PUT /tasks/{id}/tech-design
func HandleUpdateTaskTechDesign(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")

		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "task_id is required",
			})
			return
		}

		var req struct {
			Content string `json:"content" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "content field is required",
			})
			return
		}

		if err := meetings.SaveDocumentWithHistory(reg, taskID, meetings.DocTypeTechDesign, req.Content); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to save tech-design document",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "tech-design document updated successfully",
		})
	}
}

// HandleUpdateTaskPolish handles PUT /tasks/{id}/polish
func HandleUpdateTaskPolish(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")

		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "task_id is required",
			})
			return
		}

		var req struct {
			Content string `json:"content" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "content field is required",
			})
			return
		}

		if err := meetings.SaveDocumentWithHistory(reg, taskID, meetings.DocTypePolish, req.Content); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to save polish document",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "polish document updated successfully",
		})
	}
}

// HandleGetTaskAudio handles GET /tasks/{id}/audio
func HandleGetTaskAudio(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")

		if taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "task_id is required",
			})
			return
		}

		t := reg.Get(taskID)
		if t == nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "task not found",
			})
			return
		}

		// Check if specific audio file is requested
		filename := c.Query("file")
		if filename != "" {
			// Serve specific audio file
			audioPath := filepath.Join(t.Cfg.OutputDir, filename)
			if !strings.HasSuffix(filename, ".m4a") && !strings.HasSuffix(filename, ".wav") {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "only .m4a and .wav files are supported",
				})
				return
			}

			if _, err := os.Stat(audioPath); os.IsNotExist(err) {
				c.JSON(http.StatusNotFound, gin.H{
					"error": "audio file not found",
				})
				return
			}

			c.File(audioPath)
			return
		}

		// List all audio files
		files, err := filepath.Glob(filepath.Join(t.Cfg.OutputDir, "*.m4a"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to scan audio files",
			})
			return
		}

		wavFiles, err := filepath.Glob(filepath.Join(t.Cfg.OutputDir, "*.wav"))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to scan wav files",
			})
			return
		}

		files = append(files, wavFiles...)

		var audioFiles []map[string]interface{}
		for _, file := range files {
			stat, err := os.Stat(file)
			if err != nil {
				continue
			}

			audioFiles = append(audioFiles, map[string]interface{}{
				"name":     filepath.Base(file),
				"size":     stat.Size(),
				"modified": stat.ModTime(),
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"audio_files": audioFiles,
		})
	}
}

// HandleCopyFeatureList copies feature_list.md from source task to target task
// POST /api/v1/tasks/:id/copy-feature-list
// Required Scopes: users.ScopeFeatureWrite
func HandleCopyFeatureList(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		targetID := c.Param("id")
		targetTask := reg.Get(targetID)
		if targetTask == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "target task not found"})
			return
		}

		var req struct {
			SourceTaskId string `json:"sourceTaskId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		sourceTask := reg.Get(req.SourceTaskId)
		if sourceTask == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "source task not found"})
			return
		}

		sourcePath := filepath.Join(sourceTask.Cfg.OutputDir, "feature_list.md")
		targetPath := filepath.Join(targetTask.Cfg.OutputDir, "feature_list.md")

		sourceContent, err := os.ReadFile(sourcePath)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "source file not found"})
			return
		}

		err = os.WriteFile(targetPath, sourceContent, 0644)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to copy file"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// HandleCopyArchitectureDesign copies architecture_new.md from source task to target task
// POST /api/v1/tasks/:id/copy-architecture-design
// Required Scopes: users.ScopeArchWrite
func HandleCopyArchitectureDesign(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		targetID := c.Param("id")
		targetTask := reg.Get(targetID)
		if targetTask == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "target task not found"})
			return
		}

		var req struct {
			SourceTaskId string `json:"sourceTaskId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		sourceTask := reg.Get(req.SourceTaskId)
		if sourceTask == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "source task not found"})
			return
		}

		sourcePath := filepath.Join(sourceTask.Cfg.OutputDir, "architecture_new.md")
		targetPath := filepath.Join(targetTask.Cfg.OutputDir, "architecture_new.md")

		sourceContent, err := os.ReadFile(sourcePath)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "source file not found"})
			return
		}

		err = os.WriteFile(targetPath, sourceContent, 0644)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to copy file"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true})
	}
}

// HandleCopyTechDesign copies tech_design_*.md from source task to target task
// POST /api/v1/tasks/:id/copy-tech-design
// Required Scopes: users.ScopeTechWrite
func HandleCopyTechDesign(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		targetID := c.Param("id")
		targetTask := reg.Get(targetID)
		if targetTask == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "target task not found"})
			return
		}

		var req struct {
			SourceTaskId string `json:"sourceTaskId"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		sourceTask := reg.Get(req.SourceTaskId)
		if sourceTask == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "source task not found"})
			return
		}

		// 查找源任务中的 tech_design_*.md 文件
		sourceFiles, err := filepath.Glob(filepath.Join(sourceTask.Cfg.OutputDir, "tech_design_*.md"))
		if err != nil || len(sourceFiles) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "source tech design file not found"})
			return
		}

		// 使用第一个匹配的文件
		sourcePath := sourceFiles[0]
		filename := filepath.Base(sourcePath)
		targetPath := filepath.Join(targetTask.Cfg.OutputDir, filename)

		sourceContent, err := os.ReadFile(sourcePath)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "failed to read source file"})
			return
		}

		err = os.WriteFile(targetPath, sourceContent, 0644)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to copy file"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"success": true, "filename": filename})
	}
}
