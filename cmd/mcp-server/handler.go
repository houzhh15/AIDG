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

	"github.com/golang-jwt/jwt/v5"
	"github.com/houzhh15/AIDG/cmd/mcp-server/shared"
	"github.com/houzhh15/AIDG/cmd/mcp-server/tools"
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

	// ===== 通用文档工具 (9个) =====
	// 任务文档通用工具 (3个)
	registry.Register(&tools.GetTaskDocumentTool{Registry: slotRegistry})
	registry.Register(&tools.UpdateTaskDocumentTool{Registry: slotRegistry})
	registry.Register(&tools.AppendTaskDocumentTool{Registry: slotRegistry})

	// 会议文档通用工具 (2个)
	registry.Register(&tools.GetMeetingDocumentTool{Registry: slotRegistry})
	registry.Register(&tools.UpdateMeetingDocumentTool{Registry: slotRegistry})

	// 会议章节管理工具 (2个)
	registry.Register(&tools.GetMeetingDocSectionsTool{})
	registry.Register(&tools.UpdateMeetingDocSectionTool{})
	// SyncMeetingDocSectionsTool - 暂时屏蔽，后端 docslot 服务未实现 sync endpoint
	// registry.Register(&tools.SyncMeetingDocSectionsTool{})

	// 项目文档通用工具 (2个)
	registry.Register(&tools.GetProjectDocumentTool{Registry: slotRegistry})
	registry.Register(&tools.UpdateProjectDocumentTool{Registry: slotRegistry})

	// === 以下工具暂时屏蔽（代码保留但不注册） ===
	// 多层级文档内容工具 (2个) - 暂时屏蔽
	// registry.Register(&tools.ReadDocumentContentTool{})
	// registry.Register(&tools.WriteDocumentContentTool{})

	// 多层级文档结构工具 (3个) - 暂时屏蔽
	// registry.Register(&tools.GetHierarchicalDocumentsTool{})
	// registry.Register(&tools.AnalyzeDocumentRelationshipsTool{})
	// registry.Register(&tools.ManageDocumentReferenceTool{})

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

	// 项目进展和任务总结工具 - 暂时屏蔽 progress_summary 和 update_progress
	// registry.Register(&tools.ProgressSummaryTool{})
	registry.Register(&tools.TaskSummaryTool{})
	// registry.Register(&tools.UpdateProgressTool{})

	log.Printf("✅ [REGISTRY] 已注册 %d 个工具", len(registry.List()))

	// 初始化 Prompts 管理器
	promptManager := NewPromptManager()

	// 初始化通知中心
	notificationHub := NewNotificationHub()

	return &MCPHandler{
		apiClient:       apiClient,
		registry:        registry,
		PromptManager:   promptManager,
		NotificationHub: notificationHub,
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

// getUsernameFromToken 从 JWT token 中解析用户名
// 参数:
//   - token: JWT token 字符串
//
// 返回:
//   - string: 解析出的用户名
//   - error: token 无效或解析失败时返回错误
func (h *MCPHandler) getUsernameFromToken(token string) (string, error) {
	if token == "" {
		return "", fmt.Errorf("token is empty")
	}

	// 直接解析 JWT token 获取用户名
	// 使用与 Web Server 相同的密钥
	secret := os.Getenv("USER_JWT_SECRET")
	if secret == "" {
		secret = "dev-user-jwt-secret-at-least-32-chars" // 默认开发密钥
	}

	parsed, err := jwt.ParseWithClaims(token, &jwt.MapClaims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil {
		return "", fmt.Errorf("failed to parse token: %w", err)
	}

	if !parsed.Valid {
		return "", fmt.Errorf("invalid token")
	}

	claims, ok := parsed.Claims.(*jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid claims")
	}

	username, ok := (*claims)["username"].(string)
	if !ok || username == "" {
		return "", fmt.Errorf("username not found in token")
	}

	return username, nil
} // handleResourcesList 处理 resources/list 请求
// 参数:
//   - w: HTTP 响应写入器
//   - req: MCP 请求对象
//   - r: 原始 HTTP 请求
//
// 功能:
//   - 提取 Authorization Bearer token
//   - 调用 getUsernameFromToken 解析用户名
//   - 调用 Web Server API 获取资源列表
//   - 转换为 MCP 协议格式并返回
func (h *MCPHandler) handleResourcesList(w http.ResponseWriter, req MCPRequest, r *http.Request) {
	// 1. 提取 token
	clientToken := h.extractTokenFromRequest(r)
	if clientToken == "" {
		h.sendErrorResponse(w, req.ID, -32602, "Missing authentication token", nil)
		return
	}

	// 2. 解析 token 获取 username
	username, err := h.getUsernameFromToken(clientToken)
	if err != nil {
		h.sendErrorResponse(w, req.ID, -32602, "Invalid token", err.Error())
		return
	}

	// 3. 调用 Web Server API
	url := fmt.Sprintf("/api/v1/users/%s/resources", username)
	resp, err := h.apiClient.MakeRequestWithToken("GET", url, nil, clientToken)
	if err != nil {
		h.sendErrorResponse(w, req.ID, -32603, "Failed to fetch resources", err.Error())
		return
	}

	// 4. 解析响应
	var apiResp struct {
		Success bool `json:"success"`
		Data    []struct {
			URI         string `json:"uri"`
			Name        string `json:"name"`
			Description string `json:"description"`
			MimeType    string `json:"mime_type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		h.sendErrorResponse(w, req.ID, -32603, "Failed to parse response", err.Error())
		return
	}

	// 5. 转换为 MCP 协议格式
	resources := make([]map[string]interface{}, len(apiResp.Data))
	for i, r := range apiResp.Data {
		resources[i] = map[string]interface{}{
			"uri":         r.URI,
			"name":        r.Name,
			"description": r.Description,
			"mimeType":    r.MimeType,
		}
	}

	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      req.ID,
		"result": map[string]interface{}{
			"resources": resources,
		},
	}
	json.NewEncoder(w).Encode(response)
}

// handleResourcesRead 处理 resources/read 请求
// 参数:
//   - w: HTTP 响应写入器
//   - req: MCP 请求对象
//   - r: 原始 HTTP 请求
//
// 功能:
//   - 提取 URI 参数并校验
//   - 调用 parseResourceURI 解析资源类型
//   - 根据类型调用不同 Web Server API
//   - 构造 MCP 协议格式并返回
func (h *MCPHandler) handleResourcesRead(w http.ResponseWriter, req MCPRequest, r *http.Request) {
	// 1. 提取 URI 参数
	uri, ok := req.Params["uri"].(string)
	if !ok || uri == "" {
		h.sendErrorResponse(w, req.ID, -32602, "Missing or invalid URI parameter", nil)
		return
	}

	// 2. 解析 URI
	parsedURI, err := parseResourceURI(uri)
	if err != nil {
		h.sendErrorResponse(w, req.ID, -32602, "Invalid URI format", err.Error())
		return
	}

	// 3. 提取 token
	clientToken := h.extractTokenFromRequest(r)

	// 4. 根据 URI 类型调用不同的 API
	var content string
	var mimeType string

	switch parsedURI.Type {
	case "task_document":
		url := fmt.Sprintf("/api/v1/projects/%s/tasks/%s/%s/export",
			parsedURI.ProjectID, parsedURI.TaskID, parsedURI.DocType)
		resp, err := h.apiClient.MakeRequestWithToken("GET", url, nil, clientToken)
		if err != nil {
			h.sendErrorResponse(w, req.ID, -32603, "Failed to fetch task document", err.Error())
			return
		}
		var docResp struct {
			Content string `json:"content"`
			Version int    `json:"version"`
			ETag    string `json:"etag"`
		}
		if err := json.Unmarshal(resp, &docResp); err != nil {
			h.sendErrorResponse(w, req.ID, -32603, "Failed to parse document response", err.Error())
			return
		}
		content = docResp.Content
		mimeType = "text/markdown"

	case "project_document":
		// DocType 已经是 API 格式（连字符），直接使用
		url := fmt.Sprintf("/api/v1/projects/%s/%s",
			parsedURI.ProjectID, parsedURI.DocType)
		resp, err := h.apiClient.MakeRequestWithToken("GET", url, nil, clientToken)
		if err != nil {
			h.sendErrorResponse(w, req.ID, -32603, "Failed to fetch project document", err.Error())
			return
		}
		var docResp struct {
			Content string `json:"content"`
			Exists  bool   `json:"exists"`
		}
		if err := json.Unmarshal(resp, &docResp); err != nil {
			h.sendErrorResponse(w, req.ID, -32603, "Failed to parse document response", err.Error())
			return
		}
		content = docResp.Content
		mimeType = "text/markdown"

	case "legacy_document":
		// 引用文档：从Web Server获取内容
		// 使用内部API (如果存在) 或者让Web Server处理文件读取
		url := fmt.Sprintf("/api/v1/projects/%s/legacy-documents/%s",
			parsedURI.ProjectID, parsedURI.DocID)
		resp, err := h.apiClient.MakeRequestWithToken("GET", url, nil, clientToken)
		if err != nil {
			h.sendErrorResponse(w, req.ID, -32603, "Failed to fetch legacy document", err.Error())
			return
		}
		var docResp struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(resp, &docResp); err != nil {
			h.sendErrorResponse(w, req.ID, -32603, "Failed to parse document response", err.Error())
			return
		}
		content = docResp.Content
		mimeType = "text/markdown"

	case "custom_resource":
		url := fmt.Sprintf("/api/v1/users/%s/resources/%s",
			parsedURI.Username, parsedURI.ResourceID)
		resp, err := h.apiClient.MakeRequestWithToken("GET", url, nil, clientToken)
		if err != nil {
			h.sendErrorResponse(w, req.ID, -32603, "Failed to fetch custom resource", err.Error())
			return
		}
		var resResp struct {
			Success bool `json:"success"`
			Data    struct {
				Content  string `json:"content"`
				MimeType string `json:"mime_type"`
			} `json:"data"`
		}
		if err := json.Unmarshal(resp, &resResp); err != nil {
			h.sendErrorResponse(w, req.ID, -32603, "Failed to parse resource response", err.Error())
			return
		}
		content = resResp.Data.Content
		mimeType = resResp.Data.MimeType

	default:
		h.sendErrorResponse(w, req.ID, -32602, "Unsupported resource type", nil)
		return
	}

	// 5. 返回 MCP 协议格式
	response := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      req.ID,
		"result": map[string]interface{}{
			"contents": []map[string]interface{}{
				{
					"uri":      uri,
					"mimeType": mimeType,
					"text":     content,
				},
			},
		},
	}
	json.NewEncoder(w).Encode(response)
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
		h.handlePromptsList(w, mcpReq, r)
		return

	case "prompts/get":
		h.handlePromptsGet(w, mcpReq, r)
		return

	case "resources/list":
		h.handleResourcesList(w, mcpReq, r)
		return

	case "resources/read":
		h.handleResourcesRead(w, mcpReq, r)
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
				"tools": map[string]interface{}{},
				"prompts": map[string]interface{}{
					"listChanged": true, // 支持 prompts list_changed 通知
				},
				"resources": map[string]interface{}{
					"subscribe":   false,
					"listChanged": false,
				},
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
func (h *MCPHandler) handlePromptsList(w http.ResponseWriter, req MCPRequest, r *http.Request) {
	// 1. 提取 token
	clientToken := h.extractTokenFromRequest(r)

	// 2. 解析 username
	username := ""
	if clientToken != "" {
		if parsedUsername, err := h.getUsernameFromToken(clientToken); err == nil {
			username = parsedUsername
		}
	}

	// 3. 获取当前任务信息（projectID 和 taskID）
	projectID := ""
	taskID := ""
	if username != "" {
		// 调用后端 API 获取用户当前任务（不需要 username 路径参数）
		url := "/api/v1/user/current-task"
		resultStr, err := shared.CallAPI(h.apiClient, "GET", url, nil, clientToken)
		if err == nil {
			var result map[string]interface{}
			if err := json.Unmarshal([]byte(resultStr), &result); err == nil {
				if data, ok := result["data"].(map[string]interface{}); ok {
					if pid, ok := data["project_id"].(string); ok {
						projectID = pid
					}
					if tid, ok := data["task_id"].(string); ok {
						taskID = tid
					}
				}
			}
		}
	}

	// 4. 调用 PromptManager 获取模版列表（包含动态 Prompts）
	var prompts []PromptMetadata
	var err error

	if username != "" {
		// 使用 GetUserPrompts 合并静态+动态 Prompts
		prompts, err = h.PromptManager.GetUserPrompts(username, projectID, taskID)
	} else {
		// 未登录用户只显示静态 Prompts
		prompts, err = h.PromptManager.ListPrompts()
	}

	if err != nil {
		h.sendErrorResponse(w, req.ID, -32603, fmt.Sprintf("加载模版失败: %v", err), nil)
		return
	}

	log.Printf("📋 [PROMPTS] 返回 %d 个 Prompts (username=%s, project=%s, task=%s)",
		len(prompts), username, projectID, taskID)

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
func (h *MCPHandler) handlePromptsGet(w http.ResponseWriter, req MCPRequest, r *http.Request) {
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
	result, err := h.PromptManager.GetPrompt(name, args)
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
