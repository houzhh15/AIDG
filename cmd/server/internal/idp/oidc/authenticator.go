// Package oidc 提供 OIDC 认证器实现
package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	idp "github.com/houzhh15/AIDG/cmd/server/internal/idp"
)

// Authenticator OIDC 认证器
type Authenticator struct {
	config       *idp.OIDCConfig
	provider     *oidc.Provider
	oauth2Config *oauth2.Config
	verifier     *oidc.IDTokenVerifier
}

// NewAuthenticator 创建 OIDC 认证器
func NewAuthenticator(ctx context.Context, config *idp.OIDCConfig) (*Authenticator, error) {
	if config == nil {
		return nil, idp.ErrConfigInvalid
	}

	// 使用 go-oidc 发现 OIDC provider
	provider, err := oidc.NewProvider(ctx, config.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("failed to discover OIDC provider: %w", err)
	}

	// 配置 OAuth2
	oauth2Config := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		RedirectURL:  config.RedirectURI,
		Endpoint:     provider.Endpoint(),
		Scopes:       config.Scopes,
	}

	// 如果未指定 scopes，使用默认值
	if len(oauth2Config.Scopes) == 0 {
		oauth2Config.Scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}

	// 创建 ID Token 验证器
	verifier := provider.Verifier(&oidc.Config{
		ClientID: config.ClientID,
	})

	return &Authenticator{
		config:       config,
		provider:     provider,
		oauth2Config: oauth2Config,
		verifier:     verifier,
	}, nil
}

// Type 返回认证器类型
func (a *Authenticator) Type() string {
	return idp.TypeOIDC
}

// GetAuthorizationURL 生成授权 URL
func (a *Authenticator) GetAuthorizationURL(state string) string {
	return a.oauth2Config.AuthCodeURL(state)
}

// ExchangeCode 使用授权码交换 token
func (a *Authenticator) ExchangeCode(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := a.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}
	return token, nil
}

// GetUserInfo 获取用户信息
func (a *Authenticator) GetUserInfo(ctx context.Context, token *oauth2.Token) (map[string]any, error) {
	// 获取 ID Token
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in token response")
	}

	// 验证 ID Token
	idToken, err := a.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}

	// 提取 claims
	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("failed to parse claims: %w", err)
	}

	// 可选：从 userinfo endpoint 获取更多信息
	userInfo, err := a.provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if err == nil {
		var userInfoClaims map[string]any
		if err := userInfo.Claims(&userInfoClaims); err == nil {
			// 合并 userinfo claims
			for k, v := range userInfoClaims {
				if _, exists := claims[k]; !exists {
					claims[k] = v
				}
			}
		}
	}

	return claims, nil
}

// Authenticate 执行完整的 OIDC 认证流程
// credential 参数为授权码（code）
func (a *Authenticator) Authenticate(username, credential string) (*idp.AuthResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 交换 code 获取 token
	token, err := a.ExchangeCode(ctx, credential)
	if err != nil {
		return nil, idp.ErrAuthFailed
	}

	// 获取用户信息
	claims, err := a.GetUserInfo(ctx, token)
	if err != nil {
		return nil, idp.ErrAuthFailed
	}

	// 提取用户名
	usernameClaim := a.config.UsernameClaim
	if usernameClaim == "" {
		usernameClaim = "preferred_username"
	}

	extractedUsername, _ := getClaimString(claims, usernameClaim)
	if extractedUsername == "" {
		// 尝试其他常见 claims
		for _, claim := range []string{"preferred_username", "email", "sub"} {
			if v, ok := getClaimString(claims, claim); ok && v != "" {
				extractedUsername = v
				break
			}
		}
	}

	if extractedUsername == "" {
		return nil, fmt.Errorf("%w: cannot determine username from claims", idp.ErrAuthFailed)
	}

	// 提取其他信息
	email, _ := getClaimString(claims, "email")
	fullname, _ := getClaimString(claims, "name")
	if fullname == "" {
		fullname, _ = getClaimString(claims, "displayName")
	}

	// 提取外部 ID（优先使用 sub）
	externalID, _ := getClaimString(claims, "sub")
	if externalID == "" {
		externalID, _ = getClaimString(claims, "oid") // Azure AD
	}

	return &idp.AuthResult{
		ExternalID: externalID,
		Username:   extractedUsername,
		Email:      email,
		Fullname:   fullname,
		RawClaims:  claims,
	}, nil
}

// TestConnection 测试 OIDC 连接
func (a *Authenticator) TestConnection() (*idp.TestResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 尝试获取 discovery document
	discoveryURL := a.config.IssuerURL + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, "GET", discoveryURL, nil)
	if err != nil {
		return &idp.TestResult{
			Success: false,
			Message: fmt.Sprintf("failed to create request: %v", err),
		}, nil
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return &idp.TestResult{
			Success: false,
			Message: fmt.Sprintf("failed to connect to OIDC provider: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &idp.TestResult{
			Success: false,
			Message: fmt.Sprintf("OIDC provider returned status: %d", resp.StatusCode),
		}, nil
	}

	// 解析 discovery document
	var discovery map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return &idp.TestResult{
			Success: false,
			Message: fmt.Sprintf("failed to parse discovery document: %v", err),
		}, nil
	}

	// 提取端点信息
	authEndpoint, _ := discovery["authorization_endpoint"].(string)
	tokenEndpoint, _ := discovery["token_endpoint"].(string)
	userinfoEndpoint, _ := discovery["userinfo_endpoint"].(string)

	return &idp.TestResult{
		Success: true,
		Message: "OIDC provider is accessible",
		Details: map[string]any{
			"issuer":                 a.config.IssuerURL,
			"authorization_endpoint": authEndpoint,
			"token_endpoint":         tokenEndpoint,
			"userinfo_endpoint":      userinfoEndpoint,
		},
	}, nil
}

// GetConfig 返回 OIDC 配置
func (a *Authenticator) GetConfig() *idp.OIDCConfig {
	return a.config
}

// IsAutoCreateEnabled 返回是否启用自动创建用户
func (a *Authenticator) IsAutoCreateEnabled() bool {
	return a.config.AutoCreateUser
}

// GetDefaultScopes 返回新用户默认权限
func (a *Authenticator) GetDefaultScopes() []string {
	return a.config.DefaultScopes
}

// getClaimString 从 claims 中安全获取字符串值
func getClaimString(claims map[string]any, key string) (string, bool) {
	if v, ok := claims[key]; ok {
		if s, ok := v.(string); ok {
			return s, true
		}
	}
	return "", false
}
