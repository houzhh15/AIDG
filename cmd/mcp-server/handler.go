package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/houzhh15-hub/AIDG/cmd/mcp-server/shared"
	"github.com/houzhh15-hub/AIDG/cmd/mcp-server/tools"
)

// NewMCPHandler 创建新的 MCP Handler 实例并注册所有工具
func NewMCPHandler(apiClient *shared.APIClient) *MCPHandler {
	registry := NewToolRegistry()
	slotRegistry := shared.NewSlotRegistry()

	// 用户工具 (2个)
	registry.Register(&tools.GetUserCurrentTaskTool{})
	registry.Register(&tools.SetUserCurrentTaskTool{})

	// 执行计划工具 (4个)
	registry.Register(&tools.GetExecutionPlanTool{})
	registry.Register(&tools.UpdateExecutionPlanTool{})
	registry.Register(&tools.GetNextExecutableStepTool{})
	registry.Register(&tools.UpdatePlanStepStatusTool{})

	// 会议列表工具 (1个)
	registry.Register(&tools.ListAllMeetingsTool{})

	// ===== 通用文档工具 (7个) =====
	// 任务文档通用工具 (3个)
	registry.Register(&tools.GetTaskDocumentTool{Registry: slotRegistry})
	registry.Register(&tools.UpdateTaskDocumentTool{Registry: slotRegistry})
	registry.Register(&tools.AppendTaskDocumentTool{Registry: slotRegistry})

	// 会议文档通用工具 (2个)
	registry.Register(&tools.GetMeetingDocumentTool{Registry: slotRegistry})
	registry.Register(&tools.UpdateMeetingDocumentTool{Registry: slotRegistry})

	// 项目文档通用工具 (2个)
	registry.Register(&tools.GetProjectDocumentTool{Registry: slotRegistry})
	registry.Register(&tools.UpdateProjectDocumentTool{Registry: slotRegistry})

	// 任务管理工具 (7个)
	registry.Register(&tools.ListProjectTasksTool{})
	registry.Register(&tools.CreateProjectTaskTool{})
	registry.Register(&tools.GetProjectTaskTool{})
	registry.Register(&tools.UpdateProjectTaskTool{})
	registry.Register(&tools.DeleteProjectTaskTool{})
	registry.Register(&tools.GetProjectTaskPromptsTool{})
	registry.Register(&tools.CreateProjectTaskPromptTool{})

	// 章节管理工具 (6个)
	registry.Register(&tools.GetTaskDocSectionsTool{})
	registry.Register(&tools.GetTaskDocSectionTool{})
	registry.Register(&tools.UpdateTaskDocSectionTool{})
	registry.Register(&tools.InsertTaskDocSectionTool{})
	registry.Register(&tools.DeleteTaskDocSectionTool{})
	registry.Register(&tools.SyncTaskDocSectionsTool{})

	// 项目进展和任务总结工具 (3个)
	registry.Register(&tools.ProgressSummaryTool{})
	registry.Register(&tools.TaskSummaryTool{})
	registry.Register(&tools.UpdateProgressTool{})

	log.Printf("✅ [REGISTRY] 已注册 %d 个工具", len(registry.List()))

	// 初始化 Prompts 管理器
	promptManager := NewPromptManager()

	return &MCPHandler{
		apiClient:     apiClient,
		registry:      registry,
		promptManager: promptManager,
	}
}

// 从HTTP请求中提取token
func (h *MCPHandler) extractTokenFromRequest(r *http.Request) string {
	// 1. Authorization: Bearer token
	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Bearer ") {
			return strings.TrimPrefix(auth, "Bearer ")
		}
	}

	// 2. X-MCP-Token
	if mcpToken := r.Header.Get("X-MCP-Token"); mcpToken != "" {
		return mcpToken
	}

	// 3. X-Auth-Token
	if authToken := r.Header.Get("X-Auth-Token"); authToken != "" {
		return authToken
	}

	return ""
}

// ServeHTTP 实现 http.Handler 接口
func (h *MCPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 调试信息：打印所有相关的token头部
	authHeader := r.Header.Get("Authorization")
	mcpToken := r.Header.Get("X-MCP-Token")
	authToken := r.Header.Get("X-Auth-Token")

	log.Printf("🔍 [DEBUG] 接收到请求: %s %s", r.Method, r.URL.Path)
	if authHeader != "" {
		log.Printf("🔑 [DEBUG] Authorization头部: %s", authHeader)
	}
	if mcpToken != "" {
		log.Printf("🔑 [DEBUG] X-MCP-Token头部: %s", mcpToken)
	}
	if authToken != "" {
		log.Printf("🔑 [DEBUG] X-Auth-Token头部: %s", authToken)
	}
	if authHeader == "" && mcpToken == "" && authToken == "" {
		log.Printf("⚠️  [DEBUG] 未找到任何token头部")
	}

	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Auth-Token, X-MCP-Token")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method == "GET" {
		// SSE endpoint for Claude Desktop
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.WriteHeader(http.StatusOK)

		// Send keep-alive ping every 30 seconds to maintain connection
		fmt.Fprintf(w, "data: {\"jsonrpc\":\"2.0\",\"method\":\"notifications/message\",\"params\":{\"message\":\"SSE connection established\"}}\n\n")
		w.(http.Flusher).Flush()

		flusher, ok := w.(http.Flusher)
		if !ok {
			log.Println("[SSE] flusher not supported, closing")
			return
		}
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-r.Context().Done():
				log.Println("[SSE] client disconnected")
				return
			case t := <-ticker.C:
				// SSE comment line as heartbeat (ignored by clients but keeps connection active)
				if _, err := fmt.Fprintf(w, ": keepalive %s\n\n", t.Format(time.RFC3339)); err != nil {
					log.Printf("[SSE] write heartbeat failed: %v", err)
					return
				}
				flusher.Flush()
			}
		}
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}

	var mcpReq struct {
		Jsonrpc string                 `json:"jsonrpc"`
		ID      interface{}            `json:"id"`
		Method  string                 `json:"method"`
		Params  map[string]interface{} `json:"params"`
	}

	if err := json.Unmarshal(body, &mcpReq); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Handle MCP protocol methods
	switch mcpReq.Method {
	case "initialize":
		h.handleInitialize(w, mcpReq)
		return

	case "tools/list":
		h.handleToolsList(w, mcpReq)
		return

	case "tools/call":
		h.handleToolsCall(w, mcpReq, r)
		return

	case "prompts/list":
		h.handlePromptsList(w, mcpReq)
		return

	case "prompts/get":
		h.handlePromptsGet(w, mcpReq)
		return

	default:
		h.sendErrorResponse(w, mcpReq.ID, -32601, "Method not found", nil)
	}
}

// handleInitialize 处理 MCP 初始化请求
func (h *MCPHandler) handleInitialize(w http.ResponseWriter, req struct {
	Jsonrpc string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params"`
}) {
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      req.ID,
		"result": map[string]interface{}{
			"protocolVersion": "2025-06-18",
			"capabilities": map[string]interface{}{
				"tools":   map[string]interface{}{},
				"prompts": map[string]interface{}{},
			},
			"serverInfo": map[string]interface{}{
				"name":    "Meeting Recorder MCP Server V2",
				"version": "0.0.6",
			},
		},
	}
	json.NewEncoder(w).Encode(response)
}

// handleToolsList 处理工具列表请求
func (h *MCPHandler) handleToolsList(w http.ResponseWriter, req struct {
	Jsonrpc string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params"`
}) {
	tools := h.registry.List()
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      req.ID,
		"result": map[string]interface{}{
			"tools": tools,
		},
	}
	json.NewEncoder(w).Encode(response)
}

// handleToolsCall 处理工具调用请求
func (h *MCPHandler) handleToolsCall(w http.ResponseWriter, req struct {
	Jsonrpc string                 `json:"jsonrpc"`
	ID      interface{}            `json:"id"`
	Method  string                 `json:"method"`
	Params  map[string]interface{} `json:"params"`
}, r *http.Request) {
	// 添加panic恢复机制
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ [PANIC] 工具调用发生panic: %v", r)
			h.sendErrorResponse(w, req.ID, -32603, "Internal server error", fmt.Sprintf("Panic occurred: %v", r))
		}
	}()

	name, ok := req.Params["name"].(string)
	if !ok {
		log.Printf("⚠️  [TOOL] 工具名称无效或缺失")
		h.sendErrorResponse(w, req.ID, -32602, "Invalid params", "Missing or invalid tool name")
		return
	}

	arguments, ok := req.Params["arguments"].(map[string]interface{})
	if !ok && req.Params["arguments"] != nil {
		log.Printf("⚠️  [TOOL] 参数格式无效: %T", req.Params["arguments"])
		h.sendErrorResponse(w, req.ID, -32602, "Invalid params", "Arguments must be an object")
		return
	}

	if arguments == nil {
		arguments = make(map[string]interface{})
	}

	// 从请求中提取token
	clientToken := h.extractTokenFromRequest(r)

	log.Printf("🔧 [TOOL] 处理工具调用: %s", name)
	if clientToken != "" {
		log.Printf("🔑 [TOOL] 使用客户端token: %s (前20字符)", clientToken[:min(20, len(clientToken))])
	}

	// 使用 ToolRegistry 执行工具
	result, err := h.registry.Execute(name, arguments, clientToken, h.apiClient)
	if err != nil {
		log.Printf("❌ [TOOL] 工具调用失败: %s, 错误: %v", name, err)
		h.sendErrorResponse(w, req.ID, -32603, "Tool execution error", err.Error())
		return
	}

	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      req.ID,
		"result": map[string]interface{}{
			"content": []map[string]interface{}{
				{
					"type": "text",
					"text": result,
				},
			},
		},
	}
	json.NewEncoder(w).Encode(response)
}

// handleDebugClientInfo 处理调试信息收集
func (h *MCPHandler) handleDebugClientInfo(r *http.Request) string {
	// 收集调试信息
	debugInfo := map[string]interface{}{
		"method":      r.Method,
		"url":         r.URL.String(),
		"remote_addr": r.RemoteAddr,
		"user_agent":  r.UserAgent(),
	}

	// 检查HTTP headers
	headers := make(map[string][]string)
	for k, v := range r.Header {
		headers[k] = v
	}
	debugInfo["headers"] = headers
	debugInfo["headers_count"] = len(r.Header)

	// 检查认证相关的头部
	authHeaders := []string{"Authorization", "Bearer", "Token", "X-Auth-Token", "X-API-Key", "X-MCP-Token"}
	foundAuth := make(map[string][]string)
	for _, authHeader := range authHeaders {
		if vals := r.Header[authHeader]; len(vals) > 0 {
			// 只显示前20个字符以保护隐私
			maskedVals := make([]string, len(vals))
			for i, val := range vals {
				if len(val) > 20 {
					maskedVals[i] = val[:20] + "..."
				} else {
					maskedVals[i] = val
				}
			}
			foundAuth[authHeader] = maskedVals
		}
		// 也检查小写版本
		lowerHeader := strings.ToLower(authHeader)
		if vals := r.Header[lowerHeader]; len(vals) > 0 {
			maskedVals := make([]string, len(vals))
			for i, val := range vals {
				if len(val) > 20 {
					maskedVals[i] = val[:20] + "..."
				} else {
					maskedVals[i] = val
				}
			}
			foundAuth[lowerHeader] = maskedVals
		}
	}
	debugInfo["auth_headers"] = foundAuth

	// 检查环境变量中的token信息
	envTokens := map[string]string{
		"MCP_BEARER_TOKEN": os.Getenv("MCP_BEARER_TOKEN"),
		"MCP_USERNAME":     os.Getenv("MCP_USERNAME"),
		"MCP_PASSWORD":     os.Getenv("MCP_PASSWORD"),
		"MCP_MODE":         os.Getenv("MCP_MODE"),
		"MCP_HTTP_PORT":    os.Getenv("MCP_HTTP_PORT"),
	}
	// 只显示前10个字符以保护隐私
	for k, v := range envTokens {
		if v != "" {
			if len(v) > 10 && k != "MCP_MODE" && k != "MCP_HTTP_PORT" {
				envTokens[k] = v[:10] + "..."
			}
		} else {
			envTokens[k] = "(not set)"
		}
	}
	debugInfo["env_tokens"] = envTokens

	// 获取并显示实际使用的token
	actualToken := h.extractTokenFromRequest(r)
	if actualToken != "" {
		debugInfo["extracted_token"] = maskToken(actualToken)
	} else {
		debugInfo["extracted_token"] = "(none)"
	}

	// 将结果编码为JSON
	result, err := json.MarshalIndent(debugInfo, "", "  ")
	if err != nil {
		return fmt.Sprintf("Error marshaling debug info: %v", err)
	}

	return string(result)
}

// ===== Prompts 协议方法 =====

// handlePromptsList 处理 prompts/list 请求
func (h *MCPHandler) handlePromptsList(w http.ResponseWriter, req MCPRequest) {
	// 调用 PromptManager 获取模版列表
	prompts, err := h.promptManager.ListPrompts()
	if err != nil {
		h.sendErrorResponse(w, req.ID, -32603, fmt.Sprintf("加载模版失败: %v", err), nil)
		return
	}

	// 构造 MCP 响应
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      req.ID,
		"result": map[string]interface{}{
			"prompts": prompts,
		},
	}

	json.NewEncoder(w).Encode(response)
}

// handlePromptsGet 处理 prompts/get 请求
func (h *MCPHandler) handlePromptsGet(w http.ResponseWriter, req MCPRequest) {
	// 提取 name 参数
	name, ok := req.Params["name"].(string)
	if !ok || name == "" {
		h.sendErrorResponse(w, req.ID, -32602, "缺少参数: name", nil)
		return
	}

	// 提取 arguments 参数（可选）
	args := make(map[string]string)
	if argsRaw, ok := req.Params["arguments"].(map[string]interface{}); ok {
		for k, v := range argsRaw {
			if strVal, ok := v.(string); ok {
				args[k] = strVal
			}
		}
	}

	// 调用 PromptManager 获取模版
	result, err := h.promptManager.GetPrompt(name, args)
	if err != nil {
		// 根据错误类型返回不同的错误码
		errMsg := err.Error()
		if strings.Contains(errMsg, "模版不存在") {
			h.sendErrorResponse(w, req.ID, -32602, errMsg, nil)
		} else if strings.Contains(errMsg, "缺少必填参数") {
			h.sendErrorResponse(w, req.ID, -32602, errMsg, nil)
		} else {
			h.sendErrorResponse(w, req.ID, -32603, errMsg, nil)
		}
		return
	}

	// 构造 MCP 响应
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      req.ID,
		"result":  result,
	}

	json.NewEncoder(w).Encode(response)
}
