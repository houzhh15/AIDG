package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newMeetingCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "meeting",
		Aliases: []string{"mtg"},
		Short:   "会议管理 (列表、文档、章节)",
	}
	cmd.AddCommand(newMeetingListCmd())
	cmd.AddCommand(newMeetingDocGetCmd())
	cmd.AddCommand(newMeetingDocUpdateCmd())
	cmd.AddCommand(newMeetingSectionsCmd())
	return cmd
}

func newMeetingListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "列出所有会议",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			resp, err := client.Get("/api/v1/tasks")
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
}

func newMeetingDocGetCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "doc-get",
		Short: "获取会议文档",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			mid := mustGetString(cmd, "meeting-id")
			slotKey := mustGetString(cmd, "slot-key")

			// 根据 slot_key 选择不同的 API 路径
			var path string
			switch slotKey {
			case "meeting_info":
				path = fmt.Sprintf("/api/v1/tasks/%s", mid)
			case "context":
				path = fmt.Sprintf("/api/v1/tasks/%s/meeting-context", mid)
			case "merged_all":
				path = fmt.Sprintf("/api/v1/tasks/%s/merged_all", mid)
			case "polish", "summary", "topic":
				path = fmt.Sprintf("/api/v1/meetings/%s/docs/%s/export", mid, slotKey)
			default:
				return fmt.Errorf("invalid slot-key: %s (valid: meeting_info/polish/context/summary/topic/merged_all)", slotKey)
			}

			resp, err := client.Get(path)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("meeting-id", "", "会议ID（必选）")
	c.Flags().String("slot-key", "", "文档类型: meeting_info/polish/context/summary/topic/merged_all（必选）")
	_ = c.MarkFlagRequired("meeting-id")
	_ = c.MarkFlagRequired("slot-key")
	return c
}

func newMeetingDocUpdateCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "doc-update",
		Short: "更新会议文档",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			mid := mustGetString(cmd, "meeting-id")
			slotKey := mustGetString(cmd, "slot-key")
			content := mustGetString(cmd, "content")

			// 统一 API (polish/summary/topic)
			switch slotKey {
			case "summary", "topic", "polish":
				body := map[string]interface{}{
					"content": content,
					"op":      "replace_full",
					"source":  "cli_tool",
				}
				path := fmt.Sprintf("/api/v1/meetings/%s/docs/%s/append", mid, slotKey)
				resp, err := client.Request("POST", path, body)
				if err != nil {
					return err
				}
				return printOutput(cfg.Output, resp)
			default:
				return fmt.Errorf("invalid slot-key for update: %s (valid: summary/topic/polish)", slotKey)
			}
		},
	}
	c.Flags().String("meeting-id", "", "会议ID（必选）")
	c.Flags().String("slot-key", "", "文档类型: summary/topic/polish（必选）")
	c.Flags().String("content", "", "文档内容（必选）")
	_ = c.MarkFlagRequired("meeting-id")
	_ = c.MarkFlagRequired("slot-key")
	_ = c.MarkFlagRequired("content")
	return c
}

func newMeetingSectionsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "sections",
		Short: "获取会议文档的章节树结构",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			mid := mustGetString(cmd, "meeting-id")
			slotKey := mustGetString(cmd, "slot-key")
			resp, err := client.Get(fmt.Sprintf("/api/v1/meetings/%s/docs/%s/sections", mid, slotKey))
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("meeting-id", "", "会议ID（必选）")
	c.Flags().String("slot-key", "", "文档类型: polish/summary/topic（必选）")
	_ = c.MarkFlagRequired("meeting-id")
	_ = c.MarkFlagRequired("slot-key")
	return c
}
