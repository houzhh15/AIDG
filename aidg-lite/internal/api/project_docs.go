package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"

	"aidg-lite/internal/domain/projects"
)

// ContentHistory represents a single version history entry
type ContentHistory struct {
	Version   int       `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Content   string    `json:"content"`
}

// HandlePutProjectFeatureList handles PUT /projects/:id/feature-list
func HandlePutProjectFeatureList(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		p := reg.Get(id)
		if p == nil {
			notFoundResponse(c, "project")
			return
		}

		var req struct {
			Content string `json:"content"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			badRequestResponse(c, "invalid request body")
			return
		}

		projDir := filepath.Join(projectsRoot(), p.ID)
		if err := saveContentWithHistory(projDir, "docs/feature_list.md", req.Content); err != nil {
			log.Printf("[ERROR] HandlePutProjectFeatureList: failed to save feature list for project %s: %v", p.ID, err)
			errorResponse(c, http.StatusInternalServerError, fmt.Sprintf("failed to save feature list: %v", err))
			return
		}

		successResponse(c, gin.H{
			"success": true,
			"message": "feature list updated",
		})
	}
}

// HandlePutProjectArchitectureDesign handles PUT /projects/:id/architecture-design
func HandlePutProjectArchitectureDesign(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		p := reg.Get(id)
		if p == nil {
			notFoundResponse(c, "project")
			return
		}

		var req struct {
			Content string `json:"content"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			badRequestResponse(c, "invalid request body")
			return
		}

		projDir := filepath.Join(projectsRoot(), p.ID)
		if err := saveContentWithHistory(projDir, "docs/architecture_design.md", req.Content); err != nil {
			log.Printf("[ERROR] HandlePutProjectArchitectureDesign: failed to save architecture design for project %s: %v", p.ID, err)
			errorResponse(c, http.StatusInternalServerError, fmt.Sprintf("failed to save architecture design: %v", err))
			return
		}

		successResponse(c, gin.H{
			"success": true,
			"message": "architecture design updated",
		})
	}
}

// saveContentWithHistory saves content to a file with version history
func saveContentWithHistory(dir, filename, content string) error {
	filePath := filepath.Join(dir, filename)
	historyDir := filepath.Join(dir, ".history")
	historyFile := filepath.Join(historyDir, filename+".history.json")

	fileDir := filepath.Dir(filePath)
	if err := os.MkdirAll(fileDir, 0755); err != nil {
		return err
	}

	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return err
	}

	historyFileDir := filepath.Dir(historyFile)
	if err := os.MkdirAll(historyFileDir, 0755); err != nil {
		return err
	}

	var history []ContentHistory
	if data, err := os.ReadFile(historyFile); err == nil {
		_ = json.Unmarshal(data, &history)
	}

	var currentContent string
	if data, err := os.ReadFile(filePath); err == nil {
		currentContent = string(data)
	}

	if currentContent != content && currentContent != "" {
		newRecord := ContentHistory{
			Version:   len(history) + 1,
			Timestamp: time.Now(),
			Content:   currentContent,
		}
		history = append(history, newRecord)

		if len(history) > 50 {
			history = history[len(history)-50:]
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

	return os.WriteFile(filePath, []byte(content), 0644)
}
