package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

// Config 保存 CLI 全局配置
type Config struct {
	ServerURL string `yaml:"server_url" json:"server_url"`
	Token     string `yaml:"token" json:"token"`
	ProjectID string `yaml:"default_project_id" json:"default_project_id"`
	TaskID    string `yaml:"-" json:"-"`
	Output    string `yaml:"-" json:"-"`
}

// LoadConfig 从命令行标志、环境变量、配置文件加载配置（优先级从高到低）
func LoadConfig(cmd *cobra.Command) *Config {
	cfg := &Config{}

	// 尝试从配置文件读取基础值
	loadConfigFile(cfg)

	// 环境变量覆盖配置文件
	if v := os.Getenv("AIDG_SERVER_URL"); v != "" {
		cfg.ServerURL = v
	}
	if v := os.Getenv("AIDG_TOKEN"); v != "" {
		cfg.Token = v
	}
	if v := os.Getenv("AIDG_PROJECT_ID"); v != "" {
		cfg.ProjectID = v
	}

	// 命令行标志覆盖环境变量
	if v, _ := cmd.Flags().GetString("server-url"); v != "" {
		cfg.ServerURL = v
	}
	if v, _ := cmd.Flags().GetString("token"); v != "" {
		cfg.Token = v
	}
	if v, _ := cmd.Flags().GetString("project-id"); v != "" {
		cfg.ProjectID = v
	}
	if v, _ := cmd.Flags().GetString("task-id"); v != "" {
		cfg.TaskID = v
	}
	if v, _ := cmd.Flags().GetString("output"); v != "" {
		cfg.Output = v
	}

	// 默认值
	if cfg.ServerURL == "" {
		cfg.ServerURL = "http://localhost:8000"
	}
	if cfg.Output == "" {
		cfg.Output = "text"
	}

	return cfg
}

// loadConfigFile 从 ~/.aidg/config.yaml 读取配置
func loadConfigFile(cfg *Config) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	path := filepath.Join(home, ".aidg", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	_ = yaml.Unmarshal(data, cfg)
}

// addGlobalFlags 为 root 命令添加全局标志
func addGlobalFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().String("server-url", "", fmt.Sprintf("服务器地址 (env: AIDG_SERVER_URL, 默认: http://localhost:8000)"))
	cmd.PersistentFlags().String("token", "", "认证令牌 (env: AIDG_TOKEN)")
	cmd.PersistentFlags().StringP("project-id", "p", "", "项目ID (env: AIDG_PROJECT_ID, 可选)")
	cmd.PersistentFlags().StringP("task-id", "t", "", "任务ID (可选)")
	cmd.PersistentFlags().StringP("output", "o", "", "输出格式: json / text (默认: text)")
}
