package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/remotes"
)

// RemoteHandler handles remote AIDG peer management
type RemoteHandler struct {
	service *remotes.Service
}

// NewRemoteHandler creates a new RemoteHandler
func NewRemoteHandler(svc *remotes.Service) *RemoteHandler {
	return &RemoteHandler{service: svc}
}

// HandleListRemotes GET /api/v1/remotes
func (h *RemoteHandler) HandleListRemotes(c *gin.Context) {
	list := h.service.List()
	c.JSON(http.StatusOK, gin.H{"remotes": list})
}

// HandleGetRemote GET /api/v1/remotes/:id
func (h *RemoteHandler) HandleGetRemote(c *gin.Context) {
	id := c.Param("id")
	r, ok := h.service.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "remote not found"})
		return
	}
	c.JSON(http.StatusOK, r.Safe())
}

// HandleCreateRemote POST /api/v1/remotes
func (h *RemoteHandler) HandleCreateRemote(c *gin.Context) {
	var req remotes.CreateRemoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	r, err := h.service.Create(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, r.Safe())
}

// HandleUpdateRemote PUT /api/v1/remotes/:id
func (h *RemoteHandler) HandleUpdateRemote(c *gin.Context) {
	id := c.Param("id")
	var req remotes.UpdateRemoteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	r, err := h.service.Update(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, r.Safe())
}

// HandleDeleteRemote DELETE /api/v1/remotes/:id
func (h *RemoteHandler) HandleDeleteRemote(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.Delete(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// HandleTestRemote POST /api/v1/remotes/:id/test
func (h *RemoteHandler) HandleTestRemote(c *gin.Context) {
	id := c.Param("id")
	r, ok := h.service.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "remote not found"})
		return
	}
	result := h.service.TestConnection(r.URL)
	c.JSON(http.StatusOK, result)
}

// HandleTestRemoteURL POST /api/v1/remotes/test-url
// Tests connectivity to an arbitrary URL (for quick test before saving)
func (h *RemoteHandler) HandleTestRemoteURL(c *gin.Context) {
	var req struct {
		URL string `json:"url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result := h.service.TestConnection(req.URL)
	c.JSON(http.StatusOK, result)
}
