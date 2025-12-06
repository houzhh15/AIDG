package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/houzhh15/AIDG/cmd/server/internal/idp"
	ldapauth "github.com/houzhh15/AIDG/cmd/server/internal/idp/ldap"
	oidcauth "github.com/houzhh15/AIDG/cmd/server/internal/idp/oidc"
	"github.com/houzhh15/AIDG/cmd/server/internal/idp/sync"
	"github.com/houzhh15/AIDG/cmd/server/internal/users"
)

// IdPHandler 身份源管理 API Handler
type IdPHandler struct {
	idpManager  *idp.Manager
	userManager *users.Manager
	syncService *sync.Service
}

// NewIdPHandler 创建 IdP Handler
func NewIdPHandler(idpManager *idp.Manager, userManager *users.Manager) *IdPHandler {
	return &IdPHandler{
		idpManager:  idpManager,
		userManager: userManager,
	}
}

// NewIdPHandlerWithSync 创建带同步服务的 IdP Handler
func NewIdPHandlerWithSync(idpManager *idp.Manager, userManager *users.Manager, syncService *sync.Service) *IdPHandler {
	return &IdPHandler{
		idpManager:  idpManager,
		userManager: userManager,
		syncService: syncService,
	}
}

// HandleListIdPs 列出所有身份源
// GET /api/v1/identity-providers
func (h *IdPHandler) HandleListIdPs(c *gin.Context) {
	list := h.idpManager.List()

	// 脱敏处理
	sanitized := make([]map[string]any, 0, len(list))
	for _, p := range list {
		item := h.sanitizeIdP(p)
		sanitized = append(sanitized, item)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    sanitized,
	})
}

// HandleGetIdP 获取单个身份源详情
// GET /api/v1/identity-providers/:id
func (h *IdPHandler) HandleGetIdP(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "id is required"})
		return
	}

	p, err := h.idpManager.Get(id)
	if err != nil {
		if err == idp.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "identity provider not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    h.sanitizeIdP(p),
	})
}

// HandleCreateIdP 创建身份源
// POST /api/v1/identity-providers
func (h *IdPHandler) HandleCreateIdP(c *gin.Context) {
	var input idp.IdentityProvider
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request body"})
		return
	}

	created, err := h.idpManager.Create(&input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    h.sanitizeIdP(created),
	})
}

// HandleUpdateIdP 更新身份源
// PUT /api/v1/identity-providers/:id
func (h *IdPHandler) HandleUpdateIdP(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "id is required"})
		return
	}

	var input idp.IdentityProvider
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request body"})
		return
	}

	updated, err := h.idpManager.Update(id, &input)
	if err != nil {
		if err == idp.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "identity provider not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    h.sanitizeIdP(updated),
	})
}

// HandleDeleteIdP 删除身份源
// DELETE /api/v1/identity-providers/:id
func (h *IdPHandler) HandleDeleteIdP(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "id is required"})
		return
	}

	if err := h.idpManager.Delete(id); err != nil {
		if err == idp.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "identity provider not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "identity provider deleted"})
}

// TestConnectionInput 测试连接请求体
type TestConnectionInput struct {
	ID     string          `json:"id,omitempty"` // 可选：已存在的身份源 ID，用于复用已保存的密码
	Type   string          `json:"type"`
	Config json.RawMessage `json:"config"`
}

const maskedPassword = "********"

// HandleTestConnection 测试身份源连接
// POST /api/v1/identity-providers/test
func (h *IdPHandler) HandleTestConnection(c *gin.Context) {
	var input TestConnectionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid request body"})
		return
	}

	var result *idp.TestResult
	var err error

	switch input.Type {
	case idp.TypeOIDC:
		var config idp.OIDCConfig
		if err := json.Unmarshal(input.Config, &config); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid OIDC config"})
			return
		}
		// 如果密码是掩码或为空，且提供了 idp_id，则使用已保存的密码
		if (config.ClientSecret == "" || config.ClientSecret == maskedPassword) && input.ID != "" {
			existingIdP, getErr := h.idpManager.Get(input.ID)
			if getErr == nil {
				existingConfig, _ := existingIdP.GetOIDCConfig()
				if existingConfig != nil {
					config.ClientSecret = existingConfig.ClientSecret
				}
			}
		}
		// 解密 client_secret（如果是加密的）
		if idp.IsEncrypted(config.ClientSecret) {
			decrypted, decErr := h.idpManager.DecryptSecret(config.ClientSecret)
			if decErr != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "failed to decrypt client_secret"})
				return
			}
			config.ClientSecret = decrypted
		}
		// 检查密码是否有效
		if config.ClientSecret == "" || config.ClientSecret == maskedPassword {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "client_secret is required"})
			return
		}
		auth, authErr := oidcauth.NewAuthenticator(c.Request.Context(), &config)
		if authErr != nil {
			result = &idp.TestResult{
				Success: false,
				Message: authErr.Error(),
			}
		} else {
			result, err = auth.TestConnection()
		}

	case idp.TypeLDAP:
		var config idp.LDAPConfig
		if err := json.Unmarshal(input.Config, &config); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid LDAP config"})
			return
		}
		// 如果密码是掩码或为空，且提供了 idp_id，则使用已保存的密码
		if (config.BindPassword == "" || config.BindPassword == maskedPassword) && input.ID != "" {
			existingIdP, getErr := h.idpManager.Get(input.ID)
			if getErr == nil {
				existingConfig, _ := existingIdP.GetLDAPConfig()
				if existingConfig != nil {
					config.BindPassword = existingConfig.BindPassword
				}
			}
		}
		// 解密 bind_password（如果是加密的）
		if idp.IsEncrypted(config.BindPassword) {
			decrypted, decErr := h.idpManager.DecryptSecret(config.BindPassword)
			if decErr != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "failed to decrypt bind_password"})
				return
			}
			config.BindPassword = decrypted
		}
		// 检查密码是否有效
		if config.BindPassword == "" || config.BindPassword == maskedPassword {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "bind_password is required"})
			return
		}
		auth, authErr := ldapauth.NewAuthenticator(&config)
		if authErr != nil {
			result = &idp.TestResult{
				Success: false,
				Message: authErr.Error(),
			}
		} else {
			result, err = auth.TestConnection()
		}

	default:
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "unsupported type"})
		return
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// HandleListPublicIdPs 列出公开的身份源（无需认证）
// GET /api/v1/identity-providers/public
func (h *IdPHandler) HandleListPublicIdPs(c *gin.Context) {
	public := h.idpManager.ListPublic()
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    public,
	})
}

// ==================== 认证相关 Handler ====================

// HandleOIDCLogin 发起 OIDC 登录
// GET /auth/oidc/:idp_id/login
func (h *IdPHandler) HandleOIDCLogin(c *gin.Context) {
	idpID := c.Param("idp_id")
	if idpID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "idp_id is required"})
		return
	}

	// 获取身份源配置
	provider, err := h.idpManager.GetForAuth(idpID)
	if err != nil {
		if err == idp.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "identity provider not found"})
			return
		}
		if err == idp.ErrDisabled {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "identity provider is disabled"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	if provider.Type != idp.TypeOIDC {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "not an OIDC identity provider"})
		return
	}

	// 获取 OIDC 配置
	config, err := provider.GetOIDCConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "invalid OIDC config"})
		return
	}

	// 创建认证器
	auth, err := oidcauth.NewAuthenticator(c.Request.Context(), config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 生成 state
	state, err := generateState()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to generate state"})
		return
	}

	// 将 state 和 idp_id 存入 cookie（有效期 5 分钟）
	stateData := map[string]string{"state": state, "idp_id": idpID}
	stateJSON, _ := json.Marshal(stateData)
	c.SetCookie("oidc_state", base64.StdEncoding.EncodeToString(stateJSON), 300, "/", "", false, true)

	// 重定向到 IdP
	authURL := auth.GetAuthorizationURL(state)
	c.Redirect(http.StatusFound, authURL)
}

// HandleOIDCCallback 处理 OIDC 回调
// GET /auth/callback
func (h *IdPHandler) HandleOIDCCallback(c *gin.Context) {
	// 获取授权码
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		errorDesc := c.Query("error_description")
		if errorDesc == "" {
			errorDesc = c.Query("error")
		}
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "authorization failed: " + errorDesc})
		return
	}

	// 验证 state
	stateCookie, err := c.Cookie("oidc_state")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "state cookie not found"})
		return
	}

	stateJSON, err := base64.StdEncoding.DecodeString(stateCookie)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid state cookie"})
		return
	}

	var stateData map[string]string
	if err := json.Unmarshal(stateJSON, &stateData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "invalid state data"})
		return
	}

	if stateData["state"] != state {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "state mismatch"})
		return
	}

	idpID := stateData["idp_id"]

	// 清除 state cookie
	c.SetCookie("oidc_state", "", -1, "/", "", false, true)

	// 获取身份源配置
	provider, err := h.idpManager.GetForAuth(idpID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	config, err := provider.GetOIDCConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "invalid OIDC config"})
		return
	}

	// 创建认证器并执行认证
	auth, err := oidcauth.NewAuthenticator(c.Request.Context(), config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	result, err := auth.Authenticate("", code)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "authentication failed"})
		return
	}

	// 查找或创建用户
	user, created, err := h.userManager.FindOrCreateExternalUser(
		result.ExternalID,
		result.Username,
		result.Email,
		result.Fullname,
		idpID,
		auth.GetDefaultScopes(),
		auth.IsAutoCreateEnabled(),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 生成 JWT Token
	token, err := h.userManager.GenerateToken(user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to generate token"})
		return
	}

	// 重定向到前端登录成功页
	redirectURL := "/login/success?token=" + token
	if created {
		redirectURL += "&new_user=true"
	}
	c.Redirect(http.StatusFound, redirectURL)
}

// HandleLDAPLogin 处理 LDAP 登录
// POST /api/v1/login (with idp_id)
func (h *IdPHandler) HandleLDAPLogin(c *gin.Context, idpID, username, password string) {
	// 获取身份源配置
	provider, err := h.idpManager.GetForAuth(idpID)
	if err != nil {
		if err == idp.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "identity provider not found"})
			return
		}
		if err == idp.ErrDisabled {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "identity provider is disabled"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	if provider.Type != idp.TypeLDAP {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "not an LDAP identity provider"})
		return
	}

	// 获取 LDAP 配置
	config, err := provider.GetLDAPConfig()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "invalid LDAP config"})
		return
	}

	// 创建认证器并执行认证
	auth, err := ldapauth.NewAuthenticator(config)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": fmt.Sprintf("create authenticator failed: %v", err)})
		return
	}

	result, err := auth.Authenticate(username, password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": fmt.Sprintf("authentication failed: %v", err)})
		return
	}

	// 查找或创建用户
	user, _, err := h.userManager.FindOrCreateExternalUser(
		result.ExternalID,
		result.Username,
		result.Email,
		result.Fullname,
		idpID,
		auth.GetDefaultScopes(),
		auth.IsAutoCreateEnabled(),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	// 生成 JWT Token
	token, err := h.userManager.GenerateToken(user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "failed to generate token"})
		return
	}

	// 返回格式与本地登录保持一致
	c.JSON(http.StatusOK, gin.H{
		"token":    token,
		"username": user.Username,
		"scopes":   user.Scopes,
	})
}

// ==================== 辅助方法 ====================

// sanitizeIdP 脱敏身份源配置
func (h *IdPHandler) sanitizeIdP(p *idp.IdentityProvider) map[string]any {
	result := map[string]any{
		"id":         p.ID,
		"name":       p.Name,
		"type":       p.Type,
		"status":     p.Status,
		"priority":   p.Priority,
		"created_at": p.CreatedAt,
		"updated_at": p.UpdatedAt,
	}

	// 脱敏配置中的敏感字段
	switch p.Type {
	case idp.TypeOIDC:
		config, err := p.GetOIDCConfig()
		if err == nil {
			// 密码用掩码显示，保护安全
			maskedSecret := ""
			if config.ClientSecret != "" {
				maskedSecret = "********"
			}
			result["config"] = map[string]any{
				"issuer_url":       config.IssuerURL,
				"client_id":        config.ClientID,
				"client_secret":    maskedSecret,
				"redirect_uri":     config.RedirectURI,
				"scopes":           config.Scopes,
				"username_claim":   config.UsernameClaim,
				"auto_create_user": config.AutoCreateUser,
				"default_scopes":   config.DefaultScopes,
			}
		}

	case idp.TypeLDAP:
		config, err := p.GetLDAPConfig()
		if err == nil {
			// 密码用掩码显示，保护安全
			maskedPassword := ""
			if config.BindPassword != "" {
				maskedPassword = "********"
			}
			result["config"] = map[string]any{
				"server_url":         config.ServerURL,
				"base_dn":            config.BaseDN,
				"bind_dn":            config.BindDN,
				"bind_password":      maskedPassword,
				"user_filter":        config.UserFilter,
				"group_filter":       config.GroupFilter,
				"username_attribute": config.UsernameAttribute,
				"email_attribute":    config.EmailAttribute,
				"fullname_attribute": config.FullnameAttribute,
				"use_tls":            config.UseTLS,
				"skip_verify":        config.SkipVerify,
				"auto_create_user":   config.AutoCreateUser,
				"default_scopes":     config.DefaultScopes,
			}
		}
	}

	if p.Sync != nil {
		result["sync"] = p.Sync
	}

	return result
}

// generateState 生成随机 state 值
func generateState() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// GetAuthenticatorByID 根据身份源 ID 获取认证器
func (h *IdPHandler) GetAuthenticatorByID(ctx context.Context, idpID string) (idp.Authenticator, error) {
	provider, err := h.idpManager.GetForAuth(idpID)
	if err != nil {
		return nil, err
	}

	switch provider.Type {
	case idp.TypeOIDC:
		config, err := provider.GetOIDCConfig()
		if err != nil {
			return nil, err
		}
		return oidcauth.NewAuthenticator(ctx, config)

	case idp.TypeLDAP:
		config, err := provider.GetLDAPConfig()
		if err != nil {
			return nil, err
		}
		return ldapauth.NewAuthenticator(config)

	default:
		return nil, idp.ErrConfigInvalid
	}
}

// GetIdPManager 返回 IdP Manager
func (h *IdPHandler) GetIdPManager() *idp.Manager {
	return h.idpManager
}

// GetSyncService 返回同步服务
func (h *IdPHandler) GetSyncService() *sync.Service {
	return h.syncService
}

// ==================== 同步相关 Handler ====================

// HandleTriggerSync 触发同步
// POST /api/v1/identity-providers/:id/sync
func (h *IdPHandler) HandleTriggerSync(c *gin.Context) {
	if h.syncService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "sync service not available"})
		return
	}

	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "id is required"})
		return
	}

	log, err := h.syncService.SyncNow(id)
	if err != nil {
		if err == idp.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": "identity provider not found"})
			return
		}
		if err == idp.ErrSyncInProgress {
			c.JSON(http.StatusConflict, gin.H{"success": false, "error": "sync is already in progress"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    log,
	})
}

// HandleGetSyncStatus 获取同步状态
// GET /api/v1/identity-providers/:id/sync/status
func (h *IdPHandler) HandleGetSyncStatus(c *gin.Context) {
	if h.syncService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "sync service not available"})
		return
	}

	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "id is required"})
		return
	}

	log, found := h.syncService.GetSyncStatus(id)
	if !found {
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"data": gin.H{
				"status":  "never_synced",
				"message": "No sync has been performed yet",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    log,
	})
}

// HandleGetSyncLogs 获取同步日志
// GET /api/v1/identity-providers/:id/sync/logs
func (h *IdPHandler) HandleGetSyncLogs(c *gin.Context) {
	if h.syncService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "error": "sync service not available"})
		return
	}

	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": "id is required"})
		return
	}

	limit := 10
	if limitStr := c.Query("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	logs, err := h.syncService.GetSyncLogs(id, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    logs,
	})
}
