package main

import (
	"fmt"
	"strings"
)

// utils.go - 辅助函数
// 从 main.go 提取的各种辅助函数

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// safeGetString 从参数 map 中安全获取字符串值
// 参数:
//   - args: 参数map
//   - key: 参数键名
//
// 返回:
//   - string: 字符串值
//   - error: 如果参数缺失、类型错误或为空
func safeGetString(args map[string]interface{}, key string) (string, error) {
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

// safeGetInt 从参数 map 中安全获取整数值
// 参数:
//   - args: 参数map
//   - key: 参数键名
//
// 返回:
//   - int: 整数值
//   - error: 如果参数缺失、类型错误
//
// 注意: JSON数字通常解析为float64，此函数会自动转换
func safeGetInt(args map[string]interface{}, key string) (int, error) {
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

// safeGetBool 从参数 map 中安全获取布尔值
// 参数:
//   - args: 参数map
//   - key: 参数键名
//
// 返回:
//   - bool: 布尔值
//   - error: 如果参数缺失、类型错误
func safeGetBool(args map[string]interface{}, key string) (bool, error) {
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

// maskToken 对 token 进行脱敏处理
// 参数:
//   - token: 原始 token 字符串
//
// 返回:
//   - string: 脱敏后的 token（显示前4位和后4位，中间用***代替）
//
// 示例: "abcd1234efgh5678" -> "abcd***5678"
func maskToken(token string) string {
	if token == "" {
		return ""
	}
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "***" + token[len(token)-4:]
}

// ResourceURI 解析后的 aidg:// 协议 URI 结构
// 支持三种资源类型:
//   - task_document: aidg://project/{project_id}/task/{task_id}/{doc_type}
//   - project_document: aidg://project/{project_id}/{doc_type}
//   - custom_resource: aidg://user/{username}/custom/{resource_id}
type ResourceURI struct {
	Type       string // "task_document", "project_document", "legacy_document", "custom_resource"
	ProjectID  string
	TaskID     string
	DocType    string
	DocID      string // for legacy_document
	Username   string
	ResourceID string
}

// parseResourceURI 解析 aidg:// 协议的 URI
// 参数:
//   - uri: URI 字符串
//
// 返回:
//   - *ResourceURI: 解析后的 URI 结构
//   - error: 解析失败时返回错误
func parseResourceURI(uri string) (*ResourceURI, error) {
	const prefix = "aidg://"
	if !strings.HasPrefix(uri, prefix) {
		return nil, fmt.Errorf("invalid URI scheme, must start with 'aidg://'")
	}

	path := strings.TrimPrefix(uri, prefix)
	parts := strings.Split(path, "/")

	result := &ResourceURI{}

	// aidg://project/{project_id}/task/{task_id}/{doc_type}
	// aidg://project/{project_id}/document/{doc_id}
	// aidg://project/{project_id}/{doc_type}
	// aidg://user/{username}/custom/{resource_id}
	if len(parts) >= 2 && parts[0] == "project" {
		result.ProjectID = parts[1]
		if len(parts) == 5 && parts[2] == "task" {
			// task document
			result.Type = "task_document"
			result.TaskID = parts[3]
			result.DocType = parts[4]
		} else if len(parts) == 4 && parts[2] == "document" {
			// legacy document (reference document)
			result.Type = "legacy_document"
			result.DocID = parts[3]
		} else if len(parts) == 3 {
			// project document
			result.Type = "project_document"
			result.DocType = parts[2]
		} else {
			return nil, fmt.Errorf("invalid project URI format: expected 3, 4, or 5 parts, got %d", len(parts))
		}
	} else if len(parts) == 4 && parts[0] == "user" && parts[2] == "custom" {
		// custom resource
		result.Type = "custom_resource"
		result.Username = parts[1]
		result.ResourceID = parts[3]
	} else {
		return nil, fmt.Errorf("unsupported URI format: %s", uri)
	}

	return result, nil
}
