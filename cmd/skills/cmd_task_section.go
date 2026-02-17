package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newTaskSectionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "task-section",
		Aliases: []string{"ts"},
		Short:   "任务章节级编辑 (requirements/design/test)",
	}
	cmd.AddCommand(newTaskSectionListCmd())
	cmd.AddCommand(newTaskSectionGetCmd())
	cmd.AddCommand(newTaskSectionUpdateCmd())
	cmd.AddCommand(newTaskSectionInsertCmd())
	cmd.AddCommand(newTaskSectionDeleteCmd())
	cmd.AddCommand(newTaskSectionSyncCmd())
	return cmd
}

func newTaskSectionListCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "list",
		Short: "获取文档的章节树结构",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, tid, err := resolveProjectAndTaskID(cfg, client)
			if err != nil {
				return err
			}
			docType := mustGetString(cmd, "doc-type")
			resp, err := client.Get(fmt.Sprintf("/api/v1/projects/%s/tasks/%s/%s/sections", pid, tid, docType))
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("doc-type", "", "文档类型: requirements/design/test（必选）")
	_ = c.MarkFlagRequired("doc-type")
	return c
}

func newTaskSectionGetCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "get",
		Short: "获取单个章节的内容",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, tid, err := resolveProjectAndTaskID(cfg, client)
			if err != nil {
				return err
			}
			docType := mustGetString(cmd, "doc-type")
			sectionID := mustGetString(cmd, "section-id")
			path := fmt.Sprintf("/api/v1/projects/%s/tasks/%s/%s/sections/%s", pid, tid, docType, sectionID)
			ic, _ := cmd.Flags().GetBool("include-children")
			if ic {
				path += "?include_children=true"
			}
			resp, err := client.Get(path)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("doc-type", "", "文档类型: requirements/design/test（必选）")
	c.Flags().String("section-id", "", "章节ID（必选）")
	c.Flags().Bool("include-children", false, "包含子章节内容")
	_ = c.MarkFlagRequired("doc-type")
	_ = c.MarkFlagRequired("section-id")
	return c
}

func newTaskSectionUpdateCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "update",
		Short: "更新单个章节的内容（不含标题）",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, tid, err := resolveProjectAndTaskID(cfg, client)
			if err != nil {
				return err
			}
			docType := mustGetString(cmd, "doc-type")
			sectionID := mustGetString(cmd, "section-id")
			body := map[string]interface{}{
				"content": mustGetString(cmd, "content"),
			}
			addOptionalInt(cmd, body, "expected-version", "expected_version")
			resp, err := client.Request("PUT", fmt.Sprintf("/api/v1/projects/%s/tasks/%s/%s/sections/%s", pid, tid, docType, sectionID), body)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("doc-type", "", "文档类型（必选）")
	c.Flags().String("section-id", "", "章节ID（必选）")
	c.Flags().String("content", "", "章节正文内容（必选，不含标题）")
	c.Flags().Int("expected-version", 0, "期望版本号（乐观锁）")
	_ = c.MarkFlagRequired("doc-type")
	_ = c.MarkFlagRequired("section-id")
	_ = c.MarkFlagRequired("content")
	return c
}

func newTaskSectionInsertCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "insert",
		Short: "插入新章节",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, tid, err := resolveProjectAndTaskID(cfg, client)
			if err != nil {
				return err
			}
			docType := mustGetString(cmd, "doc-type")
			body := map[string]interface{}{
				"title":   mustGetString(cmd, "title"),
				"content": mustGetString(cmd, "content"),
			}
			addOptionalString(cmd, body, "after-section-id", "after_section_id")
			resp, err := client.Request("POST", fmt.Sprintf("/api/v1/projects/%s/tasks/%s/%s/sections", pid, tid, docType), body)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("doc-type", "", "文档类型（必选）")
	c.Flags().String("title", "", "章节标题（必选）")
	c.Flags().String("content", "", "章节内容（必选）")
	c.Flags().String("after-section-id", "", "插入到此章节之后")
	_ = c.MarkFlagRequired("doc-type")
	_ = c.MarkFlagRequired("title")
	_ = c.MarkFlagRequired("content")
	return c
}

func newTaskSectionDeleteCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "delete",
		Short: "删除章节",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, tid, err := resolveProjectAndTaskID(cfg, client)
			if err != nil {
				return err
			}
			docType := mustGetString(cmd, "doc-type")
			sectionID := mustGetString(cmd, "section-id")
			path := fmt.Sprintf("/api/v1/projects/%s/tasks/%s/%s/sections/%s", pid, tid, docType, sectionID)
			cascade, _ := cmd.Flags().GetBool("cascade")
			if cascade {
				path += "?cascade=true"
			}
			resp, err := client.Request("DELETE", path, nil)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("doc-type", "", "文档类型（必选）")
	c.Flags().String("section-id", "", "章节ID（必选）")
	c.Flags().Bool("cascade", false, "级联删除子章节")
	_ = c.MarkFlagRequired("doc-type")
	_ = c.MarkFlagRequired("section-id")
	return c
}

func newTaskSectionSyncCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "sync",
		Short: "同步章节与编译文档",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := LoadConfig(cmd)
			client := NewAPIClient(cfg)
			pid, tid, err := resolveProjectAndTaskID(cfg, client)
			if err != nil {
				return err
			}
			docType := mustGetString(cmd, "doc-type")
			body := map[string]interface{}{
				"direction": mustGetString(cmd, "direction"),
			}
			resp, err := client.Request("POST", fmt.Sprintf("/api/v1/projects/%s/tasks/%s/%s/sections/sync", pid, tid, docType), body)
			if err != nil {
				return err
			}
			return printOutput(cfg.Output, resp)
		},
	}
	c.Flags().String("doc-type", "", "文档类型（必选）")
	c.Flags().String("direction", "", "同步方向: from_compiled/to_compiled（必选）")
	_ = c.MarkFlagRequired("doc-type")
	_ = c.MarkFlagRequired("direction")
	return c
}
