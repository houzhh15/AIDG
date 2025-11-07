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
	mux.Handle("/mcp", recoverWrap(NewMCPHandler(c)))
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
