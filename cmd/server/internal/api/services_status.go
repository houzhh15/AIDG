package api

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15-hub/AIDG/cmd/server/internal/orchestrator"
)

// ServicesStatusResponse 服务状态响应
// WhisperAvailable 和 DepsServiceAvailable 根据部署模式使用不同的检查方法：
// - 服务模式：实时健康检查
// - CLI/Local 模式：文件存在性检查
type ServicesStatusResponse struct {
	WhisperAvailable     bool   `json:"whisper_available"`         // Whisper服务当前是否可用
	DepsServiceAvailable bool   `json:"deps_service_available"`    // 依赖服务当前是否可用
	WhisperMode          string `json:"whisper_mode,omitempty"`    // Whisper 检查模式和状态
	DependencyMode       string `json:"dependency_mode,omitempty"` // 依赖服务检查模式和状态
}

// HandleServicesStatus 返回当前服务的部署状态
// GET /api/v1/services/status
//
// 根据部署模式使用不同的可用性检查方法：
// - WHISPER_MODE=cli: 检查 WHISPER_PROGRAM_PATH 文件存在性
// - DEPENDENCY_MODE=local: 检查 DIARIZATION_SCRIPT_PATH 和 EMBEDDING_SCRIPT_PATH 文件存在性
// - 其他模式: 执行实时健康检查（响应时间约1-2秒）
//
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
			// 即使 orchestrator 为 nil，也要根据环境变量设置模式状态
			whisperMode := strings.ToLower(strings.TrimSpace(os.Getenv("WHISPER_MODE")))
			dependencyMode := strings.ToLower(strings.TrimSpace(os.Getenv("DEPENDENCY_MODE")))

			if whisperMode == "cli" {
				whisperPath := strings.TrimSpace(os.Getenv("WHISPER_PROGRAM_PATH"))
				if whisperPath != "" {
					if _, err := os.Stat(whisperPath); err == nil {
						status.WhisperAvailable = true
						status.WhisperMode = "cli_available"
					} else {
						status.WhisperAvailable = false
						status.WhisperMode = "cli_unavailable"
					}
				} else {
					status.WhisperAvailable = false
					status.WhisperMode = "cli_no_path"
				}
			} else {
				status.WhisperAvailable = false
				status.WhisperMode = "service_unavailable"
			}

			if dependencyMode == "local" {
				diarizationPath := strings.TrimSpace(os.Getenv("DIARIZATION_SCRIPT_PATH"))
				embeddingPath := strings.TrimSpace(os.Getenv("EMBEDDING_SCRIPT_PATH"))

				diarizationExists := diarizationPath != "" && (func() bool {
					_, err := os.Stat(diarizationPath)
					return err == nil
				})()
				embeddingExists := embeddingPath != "" && (func() bool {
					_, err := os.Stat(embeddingPath)
					return err == nil
				})()

				if diarizationExists && embeddingExists {
					status.DepsServiceAvailable = true
					status.DependencyMode = "local_available"
				} else {
					status.DepsServiceAvailable = false
					status.DependencyMode = "local_unavailable"
				}
			} else {
				status.DepsServiceAvailable = false
				status.DependencyMode = "service_unavailable"
			}

			c.JSON(http.StatusOK, status)
			return
		}

		// 获取环境变量以确定检查模式
		whisperMode := strings.ToLower(strings.TrimSpace(os.Getenv("WHISPER_MODE")))
		dependencyMode := strings.ToLower(strings.TrimSpace(os.Getenv("DEPENDENCY_MODE")))

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
			if whisperMode == "cli" {
				// CLI 模式：检查 WHISPER_PROGRAM_PATH 文件存在
				whisperPath := strings.TrimSpace(os.Getenv("WHISPER_PROGRAM_PATH"))
				if whisperPath != "" {
					if _, err := os.Stat(whisperPath); err == nil {
						mu.Lock()
						status.WhisperAvailable = true
						status.WhisperMode = "cli_available"
						mu.Unlock()
						slog.Info("[ServicesStatus] Whisper CLI check passed", "path", whisperPath)
					} else {
						mu.Lock()
						status.WhisperAvailable = false
						status.WhisperMode = "cli_unavailable"
						mu.Unlock()
						slog.Warn("[ServicesStatus] Whisper CLI check failed", "path", whisperPath, "error", err)
					}
				} else {
					mu.Lock()
					status.WhisperAvailable = false
					status.WhisperMode = "cli_no_path"
					mu.Unlock()
					slog.Warn("[ServicesStatus] WHISPER_PROGRAM_PATH not set for CLI mode")
				}
			} else {
				// 其他模式：使用健康检查
				if checker := orch.GetHealthChecker(); checker != nil {
					healthStatus := checker.GetStatus()
					mu.Lock()
					status.WhisperAvailable = healthStatus.IsHealthy
					if healthStatus.IsHealthy {
						status.WhisperMode = "service_available"
					} else {
						status.WhisperMode = "service_unavailable"
					}
					mu.Unlock()

					slog.Info("[ServicesStatus] Whisper health check",
						"available", healthStatus.IsHealthy,
						"consecutive_fails", healthStatus.ConsecutiveFails,
						"error", healthStatus.ErrorMessage)
				} else {
					slog.Warn("[ServicesStatus] HealthChecker is nil")
				}
			}
		}()

		// 并行检查 deps-service
		go func() {
			defer wg.Done()
			if dependencyMode == "local" {
				// Local 模式：检查脚本文件存在
				diarizationPath := strings.TrimSpace(os.Getenv("DIARIZATION_SCRIPT_PATH"))
				embeddingPath := strings.TrimSpace(os.Getenv("EMBEDDING_SCRIPT_PATH"))

				diarizationExists := false
				embeddingExists := false

				if diarizationPath != "" {
					if _, err := os.Stat(diarizationPath); err == nil {
						diarizationExists = true
					} else {
						slog.Warn("[ServicesStatus] Diarization script not found", "path", diarizationPath, "error", err)
					}
				} else {
					slog.Warn("[ServicesStatus] DIARIZATION_SCRIPT_PATH not set for local mode")
				}

				if embeddingPath != "" {
					if _, err := os.Stat(embeddingPath); err == nil {
						embeddingExists = true
					} else {
						slog.Warn("[ServicesStatus] Embedding script not found", "path", embeddingPath, "error", err)
					}
				} else {
					slog.Warn("[ServicesStatus] EMBEDDING_SCRIPT_PATH not set for local mode")
				}

				isAvailable := diarizationExists && embeddingExists
				mu.Lock()
				status.DepsServiceAvailable = isAvailable
				if isAvailable {
					status.DependencyMode = "local_available"
				} else {
					status.DependencyMode = "local_unavailable"
				}
				mu.Unlock()

				slog.Info("[ServicesStatus] Local dependency check",
					"available", isAvailable,
					"diarization_script", diarizationExists,
					"embedding_script", embeddingExists)
			} else {
				// 其他模式：使用健康检查
				if client := orch.GetDependencyClient(); client != nil {
					err := client.HealthCheck(ctx)
					isAvailable := (err == nil)

					mu.Lock()
					status.DepsServiceAvailable = isAvailable
					if isAvailable {
						status.DependencyMode = "service_available"
					} else {
						status.DependencyMode = "service_unavailable"
					}
					mu.Unlock()

					slog.Info("[ServicesStatus] DepsService health check",
						"available", isAvailable,
						"error", err)
				} else {
					slog.Warn("[ServicesStatus] DependencyClient is nil")
				}
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
