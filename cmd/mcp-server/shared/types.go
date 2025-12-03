package shared

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// CurrentTaskInfo 当前任务信息结构
type CurrentTaskInfo struct {
	ProjectID string `json:"project_id"`
	TaskID    string `json:"task_id"`
}

// GetCurrentTask 获取当前用户的任务信息
// 调用后端 API: GET /api/v1/user/current-task
func GetCurrentTask(apiClient *APIClient, clientToken string) (*CurrentTaskInfo, error) {
	resp, err := CallAPI(apiClient, "GET", "/api/v1/user/current-task", nil, clientToken)
	if err != nil {
		return nil, fmt.Errorf("获取当前任务失败: %w", err)
	}

	var result CurrentTaskInfo
	if err := json.Unmarshal([]byte(resp), &result); err != nil {
		return nil, fmt.Errorf("解析当前任务响应失败: %w", err)
	}

	return &result, nil
}

// GetProjectIDWithFallback 获取 project_id，如果参数缺失或为空则从当前任务获取
// 参数:
//   - args: 工具参数 map
//   - apiClient: API 客户端
//   - clientToken: 认证 token
//
// 返回:
//   - string: project_id 值
//   - error: 如果既无参数又无法获取当前任务则返回错误
func GetProjectIDWithFallback(args map[string]interface{}, apiClient *APIClient, clientToken string) (string, error) {
	// 先尝试从参数获取
	projectID, err := SafeGetString(args, "project_id")
	if err == nil && projectID != "" {
		return projectID, nil
	}

	// 参数缺失或为空，尝试从当前任务获取
	currentTask, err := GetCurrentTask(apiClient, clientToken)
	if err != nil {
		return "", fmt.Errorf("参数 project_id 缺失，且无法获取当前任务: %w", err)
	}

	if currentTask.ProjectID == "" {
		return "", fmt.Errorf("参数 project_id 缺失，且当前任务未设置 project_id")
	}

	return currentTask.ProjectID, nil
}

// GetTaskIDWithFallback 获取 task_id，如果参数缺失或为空则从当前任务获取
// 参数:
//   - args: 工具参数 map
//   - apiClient: API 客户端
//   - clientToken: 认证 token
//
// 返回:
//   - string: task_id 值
//   - error: 如果既无参数又无法获取当前任务则返回错误
func GetTaskIDWithFallback(args map[string]interface{}, apiClient *APIClient, clientToken string) (string, error) {
	// 先尝试从参数获取
	taskID, err := SafeGetString(args, "task_id")
	if err == nil && taskID != "" {
		return taskID, nil
	}

	// 参数缺失或为空，尝试从当前任务获取
	currentTask, err := GetCurrentTask(apiClient, clientToken)
	if err != nil {
		return "", fmt.Errorf("参数 task_id 缺失，且无法获取当前任务: %w", err)
	}

	if currentTask.TaskID == "" {
		return "", fmt.Errorf("参数 task_id 缺失，且当前任务未设置 task_id")
	}

	return currentTask.TaskID, nil
}

// GetProjectAndTaskIDWithFallback 同时获取 project_id 和 task_id，支持从当前任务回退
// 这是一个优化版本，只调用一次 API 来获取当前任务（如果需要回退的话）
// 参数:
//   - args: 工具参数 map
//   - apiClient: API 客户端
//   - clientToken: 认证 token
//
// 返回:
//   - projectID: project_id 值
//   - taskID: task_id 值
//   - error: 如果任一参数缺失且无法从当前任务获取则返回错误
func GetProjectAndTaskIDWithFallback(args map[string]interface{}, apiClient *APIClient, clientToken string) (projectID string, taskID string, err error) {
	// 先尝试从参数获取
	projectID, projectErr := SafeGetString(args, "project_id")
	taskID, taskErr := SafeGetString(args, "task_id")

	// 如果两个参数都存在且有效，直接返回
	if projectErr == nil && projectID != "" && taskErr == nil && taskID != "" {
		return projectID, taskID, nil
	}

	// 至少有一个参数需要从当前任务获取
	currentTask, err := GetCurrentTask(apiClient, clientToken)
	if err != nil {
		missingParams := []string{}
		if projectErr != nil || projectID == "" {
			missingParams = append(missingParams, "project_id")
		}
		if taskErr != nil || taskID == "" {
			missingParams = append(missingParams, "task_id")
		}
		return "", "", fmt.Errorf("参数 %s 缺失，且无法获取当前任务: %w", strings.Join(missingParams, ", "), err)
	}

	// 用当前任务的值填充缺失的参数
	if projectErr != nil || projectID == "" {
		if currentTask.ProjectID == "" {
			return "", "", fmt.Errorf("参数 project_id 缺失，且当前任务未设置 project_id")
		}
		projectID = currentTask.ProjectID
	}

	if taskErr != nil || taskID == "" {
		if currentTask.TaskID == "" {
			return "", "", fmt.Errorf("参数 task_id 缺失，且当前任务未设置 task_id")
		}
		taskID = currentTask.TaskID
	}

	return projectID, taskID, nil
}

// APIClient API客户端结构
type APIClient struct {
	BaseURL string
	Client  *http.Client
}

// Tool 工具接口，所有工具必须实现此接口
type Tool interface {
	Name() string
	Description() string
	InputSchema() map[string]interface{}
	Execute(args map[string]interface{}, clientToken string, apiClient *APIClient) (string, error)
}

const (
	DEFAULT_SERVER_BASE_URL = "http://localhost:8000"
)

// MaskToken 对 token 进行脱敏处理
func MaskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "***" + token[len(token)-4:]
}

// MakeJSONBody 将 map 序列化为 JSON 并转换为 io.Reader
func MakeJSONBody(body interface{}) (io.Reader, error) {
	if body == nil {
		return nil, nil
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal body: %w", err)
	}
	return strings.NewReader(string(data)), nil
}

// CallAPI 是工具调用 API 的统一辅助函数
func CallAPI(apiClient *APIClient, method, path string, body interface{}, token string) (string, error) {
	reader, err := MakeJSONBody(body)
	if err != nil {
		return "", err
	}
	resp, err := apiClient.MakeRequestWithToken(method, path, reader, token)
	if err != nil {
		return "", err
	}
	return string(resp), nil
}

// MakeRequestWithToken 使用指定token发起HTTP请求
func (c *APIClient) MakeRequestWithToken(method, path string, body io.Reader, token string) ([]byte, error) {
	url := c.BaseURL + path
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	// 使用传入的 token
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(data))
	}

	return data, nil
}

// SafeGetString 从参数 map 中安全获取字符串值
func SafeGetString(args map[string]interface{}, key string) (string, error) {
	val, exists := args[key]
	if !exists {
		return "", fmt.Errorf("missing required parameter: %s", key)
	}

	if val == nil {
		return "", fmt.Errorf("parameter %s is nil", key)
	}

	str, ok := val.(string)
	if !ok {
		return "", fmt.Errorf("parameter %s must be a string, got %T", key, val)
	}

	if str == "" {
		return "", fmt.Errorf("parameter %s cannot be empty", key)
	}

	return str, nil
}

// SafeGetInt 从参数 map 中安全获取整数值
func SafeGetInt(args map[string]interface{}, key string) (int, error) {
	val, exists := args[key]
	if !exists {
		return 0, fmt.Errorf("missing required parameter: %s", key)
	}

	if val == nil {
		return 0, fmt.Errorf("parameter %s is nil", key)
	}

	// JSON数字通常解析为float64
	switch v := val.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case int64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("parameter %s must be a number, got %T", key, val)
	}
}

// SafeGetBool 从参数 map 中安全获取布尔值
func SafeGetBool(args map[string]interface{}, key string) (bool, error) {
	val, exists := args[key]
	if !exists {
		return false, fmt.Errorf("missing required parameter: %s", key)
	}

	if val == nil {
		return false, fmt.Errorf("parameter %s is nil", key)
	}

	b, ok := val.(bool)
	if !ok {
		return false, fmt.Errorf("parameter %s must be a boolean, got %T", key, val)
	}

	return b, nil
}
