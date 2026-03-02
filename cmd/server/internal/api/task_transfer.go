package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/projects"
)

// HandleTransferProjectTask POST /api/v1/projects/:id/tasks/:task_id/transfer
// 将任务从当前项目转移到目标项目
// 请求体: { "target_project_id": "target-project-id" }
//
// 操作步骤:
// 1. 验证源项目和目标项目存在
// 2. 验证任务在源项目中存在
// 3. 移动任务目录（包含所有文档：需求、设计、测试、执行计划等）
// 4. 从源项目 tasks.json 中移除任务
// 5. 添加任务到目标项目 tasks.json
func HandleTransferProjectTask(reg *projects.ProjectRegistry) gin.HandlerFunc {
	return func(c *gin.Context) {
		sourceProjectID := c.Param("id")
		taskID := c.Param("task_id")

		var req struct {
			TargetProjectID string `json:"target_project_id" binding:"required"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			badRequestResponse(c, "invalid request body: target_project_id is required")
			return
		}

		targetProjectID := req.TargetProjectID

		// 不能转移到同一项目
		if sourceProjectID == targetProjectID {
			badRequestResponse(c, "cannot transfer task to the same project")
			return
		}

		// 验证源项目存在
		if reg.Get(sourceProjectID) == nil {
			notFoundResponse(c, "source project not found")
			return
		}

		// 验证目标项目存在
		if reg.Get(targetProjectID) == nil {
			notFoundResponse(c, "target project not found")
			return
		}

		root := projectsRoot()
		sourceProjDir := filepath.Join(root, sourceProjectID)
		targetProjDir := filepath.Join(root, targetProjectID)

		// ===== 第1步: 从源项目 tasks.json 读取并移除任务 =====
		sourceTasksFile := filepath.Join(sourceProjDir, "tasks.json")
		sourceData, err := os.ReadFile(sourceTasksFile)
		if err != nil {
			notFoundResponse(c, "source tasks file not found")
			return
		}

		var sourceTaskList []map[string]interface{}
		if err := json.Unmarshal(sourceData, &sourceTaskList); err != nil {
			internalErrorResponse(c, fmt.Errorf("failed to parse source tasks: %w", err))
			return
		}

		// 查找任务
		var taskData map[string]interface{}
		newSourceTaskList := make([]map[string]interface{}, 0, len(sourceTaskList))
		for _, task := range sourceTaskList {
			if task["id"] == taskID {
				taskData = task
			} else {
				newSourceTaskList = append(newSourceTaskList, task)
			}
		}

		if taskData == nil {
			notFoundResponse(c, "task not found in source project")
			return
		}

		// ===== 第2步: 移动任务目录 =====
		sourceTaskDir := filepath.Join(sourceProjDir, "tasks", taskID)
		targetTasksDir := filepath.Join(targetProjDir, "tasks")
		targetTaskDir := filepath.Join(targetTasksDir, taskID)

		// 确保目标 tasks 目录存在
		if err := os.MkdirAll(targetTasksDir, 0755); err != nil {
			internalErrorResponse(c, fmt.Errorf("failed to create target tasks directory: %w", err))
			return
		}

		// 检查目标目录是否已存在（避免覆盖）
		if _, err := os.Stat(targetTaskDir); err == nil {
			badRequestResponse(c, fmt.Sprintf("task directory already exists in target project: %s", taskID))
			return
		}

		// 移动目录（同一文件系统用 Rename，否则需要拷贝+删除）
		if _, err := os.Stat(sourceTaskDir); err == nil {
			if err := os.Rename(sourceTaskDir, targetTaskDir); err != nil {
				// 如果跨文件系统无法 rename，尝试拷贝+删除
				if err := copyDir(sourceTaskDir, targetTaskDir); err != nil {
					internalErrorResponse(c, fmt.Errorf("failed to move task directory: %w", err))
					return
				}
				os.RemoveAll(sourceTaskDir)
			}
		}
		// 如果源任务目录不存在也没关系，可能是空任务

		// ===== 第3步: 保存源项目 tasks.json（移除任务后） =====
		updatedSourceData, _ := json.MarshalIndent(newSourceTaskList, "", "  ")
		if err := os.WriteFile(sourceTasksFile, updatedSourceData, 0644); err != nil {
			// 回滚：尝试把目录移回去
			if _, statErr := os.Stat(targetTaskDir); statErr == nil {
				os.Rename(targetTaskDir, sourceTaskDir)
			}
			internalErrorResponse(c, fmt.Errorf("failed to save source tasks: %w", err))
			return
		}

		// ===== 第4步: 添加任务到目标项目 tasks.json =====
		targetTasksFile := filepath.Join(targetProjDir, "tasks.json")
		var targetTaskList []map[string]interface{}
		if targetData, err := os.ReadFile(targetTasksFile); err == nil {
			json.Unmarshal(targetData, &targetTaskList)
		}

		targetTaskList = append(targetTaskList, taskData)
		updatedTargetData, _ := json.MarshalIndent(targetTaskList, "", "  ")
		if err := os.WriteFile(targetTasksFile, updatedTargetData, 0644); err != nil {
			// 回滚：把任务加回源项目，把目录移回去
			sourceTaskList = append(newSourceTaskList, taskData)
			rollbackData, _ := json.MarshalIndent(sourceTaskList, "", "  ")
			os.WriteFile(sourceTasksFile, rollbackData, 0644)
			if _, statErr := os.Stat(targetTaskDir); statErr == nil {
				os.Rename(targetTaskDir, sourceTaskDir)
			}
			internalErrorResponse(c, fmt.Errorf("failed to save target tasks: %w", err))
			return
		}

		// 获取目标项目名称
		targetProject := reg.Get(targetProjectID)
		targetProjectName := targetProjectID
		if targetProject != nil {
			targetProjectName = targetProject.Name
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": fmt.Sprintf("task transferred to project: %s", targetProjectName),
			"data": gin.H{
				"task_id":           taskID,
				"source_project_id": sourceProjectID,
				"target_project_id": targetProjectID,
			},
		})
	}
}

// copyDir 递归复制目录
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(srcPath)
			if err != nil {
				return err
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if err := os.WriteFile(dstPath, data, info.Mode()); err != nil {
				return err
			}
		}
	}

	return nil
}
