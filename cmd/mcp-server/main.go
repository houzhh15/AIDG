package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/houzhh15/AIDG/cmd/mcp-server/config"
)

var startTime = time.Now()

func main() {
	// 加载配置
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 验证配置
	if err := config.ValidateConfig(cfg); err != nil {
		log.Fatalf("Invalid config: %v", err)
	}

	// 创建 API 客户端
	c := NewAPIClient(cfg.Backend.ServerURL)

	// 打印启动信息
	log.Printf("=== MCP Server V2 ===")
	log.Printf("Environment: %s", cfg.Server.Environment)
	log.Printf("HTTP Port: %d", cfg.Server.HTTPPort)
	log.Printf("Backend URL: %s", cfg.Backend.ServerURL)
	log.Printf("Auth Configured: %v", cfg.HasAuth())
	log.Printf("Server URL: http://localhost:%d", cfg.Server.HTTPPort)
	log.Printf("MCP Endpoint: http://localhost:%d/mcp", cfg.Server.HTTPPort)
	log.Printf("Health Check: http://localhost:%d/health", cfg.Server.HTTPPort)
	log.Printf("=====================")

	mux := http.NewServeMux()

	// 创建 MCP Handler
	mcpHandler := NewMCPHandler(c)

	// 启动触发文件监控 goroutine（用于检测 Prompts 变更）
	go watchPromptsChanges(mcpHandler)

	// MCP 端点支持 POST 和 GET（SSE）
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v", rec)
				http.Error(w, "Internal Server Error", 500)
			}
		}()

		switch r.Method {
		case http.MethodPost:
			// JSON-RPC 请求/响应
			mcpHandler.ServeHTTP(w, r)
		case http.MethodGet:
			// SSE 流 - 用于接收服务器通知
			handleSSEStream(w, r, mcpHandler)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/health", healthCheckHandler(cfg))
	mux.HandleFunc("/readiness", readinessCheckHandler(cfg))

	addr := cfg.GetServerAddress()
	srv := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		log.Printf("Starting MCP Server on %s...", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Printf("Shutdown signal received, shutting down MCP server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("MCP Server forced to shutdown: %v", err)
	}
	log.Printf("MCP Server shutdown complete")
}

// watchPromptsChanges 监控触发文件并发送 SSE 通知
func watchPromptsChanges(handler *MCPHandler) {
	triggerFilePath := "data/.prompts_changed"
	checkInterval := 2 * time.Second

	log.Printf("🔍 [PROMPTS] 启动触发文件监控: %s (检查间隔: %v)", triggerFilePath, checkInterval)

	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for range ticker.C {
		// 检查触发文件是否存在
		if _, err := os.Stat(triggerFilePath); err == nil {
			// 触发文件存在，删除它并发送通知
			if err := os.Remove(triggerFilePath); err != nil {
				log.Printf("⚠️  [PROMPTS] 删除触发文件失败: %v", err)
				continue
			}

			log.Printf("📢 [PROMPTS] 检测到触发文件，广播 prompts/list_changed 通知")

			// 通过 NotificationHub 广播通知
			handler.NotificationHub.BroadcastPromptsChanged()

			// 同时清空 PromptManager 的缓存
			handler.PromptManager.InvalidateCache()
		}
	}
}

// HealthCheckResponse 健康检查响应
type HealthCheckResponse struct {
	Status           string    `json:"status"`
	Service          string    `json:"service"`
	Version          string    `json:"version"`
	Uptime           string    `json:"uptime"`
	Timestamp        time.Time `json:"timestamp"`
	BackendURL       string    `json:"backend_url"`
	BackendReachable bool      `json:"backend_reachable"`
	AuthConfigured   bool      `json:"auth_configured"`
}

// healthCheckHandler 健康检查处理器
func healthCheckHandler(cfg *config.MCPConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 检查后端可达性
		backendReachable := checkBackendReachability(cfg.Backend.ServerURL)

		response := HealthCheckResponse{
			Status:           "healthy",
			Service:          "mcp-server",
			Version:          "2.0.0",
			Uptime:           time.Since(startTime).String(),
			Timestamp:        time.Now(),
			BackendURL:       cfg.Backend.ServerURL,
			BackendReachable: backendReachable,
			AuthConfigured:   cfg.HasAuth(),
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
	}
}

// ReadinessCheckResponse 就绪检查响应
type ReadinessCheckResponse struct {
	Status           string    `json:"status"`
	Service          string    `json:"service"`
	Timestamp        time.Time `json:"timestamp"`
	BackendURL       string    `json:"backend_url"`
	BackendReachable bool      `json:"backend_reachable"`
	AuthConfigured   bool      `json:"auth_configured"`
}

// readinessCheckHandler 就绪检查处理器
func readinessCheckHandler(cfg *config.MCPConfig) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		backendReachable := checkBackendReachability(cfg.Backend.ServerURL)

		status := "ready"
		if !backendReachable {
			status = "degraded"
		}

		response := ReadinessCheckResponse{
			Status:           status,
			Service:          "mcp-server",
			Timestamp:        time.Now(),
			BackendURL:       cfg.Backend.ServerURL,
			BackendReachable: backendReachable,
			AuthConfigured:   cfg.HasAuth(),
		}

		httpStatus := http.StatusOK
		if !backendReachable {
			httpStatus = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(httpStatus)
		json.NewEncoder(w).Encode(response)
	}
}

// checkBackendReachability 检查后端服务是否可达
func checkBackendReachability(backendURL string) bool {
	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Get(fmt.Sprintf("%s/api/v1/health", backendURL))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// handleSSEStream 处理 SSE 流连接，用于服务器到客户端的通知
func handleSSEStream(w http.ResponseWriter, r *http.Request, handler *MCPHandler) {
	// 设置 SSE 响应头
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// 检查是否支持 Flusher
	flusher, ok := w.(http.Flusher)
	if !ok {
		log.Printf("SSE: Response writer does not support flushing")
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	log.Printf("SSE: Client connected from %s", r.RemoteAddr)

	// 创建客户端通道
	clientChan := make(chan interface{}, 10)

	// 注册客户端到通知中心
	handler.NotificationHub.RegisterSSEClient(clientChan)
	defer handler.NotificationHub.UnregisterSSEClient(clientChan)

	// 发送连接成功消息
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\"}\n\n")
	flusher.Flush()

	// 创建上下文以检测客户端断开
	ctx := r.Context()

	// 心跳 ticker
	heartbeatTicker := time.NewTicker(30 * time.Second)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			// 客户端断开连接
			log.Printf("SSE: Client disconnected from %s", r.RemoteAddr)
			return

		case <-heartbeatTicker.C:
			// 发送心跳
			fmt.Fprintf(w, ": heartbeat\n\n")
			flusher.Flush()

		case notification := <-clientChan:
			// 发送通知
			switch n := notification.(type) {
			case string:
				// 通知类型标识
				if n == "prompts_changed" {
					// 发送 MCP 标准通知（根据规范，不需要 params 字段）
					notificationJSON := `{"jsonrpc":"2.0","method":"notifications/prompts/list_changed"}`
					fmt.Fprintf(w, "event: notification\ndata: %s\n\n", notificationJSON)
					flusher.Flush()
					log.Printf("SSE: Sent prompts/list_changed notification to %s", r.RemoteAddr)
				}
			}
		}
	}
}

func recoverWrap(h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("panic: %v", rec)
				http.Error(w, "Internal Server Error", 500)
			}
		}()
		h.ServeHTTP(w, r)
	}
}
