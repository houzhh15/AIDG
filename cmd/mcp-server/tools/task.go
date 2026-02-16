package tools

import (
	"fmt"

	"github.com/houzhh15/AIDG/cmd/mcp-server/shared"
)

// ListProjectTasksTool 获取指定项目的任务列表
type ListProjectTasksTool struct{}

func (t *ListProjectTasksTool) Name() string {
	return "list_project_tasks"
}

func (t *ListProjectTasksTool) Description() string {
	return "获取指定项目的任务列表"
}

func (t *ListProjectTasksTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project_id": map[string]interface{}{
				"type":        "string",
				"description": "项目ID（可选，缺失时从当前任务获取）",
			},
		},
		"required": []string{},
	}
}

func (t *ListProjectTasksTool) Execute(arguments map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	projectID, err := shared.GetProjectIDWithFallback(arguments, apiClient, clientToken)
	if err != nil {
		return "", fmt.Errorf("list_project_tasks: %w", err)
	}
	return shared.CallAPI(apiClient, "GET", fmt.Sprintf("/api/v1/projects/%s/tasks", projectID), nil, clientToken)
}

// CreateProjectTaskTool 创建新的项目任务
type CreateProjectTaskTool struct{}

func (t *CreateProjectTaskTool) Name() string {
	return "create_project_task"
}

func (t *CreateProjectTaskTool) Description() string {
	return "创建新的项目任务"
}

func (t *CreateProjectTaskTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project_id": map[string]interface{}{
				"type":        "string",
				"description": "项目ID（可选，缺失时从当前任务获取）",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "任务名称",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "任务描述",
			},
			"assignee": map[string]interface{}{
				"type":        "string",
				"description": "任务负责人",
			},
			"status": map[string]interface{}{
				"type":        "string",
				"description": "任务状态，必须是以下值之一：todo, in-progress, review, completed",
				"enum":        []string{"todo", "in-progress", "review", "completed"},
			},
			"feature_id": map[string]interface{}{
				"type":        "string",
				"description": "关联的特性ID",
			},
			"feature_name": map[string]interface{}{
				"type":        "string",
				"description": "关联的特性名称",
			},
			"module": map[string]interface{}{
				"type":        "string",
				"description": "所属模块",
			},
		},
		"required": []string{"name"},
	}
}

func (t *CreateProjectTaskTool) Execute(arguments map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	projectID, err := shared.GetProjectIDWithFallback(arguments, apiClient, clientToken)
	if err != nil {
		return "", fmt.Errorf("create_project_task: %w", err)
	}
	name, err := shared.SafeGetString(arguments, "name")
	if err != nil {
		return "", fmt.Errorf("create_project_task: %w", err)
	}

	body := map[string]interface{}{
		"name": name,
	}

	// 处理可选参数 - 这些可以是nil，所以不使用safeGetString
	if desc, exists := arguments["description"]; exists && desc != nil {
		body["description"] = desc
	}
	if assignee, exists := arguments["assignee"]; exists && assignee != nil {
		body["assignee"] = assignee
	}
	if status, exists := arguments["status"]; exists && status != nil {
		body["status"] = status
	}
	if featureID, exists := arguments["feature_id"]; exists && featureID != nil {
		body["feature_id"] = featureID
	}
	if featureName, exists := arguments["feature_name"]; exists && featureName != nil {
		body["feature_name"] = featureName
	}
	if module, exists := arguments["module"]; exists && module != nil {
		body["module"] = module
	}

	return shared.CallAPI(apiClient, "POST", fmt.Sprintf("/api/v1/projects/%s/tasks", projectID), body, clientToken)
}

// GetProjectTaskTool 获取指定项目任务的详细信息
type GetProjectTaskTool struct{}

func (t *GetProjectTaskTool) Name() string {
	return "get_project_task"
}

func (t *GetProjectTaskTool) Description() string {
	return "获取指定项目任务的详细信息"
}

func (t *GetProjectTaskTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project_id": map[string]interface{}{
				"type":        "string",
				"description": "项目ID（可选，缺失时从当前任务获取）",
			},
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "任务ID（可选，缺失时从当前任务获取）",
			},
		},
		"required": []string{},
	}
}

func (t *GetProjectTaskTool) Execute(arguments map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	projectID, taskID, err := shared.GetProjectAndTaskIDWithFallback(arguments, apiClient, clientToken)
	if err != nil {
		return "", fmt.Errorf("get_project_task: %w", err)
	}
	return shared.CallAPI(apiClient, "GET", fmt.Sprintf("/api/v1/projects/%s/tasks/%s", projectID, taskID), nil, clientToken)
}

// UpdateProjectTaskTool 更新指定项目任务的信息
type UpdateProjectTaskTool struct{}

func (t *UpdateProjectTaskTool) Name() string {
	return "update_project_task"
}

func (t *UpdateProjectTaskTool) Description() string {
	return "更新指定项目任务的信息"
}

func (t *UpdateProjectTaskTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project_id": map[string]interface{}{
				"type":        "string",
				"description": "项目ID（可选，缺失时从当前任务获取）",
			},
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "任务ID（可选，缺失时从当前任务获取）",
			},
			"name": map[string]interface{}{
				"type":        "string",
				"description": "任务名称",
			},
			"description": map[string]interface{}{
				"type":        "string",
				"description": "任务描述",
			},
			"assignee": map[string]interface{}{
				"type":        "string",
				"description": "任务负责人",
			},
			"status": map[string]interface{}{
				"type":        "string",
				"description": "任务状态，必须是以下值之一：todo, in-progress, review, completed",
				"enum":        []string{"todo", "in-progress", "review", "completed"},
			},
			"feature_id": map[string]interface{}{
				"type":        "string",
				"description": "关联的特性ID",
			},
			"feature_name": map[string]interface{}{
				"type":        "string",
				"description": "关联的特性名称",
			},
			"module": map[string]interface{}{
				"type":        "string",
				"description": "所属模块",
			},
		},
		"required": []string{},
	}
}

func (t *UpdateProjectTaskTool) Execute(arguments map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	projectID, taskID, err := shared.GetProjectAndTaskIDWithFallback(arguments, apiClient, clientToken)
	if err != nil {
		return "", fmt.Errorf("update_project_task: %w", err)
	}

	body := map[string]interface{}{}

	// 处理可选参数 - 这些可以是nil，所以不使用safeGetString
	if name, exists := arguments["name"]; exists && name != nil {
		body["name"] = name
	}
	if desc, exists := arguments["description"]; exists && desc != nil {
		body["description"] = desc
	}
	if assignee, exists := arguments["assignee"]; exists && assignee != nil {
		body["assignee"] = assignee
	}
	if status, exists := arguments["status"]; exists && status != nil {
		body["status"] = status
	}
	if featureID, exists := arguments["feature_id"]; exists && featureID != nil {
		body["feature_id"] = featureID
	}
	if featureName, exists := arguments["feature_name"]; exists && featureName != nil {
		body["feature_name"] = featureName
	}
	if module, exists := arguments["module"]; exists && module != nil {
		body["module"] = module
	}

	return shared.CallAPI(apiClient, "PUT", fmt.Sprintf("/api/v1/projects/%s/tasks/%s", projectID, taskID), body, clientToken)
}

// DeleteProjectTaskTool 删除指定项目任务
type DeleteProjectTaskTool struct{}

func (t *DeleteProjectTaskTool) Name() string {
	return "delete_project_task"
}

func (t *DeleteProjectTaskTool) Description() string {
	return "删除指定项目任务"
}

func (t *DeleteProjectTaskTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project_id": map[string]interface{}{
				"type":        "string",
				"description": "项目ID（可选，缺失时从当前任务获取）",
			},
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "任务ID（可选，缺失时从当前任务获取）",
			},
		},
		"required": []string{},
	}
}

func (t *DeleteProjectTaskTool) Execute(arguments map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	projectID, taskID, err := shared.GetProjectAndTaskIDWithFallback(arguments, apiClient, clientToken)
	if err != nil {
		return "", fmt.Errorf("delete_project_task: %w", err)
	}
	return shared.CallAPI(apiClient, "DELETE", fmt.Sprintf("/api/v1/projects/%s/tasks/%s", projectID, taskID), nil, clientToken)
}

// GetProjectTaskPromptsTool 获取项目任务的提示词历史记录
type GetProjectTaskPromptsTool struct{}

func (t *GetProjectTaskPromptsTool) Name() string {
	return "get_project_task_prompts"
}

func (t *GetProjectTaskPromptsTool) Description() string {
	return "获取项目任务的提示词历史记录"
}

func (t *GetProjectTaskPromptsTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project_id": map[string]interface{}{
				"type":        "string",
				"description": "项目ID（可选，缺失时从当前任务获取）",
			},
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "任务ID（可选，缺失时从当前任务获取）",
			},
		},
		"required": []string{},
	}
}

func (t *GetProjectTaskPromptsTool) Execute(arguments map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	projectID, taskID, err := shared.GetProjectAndTaskIDWithFallback(arguments, apiClient, clientToken)
	if err != nil {
		return "", fmt.Errorf("get_project_task_prompts: %w", err)
	}
	return shared.CallAPI(apiClient, "GET", fmt.Sprintf("/api/v1/projects/%s/tasks/%s/prompts", projectID, taskID), nil, clientToken)
}

// CreateProjectTaskPromptTool 创建项目任务的提示词记录
type CreateProjectTaskPromptTool struct{}

func (t *CreateProjectTaskPromptTool) Name() string {
	return "create_project_task_prompt"
}

func (t *CreateProjectTaskPromptTool) Description() string {
	return "创建项目任务的提示词记录"
}

func (t *CreateProjectTaskPromptTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project_id": map[string]interface{}{
				"type":        "string",
				"description": "项目ID（可选，缺失时从当前任务获取）",
			},
			"task_id": map[string]interface{}{
				"type":        "string",
				"description": "任务ID（可选，缺失时从当前任务获取）",
			},
			"username": map[string]interface{}{
				"type":        "string",
				"description": "用户名",
			},
			"content": map[string]interface{}{
				"type":        "string",
				"description": "提示词内容",
			},
		},
		"required": []string{"username", "content"},
	}
}

func (t *CreateProjectTaskPromptTool) Execute(arguments map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	projectID, taskID, err := shared.GetProjectAndTaskIDWithFallback(arguments, apiClient, clientToken)
	if err != nil {
		return "", fmt.Errorf("create_project_task_prompt: %w", err)
	}
	username, err := shared.SafeGetString(arguments, "username")
	if err != nil {
		return "", fmt.Errorf("create_project_task_prompt: %w", err)
	}
	content, err := shared.SafeGetString(arguments, "content")
	if err != nil {
		return "", fmt.Errorf("create_project_task_prompt: %w", err)
	}

	body := map[string]interface{}{
		"username": username,
		"content":  content,
	}
	return shared.CallAPI(apiClient, "POST", fmt.Sprintf("/api/v1/projects/%s/tasks/%s/prompts", projectID, taskID), body, clientToken)
}

// GetNextIncompleteTaskTool 获取下一个未完成文档的任务
type GetNextIncompleteTaskTool struct{}

func (t *GetNextIncompleteTaskTool) Name() string {
	return "get_next_incomplete_task"
}

func (t *GetNextIncompleteTaskTool) Description() string {
	return "获取项目中下一个有未完成文档的任务。可指定要检查的文档类型（requirements/design/plan/execution/test），不指定则检查全部五项。返回推荐优先完成的文档类型。"
}

func (t *GetNextIncompleteTaskTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"project_id": map[string]interface{}{
				"type":        "string",
				"description": "项目ID（可选，缺失时从当前任务获取）",
			},
			"doc_type": map[string]interface{}{
				"type":        "string",
				"description": "要检查的文档类型（可选）。可选值：requirements（需求文档）、design（设计文档）、plan（执行计划）、execution（计划执行完成）、test（测试文档）。不指定则返回任意一项未完成的任务。",
				"enum":        []string{"requirements", "design", "plan", "execution", "test"},
			},
		},
		"required": []string{},
	}
}

func (t *GetNextIncompleteTaskTool) Execute(arguments map[string]interface{}, clientToken string, apiClient *shared.APIClient) (string, error) {
	projectID, err := shared.GetProjectIDWithFallback(arguments, apiClient, clientToken)
	if err != nil {
		return "", fmt.Errorf("get_next_incomplete_task: %w", err)
	}

	path := fmt.Sprintf("/api/v1/projects/%s/tasks/next-incomplete", projectID)

	// 如果指定了 doc_type 参数，添加到查询字符串
	if docType, exists := arguments["doc_type"]; exists && docType != nil {
		if dt, ok := docType.(string); ok && dt != "" {
			path += "?doc_type=" + dt
		}
	}

	return shared.CallAPI(apiClient, "GET", path, nil, clientToken)
}
