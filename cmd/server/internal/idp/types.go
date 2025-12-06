// Package idp 提供身份源集成模块的数据类型定义
package idp

import (
	"encoding/json"
	"time"
)

// IdP 类型常量
const (
	TypeOIDC = "OIDC"
	TypeLDAP = "LDAP"
)

// IdP 状态常量
const (
	StatusEnabled  = "Enabled"
	StatusDisabled = "Disabled"
)

// 冲突策略常量
const (
	ConflictPolicyOverride = "override"
	ConflictPolicyIgnore   = "ignore"
)

// IdentityProvider 身份源配置
type IdentityProvider struct {
	ID        string          `json:"id"`             // 唯一标识，如 idp_xxxxxxxx
	Name      string          `json:"name"`           // 显示名称
	Type      string          `json:"type"`           // OIDC | LDAP
	Status    string          `json:"status"`         // Enabled | Disabled
	Priority  int             `json:"priority"`       // 优先级，越小越高
	Config    json.RawMessage `json:"config"`         // 类型相关配置（延迟解析）
	Sync      *SyncConfig     `json:"sync,omitempty"` // 同步配置
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}

// OIDCConfig OIDC 配置
type OIDCConfig struct {
	IssuerURL      string   `json:"issuer_url"`                 // OIDC Issuer URL
	ClientID       string   `json:"client_id"`                  // OAuth2 Client ID
	ClientSecret   string   `json:"client_secret"`              // OAuth2 Client Secret (加密存储)
	RedirectURI    string   `json:"redirect_uri"`               // OAuth2 Redirect URI
	Scopes         []string `json:"scopes"`                     // OAuth2 Scopes
	UsernameClaim  string   `json:"username_claim,omitempty"`   // 用户名 claim 字段，默认 preferred_username
	AutoCreateUser bool     `json:"auto_create_user,omitempty"` // 是否自动创建用户
	DefaultScopes  []string `json:"default_scopes,omitempty"`   // 新用户默认权限
}

// LDAPConfig LDAP 配置
type LDAPConfig struct {
	ServerURL         string   `json:"server_url"`                   // LDAP Server URL (ldap:// 或 ldaps://)
	BaseDN            string   `json:"base_dn"`                      // 搜索基础 DN
	BindDN            string   `json:"bind_dn"`                      // 服务账号 DN
	BindPassword      string   `json:"bind_password"`                // 服务账号密码 (加密存储)
	UserFilter        string   `json:"user_filter"`                  // 用户搜索过滤器，如 (sAMAccountName=%s)
	GroupFilter       string   `json:"group_filter,omitempty"`       // 组搜索过滤器
	UsernameAttribute string   `json:"username_attribute,omitempty"` // 用户名属性，默认 sAMAccountName
	EmailAttribute    string   `json:"email_attribute,omitempty"`    // 邮箱属性，默认 mail
	FullnameAttribute string   `json:"fullname_attribute,omitempty"` // 全名属性，默认 displayName
	UseTLS            bool     `json:"use_tls,omitempty"`            // 是否使用 StartTLS
	SkipVerify        bool     `json:"skip_verify,omitempty"`        // 是否跳过证书验证
	AutoCreateUser    bool     `json:"auto_create_user,omitempty"`   // 是否自动创建用户
	DefaultScopes     []string `json:"default_scopes,omitempty"`     // 新用户默认权限
}

// SyncConfig 同步配置
type SyncConfig struct {
	SyncEnabled     bool   `json:"sync_enabled"`                // 是否启用同步
	SyncInterval    string `json:"sync_interval,omitempty"`     // 同步间隔，如 "1h", "6h"
	ConflictPolicy  string `json:"conflict_policy,omitempty"`   // 冲突策略: override | ignore
	DisableOnRemove bool   `json:"disable_on_remove,omitempty"` // 用户删除时禁用
}

// AuthResult 认证结果
type AuthResult struct {
	ExternalID string         `json:"external_id"`          // 外部系统用户 ID
	Username   string         `json:"username"`             // 用户名
	Email      string         `json:"email,omitempty"`      // 邮箱
	Fullname   string         `json:"fullname,omitempty"`   // 全名
	RawClaims  map[string]any `json:"raw_claims,omitempty"` // 原始 claims/attributes
}

// TestResult 连接测试结果
type TestResult struct {
	Success bool           `json:"success"`           // 是否成功
	Message string         `json:"message"`           // 结果消息
	Details map[string]any `json:"details,omitempty"` // 详细信息
}

// SyncLog 同步日志
type SyncLog struct {
	SyncID     string    `json:"sync_id"`         // 同步任务 ID
	IdpID      string    `json:"idp_id"`          // 身份源 ID
	StartedAt  time.Time `json:"started_at"`      // 开始时间
	FinishedAt time.Time `json:"finished_at"`     // 结束时间
	Status     string    `json:"status"`          // running | completed | failed
	Stats      SyncStats `json:"stats"`           // 统计信息
	Error      string    `json:"error,omitempty"` // 错误信息
}

// SyncStats 同步统计
type SyncStats struct {
	TotalFetched int `json:"total_fetched"` // 获取的总用户数
	Created      int `json:"created"`       // 新创建用户数
	Updated      int `json:"updated"`       // 更新用户数
	Disabled     int `json:"disabled"`      // 禁用用户数
	Skipped      int `json:"skipped"`       // 跳过用户数
	Errors       int `json:"errors"`        // 错误数
}

// SyncLogStatus 同步日志状态常量
const (
	SyncStatusRunning   = "running"
	SyncStatusCompleted = "completed"
	SyncStatusFailed    = "failed"
)

// Authenticator 认证器接口
type Authenticator interface {
	// Type 返回认证器类型
	Type() string

	// Authenticate 执行认证
	// 对于 OIDC：credential 为授权码
	// 对于 LDAP：credential 为用户密码
	Authenticate(username, credential string) (*AuthResult, error)

	// TestConnection 测试连接
	TestConnection() (*TestResult, error)
}

// GetOIDCConfig 从 IdentityProvider 解析 OIDC 配置
func (idp *IdentityProvider) GetOIDCConfig() (*OIDCConfig, error) {
	if idp.Type != TypeOIDC {
		return nil, ErrTypeMismatch
	}
	var config OIDCConfig
	if err := json.Unmarshal(idp.Config, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// GetLDAPConfig 从 IdentityProvider 解析 LDAP 配置
func (idp *IdentityProvider) GetLDAPConfig() (*LDAPConfig, error) {
	if idp.Type != TypeLDAP {
		return nil, ErrTypeMismatch
	}
	var config LDAPConfig
	if err := json.Unmarshal(idp.Config, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

// SetConfig 设置配置（序列化为 json.RawMessage）
func (idp *IdentityProvider) SetConfig(config any) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}
	idp.Config = data
	return nil
}

// PublicInfo 返回公开信息（无敏感字段）
type PublicInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Priority int    `json:"priority"`
}

// ToPublicInfo 转换为公开信息
func (idp *IdentityProvider) ToPublicInfo() *PublicInfo {
	return &PublicInfo{
		ID:       idp.ID,
		Name:     idp.Name,
		Type:     idp.Type,
		Priority: idp.Priority,
	}
}
