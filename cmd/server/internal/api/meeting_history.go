package api

import (
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/houzhh15-hub/AIDG/cmd/server/internal/domain/meetings"

	"github.com/gin-gonic/gin"
)

func meetingTaskDir(taskID string, task *meetings.Task) string {
	if task != nil && task.Cfg.OutputDir != "" {
		return task.Cfg.OutputDir
	}
	return filepath.Join(meetings.TasksRoot(), taskID)
}

// ========== Meeting Document History Handlers ==========

// HandleGetMeetingSummaryHistory returns the history of meeting_summary.md
// Required Scopes: users.ScopeMeetingRead
func HandleGetMeetingSummaryHistory(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")
		task := reg.Get(taskID)
		if task == nil {
			notFoundResponse(c, "task")
			return
		}

		taskDir := meetingTaskDir(taskID, task)
		history, err := getContentHistory(taskDir, "meeting_summary.md")
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "failed to get history")
			return
		}

		c.JSON(http.StatusOK, gin.H{"history": history})
	}
}

// HandleDeleteMeetingSummaryHistory deletes a specific version of meeting_summary.md
// Required Scopes: users.ScopeMeetingWrite
func HandleDeleteMeetingSummaryHistory(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")
		versionStr := c.Param("version")

		task := reg.Get(taskID)
		if task == nil {
			notFoundResponse(c, "task")
			return
		}

		version, err := strconv.Atoi(versionStr)
		if err != nil {
			errorResponse(c, http.StatusBadRequest, "invalid version")
			return
		}

		taskDir := meetingTaskDir(taskID, task)
		if err := deleteContentHistory(taskDir, "meeting_summary.md", version); err != nil {
			errorResponse(c, http.StatusInternalServerError, "failed to delete history")
			return
		}

		successResponse(c, gin.H{
			"success": true,
			"message": "history version deleted",
		})
	}
}

// HandleGetTopicHistory returns the history of topic.md
// Required Scopes: users.ScopeMeetingRead
func HandleGetTopicHistory(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")
		task := reg.Get(taskID)
		if task == nil {
			notFoundResponse(c, "task")
			return
		}

		taskDir := meetingTaskDir(taskID, task)
		history, err := getContentHistory(taskDir, "topic.md")
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "failed to get history")
			return
		}

		c.JSON(http.StatusOK, gin.H{"history": history})
	}
}

// HandleDeleteTopicHistory deletes a specific version of topic.md
// Required Scopes: users.ScopeMeetingWrite
func HandleDeleteTopicHistory(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")
		versionStr := c.Param("version")

		task := reg.Get(taskID)
		if task == nil {
			notFoundResponse(c, "task")
			return
		}

		version, err := strconv.Atoi(versionStr)
		if err != nil {
			errorResponse(c, http.StatusBadRequest, "invalid version")
			return
		}

		taskDir := meetingTaskDir(taskID, task)
		if err := deleteContentHistory(taskDir, "topic.md", version); err != nil {
			errorResponse(c, http.StatusInternalServerError, "failed to delete history")
			return
		}

		successResponse(c, gin.H{
			"success": true,
			"message": "history version deleted",
		})
	}
}

// HandleGetMeetingFeatureListHistory returns the history of feature_list.md
// Required Scopes: users.ScopeFeatureRead
func HandleGetMeetingFeatureListHistory(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")
		task := reg.Get(taskID)
		if task == nil {
			notFoundResponse(c, "task")
			return
		}

		taskDir := meetingTaskDir(taskID, task)
		history, err := getContentHistory(taskDir, "feature_list.md")
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "failed to get history")
			return
		}

		c.JSON(http.StatusOK, gin.H{"history": history})
	}
}

// HandleDeleteMeetingFeatureListHistory deletes a specific version of feature_list.md
// Required Scopes: users.ScopeFeatureWrite
func HandleDeleteMeetingFeatureListHistory(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")
		versionStr := c.Param("version")

		task := reg.Get(taskID)
		if task == nil {
			notFoundResponse(c, "task")
			return
		}

		version, err := strconv.Atoi(versionStr)
		if err != nil {
			errorResponse(c, http.StatusBadRequest, "invalid version")
			return
		}

		taskDir := meetingTaskDir(taskID, task)
		if err := deleteContentHistory(taskDir, "feature_list.md", version); err != nil {
			errorResponse(c, http.StatusInternalServerError, "failed to delete history")
			return
		}

		successResponse(c, gin.H{
			"success": true,
			"message": "history version deleted",
		})
	}
}

// HandleGetMeetingArchitectureHistory returns the history of architecture_new.md
// Required Scopes: users.ScopeArchRead
func HandleGetMeetingArchitectureHistory(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")
		task := reg.Get(taskID)
		if task == nil {
			notFoundResponse(c, "task")
			return
		}

		taskDir := meetingTaskDir(taskID, task)
		history, err := getContentHistory(taskDir, "architecture_new.md")
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "failed to get history")
			return
		}

		c.JSON(http.StatusOK, gin.H{"history": history})
	}
}

// HandleDeleteMeetingArchitectureHistory deletes a specific version of architecture_new.md
// Required Scopes: users.ScopeArchWrite
func HandleDeleteMeetingArchitectureHistory(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")
		versionStr := c.Param("version")

		task := reg.Get(taskID)
		if task == nil {
			notFoundResponse(c, "task")
			return
		}

		version, err := strconv.Atoi(versionStr)
		if err != nil {
			errorResponse(c, http.StatusBadRequest, "invalid version")
			return
		}

		taskDir := meetingTaskDir(taskID, task)
		if err := deleteContentHistory(taskDir, "architecture_new.md", version); err != nil {
			errorResponse(c, http.StatusInternalServerError, "failed to delete history")
			return
		}

		successResponse(c, gin.H{
			"success": true,
			"message": "history version deleted",
		})
	}
}

// HandleGetMeetingTechDesignHistory returns the history of tech_design_*.md
// Required Scopes: users.ScopeTechRead
func HandleGetMeetingTechDesignHistory(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")
		task := reg.Get(taskID)
		if task == nil {
			notFoundResponse(c, "task")
			return
		}

		taskDir := meetingTaskDir(taskID, task)

		// 查找现有的 tech_design_*.md 文件
		files, err := filepath.Glob(filepath.Join(taskDir, "tech_design_*.md"))
		filename := "tech_design_v1.md"
		if err == nil && len(files) > 0 {
			filename = filepath.Base(files[0])
		}

		history, err := getContentHistory(taskDir, filename)
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "failed to get history")
			return
		}

		c.JSON(http.StatusOK, gin.H{"history": history})
	}
}

// HandleDeleteMeetingTechDesignHistory deletes a specific version of tech_design_*.md
// Required Scopes: users.ScopeTechWrite
func HandleDeleteMeetingTechDesignHistory(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")
		versionStr := c.Param("version")

		task := reg.Get(taskID)
		if task == nil {
			notFoundResponse(c, "task")
			return
		}

		version, err := strconv.Atoi(versionStr)
		if err != nil {
			errorResponse(c, http.StatusBadRequest, "invalid version")
			return
		}

		taskDir := meetingTaskDir(taskID, task)

		// 查找现有的 tech_design_*.md 文件
		files, ferr := filepath.Glob(filepath.Join(taskDir, "tech_design_*.md"))
		filename := "tech_design_v1.md"
		if ferr == nil && len(files) > 0 {
			filename = filepath.Base(files[0])
		}

		if err := deleteContentHistory(taskDir, filename, version); err != nil {
			errorResponse(c, http.StatusInternalServerError, "failed to delete history")
			return
		}

		successResponse(c, gin.H{
			"success": true,
			"message": "history version deleted",
		})
	}
}

// HandleGetPolishHistory returns the history of polish_all.md
// Required Scopes: users.ScopeMeetingRead
func HandleGetPolishHistory(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")
		task := reg.Get(taskID)
		if task == nil {
			notFoundResponse(c, "task")
			return
		}

		taskDir := meetingTaskDir(taskID, task)
		history, err := getContentHistory(taskDir, "polish_all.md")
		if err != nil {
			errorResponse(c, http.StatusInternalServerError, "failed to get history")
			return
		}

		c.JSON(http.StatusOK, gin.H{"history": history})
	}
}

// HandleDeletePolishHistory deletes a specific version of polish_all.md
// Required Scopes: users.ScopeMeetingWrite
func HandleDeletePolishHistory(reg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		taskID := c.Param("id")
		versionStr := c.Param("version")

		task := reg.Get(taskID)
		if task == nil {
			notFoundResponse(c, "task")
			return
		}

		version, err := strconv.Atoi(versionStr)
		if err != nil {
			errorResponse(c, http.StatusBadRequest, "invalid version")
			return
		}

		taskDir := meetingTaskDir(taskID, task)
		if err := deleteContentHistory(taskDir, "polish_all.md", version); err != nil {
			errorResponse(c, http.StatusInternalServerError, "failed to delete history")
			return
		}

		successResponse(c, gin.H{
			"success": true,
			"message": "history version deleted",
		})
	}
}
