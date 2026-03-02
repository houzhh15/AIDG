package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	copydomain "github.com/houzhh15/AIDG/cmd/server/internal/domain/copy"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/meetings"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/projects"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/remotes"
)

// CopyHandler handles cross-system resource copy operations
type CopyHandler struct {
	remoteSvc   *remotes.Service
	meetingsReg *meetings.Registry
	projectsReg *projects.ProjectRegistry
	secret      string
}

// NewCopyHandler creates a CopyHandler
func NewCopyHandler(remoteSvc *remotes.Service, meetingsReg *meetings.Registry, projectsReg *projects.ProjectRegistry) *CopyHandler {
	secret := os.Getenv("SYNC_SHARED_SECRET")
	if secret == "" {
		secret = "neteye@123"
	}
	return &CopyHandler{
		remoteSvc:   remoteSvc,
		meetingsReg: meetingsReg,
		projectsReg: projectsReg,
		secret:      secret,
	}
}

// HandleCopyPush POST /api/v1/copy/push
// Pushes selected resources to a remote AIDG system
func (h *CopyHandler) HandleCopyPush(c *gin.Context) {
	var req copydomain.CopyPushRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Determine target URL and secret
	targetURL := req.RemoteURL
	targetSecret := h.secret
	if req.RemoteID != "" {
		remote, ok := h.remoteSvc.Get(req.RemoteID)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "remote not found: " + req.RemoteID})
			return
		}
		targetURL = remote.URL
		targetSecret = remote.Secret
	}
	if targetURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target URL required (via remote_id or remote_url)"})
		return
	}

	mode := req.Mode
	if mode == "" {
		mode = copydomain.ModeOverwrite
	}

	// Collect resources
	payloads := make([]copydomain.ResourcePayload, 0, len(req.Resources))
	for _, res := range req.Resources {
		payload, err := copydomain.CollectResource(res, h.meetingsReg, h.projectsReg, req.Options)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error(), "resource": res})
			return
		}
		payloads = append(payloads, *payload)
	}

	// Build signed envelope
	sig := copydomain.MakeSignature(payloads, mode, targetSecret)
	envelope := copydomain.CopyEnvelope{
		Timestamp: time.Now().Unix(),
		Signature: sig,
		Mode:      mode,
		Resources: payloads,
	}

	// Send to remote
	bodyBytes, err := json.Marshal(envelope)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "marshal envelope: " + err.Error()})
		return
	}

	endpoint := strings.TrimSuffix(targetURL, "/") + "/api/v1/copy/receive"
	httpReq, _ := http.NewRequest("POST", endpoint, strings.NewReader(string(bodyBytes)))
	httpReq.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 120 * time.Second} // longer timeout for large payloads
	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "push failed: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": "remote error", "body": string(respBody)})
		return
	}

	var remoteResp copydomain.CopyReceiveResponse
	json.Unmarshal(respBody, &remoteResp)

	c.JSON(http.StatusOK, gin.H{
		"pushed_resources": len(payloads),
		"remote_response":  remoteResp,
	})
}

// HandleCopyReceive POST /api/v1/copy/receive
// Receives resources from another AIDG system
func (h *CopyHandler) HandleCopyReceive(c *gin.Context) {
	var envelope copydomain.CopyEnvelope
	if err := c.ShouldBindJSON(&envelope); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bad_request: " + err.Error()})
		return
	}

	// Verify HMAC signature
	if !copydomain.VerifySignature(envelope.Resources, envelope.Mode, envelope.Signature, h.secret) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid signature"})
		return
	}

	mode := envelope.Mode
	if mode == "" {
		mode = copydomain.ModeOverwrite
	}

	results := make([]copydomain.CopyResult, 0, len(envelope.Resources))
	summary := copydomain.CopyResultSummary{Total: len(envelope.Resources)}

	for _, res := range envelope.Resources {
		result, err := copydomain.WriteResource(res, mode, h.meetingsReg, h.projectsReg)
		if err != nil {
			results = append(results, copydomain.CopyResult{
				Type:   res.Type,
				ID:     res.ID,
				Status: "error",
				Error:  err.Error(),
			})
			summary.Errors++
			continue
		}
		results = append(results, *result)

		switch result.Status {
		case "created":
			summary.Created++
		case "updated":
			summary.Updated++
		case "skipped":
			summary.Skipped++
		}
	}

	// Trigger registry reload for meetings if any meeting resources were received
	needMeetingReload := false
	needProjectReload := false
	for _, r := range envelope.Resources {
		if r.Type == "meeting" {
			needMeetingReload = true
		}
		if r.Type == "project" || r.Type == "task" {
			needProjectReload = true
		}
	}

	if needMeetingReload {
		newReg := meetings.NewRegistry()
		if err := meetings.LoadTasks(newReg); err == nil {
			// Copy entries from newly loaded registry
			for _, t := range newReg.List() {
				h.meetingsReg.Set(t)
			}
		}
	}
	if needProjectReload {
		newReg := projects.NewProjectRegistry()
		if err := projects.LoadProjects(newReg); err == nil {
			for _, p := range newReg.List() {
				h.projectsReg.Set(p)
			}
		}
	}

	c.JSON(http.StatusOK, copydomain.CopyReceiveResponse{
		Success:   summary.Errors == 0,
		Resources: results,
		Summary:   summary,
	})
}

// HandleCopyResources GET /api/v1/copy/resources
// Returns a list of locally available resources for remote browsing
func (h *CopyHandler) HandleCopyResources(c *gin.Context) {
	meetingList := make([]copydomain.RemoteResourceInfo, 0)
	for _, t := range h.meetingsReg.List() {
		meetingList = append(meetingList, copydomain.RemoteResourceInfo{
			ID:   t.ID,
			Name: t.ID,
			Type: "meeting",
		})
	}

	projectList := make([]copydomain.RemoteResourceInfo, 0)
	for _, p := range h.projectsReg.List() {
		projectList = append(projectList, copydomain.RemoteResourceInfo{
			ID:   p.ID,
			Name: p.Name,
			Type: "project",
		})
	}

	c.JSON(http.StatusOK, copydomain.RemoteResourceListResponse{
		Meetings: meetingList,
		Projects: projectList,
	})
}
