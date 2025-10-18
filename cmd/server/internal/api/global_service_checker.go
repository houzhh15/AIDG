package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
)

// GlobalServiceChecker 全局服务状态检查器
// 不依赖于任何会议任务或 orchestrator，直接检查服务端点
type GlobalServiceChecker struct {
	whisperURL     string
	depsServiceURL string
	httpClient     *http.Client
}

// NewGlobalServiceChecker 创建全局服务检查器
func NewGlobalServiceChecker() *GlobalServiceChecker {
	whisperURL := os.Getenv("WHISPER_API_URL")
	if whisperURL == "" {
		whisperURL = "http://whisper:80"
	}

	depsServiceURL := os.Getenv("DEPS_SERVICE_URL")
	if depsServiceURL == "" {
		depsServiceURL = "http://aidg-deps-service:8080"
	}

	return &GlobalServiceChecker{
		whisperURL:     whisperURL,
		depsServiceURL: depsServiceURL,
		httpClient: &http.Client{
			Timeout: 2 * time.Second,
		},
	}
}

// CheckWhisperHealth 检查 Whisper 服务健康状态
func (g *GlobalServiceChecker) CheckWhisperHealth(ctx context.Context) (bool, error) {
	// go-whisper 容器使用 /v1/models 端点来检查可用性
	// 如果返回 200，说明服务正常；如果连接失败或 500 错误，说明不可用
	checkURL := fmt.Sprintf("%s/v1/models", g.whisperURL)

	req, err := http.NewRequestWithContext(ctx, "GET", checkURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// 200-299 状态码认为服务可用
	// 404 说明服务在运行但端点不存在，也算可用（至少能连接）
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true, nil
	}

	// 404 不一定意味着服务不可用，可能只是这个端点不存在
	// 但如果是 500+ 或连接失败，才认为不可用
	if resp.StatusCode >= 500 {
		return false, fmt.Errorf("server error: status code %d", resp.StatusCode)
	}

	// 对于 404 等其他状态码，尝试简单的连接测试
	// 如果能连接到根路径，说明服务至少在运行
	checkURL = fmt.Sprintf("%s/", g.whisperURL)
	req2, _ := http.NewRequestWithContext(ctx, "GET", checkURL, nil)
	resp2, err2 := g.httpClient.Do(req2)
	if err2 != nil {
		return false, fmt.Errorf("service unreachable: %w", err2)
	}
	defer resp2.Body.Close()

	// 只要能连接并返回响应（即使是404），就认为服务可用
	return true, nil
}

// CheckDepsServiceHealth 检查 deps-service 健康状态
func (g *GlobalServiceChecker) CheckDepsServiceHealth(ctx context.Context) (bool, error) {
	// deps-service 提供 /api/v1/health 端点
	checkURL := fmt.Sprintf("%s/api/v1/health", g.depsServiceURL)

	req, err := http.NewRequestWithContext(ctx, "GET", checkURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// 检查状态码
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return true, nil
	}

	return false, fmt.Errorf("unhealthy status code: %d", resp.StatusCode)
}

// CheckAllServices 并发检查所有服务
func (g *GlobalServiceChecker) CheckAllServices(ctx context.Context) ServicesStatusResponse {
	status := ServicesStatusResponse{
		WhisperAvailable:     false,
		DepsServiceAvailable: false,
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(2)

	// 并发检查 Whisper
	go func() {
		defer wg.Done()
		healthy, err := g.CheckWhisperHealth(ctx)

		mu.Lock()
		defer mu.Unlock()

		status.WhisperAvailable = healthy
		if healthy {
			status.WhisperMode = "available"
		} else {
			status.WhisperMode = fmt.Sprintf("unavailable: %v", err)
		}

		slog.Info("[GlobalServiceChecker] Whisper health check",
			"available", healthy,
			"error", err)
	}()

	// 并发检查 DepsService
	go func() {
		defer wg.Done()
		healthy, err := g.CheckDepsServiceHealth(ctx)

		mu.Lock()
		defer mu.Unlock()

		status.DepsServiceAvailable = healthy
		if healthy {
			status.DependencyMode = "available"
		} else {
			status.DependencyMode = fmt.Sprintf("unavailable: %v", err)
		}

		slog.Info("[GlobalServiceChecker] DepsService health check",
			"available", healthy,
			"error", err)
	}()

	wg.Wait()
	return status
}
