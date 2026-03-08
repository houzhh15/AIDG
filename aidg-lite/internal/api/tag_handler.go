package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/gin-gonic/gin"

	"aidg-lite/internal/util"
)

// TagHandler 处理tag版本管理相关的HTTP请求
type TagHandler struct {
	ProjectsRoot string
}

// TagMetadata tag元数据结构
type TagMetadata struct {
	TagName   string    `json:"tag_name"`
	CreatedAt time.Time `json:"created_at"`
	MD5Hash   string    `json:"md5_hash"`
	FileSize  int64     `json:"file_size"`
	Creator   string    `json:"creator"`
}

func NewTagHandler(projectsRoot string) *TagHandler {
	return &TagHandler{ProjectsRoot: projectsRoot}
}

// CreateTag 创建新的tag版本
func (h *TagHandler) CreateTag(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")
	docType := c.Param("docType")

	var req struct {
		TagName string `json:"tag_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequestResponse(c, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	if !util.ValidateTagName(req.TagName) {
		badRequestResponse(c, "invalid tag name: must be 1-50 characters, alphanumeric, underscore or dash only")
		return
	}

	docPath := h.getDocPath(projectID, taskID, docType)

	var compiledPath string
	if docType == "execution-plan" {
		compiledPath = filepath.Join(docPath, "execution_plan.md")
	} else {
		compiledPath = filepath.Join(docPath, "compiled.md")
	}

	if _, err := os.Stat(compiledPath); os.IsNotExist(err) {
		notFoundResponse(c, "document")
		return
	}

	tagsDir := filepath.Join(docPath, "tags")
	tagDir := filepath.Join(tagsDir, req.TagName)
	if _, err := os.Stat(tagDir); err == nil {
		c.JSON(http.StatusConflict, gin.H{"success": false, "error": fmt.Sprintf("tag already exists: %s", req.TagName)})
		return
	}

	md5Hash, err := util.CalculateMD5(compiledPath)
	if err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to calculate MD5: %w", err))
		return
	}

	fileInfo, err := os.Stat(compiledPath)
	if err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to get file info: %w", err))
		return
	}

	if err := os.MkdirAll(tagDir, 0755); err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to create tag directory: %w", err))
		return
	}

	var targetFileName string
	if docType == "execution-plan" {
		targetFileName = "execution_plan.md"
	} else {
		targetFileName = "compiled.md"
	}
	tagCompiledPath := filepath.Join(tagDir, targetFileName)
	if err := util.CopyFile(compiledPath, tagCompiledPath); err != nil {
		os.RemoveAll(tagDir)
		internalErrorResponse(c, fmt.Errorf("failed to copy document: %w", err))
		return
	}

	sectionsPath := filepath.Join(docPath, "sections")
	if _, err := os.Stat(sectionsPath); err == nil {
		tagSectionsPath := filepath.Join(tagDir, "sections")
		if err := util.CopyDirectory(sectionsPath, tagSectionsPath); err != nil {
			os.RemoveAll(tagDir)
			internalErrorResponse(c, fmt.Errorf("failed to copy sections: %w", err))
			return
		}
	}

	metadata := TagMetadata{
		TagName:   req.TagName,
		CreatedAt: time.Now(),
		MD5Hash:   md5Hash,
		FileSize:  fileInfo.Size(),
		Creator:   currentUser(c),
	}

	metaPath := filepath.Join(tagDir, "meta.json")
	if err := saveTagMetadata(metaPath, metadata); err != nil {
		os.RemoveAll(tagDir)
		internalErrorResponse(c, fmt.Errorf("failed to save metadata: %w", err))
		return
	}

	successResponse(c, gin.H{"success": true, "data": metadata})
}

// ListTags 列举所有tag版本
func (h *TagHandler) ListTags(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")
	docType := c.Param("docType")

	docPath := h.getDocPath(projectID, taskID, docType)
	tagsDir := filepath.Join(docPath, "tags")

	if _, err := os.Stat(tagsDir); os.IsNotExist(err) {
		successResponse(c, gin.H{"tags": []TagMetadata{}})
		return
	}

	entries, err := os.ReadDir(tagsDir)
	if err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to read tags directory: %w", err))
		return
	}

	var tags []TagMetadata
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(tagsDir, entry.Name(), "meta.json")
		metadata, err := loadTagMetadata(metaPath)
		if err != nil {
			continue
		}
		tags = append(tags, metadata)
	}

	sort.Slice(tags, func(i, j int) bool {
		return tags[i].CreatedAt.After(tags[j].CreatedAt)
	})

	successResponse(c, gin.H{"tags": tags})
}

// SwitchTag 切换到指定tag版本
func (h *TagHandler) SwitchTag(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")
	docType := c.Param("docType")
	tagName := c.Param("tagName")

	var req struct {
		Force bool `json:"force"`
	}
	_ = c.ShouldBindJSON(&req)

	docPath := h.getDocPath(projectID, taskID, docType)

	var compiledPath string
	if docType == "execution-plan" {
		compiledPath = filepath.Join(docPath, "execution_plan.md")
	} else {
		compiledPath = filepath.Join(docPath, "compiled.md")
	}

	if _, err := os.Stat(compiledPath); os.IsNotExist(err) {
		notFoundResponse(c, "document")
		return
	}

	currentMD5, err := util.CalculateMD5(compiledPath)
	if err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to calculate current MD5: %w", err))
		return
	}

	tagsDir := filepath.Join(docPath, "tags")
	hasUnsavedChanges := true

	if _, err := os.Stat(tagsDir); err == nil {
		entries, _ := os.ReadDir(tagsDir)
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			metaPath := filepath.Join(tagsDir, entry.Name(), "meta.json")
			metadata, err := loadTagMetadata(metaPath)
			if err == nil && metadata.MD5Hash == currentMD5 {
				hasUnsavedChanges = false
				break
			}
		}
	}

	if hasUnsavedChanges && !req.Force {
		successResponse(c, gin.H{
			"success": false, "needConfirm": true,
			"message":    "Current version has unsaved changes",
			"currentMd5": currentMD5, "targetTag": tagName,
		})
		return
	}

	tagDir := filepath.Join(tagsDir, tagName)
	if _, err := os.Stat(tagDir); os.IsNotExist(err) {
		notFoundResponse(c, "tag")
		return
	}

	var targetFileName string
	if docType == "execution-plan" {
		targetFileName = "execution_plan.md"
	} else {
		targetFileName = "compiled.md"
	}

	tagCompiledPath := filepath.Join(tagDir, targetFileName)
	tagSectionsPath := filepath.Join(tagDir, "sections")

	tmpDir := filepath.Join(docPath, fmt.Sprintf(".tmp_%d", time.Now().UnixNano()))
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to create temp directory: %w", err))
		return
	}
	defer os.RemoveAll(tmpDir)

	tmpCompiledPath := filepath.Join(tmpDir, targetFileName)
	if err := util.CopyFile(tagCompiledPath, tmpCompiledPath); err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to copy tag document: %w", err))
		return
	}

	tmpSectionsPath := filepath.Join(tmpDir, "sections")
	if _, err := os.Stat(tagSectionsPath); err == nil {
		if err := util.CopyDirectory(tagSectionsPath, tmpSectionsPath); err != nil {
			internalErrorResponse(c, fmt.Errorf("failed to copy tag sections: %w", err))
			return
		}
	}

	if err := util.CopyFile(tmpCompiledPath, compiledPath); err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to overwrite compiled.md: %w", err))
		return
	}

	sectionsPath := filepath.Join(docPath, "sections")
	if _, err := os.Stat(tmpSectionsPath); err == nil {
		os.RemoveAll(sectionsPath)
		if err := util.CopyDirectory(tmpSectionsPath, sectionsPath); err != nil {
			internalErrorResponse(c, fmt.Errorf("failed to overwrite sections: %w", err))
			return
		}
	}

	if err := SyncSectionsAfterSwitch(docPath); err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to sync sections metadata: %w", err))
		return
	}

	warningMsg := ""
	if hasUnsavedChanges {
		warningMsg = "当前版本未打tag，是否创建？"
	}

	successResponse(c, gin.H{
		"success": true,
		"data": gin.H{
			"switched_to": tagName, "has_unsaved_changes": hasUnsavedChanges,
			"current_md5": currentMD5, "warning": warningMsg,
		},
	})
}

// GetTagInfo 获取单个tag的元数据
func (h *TagHandler) GetTagInfo(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")
	docType := c.Param("docType")
	tagName := c.Param("tagName")

	docPath := h.getDocPath(projectID, taskID, docType)
	metaPath := filepath.Join(docPath, "tags", tagName, "meta.json")

	metadata, err := loadTagMetadata(metaPath)
	if err != nil {
		notFoundResponse(c, "tag")
		return
	}

	successResponse(c, gin.H{"success": true, "data": metadata})
}

// DeleteTag 删除指定tag
func (h *TagHandler) DeleteTag(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")
	docType := c.Param("docType")
	tagName := c.Param("tagName")

	docPath := h.getDocPath(projectID, taskID, docType)
	tagDir := filepath.Join(docPath, "tags", tagName)

	if _, err := os.Stat(tagDir); os.IsNotExist(err) {
		notFoundResponse(c, "tag")
		return
	}

	if err := os.RemoveAll(tagDir); err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to delete tag: %w", err))
		return
	}

	successResponse(c, gin.H{"success": true, "message": fmt.Sprintf("tag '%s' deleted successfully", tagName)})
}

func (h *TagHandler) getDocPath(projectID, taskID, docType string) string {
	if docType == "execution-plan" {
		return filepath.Join(h.ProjectsRoot, projectID, "tasks", taskID)
	}
	return filepath.Join(h.ProjectsRoot, projectID, "tasks", taskID, "docs", docType)
}

func saveTagMetadata(path string, metadata TagMetadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}

func loadTagMetadata(path string) (TagMetadata, error) {
	var metadata TagMetadata
	data, err := os.ReadFile(path)
	if err != nil {
		return metadata, err
	}
	err = json.Unmarshal(data, &metadata)
	return metadata, err
}

// withDocType 返回一个中间件，将 docType 注入到 gin 的 Params 中
func withDocType(docType string, handler func(*gin.Context)) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Params = append(c.Params, gin.Param{Key: "docType", Value: docType})
		handler(c)
	}
}

func (h *TagHandler) CreateTagWithDocType(docType string) gin.HandlerFunc {
	return withDocType(docType, h.CreateTag)
}
func (h *TagHandler) ListTagsWithDocType(docType string) gin.HandlerFunc {
	return withDocType(docType, h.ListTags)
}
func (h *TagHandler) SwitchTagWithDocType(docType string) gin.HandlerFunc {
	return withDocType(docType, h.SwitchTag)
}
func (h *TagHandler) GetTagInfoWithDocType(docType string) gin.HandlerFunc {
	return withDocType(docType, h.GetTagInfo)
}
func (h *TagHandler) DeleteTagWithDocType(docType string) gin.HandlerFunc {
	return withDocType(docType, h.DeleteTag)
}
