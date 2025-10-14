package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15-hub/AIDG/cmd/server/internal/orchestrator/degradation"
	"github.com/houzhh15-hub/AIDG/cmd/server/internal/orchestrator/health"
)

// HandleWhisperHealthCheck 创建Whisper健康检查的HTTP处理函数
// 参数:
//
//	degradationCtrl: 降级控制器实例 (从orchestrator获取)
//	healthChecker: 健康检查器实例 (从orchestrator获取)
//
// 返回:
//
//	gin.HandlerFunc: 可以注册到路由的处理函数
//
// 响应格式:
//
//	{
//	  "success": true,
//	  "data": {
//	    "implementation": "go-whisper",
//	    "is_healthy": true,
//	    "is_degraded": false,
//	    "last_check_time": "2025-10-11T02:20:00Z",
//	    "consecutive_fails": 0,
//	    "error_message": ""
//	  }
//	}
func HandleWhisperHealthCheck(
	degradationCtrl *degradation.DegradationController,
	healthChecker *health.HealthChecker,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 检查依赖是否为nil (服务未启动或未初始化)
		if degradationCtrl == nil || healthChecker == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"success": false,
				"error":   "Whisper service not initialized",
			})
			return
		}

		// 获取当前使用的Transcriber
		currentTranscriber := degradationCtrl.GetTranscriber()
		implementation := currentTranscriber.Name()

		// 获取健康状态
		status := healthChecker.GetStatus()

		// 获取降级状态
		isDegraded := degradationCtrl.IsDegraded()

		// 构造响应
		response := gin.H{
			"success": true,
			"data": gin.H{
				"implementation":    implementation,
				"is_healthy":        status.IsHealthy,
				"is_degraded":       isDegraded,
				"last_check_time":   status.LastCheckTime,
				"consecutive_fails": status.ConsecutiveFails,
				"error_message":     status.ErrorMessage,
			},
		}

		// 返回JSON响应
		c.JSON(http.StatusOK, response)
	}
}
