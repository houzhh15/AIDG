package main

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

func newSummaryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summary",
		Short: "任务总结管理 (列表/新增/更新/删除/按周查询)",
	}
	cmd.AddCommand(newSummaryListCmd())
	cmd.AddCommand(newSummaryAddCmd())
	cmd.AddCommand(newSummaryUpdateCmd())
	cmd.AddCommand(newSummaryDeleteCmd())
	cmd.AddCommand(newSummaryQueryByWeekCmd())
	return cmd
}

func newSummaryListCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "list",
		Short: "列出任务的所有总结",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, tid, err := resolveProjectAndTaskID(cfg, client)
			if err != nil {
				return err
			}
			path := fmt.Sprintf("/api/v1/projects/%s/tasks/%s/summaries", pid, tid)
			params := url.Values{}
			if v, _ := cmd.Flags().GetString("start-week"); v != "" {
				params.Set("start_week", v)
			}
			if v, _ := cmd.Flags().GetString("end-week"); v != "" {
				params.Set("end_week", v)
			}
			if len(params) > 0 {
				path += "?" + params.Encode()
			}
			resp, err := client.Get(path)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("start-week", "", "起始周 (如: 2026-W01)")
	c.Flags().String("end-week", "", "结束周 (如: 2026-W04)")
	return c
}

func newSummaryAddCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "add",
		Short: "新增任务总结",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, tid, err := resolveProjectAndTaskID(cfg, client)
			if err != nil {
				return err
			}
			body := map[string]interface{}{
				"time":    mustGetString(cmd, "time"),
				"content": mustGetString(cmd, "content"),
			}
			resp, err := client.Request("POST", fmt.Sprintf("/api/v1/projects/%s/tasks/%s/summaries", pid, tid), body)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("time", "", "时间 (必选)")
	c.Flags().String("content", "", "总结内容 (必选)")
	_ = c.MarkFlagRequired("time")
	_ = c.MarkFlagRequired("content")
	return c
}

func newSummaryUpdateCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "update",
		Short: "更新任务总结",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, tid, err := resolveProjectAndTaskID(cfg, client)
			if err != nil {
				return err
			}
			summaryID := mustGetString(cmd, "summary-id")
			body := map[string]interface{}{}
			addOptionalString(cmd, body, "time")
			addOptionalString(cmd, body, "content")
			if len(body) == 0 {
				return fmt.Errorf("at least one of --time or --content must be provided")
			}
			resp, err := client.Request("PUT", fmt.Sprintf("/api/v1/projects/%s/tasks/%s/summaries/%s", pid, tid, summaryID), body)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("summary-id", "", "总结ID (必选)")
	c.Flags().String("time", "", "时间")
	c.Flags().String("content", "", "总结内容")
	_ = c.MarkFlagRequired("summary-id")
	return c
}

func newSummaryDeleteCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "delete",
		Short: "删除任务总结",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, tid, err := resolveProjectAndTaskID(cfg, client)
			if err != nil {
				return err
			}
			summaryID := mustGetString(cmd, "summary-id")
			resp, err := client.Request("DELETE", fmt.Sprintf("/api/v1/projects/%s/tasks/%s/summaries/%s", pid, tid, summaryID), nil)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("summary-id", "", "总结ID (必选)")
	_ = c.MarkFlagRequired("summary-id")
	return c
}

func newSummaryQueryByWeekCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "query-by-week",
		Short: "按周范围查询任务总结",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, err := resolveProjectID(cfg, client)
			if err != nil {
				return err
			}
			params := url.Values{}
			params.Set("start_week", mustGetString(cmd, "start-week"))
			if v, _ := cmd.Flags().GetString("end-week"); v != "" {
				params.Set("end_week", v)
			}
			if cfg.TaskID != "" {
				params.Set("task_id", cfg.TaskID)
			}
			path := fmt.Sprintf("/api/v1/projects/%s/summaries/by-week?%s", pid, params.Encode())
			resp, err := client.Get(path)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("start-week", "", "起始周 (必选，如: 2026-W01)")
	c.Flags().String("end-week", "", "结束周 (如: 2026-W04)")
	_ = c.MarkFlagRequired("start-week")
	return c
}
