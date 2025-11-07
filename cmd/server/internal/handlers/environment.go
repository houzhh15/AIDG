package handlers

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/houzhh15/AIDG/cmd/server/internal/orchestrator"
)

// EnvironmentHandler 处理环境检查相关的 HTTP 请求
type EnvironmentHandler struct {
	cachedStatus *orchestrator.EnvironmentStatus
	mutex        sync.RWMutex
}

// NewEnvironmentHandler 创建新的环境检查处理器
func NewEnvironmentHandler() *EnvironmentHandler {
	return &EnvironmentHandler{}
}

// GetStatus 处理 GET /api/v1/environment/status 请求
// 支持 force=true 查询参数强制重新检查
func (h *EnvironmentHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	force := r.URL.Query().Get("force") == "true"

	h.mutex.Lock()
	if force || h.cachedStatus == nil {
		h.cachedStatus = orchestrator.CheckEnvironment()
	}
	status := h.cachedStatus
	h.mutex.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(status); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
