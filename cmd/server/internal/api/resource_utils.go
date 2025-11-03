package api

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/houzhh15-hub/AIDG/cmd/server/internal/documents"
	"github.com/houzhh15-hub/AIDG/cmd/server/internal/resource"
)

// addTaskResources 自动添加任务相关资源
// 参数:
//   - resourceManager: ResourceManager 实例
//   - username: 用户名
//   - projectID: 项目ID
//   - taskID: 任务ID
//   - docHandler: 文档处理器（用于获取引用和文档内容）
//
// 功能:
//   - 遍历任务级文档类型 (requirements, design, test, execution_plan)
//   - 检查文档是否存在
//   - 构造 aidg:// URI
//   - 创建 Resource 对象标记 AutoAdded=true
//   - 调用 resourceManager.AddResource 添加资源
//   - 遍历项目级文档类型 (architecture_design, feature_list)
//   - 构造项目级资源 URI 并添加
//   - 添加任务关联的引用文档
//   - 所有资源的 Visibility 设为 private
//   - 异常情况记录日志但不阻塞主流程
func addTaskResources(resourceManager *resource.ResourceManager, username, projectID, taskID string, docHandler *documents.Handler) {
	if resourceManager == nil {
		log.Printf("[WARN] addTaskResources: resourceManager is nil, skipping")
		return
	}

	now := time.Now()

	// 文档类型名称映射
	docTypeNameMap := map[string]string{
		"requirements":        "需求",
		"design":              "设计",
		"test":                "测试",
		"execution_plan":      "执行计划",
		"architecture_design": "架构设计",
		"feature_list":        "特性列表",
	}

	// 1. 添加任务级资源
	taskDocTypes := []string{"requirements", "design", "test", "execution_plan"}
	for _, docType := range taskDocTypes {
		// 检查文档是否存在
		if !taskDocumentExists(projectID, taskID, docType) {
			log.Printf("[DEBUG] addTaskResources: task document not found - project=%s, task=%s, docType=%s", projectID, taskID, docType)
			continue
		}

		// 读取文档内容
		content, err := readTaskDocumentContent(projectID, taskID, docType)
		if err != nil {
			log.Printf("[WARN] addTaskResources: failed to read task document content - project=%s, task=%s, docType=%s, error=%v", projectID, taskID, docType, err)
			continue
		}
		log.Printf("[DEBUG] addTaskResources: read task document content - project=%s, task=%s, docType=%s, content_length=%d", projectID, taskID, docType, len(content))

		uri := fmt.Sprintf("aidg://project/%s/task/%s/%s", projectID, taskID, docType)
		displayName := docTypeNameMap[docType]
		if displayName == "" {
			displayName = docType
		}

		res := &resource.Resource{
			ResourceID:  fmt.Sprintf("auto_task_%s_%s_%d", taskID, docType, now.Unix()),
			ProjectID:   projectID,
			TaskID:      taskID,
			URI:         uri,
			Name:        fmt.Sprintf("任务 %s 文档", displayName),
			Description: fmt.Sprintf("任务 %s 的 %s 文档", taskID, displayName),
			MimeType:    "text/markdown",
			Visibility:  "private",
			AutoAdded:   true,
			Content:     content,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		if err := resourceManager.AddResource(username, res); err != nil {
			log.Printf("[ERROR] addTaskResources: failed to add task resource - uri=%s, error=%v", uri, err)
		} else {
			log.Printf("[INFO] addTaskResources: added task resource - uri=%s", uri)
		}
	}

	// 2. 添加项目级资源
	projectDocTypes := []string{"architecture_design", "feature_list"}
	for _, docType := range projectDocTypes {
		// 检查项目文档是否存在
		if !projectDocumentExists(projectID, docType) {
			log.Printf("[DEBUG] addTaskResources: project document not found - project=%s, docType=%s", projectID, docType)
			continue
		}

		// 读取项目文档内容
		content, err := readProjectDocumentContent(projectID, docType)
		if err != nil {
			log.Printf("[WARN] addTaskResources: failed to read project document content - project=%s, docType=%s, error=%v", projectID, docType, err)
			continue
		}

		// 转换为 API 路径格式（下划线转连字符）
		apiDocType := strings.ReplaceAll(docType, "_", "-")
		uri := fmt.Sprintf("aidg://project/%s/%s", projectID, apiDocType)
		displayName := docTypeNameMap[docType]
		if displayName == "" {
			displayName = docType
		}

		res := &resource.Resource{
			ResourceID:  fmt.Sprintf("auto_project_%s_%s_%d", projectID, docType, now.Unix()),
			ProjectID:   projectID,
			URI:         uri,
			Name:        fmt.Sprintf("项目 %s", displayName),
			Description: fmt.Sprintf("项目 %s 的 %s", projectID, displayName),
			MimeType:    "text/markdown",
			Visibility:  "private",
			AutoAdded:   true,
			Content:     content,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		if err := resourceManager.AddResource(username, res); err != nil {
			log.Printf("[ERROR] addTaskResources: failed to add project resource - uri=%s, error=%v", uri, err)
		} else {
			log.Printf("[INFO] addTaskResources: added project resource - uri=%s", uri)
		}
	}

	// 3. 添加任务关联的引用文档资源
	references, err := getTaskReferences(projectID, taskID)
	if err != nil {
		log.Printf("[WARN] addTaskResources: failed to get task references - project=%s, task=%s, error=%v", projectID, taskID, err)
	} else if len(references) > 0 {
		log.Printf("[INFO] addTaskResources: found %d references for task %s", len(references), taskID)

		for i, ref := range references {
			// 直接读取引用文档内容（从旧文档系统）
			refContent, err := readLegacyDocumentContent(projectID, ref.DocumentID)
			if err != nil {
				log.Printf("[WARN] addTaskResources: failed to read reference document content - project=%s, doc_id=%s, error=%v", projectID, ref.DocumentID, err)
				continue
			}

			// 尝试从新文档系统获取标题
			docTitle := ref.DocumentID
			if docHandler != nil {
				docInfo, err := docHandler.GetDocumentContentInternal(projectID, ref.DocumentID)
				if err == nil {
					if meta, ok := docInfo["meta"].(map[string]interface{}); ok {
						if title, ok := meta["title"].(string); ok && title != "" {
							docTitle = title
						}
					}
				}
			}

			// 如果新系统没有，则从内容提取
			if docTitle == ref.DocumentID {
				docTitle = extractTitleFromMarkdown(refContent)
				if docTitle == "" {
					docTitle = ref.DocumentID
				}
			}

			uri := fmt.Sprintf("aidg://project/%s/document/%s", projectID, ref.DocumentID)

			res := &resource.Resource{
				ResourceID:  fmt.Sprintf("auto_ref_%s_%s_%d_%d", taskID, ref.DocumentID, now.Unix(), i),
				ProjectID:   projectID,
				TaskID:      taskID,
				URI:         uri,
				Name:        fmt.Sprintf("引用文档 - %s", docTitle),
				Description: fmt.Sprintf("任务 %s 引用的文档: %s", taskID, docTitle),
				MimeType:    "text/markdown",
				Visibility:  "private",
				AutoAdded:   true,
				Content:     refContent,
				CreatedAt:   now,
				UpdatedAt:   now,
			}

			if err := resourceManager.AddResource(username, res); err != nil {
				log.Printf("[ERROR] addTaskResources: failed to add reference resource - uri=%s, error=%v", uri, err)
			} else {
				log.Printf("[INFO] addTaskResources: added reference resource - uri=%s, name=%s", uri, docTitle)
			}
		}
	}
}

// taskDocumentExists 检查任务文档是否存在
// 参数:
//   - projectID: 项目ID
//   - taskID: 任务ID
//   - docType: 文档类型 (requirements, design, test, execution_plan)
//
// 返回:
//   - bool: 文档存在返回 true，否则返回 false
func taskDocumentExists(projectID, taskID, docType string) bool {
	// 获取项目数据目录
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// 构造文档文件路径
	// 路径格式: data/projects/{project_id}/tasks/{task_id}/docs/{doc_type}/compiled.md
	docFile := filepath.Join(dataDir, "projects", projectID, "tasks", taskID, "docs", docType, "compiled.md")

	// 检查文件是否存在
	_, err := os.Stat(docFile)
	return err == nil
}

// projectDocumentExists 检查项目文档是否存在
// 参数:
//   - projectID: 项目ID
//   - docType: 文档类型 (architecture_design, feature_list)
//
// 返回:
//   - bool: 文档存在返回 true，否则返回 false
func projectDocumentExists(projectID, docType string) bool {
	// 获取项目数据目录
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// 构造文档文件路径
	// 路径格式: data/projects/{project_id}/docs/{doc_type}.md
	docFile := filepath.Join(dataDir, "projects", projectID, "docs", fmt.Sprintf("%s.md", docType))

	// 检查文件是否存在
	_, err := os.Stat(docFile)
	return err == nil
}

// readTaskDocumentContent 读取任务文档内容
// 参数:
//   - projectID: 项目ID
//   - taskID: 任务ID
//   - docType: 文档类型 (requirements, design, test, execution_plan)
//
// 返回:
//   - string: 文档内容
//   - error: 读取失败时返回错误
func readTaskDocumentContent(projectID, taskID, docType string) (string, error) {
	// 获取项目数据目录
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// 构造文档文件路径
	// 路径格式: data/projects/{project_id}/tasks/{task_id}/docs/{docType}/compiled.md
	docFile := filepath.Join(dataDir, "projects", projectID, "tasks", taskID, "docs", docType, "compiled.md")

	// 读取文件内容
	content, err := os.ReadFile(docFile)
	if err != nil {
		return "", fmt.Errorf("failed to read task document: %w", err)
	}

	return string(content), nil
}

// readProjectDocumentContent 读取项目文档内容
// 参数:
//   - projectID: 项目ID
//   - docType: 文档类型 (architecture_design, feature_list)
//
// 返回:
//   - string: 文档内容
//   - error: 读取失败时返回错误
func readProjectDocumentContent(projectID, docType string) (string, error) {
	// 获取项目数据目录
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// 构造文档文件路径
	// 路径格式: data/projects/{project_id}/docs/{doc_type}.md
	docFile := filepath.Join(dataDir, "projects", projectID, "docs", fmt.Sprintf("%s.md", docType))

	// 读取文件内容
	content, err := os.ReadFile(docFile)
	if err != nil {
		return "", fmt.Errorf("failed to read project document: %w", err)
	}

	return string(content), nil
}

// TaskReference 任务引用结构
type TaskReference struct {
	DocumentID string `json:"document_id"`
	Anchor     string `json:"anchor"`
	Context    string `json:"context"`
}

// getTaskReferences 获取任务关联的引用文档列表
// 参数:
//   - projectID: 项目ID
//   - taskID: 任务ID
//
// 返回:
//   - []TaskReference: 引用文档列表
//   - error: 获取失败时返回错误
func getTaskReferences(projectID, taskID string) ([]TaskReference, error) {
	// 获取项目数据目录
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// 构造引用索引文件路径
	// 路径格式: data/projects/{project_id}/documents/references_index.json
	refFile := filepath.Join(dataDir, "projects", projectID, "documents", "references_index.json")

	// 检查文件是否存在
	_, err := os.Stat(refFile)
	if os.IsNotExist(err) {
		// 文件不存在，返回空列表
		return []TaskReference{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to stat references index file: %w", err)
	}

	// 读取文件内容
	data, err := os.ReadFile(refFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read references index file: %w", err)
	}

	// 解析JSON
	var indexData struct {
		References map[string]struct {
			ID         string `json:"id"`
			TaskID     string `json:"task_id"`
			DocumentID string `json:"document_id"`
			Anchor     string `json:"anchor"`
			Context    string `json:"context"`
			Status     string `json:"status"`
		} `json:"references"`
	}
	if err := json.Unmarshal(data, &indexData); err != nil {
		return nil, fmt.Errorf("failed to parse references index file: %w", err)
	}

	// 过滤出该任务的引用
	var refs []TaskReference
	for _, ref := range indexData.References {
		if ref.TaskID == taskID && ref.Status == "active" {
			refs = append(refs, TaskReference{
				DocumentID: ref.DocumentID,
				Anchor:     ref.Anchor,
				Context:    ref.Context,
			})
		}
	}

	return refs, nil
}

// readHierarchicalDocumentContent 读取多层级文档内容
// 参数:
//   - projectID: 项目ID
//   - nodeID: 文档节点ID
//
// 返回:
//   - string: 文档内容
//   - error: 读取失败时返回错误
//
// readHierarchicalDocumentContent 读取层级文档内容
func readHierarchicalDocumentContent(projectID, docID string, docHandler interface{}) (string, error) {
	// 使用文档处理器的内部方法获取内容
	type ContentGetter interface {
		GetDocumentContentInternal(projectID, docID string) (map[string]interface{}, error)
	}

	getter, ok := docHandler.(ContentGetter)
	if !ok {
		return "", fmt.Errorf("docHandler does not implement ContentGetter interface")
	}

	result, err := getter.GetDocumentContentInternal(projectID, docID)
	if err != nil {
		return "", fmt.Errorf("failed to get document content: %w", err)
	}

	content, ok := result["content"].(string)
	if !ok {
		return "", fmt.Errorf("invalid content format for document: %s", docID)
	}

	return content, nil
}

// readLegacyDocumentContent 从旧文档系统读取文档内容
// 参数:
//   - projectID: 项目ID
//   - docID: 文档ID
//
// 返回:
//   - string: 文档内容
//   - error: 读取失败时返回错误
func readLegacyDocumentContent(projectID, docID string) (string, error) {
	// 获取数据目录
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}

	// 构造文档路径
	// 路径格式: data/projects/{project_id}/documents/{doc_id}.md
	docPath := filepath.Join(dataDir, "projects", projectID, "documents", docID+".md")

	// 读取文件内容
	content, err := os.ReadFile(docPath)
	if err != nil {
		return "", fmt.Errorf("failed to read document file: %w", err)
	}

	return string(content), nil
}

// extractTitleFromMarkdown 从 Markdown 内容中提取标题
// 提取第一个 # 标题作为文档标题
func extractTitleFromMarkdown(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "#"))
		}
	}
	return ""
}

// RefreshTaskResources 刷新任务相关的 MCP 资源
// 这是 addTaskResources 的公共包装函数，供其他包调用
// 用于在文档更新后刷新 MCP Resources
func RefreshTaskResources(resourceManager *resource.ResourceManager, username, projectID, taskID string, docHandler *documents.Handler) {
	addTaskResources(resourceManager, username, projectID, taskID, docHandler)
}
