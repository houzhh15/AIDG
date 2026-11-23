package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/projects"
)

// HandleTransferProjectTask rejects the legacy multi-project move. A transfer
// mutates two task repositories plus a document tree and cannot participate in
// the v2 single-project lifecycle/review transaction. A future migration API
// must provide its own revision-bound, recoverable transaction.
func HandleTransferProjectTask(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		sourceProjectID := c.Param("id")
		taskID := c.Param("task_id")
		var req struct {
			TargetProjectID string `json:"target_project_id"`
		}
		if err := BindDocumentWrite(c, &req); err != nil || req.TargetProjectID == "" {
			badRequestResponse(c, "invalid request body: target_project_id is required")
			return
		}
		if sourceProjectID == req.TargetProjectID {
			badRequestResponse(c, "cannot transfer task to the same project")
			return
		}
		if reg.Get(sourceProjectID) == nil {
			notFoundResponse(c, "source project not found")
			return
		}
		if reg.Get(req.TargetProjectID) == nil {
			notFoundResponse(c, "target project not found")
			return
		}
		if _, err := taskLifecycleService().Get(sourceProjectID, taskID); err != nil {
			handleTaskLifecycleError(c, err)
			return
		}
		c.JSON(http.StatusConflict, gin.H{
			"error":   "task_transfer_not_supported",
			"message": "task transfer requires an explicit versioned migration operation",
		})
	}
}
