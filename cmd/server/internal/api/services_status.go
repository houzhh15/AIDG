package api

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15-hub/AIDG/cmd/server/internal/orchestrator"
)

// ServicesStatusResponse 服务状态响应
// WhisperAvailable 和 DepsServiceAvailable 反映实时健康检查结果，而非对象存在性
type ServicesStatusResponse struct {
	WhisperAvailable     bool   `json:"whisper_available"`      // Whisper服务当前是否可用（实时检查）
	DepsServiceAvailable bool   `json:"deps_service_available"` // 依赖服务当前是否可用（实时检查）
	WhisperMode          string `json:"whisper_mode,omitempty"`
	DependencyMode       string `json:"dependency_mode,omitempty"`
}

// HandleServicesStatus 返回当前服务的部署状态
// GET /api/v1/services/status
//
// 本接口执行实时健康检查以确定服务可用性，响应时间约1-2秒。
// 建议前端适当缓存结果，避免频繁调用。
func HandleServicesStatus(orch *orchestrator.Orchestrator) gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		status := ServicesStatusResponse{
			WhisperAvailable:     false,
			DepsServiceAvailable: false,
		}

		if orch == nil {
			slog.Warn("[ServicesStatus] Orchestrator is nil")
			c.JSON(http.StatusOK, status)
			return
		}

		// 创建1秒超时的context用于健康检查
		ctx, cancel := context.WithTimeout(c.Request.Context(), 1*time.Second)
		defer cancel()

		// 使用 mutex 保护并发访问 status 结构体
		var mu sync.Mutex
		var wg sync.WaitGroup
		wg.Add(2)

		// 并行检查 Whisper 服务
		go func() {
			defer wg.Done()
			if checker := orch.GetHealthChecker(); checker != nil {
				healthStatus := checker.GetStatus()
				mu.Lock()
				status.WhisperAvailable = healthStatus.IsHealthy
				if healthStatus.IsHealthy {
					status.WhisperMode = "available"
				} else {
					status.WhisperMode = "unavailable"
				}
				mu.Unlock()

				slog.Info("[ServicesStatus] Whisper health check",
					"available", healthStatus.IsHealthy,
					"consecutive_fails", healthStatus.ConsecutiveFails,
					"error", healthStatus.ErrorMessage)
			} else {
				slog.Warn("[ServicesStatus] HealthChecker is nil")
			}
		}()

		// 并行检查 deps-service
		go func() {
			defer wg.Done()
			if client := orch.GetDependencyClient(); client != nil {
				err := client.HealthCheck(ctx)
				isAvailable := (err == nil)

				mu.Lock()
				status.DepsServiceAvailable = isAvailable
				if isAvailable {
					status.DependencyMode = "available"
				} else {
					status.DependencyMode = "unavailable"
				}
				mu.Unlock()

				slog.Info("[ServicesStatus] DepsService health check",
					"available", isAvailable,
					"error", err)
			} else {
				slog.Warn("[ServicesStatus] DependencyClient is nil")
			}
		}()

		// 等待所有检查完成
		wg.Wait()

		duration := time.Since(startTime)
		slog.Info("[ServicesStatus] Health check completed",
			"whisper_available", status.WhisperAvailable,
			"deps_service_available", status.DepsServiceAvailable,
			"duration_ms", duration.Milliseconds())

		c.JSON(http.StatusOK, status)
	}
}
