package config

import (
	"fmt"
	"os"
	"strconv"
)

// MCPConfig MCP Server 配置结构
type MCPConfig struct {
	// Server 服务器配置
	Server ServerConfig
	// Backend 后端API配置
	Backend BackendConfig
	// Auth 认证配置
	Auth AuthConfig
}

// ServerConfig 服务器配置
type ServerConfig struct {
	// HTTPPort HTTP服务端口
	HTTPPort int
	// Environment 运行环境 (development, production)
	Environment string
}

// BackendConfig 后端API配置
type BackendConfig struct {
	// ServerURL 后端服务器地址
	ServerURL string
	// Timeout 请求超时时间(秒)
	Timeout int
}

// AuthConfig 认证配置
type AuthConfig struct {
	// BearerToken Bearer Token (可选)
	BearerToken string
	// Username 用户名 (可选)
	Username string
	// Password 密码 (可选)
	Password string
}

// LoadConfig 从环境变量加载配置
func LoadConfig() (*MCPConfig, error) {
	cfg := &MCPConfig{
		Server: ServerConfig{
			HTTPPort:    getEnvAsInt("MCP_HTTP_PORT", 8081),
			Environment: getEnv("ENV", "development"),
		},
		Backend: BackendConfig{
			ServerURL: getEnv("MCP_SERVER_URL", "http://localhost:8000"),
			Timeout:   getEnvAsInt("MCP_BACKEND_TIMEOUT", 30),
		},
		Auth: AuthConfig{
			BearerToken: getEnv("MCP_BEARER_TOKEN", ""),
			Username:    getEnv("MCP_USERNAME", ""),
			Password:    getEnv("MCP_PASSWORD", ""),
		},
	}

	return cfg, nil
}

// ValidateConfig 验证配置
func ValidateConfig(cfg *MCPConfig) error {
	// 验证端口范围
	if cfg.Server.HTTPPort < 1 || cfg.Server.HTTPPort > 65535 {
		return fmt.Errorf("invalid HTTP port: %d (must be between 1-65535)", cfg.Server.HTTPPort)
	}

	// 验证后端服务器地址
	if cfg.Backend.ServerURL == "" {
		return fmt.Errorf("backend server URL is required (MCP_SERVER_URL)")
	}

	// 验证超时时间
	if cfg.Backend.Timeout < 1 || cfg.Backend.Timeout > 300 {
		return fmt.Errorf("invalid timeout: %d seconds (must be between 1-300)", cfg.Backend.Timeout)
	}

	// 验证环境
	if cfg.Server.Environment != "development" && cfg.Server.Environment != "production" {
		return fmt.Errorf("invalid environment: %s (must be 'development' or 'production')", cfg.Server.Environment)
	}

	return nil
}

// GetServerAddress 获取服务器监听地址
func (c *MCPConfig) GetServerAddress() string {
	return fmt.Sprintf(":%d", c.Server.HTTPPort)
}

// IsProduction 是否为生产环境
func (c *MCPConfig) IsProduction() bool {
	return c.Server.Environment == "production"
}

// HasAuth 是否配置了认证信息
func (c *MCPConfig) HasAuth() bool {
	return c.Auth.BearerToken != "" || (c.Auth.Username != "" && c.Auth.Password != "")
}

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt 获取整数类型的环境变量，如果不存在或解析失败则返回默认值
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}
