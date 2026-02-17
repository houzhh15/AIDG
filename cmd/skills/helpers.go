package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// resolveProjectID 获取项目 ID（标志 > 环境变量 > 当前任务回退）
func resolveProjectID(cfg *Config, client *APIClient) (string, error) {
	if cfg.ProjectID != "" {
		return cfg.ProjectID, nil
	}
	pid, _, err := fetchCurrentTaskIDs(client)
	return pid, err
}

// resolveProjectAndTaskID 获取项目 ID 和任务 ID（标志 > 环境变量 > 当前任务回退）
func resolveProjectAndTaskID(cfg *Config, client *APIClient) (string, string, error) {
	if cfg.ProjectID != "" && cfg.TaskID != "" {
		return cfg.ProjectID, cfg.TaskID, nil
	}
	pid, tid, err := fetchCurrentTaskIDs(client)
	if err != nil {
		return "", "", err
	}
	if cfg.ProjectID != "" {
		pid = cfg.ProjectID
	}
	if cfg.TaskID != "" {
		tid = cfg.TaskID
	}
	return pid, tid, nil
}

// fetchCurrentTaskIDs 从 /api/v1/user/current-task 获取当前绑定的 project_id 和 task_id
func fetchCurrentTaskIDs(client *APIClient) (string, string, error) {
	data, err := client.Get("/api/v1/user/current-task")
	if err != nil {
		return "", "", fmt.Errorf("get current task for fallback: %w", err)
	}
	var resp struct {
		Data struct {
			ProjectID string `json:"project_id"`
			TaskID    string `json:"task_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return "", "", fmt.Errorf("parse current task: %w", err)
	}
	if resp.Data.ProjectID == "" {
		return "", "", fmt.Errorf("no current project set, use --project-id or set via 'aidg user set-current-task'")
	}
	return resp.Data.ProjectID, resp.Data.TaskID, nil
}

// addOptionalString 如果命令行标志有值则添加到 body map
func addOptionalString(cmd *cobra.Command, body map[string]interface{}, flag string, jsonKeys ...string) {
	v, _ := cmd.Flags().GetString(flag)
	if v == "" {
		return
	}
	key := flag
	if len(jsonKeys) > 0 {
		key = jsonKeys[0]
	}
	body[key] = v
}

// addOptionalBool 如果命令行标志被设置则添加到 body map
func addOptionalBool(cmd *cobra.Command, body map[string]interface{}, flag string, jsonKeys ...string) {
	if !cmd.Flags().Changed(flag) {
		return
	}
	v, _ := cmd.Flags().GetBool(flag)
	key := flag
	if len(jsonKeys) > 0 {
		key = jsonKeys[0]
	}
	body[key] = v
}

// addOptionalInt 如果命令行标志被设置则添加到 body map
func addOptionalInt(cmd *cobra.Command, body map[string]interface{}, flag string, jsonKeys ...string) {
	if !cmd.Flags().Changed(flag) {
		return
	}
	v, _ := cmd.Flags().GetInt(flag)
	key := flag
	if len(jsonKeys) > 0 {
		key = jsonKeys[0]
	}
	body[key] = v
}

// mustGetString 获取必选的字符串标志
func mustGetString(cmd *cobra.Command, flag string) string {
	v, _ := cmd.Flags().GetString(flag)
	return v
}
