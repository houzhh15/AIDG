package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15-hub/AIDG/cmd/server/internal/orchestrator"
)

// ServicesStatusResponse 服务状态响应
type ServicesStatusResponse struct {
	WhisperAvailable     bool   `json:"whisper_available"`
	DepsServiceAvailable bool   `json:"deps_service_available"`
	WhisperMode          string `json:"whisper_mode,omitempty"`
	DependencyMode       string `json:"dependency_mode,omitempty"`
}

// HandleServicesStatus 返回当前服务的部署状态
// GET /api/v1/services/status
func HandleServicesStatus(orch *orchestrator.Orchestrator) gin.HandlerFunc {
	return func(c *gin.Context) {
		status := ServicesStatusResponse{
			WhisperAvailable:     false,
			DepsServiceAvailable: false,
		}

		// 如果 orchestrator 存在，检查服务状态
		if orch != nil {
			// 通过反射或公共方法获取配置
			// 由于 cfg 是私有字段，我们需要通过其他方式判断服务状态

			// 检查 Whisper 服务 - 通过健康检查器判断
			if orch.GetHealthChecker() != nil {
				status.WhisperAvailable = true
			}

			// 检查 deps-service 状态 - 通过依赖客户端判断
			if orch.GetDependencyClient() != nil {
				status.DepsServiceAvailable = true
			}
		}

		c.JSON(http.StatusOK, status)
	}
}
