package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config 统一配置结构
type Config struct {
	Server   ServerConfig
	Data     DataConfig
	Log      LogConfig
	Security SecurityConfig
	MCP      MCPConfig
	Frontend FrontendConfig
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Env  string // dev, staging, production
	Port string
}

// DataConfig 数据目录配置
type DataConfig struct {
	ProjectsDir  string
	UsersDir     string
	MeetingsDir  string
	AuditLogsDir string
}

// LogConfig 日志配置
type LogConfig struct {
	Level  string // debug, info, warn, error
	Format string // console, json
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	JWTSecret            string
	AdminDefaultPassword string
	CORSAllowedOrigins   []string
}

// MCPConfig MCP 服务配置
type MCPConfig struct {
	ServerURL string
	Password  string
}

// FrontendConfig 前端配置
type FrontendConfig struct {
	DistDir string
}

// GlobalConfig 全局配置实例
var GlobalConfig *Config

// LoadConfig 从环境变量加载配置
func LoadConfig() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Env:  getEnv("ENV", "dev"),
			Port: getEnv("PORT", "8000"),
		},
		Data: DataConfig{
			ProjectsDir:  getEnv("PROJECTS_DIR", "./projects"),
			UsersDir:     getEnv("USERS_DIR", "./users"),
			MeetingsDir:  getEnv("MEETINGS_DIR", "./meetings"),
			AuditLogsDir: getEnv("AUDIT_LOGS_DIR", "./audit_logs"),
		},
		Log: LogConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "console"),
		},
		Security: SecurityConfig{
			JWTSecret:            getEnv("USER_JWT_SECRET", ""),
			AdminDefaultPassword: getEnv("ADMIN_DEFAULT_PASSWORD", ""),
			CORSAllowedOrigins:   parseStringList(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5173")),
		},
		MCP: MCPConfig{
			ServerURL: getEnv("MCP_SERVER_URL", "http://localhost:8081"),
			Password:  getEnv("MCP_PASSWORD", ""),
		},
		Frontend: FrontendConfig{
			DistDir: getEnv("FRONTEND_DIST_DIR", "./frontend/dist"),
		},
	}

	GlobalConfig = cfg
	return cfg, nil
}

// ValidateConfig 验证配置的有效性
func ValidateConfig(cfg *Config) error {
	var errors []string

	// 1. JWT Secret 验证
	if cfg.Security.JWTSecret == "" {
		errors = append(errors, "USER_JWT_SECRET is required")
	} else if len(cfg.Security.JWTSecret) < 32 {
		errors = append(errors, "USER_JWT_SECRET must be at least 32 characters long")
	}

	// 2. 生产环境必须配置管理员密码
	if cfg.Server.Env == "production" {
		if cfg.Security.AdminDefaultPassword == "" {
			errors = append(errors, "ADMIN_DEFAULT_PASSWORD is required in production environment")
		}
		if cfg.Security.AdminDefaultPassword == "admin123" ||
			cfg.Security.AdminDefaultPassword == "changeme" ||
			cfg.Security.AdminDefaultPassword == "neteye@123" {
			errors = append(errors, "ADMIN_DEFAULT_PASSWORD cannot be a weak/default password in production")
		}
		if len(cfg.Security.AdminDefaultPassword) < 8 {
			errors = append(errors, "ADMIN_DEFAULT_PASSWORD must be at least 8 characters long in production")
		}
	}

	// 3. MCP 密码验证（生产环境）
	if cfg.Server.Env == "production" && cfg.MCP.Password == "" {
		errors = append(errors, "MCP_PASSWORD is required in production environment")
	}

	// 4. 端口验证
	if port, err := strconv.Atoi(cfg.Server.Port); err != nil || port < 1 || port > 65535 {
		errors = append(errors, fmt.Sprintf("invalid PORT value: %s (must be 1-65535)", cfg.Server.Port))
	}

	// 5. 日志级别验证
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[cfg.Log.Level] {
		errors = append(errors, fmt.Sprintf("invalid LOG_LEVEL: %s (must be: debug, info, warn, error)", cfg.Log.Level))
	}

	// 6. 日志格式验证
	validLogFormats := map[string]bool{"console": true, "json": true}
	if !validLogFormats[cfg.Log.Format] {
		errors = append(errors, fmt.Sprintf("invalid LOG_FORMAT: %s (must be: console, json)", cfg.Log.Format))
	}

	// 7. 环境验证
	validEnvs := map[string]bool{"dev": true, "development": true, "staging": true, "production": true}
	if !validEnvs[cfg.Server.Env] {
		errors = append(errors, fmt.Sprintf("invalid ENV: %s (must be: dev, development, staging, production)", cfg.Server.Env))
	}

	if len(errors) > 0 {
		return fmt.Errorf("configuration validation failed:\n  - %s", strings.Join(errors, "\n  - "))
	}

	return nil
}

// IsProduction 判断是否为生产环境
func (c *Config) IsProduction() bool {
	return c.Server.Env == "production"
}

// IsDevelopment 判断是否为开发环境
func (c *Config) IsDevelopment() bool {
	return c.Server.Env == "dev" || c.Server.Env == "development"
}

// GetServerAddr 获取服务器监听地址
func (c *Config) GetServerAddr() string {
	return ":" + c.Server.Port
}

// PrintConfig 打印配置（脱敏）
func (c *Config) PrintConfig() string {
	return fmt.Sprintf(`Configuration Loaded:
  Environment: %s
  Server Port: %s
  Data Directories:
    - Projects: %s
    - Users: %s
    - Meetings: %s
    - Audit Logs: %s
  Logging:
    - Level: %s
    - Format: %s
  Security:
    - JWT Secret: %s
    - Admin Password: %s
    - CORS Origins: %v
  MCP:
    - Server URL: %s
    - Password: %s
  Frontend:
    - Dist Dir: %s`,
		c.Server.Env,
		c.Server.Port,
		c.Data.ProjectsDir,
		c.Data.UsersDir,
		c.Data.MeetingsDir,
		c.Data.AuditLogsDir,
		c.Log.Level,
		c.Log.Format,
		maskSecret(c.Security.JWTSecret),
		maskSecret(c.Security.AdminDefaultPassword),
		c.Security.CORSAllowedOrigins,
		c.MCP.ServerURL,
		maskSecret(c.MCP.Password),
		c.Frontend.DistDir,
	)
}

// 辅助函数

// getEnv 获取环境变量，如果不存在则返回默认值
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// parseStringList 解析逗号分隔的字符串列表
func parseStringList(value string) []string {
	if value == "" {
		return []string{}
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// maskSecret 对敏感信息进行脱敏
func maskSecret(secret string) string {
	if secret == "" {
		return "<not set>"
	}
	if len(secret) <= 8 {
		return "***"
	}
	return secret[:4] + "***" + secret[len(secret)-4:]
}
