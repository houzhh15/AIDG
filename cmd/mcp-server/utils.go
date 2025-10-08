package main

import (
	"fmt"
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
