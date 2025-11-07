package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/meetings"

	"github.com/gin-gonic/gin"
)

// ContentHistory represents a historical version of a document
type ContentHistory struct {
	Version   int       `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Content   string    `json:"content"`
}

// HandleListDocumentVersions handles GET /projects/{project_id}/tasks/{task_id}/{doc_type}/history
func HandleListDocumentVersions(reg *meetings.Registry, docType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		taskID := c.Param("task_id")

		if projectID == "" || taskID == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "project_id and task_id are required",
			})
			return
		}

		history, err := meetings.GetDocumentHistory(reg, taskID, docType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to get document history",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"history": history,
		})
	}
}

// HandleGetDocumentVersion handles GET /projects/{project_id}/tasks/{task_id}/{doc_type}/history/{version}
func HandleGetDocumentVersion(reg *meetings.Registry, docType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		taskID := c.Param("task_id")
		versionStr := c.Param("version")

		if projectID == "" || taskID == "" || versionStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "project_id, task_id and version are required",
			})
			return
		}

		version, err := strconv.Atoi(versionStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid version number",
			})
			return
		}

		// Get history first to validate version exists
		history, err := meetings.GetDocumentHistory(reg, taskID, docType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to get document history",
			})
			return
		}

		// Find the requested version
		var content string
		var found bool
		for _, h := range history {
			if h.Version == version {
				content = h.Content
				found = true
				break
			}
		}

		if !found {
			c.JSON(http.StatusNotFound, gin.H{
				"error": "version not found",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"content": content,
			"version": version,
		})
	}
}

// HandleDeleteDocumentVersion handles DELETE /projects/{project_id}/tasks/{task_id}/{doc_type}/history/{version}
func HandleDeleteDocumentVersion(reg *meetings.Registry, docType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")
		taskID := c.Param("task_id")
		versionStr := c.Param("version")

		if projectID == "" || taskID == "" || versionStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "project_id, task_id and version are required",
			})
			return
		}

		version, err := strconv.Atoi(versionStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid version number",
			})
			return
		}

		if err := meetings.DeleteDocumentHistory(reg, taskID, docType, version); err != nil {
			if err.Error() == "version not found" {
				c.JSON(http.StatusNotFound, gin.H{
					"error": "version not found",
				})
				return
			}
			if err.Error() == "cannot delete current version" {
				c.JSON(http.StatusForbidden, gin.H{
					"error": "cannot delete current version",
				})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to delete document version",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"message": "version deleted successfully",
		})
	}
}

// ========== Generic History Management Functions ==========

// saveContentWithHistory saves content and maintains history
// If the content differs from the current file, archives the current version
func saveContentWithHistory(dir, filename, content string) error {
	filePath := filepath.Join(dir, filename)
	historyDir := filepath.Join(dir, ".history")
	historyFile := filepath.Join(historyDir, filename+".history.json")

	// Ensure the directory for the file exists (e.g., docs/)
	fileDir := filepath.Dir(filePath)
	if err := os.MkdirAll(fileDir, 0755); err != nil {
		return err
	}

	// Ensure history directory exists
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return err
	}

	// Ensure the directory for the history file exists (e.g., .history/docs/)
	historyFileDir := filepath.Dir(historyFile)
	if err := os.MkdirAll(historyFileDir, 0755); err != nil {
		return err
	}

	// Load existing history
	var history []ContentHistory
	if data, err := os.ReadFile(historyFile); err == nil {
		_ = json.Unmarshal(data, &history)
	}

	// Read current content if file exists
	var currentContent string
	if data, err := os.ReadFile(filePath); err == nil {
		currentContent = string(data)
	}

	// Only save to history if content is different and current content exists
	if currentContent != content && currentContent != "" {
		newRecord := ContentHistory{
			Version:   len(history) + 1,
			Timestamp: time.Now(),
			Content:   currentContent,
		}
		history = append(history, newRecord)

		// Keep only last 50 versions
		if len(history) > 50 {
			history = history[len(history)-50:]
			// Re-number versions
			for i := range history {
				history[i].Version = i + 1
			}
		}

		historyData, err := json.MarshalIndent(history, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(historyFile, historyData, 0644); err != nil {
			return err
		}
	}

	// Save new content
	return os.WriteFile(filePath, []byte(content), 0644)
}

// getContentHistory loads the history list for a file
func getContentHistory(dir, filename string) ([]ContentHistory, error) {
	historyFile := filepath.Join(dir, ".history", filename+".history.json")

	var history []ContentHistory
	data, err := os.ReadFile(historyFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No history file means empty history, not an error
			return history, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}

	return history, nil
}

// deleteContentHistory removes a specific version from history
// Re-numbers remaining versions sequentially
func deleteContentHistory(dir, filename string, version int) error {
	history, err := getContentHistory(dir, filename)
	if err != nil {
		return err
	}

	if version <= 0 {
		// Invalid version, nothing to do
		return nil
	}

	// Filter out the specified version
	newHist := make([]ContentHistory, 0, len(history))
	for _, rec := range history {
		if rec.Version != version {
			newHist = append(newHist, rec)
		}
	}

	// Reassign versions sequentially
	for i := range newHist {
		newHist[i].Version = i + 1
	}

	// Save updated history
	historyFile := filepath.Join(dir, ".history", filename+".history.json")
	if err := os.MkdirAll(filepath.Dir(historyFile), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(newHist, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(historyFile, data, 0644)
}

// getContentHistoryWithFallback tries new path first, then falls back to old path for backward compatibility
func getContentHistoryWithFallback(dir, newFilename, oldFilename string) ([]ContentHistory, error) {
	// Try new path first
	history, err := getContentHistory(dir, newFilename)
	if err == nil && len(history) > 0 {
		return history, nil
	}

	// Fallback to old path
	return getContentHistory(dir, oldFilename)
}
