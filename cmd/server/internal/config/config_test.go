package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func clearConfigEnvironment(t *testing.T) {
	t.Helper()
	keys := []string{
		"AIDG_CONFIG_FILE", "CONFIG_FILE",
		"ENV", "PORT", "SERVER_PROTOCOL", "TLS_CERT_FILE", "TLS_KEY_FILE",
		"PROJECTS_DIR", "USERS_DIR", "MEETINGS_DIR", "AUDIT_LOGS_DIR",
		"LOG_LEVEL", "LOG_FORMAT", "USER_JWT_SECRET", "ADMIN_DEFAULT_PASSWORD",
		"CORS_ALLOWED_ORIGINS", "FRONTEND_DIST_DIR",
		"AIDG_SERVER_ENV", "AIDG_SERVER_PORT", "AIDG_SERVER_PROTOCOL",
		"AIDG_SERVER_TLS_CERT_FILE", "AIDG_SERVER_TLS_KEY_FILE",
		"AIDG_DATA_PROJECTS_DIR", "AIDG_DATA_USERS_DIR", "AIDG_DATA_MEETINGS_DIR",
		"AIDG_DATA_AUDIT_LOGS_DIR", "AIDG_LOG_LEVEL", "AIDG_LOG_FORMAT",
		"AIDG_SECURITY_JWT_SECRET", "AIDG_SECURITY_ADMIN_DEFAULT_PASSWORD",
		"AIDG_SECURITY_CORS_ALLOWED_ORIGINS", "AIDG_FRONTEND_DIST_DIR",
		"AIDG_SECURITY_JWT_SECRET_FILE", "USER_JWT_SECRET_FILE",
		"AIDG_SECURITY_ADMIN_DEFAULT_PASSWORD_FILE", "ADMIN_DEFAULT_PASSWORD_FILE",
		"AIDG_MEETING_V2_ENABLED", "AIDG_MEETING_V2_MAX_CHUNK_SIZE", "AIDG_MEETING_V2_MAX_CHUNKS_PER_SESSION",
		"AIDG_ASR_ENABLED", "AIDG_ASR_PROFILE", "AIDG_ASR_ENDPOINT", "AIDG_ASR_MODEL", "AIDG_ASR_LANGUAGE",
		"AIDG_ASR_API_KEY", "AIDG_ASR_API_KEY_FILE", "AIDG_ASR_CONCURRENCY", "AIDG_ASR_REQUEST_TIMEOUT",
		"AIDG_ASR_HEALTH_TIMEOUT", "AIDG_ASR_MAX_ATTEMPTS", "AIDG_ASR_RETRY_INITIAL_DELAY", "AIDG_ASR_RETRY_MAX_DELAY",
		"AIDG_ASR_ALLOWED_HOSTS", "AIDG_ASR_ALLOWED_CIDRS",
	}
	for _, key := range keys {
		t.Setenv(key, "")
	}
}

func TestLoadConfigPrecedence(t *testing.T) {
	clearConfigEnvironment(t)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "aidg.yaml")
	configData := `
server:
  env: staging
  port: "7000"
data:
  projects_dir: /yaml/projects
logging:
  level: warn
  format: json
security:
  jwt_secret: yaml-secret-that-is-at-least-thirty-two-characters
  admin_default_password: yaml-password
  cors_allowed_origins: ["https://yaml.example"]
frontend:
  dist_dir: /yaml/frontend
`
	if err := os.WriteFile(configPath, []byte(configData), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("CONFIG_FILE", configPath)
	t.Setenv("PORT", "7100")
	t.Setenv("AIDG_SERVER_PORT", "7200")
	t.Setenv("PROJECTS_DIR", "/legacy/projects")
	t.Setenv("AIDG_LOG_LEVEL", "debug")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != "7200" {
		t.Fatalf("expected AIDG port override, got %q", cfg.Server.Port)
	}
	if cfg.Data.ProjectsDir != "/legacy/projects" {
		t.Fatalf("expected legacy directory override, got %q", cfg.Data.ProjectsDir)
	}
	if cfg.Log.Level != "debug" || cfg.Log.Format != "json" {
		t.Fatalf("unexpected logging config: %#v", cfg.Log)
	}
	if cfg.Frontend.DistDir != "/yaml/frontend" {
		t.Fatalf("expected YAML frontend path, got %q", cfg.Frontend.DistDir)
	}
}

func TestLoadConfigAcceptsHistoricalNumericPort(t *testing.T) {
	clearConfigEnvironment(t)
	path := filepath.Join(t.TempDir(), "legacy.yaml")
	if err := os.WriteFile(path, []byte("server:\n  port: 8081\nlogging:\n  format: text\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_FILE", path)
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Server.Port != "8081" || cfg.Log.Format != "text" {
		t.Fatalf("historical YAML was not loaded: %#v", cfg)
	}
}

func TestLoadConfigAcceptsLegacySectionsButRejectsUnknownKeys(t *testing.T) {
	clearConfigEnvironment(t)
	legacyPath := filepath.Join(t.TempDir(), "legacy.yaml")
	legacy := "server:\n  port: 8081\n  host: 0.0.0.0\ndata:\n  output_dir: /data\ndependency:\n  mode: fallback\nlogging:\n  output: stdout\n"
	if err := os.WriteFile(legacyPath, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_FILE", legacyPath)
	if _, err := LoadConfig(); err != nil {
		t.Fatalf("legacy config should remain loadable: %v", err)
	}

	clearConfigEnvironment(t)
	unknownPath := filepath.Join(t.TempDir(), "unknown.yaml")
	if err := os.WriteFile(unknownPath, []byte("server:\n  porrt: 8000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_FILE", unknownPath)
	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "porrt") {
		t.Fatalf("expected unknown field error, got %v", err)
	}

	clearConfigEnvironment(t)
	legacyTypoPath := filepath.Join(t.TempDir(), "legacy-typo.yaml")
	if err := os.WriteFile(legacyTypoPath, []byte("dependency:\n  modde: fallback\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_FILE", legacyTypoPath)
	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "modde") {
		t.Fatalf("expected legacy section typo error, got %v", err)
	}
}

func TestLoadConfigSecretFilesHaveHighestPrecedence(t *testing.T) {
	clearConfigEnvironment(t)
	dir := t.TempDir()
	jwtPath := filepath.Join(dir, "jwt")
	passwordPath := filepath.Join(dir, "password")
	jwtSecret := "file-secret-that-is-at-least-thirty-two-characters"
	if err := os.WriteFile(jwtPath, []byte(jwtSecret+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(passwordPath, []byte("password-from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("USER_JWT_SECRET", "legacy-secret-that-is-at-least-thirty-two-chars")
	t.Setenv("AIDG_SECURITY_JWT_SECRET", "aidg-secret-that-is-at-least-thirty-two-chars")
	t.Setenv("AIDG_SECURITY_JWT_SECRET_FILE", jwtPath)
	t.Setenv("ADMIN_DEFAULT_PASSWORD", "password-from-env")
	t.Setenv("AIDG_SECURITY_ADMIN_DEFAULT_PASSWORD_FILE", passwordPath)

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Security.JWTSecret != jwtSecret {
		t.Fatalf("expected file JWT secret, got %q", cfg.Security.JWTSecret)
	}
	if cfg.Security.AdminDefaultPassword != "password-from-file" {
		t.Fatalf("expected file password, got %q", cfg.Security.AdminDefaultPassword)
	}
}

func TestLoadConfigRejectsOversizedAndNonRegularFiles(t *testing.T) {
	clearConfigEnvironment(t)
	dir := t.TempDir()
	t.Setenv("CONFIG_FILE", dir)
	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("expected non-regular file error, got %v", err)
	}

	clearConfigEnvironment(t)
	path := filepath.Join(t.TempDir(), "large.yaml")
	if err := os.WriteFile(path, make([]byte, maxConfigFileSize+1), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONFIG_FILE", path)
	if _, err := LoadConfig(); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected oversized file error, got %v", err)
	}
}

func TestValidateConfigRejectsLiteDefaults(t *testing.T) {
	cfg := defaultConfig()
	cfg.Server.Env = "production"
	cfg.Security.JWTSecret = "lite-mode-default-secret-key-32chars!!"
	cfg.Security.AdminDefaultPassword = "lite-admin-default"
	if err := ValidateConfig(cfg); err == nil {
		t.Fatal("expected validation error for legacy Lite defaults")
	}
}

func TestValidateConfigAcceptsLegacyTextLogFormat(t *testing.T) {
	cfg := defaultConfig()
	cfg.Log.Format = "text"
	cfg.Security.JWTSecret = "valid-secret-that-is-at-least-thirty-two-characters"
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("expected text log format to remain compatible: %v", err)
	}
}

func TestASRConfigDefaultsDisabledAndValidatesBoundary(t *testing.T) {
	clearConfigEnvironment(t)
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MeetingV2.Enabled || cfg.ASR.Enabled {
		t.Fatal("new meeting and ASR features must default disabled")
	}
	cfg.Security.JWTSecret = "valid-secret-that-is-at-least-thirty-two-characters"
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("disabled ASR should not require endpoint: %v", err)
	}
	cfg.ASR.Enabled = true
	cfg.ASR.Endpoint = "file:///tmp/model"
	cfg.ASR.Model = "whisper"
	cfg.ASR.AllowedHosts = []string{"localhost"}
	if err := ValidateConfig(cfg); err == nil {
		t.Fatal("expected unsafe ASR endpoint rejection")
	}
	cfg.ASR.Endpoint = "http://localhost:11434"
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("expected valid ASR config: %v", err)
	}
}

func TestASRAPIKeyFileOverridesEnvironment(t *testing.T) {
	clearConfigEnvironment(t)
	path := filepath.Join(t.TempDir(), "asr-key")
	if err := os.WriteFile(path, []byte("file-key\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AIDG_ASR_API_KEY", "environment-key")
	t.Setenv("AIDG_ASR_API_KEY_FILE", path)
	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ASR.APIKey != "file-key" {
		t.Fatalf("secret file must have highest precedence, got %q", cfg.ASR.APIKey)
	}
	if strings.Contains(cfg.PrintConfig(), "file-key") {
		t.Fatal("ASR API key leaked in printed config")
	}
}
