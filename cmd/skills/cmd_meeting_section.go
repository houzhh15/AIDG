package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newMeetingSectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "meeting-section",
		Aliases: []string{"ms"},
		Short:   "会议章节编辑",
	}
	cmd.AddCommand(newMeetingSectionUpdateCmd())
	return cmd
}

func newMeetingSectionUpdateCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "update",
		Short: "更新会议文档章节内容",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			mid := mustGetString(cmd, "meeting-id")
			slotKey := mustGetString(cmd, "slot-key")
			sectionID := mustGetString(cmd, "section-id")
			body := map[string]interface{}{
				"content": mustGetString(cmd, "content"),
			}
			addOptionalInt(cmd, body, "expected-version", "expected_version")
			path := fmt.Sprintf("/api/v1/meetings/%s/docs/%s/sections/%s", mid, slotKey, sectionID)
			resp, err := client.Request("PUT", path, body)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("meeting-id", "", "会议ID（必选）")
	c.Flags().String("slot-key", "", "文档类型: polish/summary/topic（必选）")
	c.Flags().String("section-id", "", "章节ID（必选）")
	c.Flags().String("content", "", "章节内容（必选）")
	c.Flags().Int("expected-version", 0, "期望版本号（乐观锁）")
	_ = c.MarkFlagRequired("meeting-id")
	_ = c.MarkFlagRequired("slot-key")
	_ = c.MarkFlagRequired("section-id")
	_ = c.MarkFlagRequired("content")
	return c
}
