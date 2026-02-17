package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newPlanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "执行计划管理 (获取/更新/导航/状态回写)",
	}
	cmd.AddCommand(newPlanGetCmd())
	cmd.AddCommand(newPlanUpdateCmd())
	cmd.AddCommand(newPlanNextStepCmd())
	cmd.AddCommand(newPlanStepStatusCmd())
	return cmd
}

func newPlanGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get",
		Short: "获取执行计划",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, tid, err := resolveProjectAndTaskID(cfg, client)
			if err != nil {
				return err
			}
			resp, err := client.Get(fmt.Sprintf("/internal/api/v1/projects/%s/tasks/%s/execution-plan", pid, tid))
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
}

func newPlanUpdateCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "update",
		Short: "更新/提交执行计划",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, tid, err := resolveProjectAndTaskID(cfg, client)
			if err != nil {
				return err
			}
			body := map[string]interface{}{
				"content": mustGetString(cmd, "content"),
			}
			resp, err := client.Request("POST", fmt.Sprintf("/internal/api/v1/projects/%s/tasks/%s/execution-plan", pid, tid), body)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("content", "", "执行计划内容（Markdown格式，必选）")
	_ = c.MarkFlagRequired("content")
	return c
}

func newPlanNextStepCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "next-step",
		Short: "获取下一个可执行步骤",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, tid, err := resolveProjectAndTaskID(cfg, client)
			if err != nil {
				return err
			}
			resp, err := client.Get(fmt.Sprintf("/internal/api/v1/projects/%s/tasks/%s/execution-plan/next-step", pid, tid))
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
}

func newPlanStepStatusCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "step-status",
		Short: "更新步骤执行状态",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, tid, err := resolveProjectAndTaskID(cfg, client)
			if err != nil {
				return err
			}
			stepID := mustGetString(cmd, "step-id")
			body := map[string]interface{}{
				"status": mustGetString(cmd, "status"),
			}
			addOptionalString(cmd, body, "output")
			path := fmt.Sprintf("/internal/api/v1/projects/%s/tasks/%s/execution-plan/steps/%s/status", pid, tid, stepID)
			resp, err := client.Request("POST", path, body)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("step-id", "", "步骤ID，如 step-01（必选）")
	c.Flags().String("status", "", "状态: pending/in-progress/succeeded/failed/cancelled（必选）")
	c.Flags().String("output", "", "执行输出/日志")
	_ = c.MarkFlagRequired("step-id")
	_ = c.MarkFlagRequired("status")
	return c
}
