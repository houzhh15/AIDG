package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newUserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "用户任务绑定管理",
	}
	cmd.AddCommand(newUserGetCurrentTaskCmd())
	cmd.AddCommand(newUserSetCurrentTaskCmd())
	return cmd
}

func newUserGetCurrentTaskCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get-current-task",
		Short: "获取当前用户绑定的项目和任务",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			resp, err := client.Get("/api/v1/user/current-task")
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
}

func newUserSetCurrentTaskCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "set-current-task",
		Short: "设置当前用户绑定的项目和任务",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			body := map[string]interface{}{}
			if cfg.ProjectID != "" {
				body["project_id"] = cfg.ProjectID
			}
			if cfg.TaskID != "" {
				body["task_id"] = cfg.TaskID
			}
			resp, err := client.Request("PUT", "/api/v1/user/current-task", body)
			if err != nil {
				return err
			}
			// 验证：再获取一次确认
			resp2, err := client.Get("/api/v1/user/current-task")
			if err != nil {
				return err
			}
			var result struct {
				Data struct {
					ProjectID string `json:"project_id"`
					TaskID    string `json:"task_id"`
				} `json:"data"`
			}
			if err := json.Unmarshal(resp2, &result); err == nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "已切换: project=%s, task=%s\n", result.Data.ProjectID, result.Data.TaskID)
			}
			_ = resp
			return printOutput(cfg.Output, resp2)
		},
	}
	return c
}
