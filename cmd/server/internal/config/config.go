package config

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const maxConfigFileSize = 1 << 20
const maxSecretFileSize = 64 << 10

// Config is the server's typed configuration. Values are loaded in this order:
// defaults < YAML < legacy environment variables < AIDG_* variables < secret files.
type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Data      DataConfig      `yaml:"data"`
	Log       LogConfig       `yaml:"logging"`
	Security  SecurityConfig  `yaml:"security"`
	Frontend  FrontendConfig  `yaml:"frontend"`
	MeetingV2 MeetingV2Config `yaml:"meeting_v2"`
	ASR       ASRConfig       `yaml:"asr"`
}

type ServerConfig struct {
	Env      string `yaml:"env"`
	Port     string `yaml:"port"`
	Protocol string `yaml:"protocol"`
	TLSCert  string `yaml:"tls_cert_file"`
	TLSKey   string `yaml:"tls_key_file"`
}

// UnmarshalYAML accepts both numeric and quoted ports because historical AIDG
// configuration examples use numeric YAML values.
func (c *ServerConfig) UnmarshalYAML(node *yaml.Node) error {
	var raw struct {
		Env      *string `yaml:"env"`
		Port     any     `yaml:"port"`
		Protocol *string `yaml:"protocol"`
		TLSCert  *string `yaml:"tls_cert_file"`
		TLSKey   *string `yaml:"tls_key_file"`
	}
	if err := node.Decode(&raw); err != nil {
		return err
	}
	if raw.Env != nil {
		c.Env = *raw.Env
	}
	if raw.Port != nil {
		switch value := raw.Port.(type) {
		case string:
			c.Port = strings.TrimSpace(value)
		case int:
			c.Port = strconv.Itoa(value)
		default:
			return fmt.Errorf("server.port must be a string or integer")
		}
		if c.Port == "" {
			return fmt.Errorf("server.port must not be empty")
		}
	}
	if raw.Protocol != nil {
		c.Protocol = *raw.Protocol
	}
	if raw.TLSCert != nil {
		c.TLSCert = *raw.TLSCert
	}
	if raw.TLSKey != nil {
		c.TLSKey = *raw.TLSKey
	}
	return nil
}

type DataConfig struct {
	ProjectsDir  string `yaml:"projects_dir"`
	UsersDir     string `yaml:"users_dir"`
	MeetingsDir  string `yaml:"meetings_dir"`
	AuditLogsDir string `yaml:"audit_logs_dir"`
}

type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

type SecurityConfig struct {
	JWTSecret            string   `yaml:"jwt_secret"`
	AdminDefaultPassword string   `yaml:"admin_default_password"`
	CORSAllowedOrigins   []string `yaml:"cors_allowed_origins"`
}

type FrontendConfig struct {
	DistDir string `yaml:"dist_dir"`
}

type MeetingV2Config struct {
	Enabled             bool  `yaml:"enabled"`
	MaxChunkSize        int64 `yaml:"max_chunk_size"`
	MaxChunksPerSession int   `yaml:"max_chunks_per_session"`
}

type ASRConfig struct {
	Enabled           bool     `yaml:"enabled"`
	Profile           string   `yaml:"profile"`
	Endpoint          string   `yaml:"endpoint"`
	Model             string   `yaml:"model"`
	Language          string   `yaml:"language"`
	APIKey            string   `yaml:"-"`
	APIKeyFile        string   `yaml:"api_key_file"`
	Concurrency       int      `yaml:"concurrency"`
	RequestTimeout    string   `yaml:"request_timeout"`
	HealthTimeout     string   `yaml:"health_timeout"`
	MaxAttempts       int      `yaml:"max_attempts"`
	RetryInitialDelay string   `yaml:"retry_initial_delay"`
	RetryMaxDelay     string   `yaml:"retry_max_delay"`
	AllowedHosts      []string `yaml:"allowed_hosts"`
	AllowedCIDRs      []string `yaml:"allowed_cidrs"`
}

var GlobalConfig *Config

// LoadConfig loads configuration while preserving the environment variables and
// paths used by existing deployments. YAML loading is opt-in through
// AIDG_CONFIG_FILE or the legacy CONFIG_FILE variable.
func LoadConfig() (*Config, error) {
	cfg := defaultConfig()

	configPath := firstNonEmptyEnv("AIDG_CONFIG_FILE", "CONFIG_FILE")
	if configPath != "" {
		if err := loadYAML(configPath, cfg); err != nil {
			return nil, err
		}
	}

	applyLegacyEnvironment(cfg)
	applyAIDGEnvironment(cfg)
	if err := applySecretFiles(cfg); err != nil {
		return nil, err
	}

	GlobalConfig = cfg
	return cfg, nil
}

func defaultConfig() *Config {
	return &Config{
		Server: ServerConfig{Env: "dev", Port: "8000", Protocol: "http"},
		Data: DataConfig{
			ProjectsDir:  "./data/projects",
			UsersDir:     "./data/users",
			MeetingsDir:  "./data/meetings",
			AuditLogsDir: "./data/audit_logs",
		},
		Log: LogConfig{Level: "info", Format: "console"},
		Security: SecurityConfig{
			CORSAllowedOrigins: []string{"http://localhost:3000", "http://localhost:5173"},
		},
		Frontend: FrontendConfig{DistDir: "./frontend/dist"},
		MeetingV2: MeetingV2Config{
			MaxChunkSize:        16 << 20,
			MaxChunksPerSession: 100000,
		},
		ASR: ASRConfig{
			Profile:           "ollama",
			Concurrency:       2,
			RequestTimeout:    "120s",
			HealthTimeout:     "5s",
			MaxAttempts:       5,
			RetryInitialDelay: "2s",
			RetryMaxDelay:     "2m",
		},
	}
}

func loadYAML(path string, cfg *Config) error {
	data, err := readBoundedFile(path, maxConfigFileSize)
	if err != nil {
		return fmt.Errorf("read config file %q: %w", path, err)
	}

	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return fmt.Errorf("parse config file %q: %w", path, err)
	}
	allowed := map[string]map[string]bool{
		// host/timeouts and output/models are recognized legacy fields. They are
		// intentionally ignored while old mounted configuration files remain valid.
		"server": {"env": true, "port": true, "protocol": true, "tls_cert_file": true, "tls_key_file": true,
			"host": true, "read_timeout": true, "write_timeout": true},
		"data": {"projects_dir": true, "users_dir": true, "meetings_dir": true, "audit_logs_dir": true,
			"output_dir": true, "models_dir": true},
		"logging": {"level": true, "format": true, "output": true},
		"security": {"jwt_secret": true, "admin_default_password": true, "cors_allowed_origins": true,
			"cors_enabled": true, "dependency_service_auth_token": true},
		"frontend":   {"dist_dir": true},
		"meeting_v2": {"enabled": true, "max_chunk_size": true, "max_chunks_per_session": true},
		"asr": {"enabled": true, "profile": true, "endpoint": true, "model": true, "language": true,
			"api_key_file": true, "concurrency": true, "request_timeout": true, "health_timeout": true,
			"max_attempts": true, "retry_initial_delay": true, "retry_max_delay": true,
			"allowed_hosts": true, "allowed_cidrs": true},
	}
	legacy := map[string]map[string]bool{
		"dependency":  {"mode": true, "service_url": true, "shared_volume_path": true, "local_binary_paths": true, "timeout": true, "allowed_commands": true},
		"audio":       {"device_name": true, "sample_rate": true, "chunk_size_seconds": true},
		"whisper":     {"model_path": true, "language": true, "threads": true},
		"diarization": {"script_path": true, "num_speakers": true, "huggingface_token": true},
		"monitoring":  {"metrics_enabled": true, "metrics_port": true, "health_check_path": true, "dependency_health_check_interval": true},
		"resources":   {"main_service": true, "deps_service": true},
	}
	if err := validateYAMLKeys(&document, allowed, legacy); err != nil {
		return fmt.Errorf("parse config file %q: %w", path, err)
	}
	if err := document.Decode(cfg); err != nil {
		return fmt.Errorf("parse config file %q: %w", path, err)
	}
	return nil
}

func validateYAMLKeys(document *yaml.Node, allowed, legacy map[string]map[string]bool) error {
	if len(document.Content) == 0 {
		return nil
	}
	root := document.Content[0]
	if root.Kind != yaml.MappingNode {
		return fmt.Errorf("configuration root must be a mapping")
	}
	for i := 0; i < len(root.Content); i += 2 {
		section := root.Content[i].Value
		value := root.Content[i+1]
		fields, known := allowed[section]
		if !known {
			fields, known = legacy[section]
			if !known {
				return fmt.Errorf("unknown configuration section %q", section)
			}
		}
		if value.Kind != yaml.MappingNode {
			return fmt.Errorf("configuration section %q must be a mapping", section)
		}
		for j := 0; j < len(value.Content); j += 2 {
			field := value.Content[j].Value
			if !fields[field] {
				return fmt.Errorf("unknown configuration field %q in section %q", field, section)
			}
		}
	}
	return nil
}

func applyLegacyEnvironment(cfg *Config) {
	setStringFromEnv(&cfg.Server.Env, "ENV")
	setStringFromEnv(&cfg.Server.Port, "PORT")
	setStringFromEnv(&cfg.Server.Protocol, "SERVER_PROTOCOL")
	setStringFromEnv(&cfg.Server.TLSCert, "TLS_CERT_FILE")
	setStringFromEnv(&cfg.Server.TLSKey, "TLS_KEY_FILE")
	setStringFromEnv(&cfg.Data.ProjectsDir, "PROJECTS_DIR")
	setStringFromEnv(&cfg.Data.UsersDir, "USERS_DIR")
	setStringFromEnv(&cfg.Data.MeetingsDir, "MEETINGS_DIR")
	setStringFromEnv(&cfg.Data.AuditLogsDir, "AUDIT_LOGS_DIR")
	setStringFromEnv(&cfg.Log.Level, "LOG_LEVEL")
	setStringFromEnv(&cfg.Log.Format, "LOG_FORMAT")
	setStringFromEnv(&cfg.Security.JWTSecret, "USER_JWT_SECRET")
	setStringFromEnv(&cfg.Security.AdminDefaultPassword, "ADMIN_DEFAULT_PASSWORD")
	setListFromEnv(&cfg.Security.CORSAllowedOrigins, "CORS_ALLOWED_ORIGINS")
	setStringFromEnv(&cfg.Frontend.DistDir, "FRONTEND_DIST_DIR")
}

func applyAIDGEnvironment(cfg *Config) {
	setStringFromEnv(&cfg.Server.Env, "AIDG_SERVER_ENV")
	setStringFromEnv(&cfg.Server.Port, "AIDG_SERVER_PORT")
	setStringFromEnv(&cfg.Server.Protocol, "AIDG_SERVER_PROTOCOL")
	setStringFromEnv(&cfg.Server.TLSCert, "AIDG_SERVER_TLS_CERT_FILE")
	setStringFromEnv(&cfg.Server.TLSKey, "AIDG_SERVER_TLS_KEY_FILE")
	setStringFromEnv(&cfg.Data.ProjectsDir, "AIDG_DATA_PROJECTS_DIR")
	setStringFromEnv(&cfg.Data.UsersDir, "AIDG_DATA_USERS_DIR")
	setStringFromEnv(&cfg.Data.MeetingsDir, "AIDG_DATA_MEETINGS_DIR")
	setStringFromEnv(&cfg.Data.AuditLogsDir, "AIDG_DATA_AUDIT_LOGS_DIR")
	setStringFromEnv(&cfg.Log.Level, "AIDG_LOG_LEVEL")
	setStringFromEnv(&cfg.Log.Format, "AIDG_LOG_FORMAT")
	setStringFromEnv(&cfg.Security.JWTSecret, "AIDG_SECURITY_JWT_SECRET")
	setStringFromEnv(&cfg.Security.AdminDefaultPassword, "AIDG_SECURITY_ADMIN_DEFAULT_PASSWORD")
	setListFromEnv(&cfg.Security.CORSAllowedOrigins, "AIDG_SECURITY_CORS_ALLOWED_ORIGINS")
	setStringFromEnv(&cfg.Frontend.DistDir, "AIDG_FRONTEND_DIST_DIR")
	setBoolFromEnv(&cfg.MeetingV2.Enabled, "AIDG_MEETING_V2_ENABLED")
	setInt64FromEnv(&cfg.MeetingV2.MaxChunkSize, "AIDG_MEETING_V2_MAX_CHUNK_SIZE")
	setIntFromEnv(&cfg.MeetingV2.MaxChunksPerSession, "AIDG_MEETING_V2_MAX_CHUNKS_PER_SESSION")
	setBoolFromEnv(&cfg.ASR.Enabled, "AIDG_ASR_ENABLED")
	setStringFromEnv(&cfg.ASR.Profile, "AIDG_ASR_PROFILE")
	setStringFromEnv(&cfg.ASR.Endpoint, "AIDG_ASR_ENDPOINT")
	setStringFromEnv(&cfg.ASR.Model, "AIDG_ASR_MODEL")
	setStringFromEnv(&cfg.ASR.Language, "AIDG_ASR_LANGUAGE")
	setStringFromEnv(&cfg.ASR.APIKeyFile, "AIDG_ASR_API_KEY_FILE")
	setIntFromEnv(&cfg.ASR.Concurrency, "AIDG_ASR_CONCURRENCY")
	setStringFromEnv(&cfg.ASR.RequestTimeout, "AIDG_ASR_REQUEST_TIMEOUT")
	setStringFromEnv(&cfg.ASR.HealthTimeout, "AIDG_ASR_HEALTH_TIMEOUT")
	setIntFromEnv(&cfg.ASR.MaxAttempts, "AIDG_ASR_MAX_ATTEMPTS")
	setStringFromEnv(&cfg.ASR.RetryInitialDelay, "AIDG_ASR_RETRY_INITIAL_DELAY")
	setStringFromEnv(&cfg.ASR.RetryMaxDelay, "AIDG_ASR_RETRY_MAX_DELAY")
	setListFromEnv(&cfg.ASR.AllowedHosts, "AIDG_ASR_ALLOWED_HOSTS")
	setListFromEnv(&cfg.ASR.AllowedCIDRs, "AIDG_ASR_ALLOWED_CIDRS")
}

func applySecretFiles(cfg *Config) error {
	if value := os.Getenv("AIDG_ASR_API_KEY"); value != "" {
		cfg.ASR.APIKey = value
	}
	if path := firstNonEmptyEnv("AIDG_SECURITY_JWT_SECRET_FILE", "USER_JWT_SECRET_FILE"); path != "" {
		secret, err := readSecret(path)
		if err != nil {
			return fmt.Errorf("read JWT secret file: %w", err)
		}
		cfg.Security.JWTSecret = secret
	}
	if path := firstNonEmptyEnv("AIDG_SECURITY_ADMIN_DEFAULT_PASSWORD_FILE", "ADMIN_DEFAULT_PASSWORD_FILE"); path != "" {
		secret, err := readSecret(path)
		if err != nil {
			return fmt.Errorf("read admin password file: %w", err)
		}
		cfg.Security.AdminDefaultPassword = secret
	}
	if cfg.ASR.APIKeyFile != "" {
		secret, err := readSecret(cfg.ASR.APIKeyFile)
		if err != nil {
			return fmt.Errorf("read ASR API key file: %w", err)
		}
		cfg.ASR.APIKey = secret
	}
	return nil
}

func readSecret(path string) (string, error) {
	data, err := readBoundedFile(path, maxSecretFileSize)
	if err != nil {
		return "", err
	}
	secret := strings.TrimSpace(string(data))
	if secret == "" {
		return "", fmt.Errorf("secret file %q is empty", path)
	}
	if strings.IndexByte(secret, 0) >= 0 {
		return "", fmt.Errorf("secret file %q contains a NUL byte", path)
	}
	return secret, nil
}

func readBoundedFile(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("not a regular file")
	}
	if info.Size() > limit {
		return nil, fmt.Errorf("file exceeds %d bytes", limit)
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("file exceeds %d bytes", limit)
	}
	return data, nil
}

func ValidateConfig(cfg *Config) error {
	var validationErrors []string

	if cfg.Security.JWTSecret == "" {
		validationErrors = append(validationErrors, "USER_JWT_SECRET or AIDG_SECURITY_JWT_SECRET is required")
	} else if len(cfg.Security.JWTSecret) < 32 {
		validationErrors = append(validationErrors, "JWT secret must be at least 32 characters long")
	}

	if cfg.Server.Env == "production" && cfg.Security.AdminDefaultPassword != "" {
		if isWeakPassword(cfg.Security.AdminDefaultPassword) {
			validationErrors = append(validationErrors, "ADMIN_DEFAULT_PASSWORD cannot be a weak/default password in production")
		}
		if len(cfg.Security.AdminDefaultPassword) < 8 {
			validationErrors = append(validationErrors, "ADMIN_DEFAULT_PASSWORD must be at least 8 characters long in production")
		}
	}

	if port, err := strconv.Atoi(cfg.Server.Port); err != nil || port < 1 || port > 65535 {
		validationErrors = append(validationErrors, fmt.Sprintf("invalid server port: %s (must be 1-65535)", cfg.Server.Port))
	}
	if cfg.Server.Protocol != "http" && cfg.Server.Protocol != "https" {
		validationErrors = append(validationErrors, fmt.Sprintf("invalid server protocol: %s (must be http or https)", cfg.Server.Protocol))
	}
	if cfg.Server.Protocol == "https" && (cfg.Server.TLSCert == "" || cfg.Server.TLSKey == "") {
		validationErrors = append(validationErrors, "TLS certificate and key are required for HTTPS")
	}

	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[cfg.Log.Level] {
		validationErrors = append(validationErrors, fmt.Sprintf("invalid log level: %s", cfg.Log.Level))
	}
	validLogFormats := map[string]bool{"console": true, "text": true, "json": true}
	if !validLogFormats[cfg.Log.Format] {
		validationErrors = append(validationErrors, fmt.Sprintf("invalid log format: %s", cfg.Log.Format))
	}
	validEnvs := map[string]bool{"dev": true, "development": true, "staging": true, "production": true}
	if !validEnvs[cfg.Server.Env] {
		validationErrors = append(validationErrors, fmt.Sprintf("invalid environment: %s", cfg.Server.Env))
	}
	if value := os.Getenv("AIDG_MEETING_V2_ENABLED"); value != "" {
		if _, err := strconv.ParseBool(value); err != nil {
			validationErrors = append(validationErrors, "AIDG_MEETING_V2_ENABLED must be a boolean")
		}
	}
	if value := os.Getenv("AIDG_ASR_ENABLED"); value != "" {
		if _, err := strconv.ParseBool(value); err != nil {
			validationErrors = append(validationErrors, "AIDG_ASR_ENABLED must be a boolean")
		}
	}
	if cfg.MeetingV2.MaxChunkSize < 1024 || cfg.MeetingV2.MaxChunkSize > 1<<30 {
		validationErrors = append(validationErrors, "meeting_v2.max_chunk_size must be between 1 KiB and 1 GiB")
	}
	if cfg.MeetingV2.MaxChunksPerSession < 1 || cfg.MeetingV2.MaxChunksPerSession > 1000000 {
		validationErrors = append(validationErrors, "meeting_v2.max_chunks_per_session must be between 1 and 1000000")
	}
	if cfg.ASR.Enabled {
		validationErrors = append(validationErrors, validateASRConfig(cfg.ASR)...)
	}

	if len(validationErrors) > 0 {
		return fmt.Errorf("configuration validation failed:\n  - %s", strings.Join(validationErrors, "\n  - "))
	}
	return nil
}

func validateASRConfig(cfg ASRConfig) []string {
	var validationErrors []string
	if cfg.Profile != "ollama" && cfg.Profile != "omlx" {
		validationErrors = append(validationErrors, "asr.profile must be ollama or omlx")
	}
	parsed, err := url.Parse(cfg.Endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
		validationErrors = append(validationErrors, "asr.endpoint must be an absolute HTTP(S) URL")
	} else {
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			validationErrors = append(validationErrors, "asr.endpoint scheme must be http or https")
		}
		if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			validationErrors = append(validationErrors, "asr.endpoint must not contain userinfo, query, or fragment")
		}
		if parsed.Path != "" && parsed.Path != "/" {
			validationErrors = append(validationErrors, "asr.endpoint must not contain an API path")
		}
	}
	if strings.TrimSpace(cfg.Model) == "" {
		validationErrors = append(validationErrors, "asr.model is required when ASR is enabled")
	}
	if cfg.Concurrency < 1 || cfg.Concurrency > 16 {
		validationErrors = append(validationErrors, "asr.concurrency must be between 1 and 16")
	}
	if cfg.MaxAttempts < 1 || cfg.MaxAttempts > 20 {
		validationErrors = append(validationErrors, "asr.max_attempts must be between 1 and 20")
	}
	requestTimeout, requestErr := time.ParseDuration(cfg.RequestTimeout)
	if requestErr != nil || requestTimeout <= 0 || requestTimeout > 30*time.Minute {
		validationErrors = append(validationErrors, "asr.request_timeout must be between 1ns and 30m")
	}
	healthTimeout, healthErr := time.ParseDuration(cfg.HealthTimeout)
	if healthErr != nil || healthTimeout <= 0 || healthTimeout > time.Minute {
		validationErrors = append(validationErrors, "asr.health_timeout must be between 1ns and 1m")
	}
	initialDelay, initialErr := time.ParseDuration(cfg.RetryInitialDelay)
	maxDelay, maxErr := time.ParseDuration(cfg.RetryMaxDelay)
	if initialErr != nil || maxErr != nil || initialDelay <= 0 || maxDelay < initialDelay || maxDelay > time.Hour {
		validationErrors = append(validationErrors, "ASR retry delays are invalid")
	}
	if len(cfg.AllowedHosts) == 0 && len(cfg.AllowedCIDRs) == 0 {
		validationErrors = append(validationErrors, "asr.allowed_hosts or asr.allowed_cidrs is required")
	}
	for _, host := range cfg.AllowedHosts {
		if strings.TrimSpace(host) == "" || strings.ContainsAny(host, "/:@") {
			validationErrors = append(validationErrors, fmt.Sprintf("invalid ASR allowed host %q", host))
		}
	}
	for _, cidr := range cfg.AllowedCIDRs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			validationErrors = append(validationErrors, fmt.Sprintf("invalid ASR allowed CIDR %q", cidr))
		}
	}
	return validationErrors
}

func isWeakPassword(password string) bool {
	switch password {
	case "admin123", "changeme", "neteye@123", "lite-admin-default":
		return true
	default:
		return false
	}
}

func (c *Config) IsProduction() bool {
	return c.Server.Env == "production"
}

func (c *Config) IsDevelopment() bool {
	return c.Server.Env == "dev" || c.Server.Env == "development"
}

func (c *Config) GetServerAddr() string {
	return ":" + c.Server.Port
}

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
  Frontend:
    - Dist Dir: %s`,
		c.Server.Env, c.Server.Port, c.Data.ProjectsDir, c.Data.UsersDir,
		c.Data.MeetingsDir, c.Data.AuditLogsDir, c.Log.Level, c.Log.Format,
		maskSecret(c.Security.JWTSecret), maskSecret(c.Security.AdminDefaultPassword),
		c.Security.CORSAllowedOrigins, c.Frontend.DistDir)
}

func firstNonEmptyEnv(keys ...string) string {
	for _, key := range keys {
		if value := os.Getenv(key); value != "" {
			return value
		}
	}
	return ""
}

func setStringFromEnv(target *string, key string) {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		*target = value
	}
}

func setListFromEnv(target *[]string, key string) {
	if value, ok := os.LookupEnv(key); ok {
		*target = parseStringList(value)
	}
}

func setBoolFromEnv(target *bool, key string) {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			*target = false // validation will reject enabled dependent configuration where applicable
			return
		}
		*target = parsed
	}
}

func setIntFromEnv(target *int, key string) {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil {
			*target = -1
			return
		}
		*target = parsed
	}
}

func setInt64FromEnv(target *int64, key string) {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			*target = -1
			return
		}
		*target = parsed
	}
}

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

func maskSecret(secret string) string {
	if secret == "" {
		return "<not set>"
	}
	if len(secret) <= 8 {
		return "********"
	}
	return secret[:4] + "****" + secret[len(secret)-4:]
}
