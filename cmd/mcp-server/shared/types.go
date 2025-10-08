package shared

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

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
