package main

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "任务管理 (CRUD、提示词、下一个未完成任务)",
	}
	cmd.AddCommand(newTaskListCmd())
	cmd.AddCommand(newTaskCreateCmd())
	cmd.AddCommand(newTaskGetCmd())
	cmd.AddCommand(newTaskUpdateCmd())
	cmd.AddCommand(newTaskDeleteCmd())
	cmd.AddCommand(newTaskNextIncompleteCmd())
	cmd.AddCommand(newTaskPromptsCmd())
	cmd.AddCommand(newTaskCreatePromptCmd())
	return cmd
}

func newTaskListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "列出项目的所有任务",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, err := resolveProjectID(cfg, client)
			if err != nil {
				return err
			}
			resp, err := client.Get(fmt.Sprintf("/api/v1/projects/%s/tasks", pid))
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
}

func newTaskCreateCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "create",
		Short: "创建新任务",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, err := resolveProjectID(cfg, client)
			if err != nil {
				return err
			}
			body := map[string]interface{}{
				"name": mustGetString(cmd, "name"),
			}
			addOptionalString(cmd, body, "description")
			addOptionalString(cmd, body, "assignee")
			addOptionalString(cmd, body, "status")
			addOptionalString(cmd, body, "feature-id", "feature_id")
			addOptionalString(cmd, body, "feature-name", "feature_name")
			addOptionalString(cmd, body, "module")
			resp, err := client.Request("POST", fmt.Sprintf("/api/v1/projects/%s/tasks", pid), body)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("name", "", "任务名称（必选）")
	_ = c.MarkFlagRequired("name")
	c.Flags().String("description", "", "任务描述")
	c.Flags().String("assignee", "", "负责人")
	c.Flags().String("status", "", "状态: todo/in-progress/review/completed")
	c.Flags().String("feature-id", "", "特性ID")
	c.Flags().String("feature-name", "", "特性名称")
	c.Flags().String("module", "", "模块")
	return c
}

func newTaskGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "获取任务详情",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, tid, err := resolveProjectAndTaskID(cfg, client)
			if err != nil {
				return err
			}
			resp, err := client.Get(fmt.Sprintf("/api/v1/projects/%s/tasks/%s", pid, tid))
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
}

func newTaskUpdateCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "update",
		Short: "更新任务信息",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, tid, err := resolveProjectAndTaskID(cfg, client)
			if err != nil {
				return err
			}
			body := map[string]interface{}{}
			addOptionalString(cmd, body, "name")
			addOptionalString(cmd, body, "description")
			addOptionalString(cmd, body, "assignee")
			addOptionalString(cmd, body, "status")
			addOptionalString(cmd, body, "feature-id", "feature_id")
			addOptionalString(cmd, body, "feature-name", "feature_name")
			addOptionalString(cmd, body, "module")
			resp, err := client.Request("PUT", fmt.Sprintf("/api/v1/projects/%s/tasks/%s", pid, tid), body)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("name", "", "任务名称")
	c.Flags().String("description", "", "任务描述")
	c.Flags().String("assignee", "", "负责人")
	c.Flags().String("status", "", "状态: todo/in-progress/review/completed")
	c.Flags().String("feature-id", "", "特性ID")
	c.Flags().String("feature-name", "", "特性名称")
	c.Flags().String("module", "", "模块")
	return c
}

func newTaskDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete",
		Short: "删除任务",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, tid, err := resolveProjectAndTaskID(cfg, client)
			if err != nil {
				return err
			}
			resp, err := client.Request("DELETE", fmt.Sprintf("/api/v1/projects/%s/tasks/%s", pid, tid), nil)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
}

func newTaskNextIncompleteCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "next-incomplete",
		Short: "获取项目中下一个有未完成文档的任务",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, err := resolveProjectID(cfg, client)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/api/v1/projects/%s/tasks/next-incomplete", pid)
			dt, _ := cmd.Flags().GetString("doc-type")
			if dt != "" {
				path += "?doc_type=" + url.QueryEscape(dt)
			}
			resp, err := client.Get(path)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("doc-type", "", "文档类型筛选: requirements/design/plan/execution/test")
	return c
}

func newTaskPromptsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "prompts",
		Short: "获取任务的提示词历史",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, tid, err := resolveProjectAndTaskID(cfg, client)
			if err != nil {
				return err
			}
			resp, err := client.Get(fmt.Sprintf("/api/v1/projects/%s/tasks/%s/prompts", pid, tid))
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
}

func newTaskCreatePromptCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "create-prompt",
		Short: "记录提示词到任务历史",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, tid, err := resolveProjectAndTaskID(cfg, client)
			if err != nil {
				return err
			}
			body := map[string]interface{}{
				"username": mustGetString(cmd, "username"),
				"content":  mustGetString(cmd, "content"),
			}
			resp, err := client.Request("POST", fmt.Sprintf("/api/v1/projects/%s/tasks/%s/prompts", pid, tid), body)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("username", "", "用户名（必选）")
	c.Flags().String("content", "", "提示词内容（必选）")
	_ = c.MarkFlagRequired("username")
	_ = c.MarkFlagRequired("content")
	return c
}
