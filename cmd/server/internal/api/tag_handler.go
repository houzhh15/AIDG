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
	"github.com/houzhh15/AIDG/cmd/server/internal/utils"
)

// TagHandler 处理tag版本管理相关的HTTP请求
type TagHandler struct {
	ProjectsRoot string // 项目根目录路径
}

// TagMetadata tag元数据结构
type TagMetadata struct {
	TagName   string    `json:"tag_name"`
	CreatedAt time.Time `json:"created_at"`
	MD5Hash   string    `json:"md5_hash"`
	FileSize  int64     `json:"file_size"`
	Creator   string    `json:"creator"`
}

// NewTagHandler 创建TagHandler实例
func NewTagHandler(projectsRoot string) *TagHandler {
	return &TagHandler{
		ProjectsRoot: projectsRoot,
	}
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

	// 验证tag名称合法性
	if !utils.ValidateTagName(req.TagName) {
		badRequestResponse(c, "invalid tag name: must be 1-50 characters, alphanumeric, underscore or dash only")
		return
	}

	// 构造文档路径
	docPath := h.getDocPath(projectID, taskID, docType)
	compiledPath := filepath.Join(docPath, "compiled.md")

	// 检查compiled.md是否存在
	if _, err := os.Stat(compiledPath); os.IsNotExist(err) {
		notFoundResponse(c, "document not found")
		return
	}

	// 检查tags目录是否存在
	tagsDir := filepath.Join(docPath, "tags")
	tagDir := filepath.Join(tagsDir, req.TagName)
	if _, err := os.Stat(tagDir); err == nil {
		conflictResponse(c, fmt.Errorf("tag already exists: %s", req.TagName))
		return
	}

	// 计算MD5哈希
	md5Hash, err := utils.CalculateMD5(compiledPath)
	if err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to calculate MD5: %w", err))
		return
	}

	// 获取文件大小
	fileInfo, err := os.Stat(compiledPath)
	if err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to get file info: %w", err))
		return
	}

	// 创建tags目录（如果不存在）
	if err := os.MkdirAll(tagDir, 0755); err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to create tag directory: %w", err))
		return
	}

	// 复制compiled.md
	tagCompiledPath := filepath.Join(tagDir, "compiled.md")
	if err := copyFile(compiledPath, tagCompiledPath); err != nil {
		os.RemoveAll(tagDir) // 清理
		internalErrorResponse(c, fmt.Errorf("failed to copy compiled.md: %w", err))
		return
	}

	// 复制sections目录（如果存在）
	sectionsPath := filepath.Join(docPath, "sections")
	if _, err := os.Stat(sectionsPath); err == nil {
		tagSectionsPath := filepath.Join(tagDir, "sections")
		if err := utils.CopyDirectory(sectionsPath, tagSectionsPath); err != nil {
			os.RemoveAll(tagDir) // 清理
			internalErrorResponse(c, fmt.Errorf("failed to copy sections: %w", err))
			return
		}
	}

	// 创建meta.json
	metadata := TagMetadata{
		TagName:   req.TagName,
		CreatedAt: time.Now(),
		MD5Hash:   md5Hash,
		FileSize:  fileInfo.Size(),
		Creator:   currentUser(c),
	}

	metaPath := filepath.Join(tagDir, "meta.json")
	if err := saveMetadata(metaPath, metadata); err != nil {
		os.RemoveAll(tagDir) // 清理
		internalErrorResponse(c, fmt.Errorf("failed to save metadata: %w", err))
		return
	}

	successResponse(c, gin.H{
		"success": true,
		"data":    metadata,
	})
}

// ListTags 列举所有tag版本
func (h *TagHandler) ListTags(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")
	docType := c.Param("docType")

	docPath := h.getDocPath(projectID, taskID, docType)
	tagsDir := filepath.Join(docPath, "tags")

	// 检查tags目录是否存在
	if _, err := os.Stat(tagsDir); os.IsNotExist(err) {
		successResponse(c, gin.H{
			"tags": []TagMetadata{},
		})
		return
	}

	// 读取所有tag目录
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
		metadata, err := loadMetadata(metaPath)
		if err != nil {
			continue // 跳过损坏的tag
		}

		tags = append(tags, metadata)
	}

	// 按创建时间倒序排列
	sort.Slice(tags, func(i, j int) bool {
		return tags[i].CreatedAt.After(tags[j].CreatedAt)
	})

	successResponse(c, gin.H{
		"tags": tags,
	})
}

// SwitchTag 切换到指定tag版本
func (h *TagHandler) SwitchTag(c *gin.Context) {
	projectID := c.Param("id")
	taskID := c.Param("task_id")
	docType := c.Param("docType")
	tagName := c.Param("tagName")

	// 解析请求体中的force参数
	var req struct {
		Force bool `json:"force"`
	}
	// 忽略解析错误，force默认为false
	_ = c.ShouldBindJSON(&req)

	docPath := h.getDocPath(projectID, taskID, docType)
	compiledPath := filepath.Join(docPath, "compiled.md")

	// 检查当前compiled.md是否存在
	if _, err := os.Stat(compiledPath); os.IsNotExist(err) {
		notFoundResponse(c, "document not found")
		return
	}

	// 计算当前版本的MD5
	currentMD5, err := utils.CalculateMD5(compiledPath)
	if err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to calculate current MD5: %w", err))
		return
	}

	// 获取所有tag的MD5列表
	tagsDir := filepath.Join(docPath, "tags")
	hasUnsavedChanges := true

	if _, err := os.Stat(tagsDir); err == nil {
		entries, _ := os.ReadDir(tagsDir)
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			metaPath := filepath.Join(tagsDir, entry.Name(), "meta.json")
			metadata, err := loadMetadata(metaPath)
			if err == nil && metadata.MD5Hash == currentMD5 {
				hasUnsavedChanges = false
				break
			}
		}
	}

	// 如果有未保存的修改且未强制切换，返回需要确认
	if hasUnsavedChanges && !req.Force {
		successResponse(c, gin.H{
			"success":     false,
			"needConfirm": true,
			"message":     "Current version has unsaved changes",
			"currentMd5":  currentMD5,
			"targetTag":   tagName,
		})
		return
	}

	// 检查目标tag是否存在
	tagDir := filepath.Join(tagsDir, tagName)
	if _, err := os.Stat(tagDir); os.IsNotExist(err) {
		notFoundResponse(c, "tag not found")
		return
	}

	tagCompiledPath := filepath.Join(tagDir, "compiled.md")
	tagSectionsPath := filepath.Join(tagDir, "sections")

	// 创建临时备份目录
	tmpDir := filepath.Join(docPath, fmt.Sprintf(".tmp_%d", time.Now().UnixNano()))
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to create temp directory: %w", err))
		return
	}
	defer os.RemoveAll(tmpDir) // 确保清理临时目录

	// 复制tag版本到临时目录
	tmpCompiledPath := filepath.Join(tmpDir, "compiled.md")
	if err := copyFile(tagCompiledPath, tmpCompiledPath); err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to copy tag compiled.md: %w", err))
		return
	}

	tmpSectionsPath := filepath.Join(tmpDir, "sections")
	if _, err := os.Stat(tagSectionsPath); err == nil {
		if err := utils.CopyDirectory(tagSectionsPath, tmpSectionsPath); err != nil {
			internalErrorResponse(c, fmt.Errorf("failed to copy tag sections: %w", err))
			return
		}
	}

	// 覆盖当前版本
	if err := copyFile(tmpCompiledPath, compiledPath); err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to overwrite compiled.md: %w", err))
		return
	}

	sectionsPath := filepath.Join(docPath, "sections")
	if _, err := os.Stat(tmpSectionsPath); err == nil {
		// 删除旧的sections目录
		os.RemoveAll(sectionsPath)
		// 复制新的sections目录
		if err := utils.CopyDirectory(tmpSectionsPath, sectionsPath); err != nil {
			internalErrorResponse(c, fmt.Errorf("failed to overwrite sections: %w", err))
			return
		}
	}

	// 同步章节元数据
	if err := SyncSectionsAfterSwitch(docPath); err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to sync sections metadata: %w", err))
		return
	}

	successResponse(c, gin.H{
		"success": true,
		"data": gin.H{
			"switched_to":         tagName,
			"has_unsaved_changes": hasUnsavedChanges,
			"current_md5":         currentMD5,
			"warning":             getWarningMessage(hasUnsavedChanges),
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

	metadata, err := loadMetadata(metaPath)
	if err != nil {
		notFoundResponse(c, "tag not found")
		return
	}

	successResponse(c, gin.H{
		"success": true,
		"data":    metadata,
	})
}

// getDocPath 构造文档路径
func (h *TagHandler) getDocPath(projectID, taskID, docType string) string {
	if docType == "execution-plan" {
		return filepath.Join(h.ProjectsRoot, projectID, "tasks", taskID)
	}
	return filepath.Join(h.ProjectsRoot, projectID, "tasks", taskID, "docs", docType)
}

// saveMetadata 保存元数据到JSON文件
func saveMetadata(path string, metadata TagMetadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata file: %w", err)
	}

	return nil
}

// loadMetadata 从JSON文件加载元数据
func loadMetadata(path string) (TagMetadata, error) {
	var metadata TagMetadata

	data, err := os.ReadFile(path)
	if err != nil {
		return metadata, fmt.Errorf("failed to read metadata file: %w", err)
	}

	if err := json.Unmarshal(data, &metadata); err != nil {
		return metadata, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	return metadata, nil
}

// getWarningMessage 获取警告消息
func getWarningMessage(hasUnsavedChanges bool) string {
	if hasUnsavedChanges {
		return "当前版本未打tag，是否创建？"
	}
	return ""
}

// conflictResponse 返回409 Conflict响应
func conflictResponse(c *gin.Context, err error) {
	c.JSON(http.StatusConflict, gin.H{
		"success": false,
		"error":   err.Error(),
	})
}
