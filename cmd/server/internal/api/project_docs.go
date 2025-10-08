package api

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15-hub/AIDG/cmd/server/internal/domain/meetings"
	"github.com/houzhh15-hub/AIDG/cmd/server/internal/domain/projects"
)

// HandlePutProjectFeatureList PUT /api/v1/projects/:id/feature-list
// 更新项目特性列表
// Required Scopes: users.ScopeFeatureWrite
func HandlePutProjectFeatureList(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		// 获取项目目录
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

		// 保存文档（使用历史管理）
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

// HandlePutProjectArchitectureDesign PUT /api/v1/projects/:id/architecture-design
// 更新项目架构设计
// Required Scopes: users.ScopeArchWrite
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

// HandlePutProjectTechDesign PUT /api/v1/projects/:id/tech-design
// 更新项目技术设计
// Required Scopes: users.ScopeTechWrite
func HandlePutProjectTechDesign(reg *projects.ProjectRegistry) gin.HandlerFunc {
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

		// 使用与 GET 接口一致的文件名逻辑
		projDir := filepath.Join(projectsRoot(), p.ID)
		filename := "docs/tech_design.md"

		if err := saveContentWithHistory(projDir, filename, req.Content); err != nil {
			log.Printf("[ERROR] HandlePutProjectTechDesign: failed to save tech design for project %s: %v", p.ID, err)
			errorResponse(c, http.StatusInternalServerError, fmt.Sprintf("failed to save tech design: %v", err))
			return
		}

		successResponse(c, gin.H{
			"success": true,
			"message": "tech design updated",
		})
	}
}

// HandleCopyFromTask POST /api/v1/projects/:id/copy-from-task
// 从会议任务复制文档到项目
// Required Scopes: users.ScopeFeatureWrite, users.ScopeArchWrite, users.ScopeTechWrite
func HandleCopyFromTask(projectsReg *projects.ProjectRegistry, meetingsReg *meetings.Registry) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID := c.Param("id")

		// 获取项目
		p := projectsReg.Get(projectID)
		if p == nil {
			notFoundResponse(c, "project")
			return
		}

		var req struct {
			SourceTaskID string   `json:"sourceTaskId"`
			Kinds        []string `json:"kinds"`
		}

		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.SourceTaskID) == "" {
			badRequestResponse(c, "invalid request: sourceTaskId is required")
			return
		}

		// 获取源任务
		sourceTask := meetingsReg.Get(req.SourceTaskID)
		if sourceTask == nil {
			notFoundResponse(c, "source task")
			return
		}

		// 如果未指定 kinds，默认复制所有文档
		kinds := req.Kinds
		if len(kinds) == 0 {
			kinds = []string{"feature-list", "architecture-design", "tech-design"}
		}

		projectDir := filepath.Join(projectsRoot(), p.ID)
		copied := []string{}

		for _, kind := range kinds {
			switch kind {
			case "feature-list":
				srcPath := filepath.Join(sourceTask.Cfg.OutputDir, "feature_list.md")
				if content, err := os.ReadFile(srcPath); err == nil {
					destPath := filepath.Join(projectDir, "feature_list.md")
					if err := os.WriteFile(destPath, content, 0644); err == nil {
						copied = append(copied, kind)
					}
				}

			case "architecture-design":
				srcPath := filepath.Join(sourceTask.Cfg.OutputDir, "architecture_new.md")
				if content, err := os.ReadFile(srcPath); err == nil {
					destPath := filepath.Join(projectDir, "architecture_new.md")
					if err := os.WriteFile(destPath, content, 0644); err == nil {
						copied = append(copied, kind)
					}
				}

			case "tech-design":
				// 查找源任务中的 tech_design_*.md 文件
				files, _ := filepath.Glob(filepath.Join(sourceTask.Cfg.OutputDir, "tech_design_*.md"))
				if len(files) > 0 {
					if content, err := os.ReadFile(files[0]); err == nil {
						filename := filepath.Base(files[0])
						destPath := filepath.Join(projectDir, filename)
						if err := os.WriteFile(destPath, content, 0644); err == nil {
							copied = append(copied, kind)
						}
					}
				}
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"copied":  copied,
		})
	}
}
