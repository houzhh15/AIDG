package api

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/docslot"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/meetings"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/projects"
)

// HandlePutProjectFeatureList PUT /api/v1/projects/:id/feature-list
// 更新项目特性列表
// Required Scopes: users.ScopeFeatureWrite
func HandlePutProjectFeatureList(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if reg.Get(id) == nil {
			notFoundResponse(c, "project")
			return
		}
		var req struct {
			Content         string `json:"content"`
			ExpectedVersion *int   `json:"expected_version,omitempty"`
		}
		if err := BindDocumentWrite(c, &req); err != nil || (req.ExpectedVersion == nil || *req.ExpectedVersion < 0) {
			badRequestResponse(c, "invalid request body")
			return
		}
		result, err := docslot.NewUnifiedDocService(projectsRoot()).Append(docslot.ScopeProject, id, "feature_list", req.Content, c.GetString("user"), req.ExpectedVersion, "replace_full", "project_api")
		if err != nil {
			handleDocSlotError(c, err)
			return
		}
		c.Header("ETag", result.ETag)
		successResponse(c, gin.H{"success": true, "version": result.Version, "etag": result.ETag})
	}
}

// HandlePutProjectArchitectureDesign PUT /api/v1/projects/:id/architecture-design
// 更新项目架构设计
// Required Scopes: users.ScopeArchWrite
func HandlePutProjectArchitectureDesign(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		if reg.Get(id) == nil {
			notFoundResponse(c, "project")
			return
		}
		var req struct {
			Content         string `json:"content"`
			ExpectedVersion *int   `json:"expected_version,omitempty"`
		}
		if err := BindDocumentWrite(c, &req); err != nil || (req.ExpectedVersion == nil || *req.ExpectedVersion < 0) {
			badRequestResponse(c, "invalid request body")
			return
		}
		result, err := docslot.NewUnifiedDocService(projectsRoot()).Append(docslot.ScopeProject, id, "architecture_design", req.Content, c.GetString("user"), req.ExpectedVersion, "replace_full", "project_api")
		if err != nil {
			handleDocSlotError(c, err)
			return
		}
		c.Header("ETag", result.ETag)
		successResponse(c, gin.H{"success": true, "version": result.Version, "etag": result.ETag})
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

		if err := BindDocumentWrite(c, &req); err != nil {
			badRequestResponse(c, "invalid request body")
			return
		}

		// Use the same symlink-contained project resolver as all legacy reads.
		projDir, err := getProjectDir(reg, p.ID)
		if err != nil {
			badRequestResponse(c, "project path escapes configured data root")
			return
		}
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
			SourceTaskID     string         `json:"sourceTaskId"`
			Kinds            []string       `json:"kinds"`
			ExpectedVersions map[string]int `json:"expected_versions"`
		}

		if err := BindDocumentWrite(c, &req); err != nil || strings.TrimSpace(req.SourceTaskID) == "" {
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

		projectDir, err := getProjectDir(projectsReg, p.ID)
		if err != nil {
			badRequestResponse(c, "project path escapes configured data root")
			return
		}
		copied := []string{}

		for _, kind := range kinds {
			switch kind {
			case "feature-list", "architecture-design":
				expected, ok := req.ExpectedVersions[kind]
				if !ok || expected < 0 {
					badRequestResponse(c, "expected_versions is required for feature-list and architecture-design")
					return
				}
				sourceName, slot := "feature_list.md", "feature_list"
				if kind == "architecture-design" {
					sourceName, slot = "architecture_new.md", "architecture_design"
				}
				content, readErr := os.ReadFile(filepath.Join(sourceTask.Cfg.OutputDir, sourceName))
				if readErr != nil {
					continue
				}
				if len(content) > maxDocumentWriteBytes || !utf8.Valid(content) {
					badRequestResponse(c, "source document exceeds size limit or is not UTF-8")
					return
				}
				if _, err := docslot.NewUnifiedDocService(projectsRoot()).Append(docslot.ScopeProject, projectID, slot, string(content), c.GetString("user"), &expected, "replace_full", "copy_from_task"); err != nil {
					handleDocSlotError(c, err)
					return
				}
				copied = append(copied, kind)

			case "tech-design":
				// 查找源任务中的 tech_design_*.md 文件
				files, _ := filepath.Glob(filepath.Join(sourceTask.Cfg.OutputDir, "tech_design_*.md"))
				if len(files) > 0 {
					if content, err := os.ReadFile(files[0]); err == nil {
						destPath := filepath.Join(projectDir, "docs/tech_design.md")
						// Ensure docs directory exists
						if err := os.MkdirAll(filepath.Join(projectDir, "docs"), 0755); err == nil {
							if err := os.WriteFile(destPath, content, 0644); err == nil {
								copied = append(copied, kind)
							}
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
