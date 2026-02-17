package main

import (
	"fmt"
	"net/url"

	"github.com/spf13/cobra"
)

func newProjectDocCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "project-doc",
		Aliases: []string{"pd"},
		Short:   "项目文档管理 (feature_list/architecture_design)",
	}
	cmd.AddCommand(newProjectDocGetCmd())
	cmd.AddCommand(newProjectDocUpdateCmd())
	return cmd
}

func newProjectDocGetCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "get",
		Short: "获取项目文档",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, err := resolveProjectID(cfg, client)
			if err != nil {
				return err
			}
			slotKey := mustGetString(cmd, "slot-key")
			path := fmt.Sprintf("/api/v1/projects/%s/docs/%s/export", pid, slotKey)
			format, _ := cmd.Flags().GetString("format")
			if format != "" {
				path += "?format=" + url.QueryEscape(format)
			}
			resp, err := client.Get(path)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("slot-key", "", "文档类型: feature_list/architecture_design（必选）")
	c.Flags().String("format", "", "输出格式: json/markdown（默认 markdown）")
	_ = c.MarkFlagRequired("slot-key")
	return c
}

func newProjectDocUpdateCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "update",
		Short: "更新项目文档",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, err := resolveProjectID(cfg, client)
			if err != nil {
				return err
			}
			slotKey := mustGetString(cmd, "slot-key")
			body := map[string]interface{}{
				"content": mustGetString(cmd, "content"),
				"op":      "replace_full",
				"source":  "cli_tool",
			}
			format, _ := cmd.Flags().GetString("format")
			if format != "" {
				body["format"] = format
			}
			resp, err := client.Request("POST", fmt.Sprintf("/api/v1/projects/%s/docs/%s/append", pid, slotKey), body)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("slot-key", "", "文档类型: feature_list/architecture_design（必选）")
	c.Flags().String("content", "", "文档内容（必选）")
	c.Flags().String("format", "", "文档格式: json/markdown")
	_ = c.MarkFlagRequired("slot-key")
	_ = c.MarkFlagRequired("content")
	return c
}
