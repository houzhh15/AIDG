package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15/aidg-lite/internal/domain/projects"
	"github.com/houzhh15/aidg-lite/pkg/currenttask"
)

// HandleGetCurrentTaskLite handles GET /api/v1/user/current-task in Lite mode.
// Identity is derived from the Authorization header via SHA-256 so that
// different callers (e.g. different VS Code windows using different tokens)
// each maintain their own independent current-task pointer.
func HandleGetCurrentTaskLite(store *currenttask.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		task, ok := store.Get(auth)
		if !ok {
			successResponse(c, gin.H{"success": true, "data": nil})
			return
		}

		// Verify the referenced project and task still exist.
		pDir, err := projectDir(task.ProjectID)
		if err != nil {
			successResponse(c, gin.H{"success": true, "data": nil})
			return
		}

		tasksFile := filepath.Join(pDir, "tasks.json")
		data, err := os.ReadFile(tasksFile)
		if err != nil {
			successResponse(c, gin.H{"success": true, "data": nil})
			return
		}

		var tasks []map[string]interface{}
		if err := json.Unmarshal(data, &tasks); err != nil {
			internalErrorResponse(c, fmt.Errorf("failed to parse tasks: %w", err))
			return
		}

		var taskInfo map[string]interface{}
		for _, t := range tasks {
			if id, _ := t["id"].(string); id == task.TaskID {
				taskInfo = t
				break
			}
		}
		if taskInfo == nil {
			// Task has been deleted – return nil rather than 404.
			successResponse(c, gin.H{"success": true, "data": nil})
			return
		}

		enrichTaskWithCompletionStatus(taskInfo, task.ProjectID, task.TaskID)

		successResponse(c, gin.H{
			"success": true,
			"data": gin.H{
				"project_id":   task.ProjectID,
				"task_id":      task.TaskID,
				"task_info":    taskInfo,
				"project_name": task.ProjectID,
				"set_at":       task.SetAt,
			},
		})
	}
}

// HandlePutCurrentTaskLite handles PUT /api/v1/user/current-task in Lite mode.
// It writes the new current-task pointer into the LRU store, keyed by the
// SHA-256 of the Authorization header value.
func HandlePutCurrentTaskLite(store *currenttask.Store, reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			ProjectID string `json:"project_id"`
			TaskID    string `json:"task_id"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}
		if req.ProjectID == "" || req.TaskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "project_id and task_id are required"})
			return
		}

		// Validate project exists.
		if reg.Get(req.ProjectID) == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "project not found"})
			return
		}

		// Validate task exists.
		tasksFile := filepath.Join(projects.ProjectsRoot, req.ProjectID, "tasks.json")
		data, err := os.ReadFile(tasksFile)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "tasks not found"})
			return
		}
		var tasks []map[string]interface{}
		if err := json.Unmarshal(data, &tasks); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to parse tasks"})
			return
		}
		found := false
		for _, t := range tasks {
			if id, _ := t["id"].(string); id == req.TaskID {
				found = true
				break
			}
		}
		if !found {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}

		auth := c.GetHeader("Authorization")
		store.Set(auth, currenttask.Entry{
			ProjectID: req.ProjectID,
			TaskID:    req.TaskID,
			SetAt:     time.Now(),
		})

		c.JSON(http.StatusOK, gin.H{"message": "current task updated"})
	}
}
