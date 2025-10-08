package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// HandleGetUserCurrentTask GET /api/v1/user/current-task
// 获取当前用户的当前任务信息
func HandleGetUserCurrentTask(c *gin.Context) {
	username := currentUser(c)

	currentTask, err := getUserCurrentTask(username)
	if err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to get current task: %w", err))
		return
	}

	if currentTask == nil {
		successResponse(c, gin.H{"success": true, "data": nil})
		return
	}

	// Get project info
	projectDir, err := projectDir(currentTask.ProjectID)
	if err != nil {
		notFoundResponse(c, "project")
		return
	}

	// Get task info
	tasksFile := filepath.Join(projectDir, "tasks.json")
	if _, err := os.Stat(tasksFile); os.IsNotExist(err) {
		notFoundResponse(c, "tasks file")
		return
	}

	data, err := os.ReadFile(tasksFile)
	if err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to read tasks: %w", err))
		return
	}

	var tasks []map[string]interface{}
	if err := json.Unmarshal(data, &tasks); err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to parse tasks: %w", err))
		return
	}

	// Find the current task
	var taskInfo map[string]interface{}
	for _, task := range tasks {
		if task["id"].(string) == currentTask.TaskID {
			taskInfo = task
			break
		}
	}

	if taskInfo == nil {
		notFoundResponse(c, "task")
		return
	}

	// Return task info with project info
	result := map[string]interface{}{
		"project_id":   currentTask.ProjectID,
		"task_id":      currentTask.TaskID,
		"task_info":    taskInfo,
		"project_name": currentTask.ProjectID,
		"set_at":       currentTask.SetAt,
	}

	successResponse(c, gin.H{"success": true, "data": result})
}

// HandlePutUserCurrentTask PUT /api/v1/user/current-task
// 设置当前用户的当前任务
func HandlePutUserCurrentTask(c *gin.Context) {
	username := currentUser(c)

	var request struct {
		ProjectID string `json:"project_id"`
		TaskID    string `json:"task_id"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		badRequestResponse(c, "invalid request body")
		return
	}

	if request.ProjectID == "" || request.TaskID == "" {
		badRequestResponse(c, "project_id and task_id are required")
		return
	}

	// Verify project exists
	_, err := projectDir(request.ProjectID)
	if err != nil {
		notFoundResponse(c, "project")
		return
	}

	// Verify task exists
	tasksFile := filepath.Join("projects", request.ProjectID, "tasks.json")
	if _, err := os.Stat(tasksFile); os.IsNotExist(err) {
		notFoundResponse(c, "tasks")
		return
	}

	data, err := os.ReadFile(tasksFile)
	if err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to read tasks: %w", err))
		return
	}

	var tasks []map[string]interface{}
	if err := json.Unmarshal(data, &tasks); err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to parse tasks: %w", err))
		return
	}

	// Verify task exists
	taskExists := false
	for _, task := range tasks {
		if task["id"].(string) == request.TaskID {
			taskExists = true
			break
		}
	}

	if !taskExists {
		notFoundResponse(c, "task")
		return
	}

	// Set current task
	if err := setUserCurrentTask(username, request.ProjectID, request.TaskID); err != nil {
		internalErrorResponse(c, fmt.Errorf("failed to set current task: %w", err))
		return
	}

	successResponseWithMessage(c, "current task updated", nil)
}

// Helper functions (from main.go)

type UserCurrentTask struct {
	ProjectID string    `json:"project_id"`
	TaskID    string    `json:"task_id"`
	SetAt     time.Time `json:"set_at"`
}

func getUserCurrentTask(username string) (*UserCurrentTask, error) {
	userFile := filepath.Join("users", username, "current_task.json")
	data, err := os.ReadFile(userFile)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read current task: %w", err)
	}

	var task UserCurrentTask
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("unmarshal current task: %w", err)
	}

	return &task, nil
}

func setUserCurrentTask(username, projectID, taskID string) error {
	userDir := filepath.Join("users", username)
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		return fmt.Errorf("create user dir: %w", err)
	}

	task := UserCurrentTask{
		ProjectID: projectID,
		TaskID:    taskID,
		SetAt:     time.Now(),
	}

	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal task: %w", err)
	}

	userFile := filepath.Join(userDir, "current_task.json")
	if err := os.WriteFile(userFile, data, 0o644); err != nil {
		return fmt.Errorf("write current task: %w", err)
	}

	return nil
}

func projectDir(projectID string) (string, error) {
	if strings.TrimSpace(projectID) == "" {
		return "", fmt.Errorf("empty project ID")
	}
	dir := filepath.Join("projects", projectID)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return "", fmt.Errorf("project does not exist")
	}
	return dir, nil
}
