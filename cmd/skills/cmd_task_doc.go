package main

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

func newTaskDocCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "task-doc",
		Aliases: []string{"td"},
		Short:   "任务文档读写 (requirements/design/test)",
	}
	cmd.AddCommand(newTaskDocGetCmd())
	cmd.AddCommand(newTaskDocUpdateCmd())
	cmd.AddCommand(newTaskDocAppendCmd())
	return cmd
}

func newTaskDocGetCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "get",
		Short: "获取任务文档内容",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, tid, err := resolveProjectAndTaskID(cfg, client)
			if err != nil {
				return err
			}
			slotKey := mustGetString(cmd, "slot-key")
			path := fmt.Sprintf("/api/v1/projects/%s/tasks/%s/%s", pid, tid, slotKey)
			ir, _ := cmd.Flags().GetBool("include-recommendations")
			if ir {
				path += "?include_recommendations=true"
			}
			resp, err := client.Get(path)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("slot-key", "", "文档类型: requirements/design/test（必选）")
	_ = c.MarkFlagRequired("slot-key")
	c.Flags().Bool("include-recommendations", false, "包含推荐内容")
	return c
}

func newTaskDocUpdateCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "update",
		Short: "更新任务文档（全文覆盖）",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, tid, err := resolveProjectAndTaskID(cfg, client)
			if err != nil {
				return err
			}
			slotKey := mustGetString(cmd, "slot-key")
			content := mustGetString(cmd, "content")
			body := map[string]interface{}{
				"content": content,
			}
			resp, err := client.Request("PUT", fmt.Sprintf("/api/v1/projects/%s/tasks/%s/%s", pid, tid, slotKey), body)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("slot-key", "", "文档类型: requirements/design/test（必选）")
	c.Flags().String("content", "", "文档内容（必选）")
	_ = c.MarkFlagRequired("slot-key")
	_ = c.MarkFlagRequired("content")
	return c
}

func newTaskDocAppendCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "append",
		Short: "追加内容到任务文档",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, tid, err := resolveProjectAndTaskID(cfg, client)
			if err != nil {
				return err
			}
			slotKey := mustGetString(cmd, "slot-key")
			content := mustGetString(cmd, "content")
			body := map[string]interface{}{
				"content": content,
			}
			addOptionalInt(cmd, body, "expected-version", "expected_version")
			path := fmt.Sprintf("/api/v1/projects/%s/tasks/%s/%s/append", pid, tid, slotKey)
			_ = url.QueryEscape // suppress unused import if needed
			resp, err := client.Request("POST", path, body)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("slot-key", "", "文档类型: requirements/design/test（必选）")
	c.Flags().String("content", "", "追加内容（必选）")
	c.Flags().Int("expected-version", 0, "期望版本号（乐观锁）")
	_ = c.MarkFlagRequired("slot-key")
	_ = c.MarkFlagRequired("content")
	return c
}
