package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
)

// UserCurrentTask represents the user's current active task
type UserCurrentTask struct {
	ProjectID string    `json:"project_id"`
	TaskID    string    `json:"task_id"`
	SetAt     time.Time `json:"set_at"`
}

// HandleGetUserCurrentTask handles GET /api/v1/user/current-task
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

	projectDir, err := projectDir(currentTask.ProjectID)
	if err != nil {
		notFoundResponse(c, "project")
		return
	}

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

	enrichTaskWithCompletionStatus(taskInfo, currentTask.ProjectID, currentTask.TaskID)

	result := map[string]interface{}{
		"project_id":   currentTask.ProjectID,
		"task_id":      currentTask.TaskID,
		"task_info":    taskInfo,
		"project_name": currentTask.ProjectID,
		"set_at":       currentTask.SetAt,
	}

	successResponse(c, gin.H{"success": true, "data": result})
}

// getUserCurrentTask reads the current task for a user
func getUserCurrentTask(username string) (*UserCurrentTask, error) {
	usersDir := os.Getenv("USERS_DIR")
	if usersDir == "" {
		usersDir = "./data/users"
	}
	userFile := filepath.Join(usersDir, username, "current_task.json")
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

// projectDir returns the directory path for a project
func projectDir(projectID string) (string, error) {
	if projectID == "" {
		return "", fmt.Errorf("empty project ID")
	}
	dir := filepath.Join(projectsRoot(), projectID)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return "", fmt.Errorf("project does not exist")
	}
	return dir, nil
}
