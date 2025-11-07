package api

import (
	"fmt"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15-hub/AIDG/cmd/server/internal/documents"
	"github.com/houzhh15-hub/AIDG/cmd/server/internal/resource"
)

// HandleRefreshUserResources POST /api/v1/user/resources/refresh
// 手动刷新当前用户的 MCP Resources
// 会清除自动添加的资源，并根据当前任务重新添加
func HandleRefreshUserResources(resourceManager *resource.ResourceManager, docHandler *documents.Handler) gin.HandlerFunc {
	return func(c *gin.Context) {
		username := currentUser(c)

		// 获取当前任务
		currentTask, err := getUserCurrentTask(username)
		if err != nil {
			internalErrorResponse(c, fmt.Errorf("failed to get current task: %w", err))
			return
		}

		if currentTask == nil {
			badRequestResponse(c, "no current task set, please select a task first")
			return
		}

		projectID := currentTask.ProjectID
		taskID := currentTask.TaskID

		log.Printf("[INFO] Refreshing MCP resources - user=%s, project=%s, task=%s", username, projectID, taskID)

		// 1. 清除旧的自动资源
		if err := resourceManager.ClearAutoAddedResources(username); err != nil {
			log.Printf("[ERROR] Failed to clear auto resources for user %s: %v", username, err)
			internalErrorResponse(c, fmt.Errorf("failed to clear resources: %w", err))
			return
		}

		// 2. 重新添加任务相关资源
		addTaskResources(resourceManager, username, projectID, taskID, docHandler)

		log.Printf("[INFO] Successfully refreshed MCP resources - user=%s, project=%s, task=%s", username, projectID, taskID)

		successResponseWithMessage(c, "resources refreshed successfully", gin.H{
			"project_id": projectID,
			"task_id":    taskID,
		})
	}
}
