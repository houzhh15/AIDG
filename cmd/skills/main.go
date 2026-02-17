package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "dev"

func main() {
	rootCmd := &cobra.Command{
		Use:     "aidg",
		Short:   "AIDG CLI - AI辅助开发治理平台命令行工具",
		Long:    "通过命令行直接调用 AIDG 后端 HTTP API，与 MCP Server 工具 1:1 对应。",
		Version: version,
	}

	// 添加全局标志
	addGlobalFlags(rootCmd)

	// 注册所有分组子命令
	rootCmd.AddCommand(newUserCmd())
	rootCmd.AddCommand(newTaskCmd())
	rootCmd.AddCommand(newTaskDocCmd())
	rootCmd.AddCommand(newTaskSectionCmd())
	rootCmd.AddCommand(newProjectDocCmd())
	rootCmd.AddCommand(newMeetingCmd())
	rootCmd.AddCommand(newMeetingSectionCmd())
	rootCmd.AddCommand(newPlanCmd())
	rootCmd.AddCommand(newSummaryCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
