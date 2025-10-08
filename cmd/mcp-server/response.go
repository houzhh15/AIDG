package main

import (
	"encoding/json"
	"net/http"
)

// response.go - 响应处理
// 从 main.go 提取的响应构建和发送函数

// sendErrorResponse 发送 JSON-RPC 2.0 错误响应
// 参数:
//   - w: HTTP响应writer
//   - id: 请求ID
//   - code: 错误代码（JSON-RPC 2.0标准错误码）
//   - message: 错误消息
//   - data: 可选的额外错误数据
func (h *MCPHandler) sendErrorResponse(w http.ResponseWriter, id interface{}, code int, message string, data interface{}) {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	}
	if data != nil {
		response["error"].(map[string]interface{})["data"] = data
	}
	json.NewEncoder(w).Encode(response)
}

// sendError 发送工具执行错误响应（MCP协议格式）
// 参数:
//   - w: HTTP响应writer
//   - id: 请求ID
//   - msg: 错误消息
func (h *MCPHandler) sendError(w http.ResponseWriter, id string, msg string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]interface{}{
			"isError": true,
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": msg,
				},
			},
		},
	})
}

// sendSuccess 发送工具执行成功响应（MCP协议格式）
// 参数:
//   - w: HTTP响应writer
//   - id: 请求ID
//   - text: 成功响应文本
func (h *MCPHandler) sendSuccess(w http.ResponseWriter, id string, text string) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result": map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": text,
				},
			},
		},
	})
}
