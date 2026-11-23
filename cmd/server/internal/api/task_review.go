package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/projects"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/tasklifecycle"
)

func HandleGetTaskReview(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID, taskID := c.Param("id"), c.Param("task_id")
		if reg.Get(projectID) == nil {
			notFoundResponse(c, "project not found")
			return
		}
		review, err := taskLifecycleService().Review(projectID, taskID)
		if err != nil {
			handleTaskLifecycleError(c, err)
			return
		}
		if !review.Exists {
			c.JSON(http.StatusNotFound, gin.H{"error": "not_found", "message": "review not found"})
			return
		}
		c.Header("ETag", review.ETag)
		c.JSON(http.StatusOK, gin.H{"success": true, "data": review})
	}
}

func HandleCreateTaskReview(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID, taskID := c.Param("id"), c.Param("task_id")
		if reg.Get(projectID) == nil {
			notFoundResponse(c, "project not found")
			return
		}
		actor := c.GetString("user")
		var req tasklifecycle.ReviewRequest
		if err := BindDocumentWrite(c, &req); err != nil {
			badRequestResponse(c, "invalid request body")
			return
		}
		task, review, err := taskLifecycleService().RequestReview(projectID, taskID, actor, req)
		if err != nil {
			handleTaskLifecycleError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"task": task, "review": review}})
	}
}

// HandlePutTaskReview accepts the fixed Mote AIDG Lite review document. It is a
// dedicated review operation, not a generic document write.
func HandlePutTaskReview(reg *projects.ProjectRegistry) gin.HandlerFunc {
	type request struct {
		Content          string `json:"content"`
		ExpectedRevision *int   `json:"expected_revision,omitempty"`
	}
	return func(c *gin.Context) {
		projectID, taskID := c.Param("id"), c.Param("task_id")
		if reg.Get(projectID) == nil {
			notFoundResponse(c, "project not found")
			return
		}
		actor := c.GetString("user")
		var req request
		if err := BindDocumentWrite(c, &req); err != nil {
			badRequestResponse(c, "invalid request body")
			return
		}
		task, review, err := taskLifecycleService().RequestReviewContent(projectID, taskID, actor, req.Content, req.ExpectedRevision)
		if err != nil {
			handleTaskLifecycleError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"task": task, "review": review}})
	}
}

func HandleUpdateTaskReviewResult(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		projectID, taskID := c.Param("id"), c.Param("task_id")
		if reg.Get(projectID) == nil {
			notFoundResponse(c, "project not found")
			return
		}
		actor := c.GetString("user")
		var req tasklifecycle.ReviewResultRequest
		if err := BindDocumentWrite(c, &req); err != nil {
			badRequestResponse(c, "invalid request body")
			return
		}
		task, review, err := taskLifecycleService().SetReviewResult(projectID, taskID, actor, req)
		if err != nil {
			handleTaskLifecycleError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"task": task, "review": review}})
	}
}
