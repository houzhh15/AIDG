// Package ldapauth 提供 LDAP 认证器实现
package ldapauth

import (
	"crypto/tls"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"

	idp "github.com/houzhh15/AIDG/cmd/server/internal/idp"
)

const (
	// 默认属性名
	defaultUsernameAttr = "sAMAccountName"
	defaultEmailAttr    = "mail"
	defaultFullnameAttr = "displayName"

	// 分页大小
	pageSize = 500

	// 连接超时
	dialTimeout = 10 * time.Second
)

// Authenticator LDAP 认证器
type Authenticator struct {
	config *idp.LDAPConfig
}

// NewAuthenticator 创建 LDAP 认证器
func NewAuthenticator(config *idp.LDAPConfig) (*Authenticator, error) {
	if config == nil {
		return nil, idp.ErrConfigInvalid
	}

	// 验证必填配置
	if config.ServerURL == "" {
		return nil, fmt.Errorf("%w: server_url is required", idp.ErrConfigInvalid)
	}
	if config.BaseDN == "" {
		return nil, fmt.Errorf("%w: base_dn is required", idp.ErrConfigInvalid)
	}

	return &Authenticator{config: config}, nil
}

// Type 返回认证器类型
func (a *Authenticator) Type() string {
	return idp.TypeLDAP
}

// connect 建立 LDAP 连接
func (a *Authenticator) connect() (*ldap.Conn, error) {
	u, err := url.Parse(a.config.ServerURL)
	if err != nil {
		return nil, fmt.Errorf("invalid server URL: %w", err)
	}

	var conn *ldap.Conn
	host := u.Host
	if u.Port() == "" {
		if u.Scheme == "ldaps" {
			host = host + ":636"
		} else {
			host = host + ":389"
		}
	}

	// 根据协议类型连接
	if u.Scheme == "ldaps" {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: a.config.SkipVerify,
		}
		conn, err = ldap.DialURL(a.config.ServerURL, ldap.DialWithTLSConfig(tlsConfig))
	} else {
		conn, err = ldap.DialURL(a.config.ServerURL)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to LDAP server: %w", err)
	}

	// 如果使用 StartTLS
	if a.config.UseTLS && u.Scheme != "ldaps" {
		tlsConfig := &tls.Config{
			InsecureSkipVerify: a.config.SkipVerify,
			ServerName:         u.Hostname(),
		}
		if err := conn.StartTLS(tlsConfig); err != nil {
			conn.Close()
			return nil, fmt.Errorf("failed to start TLS: %w", err)
		}
	}

	return conn, nil
}

// bindServiceAccount 使用服务账号绑定
func (a *Authenticator) bindServiceAccount(conn *ldap.Conn) error {
	if a.config.BindDN == "" {
		return nil // 匿名绑定
	}
	if err := conn.Bind(a.config.BindDN, a.config.BindPassword); err != nil {
		return fmt.Errorf("service account bind failed: %w", err)
	}
	return nil
}

// SearchUser 搜索用户
func (a *Authenticator) SearchUser(username string) (*ldap.Entry, error) {
	conn, err := a.connect()
	if err != nil {
		return nil, fmt.Errorf("connect failed: %w", err)
	}
	defer conn.Close()

	// 使用服务账号绑定
	if err := a.bindServiceAccount(conn); err != nil {
		return nil, fmt.Errorf("bind service account failed: %w", err)
	}

	// 构建搜索过滤器
	filter := a.config.UserFilter
	if filter == "" {
		filter = "(sAMAccountName=%s)"
	}
	// 支持多种占位符格式：%s, {username}, {{username}}
	escapedUsername := ldap.EscapeFilter(username)
	filter = strings.ReplaceAll(filter, "%s", escapedUsername)
	filter = strings.ReplaceAll(filter, "{username}", escapedUsername)
	filter = strings.ReplaceAll(filter, "{{username}}", escapedUsername)

	// 确定要检索的属性
	usernameAttr := a.getAttr(a.config.UsernameAttribute, defaultUsernameAttr)
	emailAttr := a.getAttr(a.config.EmailAttribute, defaultEmailAttr)
	fullnameAttr := a.getAttr(a.config.FullnameAttribute, defaultFullnameAttr)

	searchRequest := ldap.NewSearchRequest(
		a.config.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1, // 只需要一个结果
		0, // 无时间限制
		false,
		filter,
		[]string{"dn", "objectGUID", usernameAttr, emailAttr, fullnameAttr},
		nil,
	)

	result, err := conn.Search(searchRequest)
	if err != nil {
		return nil, fmt.Errorf("LDAP search failed: %w", err)
	}

	if len(result.Entries) == 0 {
		return nil, fmt.Errorf("user not found: %s", username)
	}

	return result.Entries[0], nil
}

// Authenticate 执行 LDAP 认证
func (a *Authenticator) Authenticate(username, password string) (*idp.AuthResult, error) {
	// 首先搜索用户获取 DN
	entry, err := a.SearchUser(username)
	if err != nil {
		return nil, idp.ErrAuthFailed
	}

	// 使用用户 DN 和密码尝试绑定
	conn, err := a.connect()
	if err != nil {
		return nil, idp.ErrConnectionFailed
	}
	defer conn.Close()

	if err := conn.Bind(entry.DN, password); err != nil {
		return nil, idp.ErrAuthFailed
	}

	// 提取用户信息
	usernameAttr := a.getAttr(a.config.UsernameAttribute, defaultUsernameAttr)
	emailAttr := a.getAttr(a.config.EmailAttribute, defaultEmailAttr)
	fullnameAttr := a.getAttr(a.config.FullnameAttribute, defaultFullnameAttr)

	extractedUsername := entry.GetAttributeValue(usernameAttr)
	if extractedUsername == "" {
		extractedUsername = username
	}

	// 获取外部 ID（优先使用 objectGUID）
	externalID := entry.GetAttributeValue("objectGUID")
	if externalID == "" {
		externalID = entry.DN
	}

	// 构建原始属性映射
	rawClaims := make(map[string]any)
	for _, attr := range entry.Attributes {
		if len(attr.Values) == 1 {
			rawClaims[attr.Name] = attr.Values[0]
		} else if len(attr.Values) > 1 {
			rawClaims[attr.Name] = attr.Values
		}
	}
	rawClaims["dn"] = entry.DN

	return &idp.AuthResult{
		ExternalID: externalID,
		Username:   extractedUsername,
		Email:      entry.GetAttributeValue(emailAttr),
		Fullname:   entry.GetAttributeValue(fullnameAttr),
		RawClaims:  rawClaims,
	}, nil
}

// FetchAllUsers 获取所有用户（用于同步）
func (a *Authenticator) FetchAllUsers() ([]*idp.AuthResult, error) {
	conn, err := a.connect()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// 使用服务账号绑定
	if err := a.bindServiceAccount(conn); err != nil {
		return nil, err
	}

	// 构建搜索过滤器
	filter := a.config.UserFilter
	if filter == "" {
		filter = "(&(objectClass=user)(objectCategory=person))"
	} else {
		// 移除用户名占位符，获取所有用户
		filter = strings.ReplaceAll(filter, "(%s)", "")
		filter = strings.ReplaceAll(filter, "%s", "*")
	}

	// 确定要检索的属性
	usernameAttr := a.getAttr(a.config.UsernameAttribute, defaultUsernameAttr)
	emailAttr := a.getAttr(a.config.EmailAttribute, defaultEmailAttr)
	fullnameAttr := a.getAttr(a.config.FullnameAttribute, defaultFullnameAttr)

	searchRequest := ldap.NewSearchRequest(
		a.config.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		filter,
		[]string{"dn", "objectGUID", usernameAttr, emailAttr, fullnameAttr},
		nil,
	)

	// 使用分页控制
	pagingControl := ldap.NewControlPaging(pageSize)
	searchRequest.Controls = append(searchRequest.Controls, pagingControl)

	var users []*idp.AuthResult

	for {
		result, err := conn.Search(searchRequest)
		if err != nil {
			return nil, fmt.Errorf("LDAP search failed: %w", err)
		}

		for _, entry := range result.Entries {
			username := entry.GetAttributeValue(usernameAttr)
			if username == "" {
				continue
			}

			externalID := entry.GetAttributeValue("objectGUID")
			if externalID == "" {
				externalID = entry.DN
			}

			users = append(users, &idp.AuthResult{
				ExternalID: externalID,
				Username:   username,
				Email:      entry.GetAttributeValue(emailAttr),
				Fullname:   entry.GetAttributeValue(fullnameAttr),
			})
		}

		// 检查是否还有更多页
		pagingResult := ldap.FindControl(result.Controls, ldap.ControlTypePaging)
		if pagingResult == nil {
			break
		}

		cookie := pagingResult.(*ldap.ControlPaging).Cookie
		if len(cookie) == 0 {
			break
		}

		pagingControl.SetCookie(cookie)
	}

	return users, nil
}

// TestConnection 测试 LDAP 连接
func (a *Authenticator) TestConnection() (*idp.TestResult, error) {
	conn, err := a.connect()
	if err != nil {
		return &idp.TestResult{
			Success: false,
			Message: fmt.Sprintf("failed to connect: %v", err),
		}, nil
	}
	defer conn.Close()

	// 尝试绑定
	if err := a.bindServiceAccount(conn); err != nil {
		return &idp.TestResult{
			Success: false,
			Message: fmt.Sprintf("bind failed: %v", err),
		}, nil
	}

	// 尝试搜索 base DN
	searchRequest := ldap.NewSearchRequest(
		a.config.BaseDN,
		ldap.ScopeBaseObject,
		ldap.NeverDerefAliases,
		1,
		0,
		false,
		"(objectClass=*)",
		[]string{"dn"},
		nil,
	)

	if _, err := conn.Search(searchRequest); err != nil {
		return &idp.TestResult{
			Success: false,
			Message: fmt.Sprintf("base DN not accessible: %v", err),
		}, nil
	}

	// 统计用户数量
	userFilter := a.config.UserFilter
	if userFilter == "" {
		userFilter = "(&(objectClass=user)(objectCategory=person))"
	} else {
		userFilter = strings.ReplaceAll(userFilter, "(%s)", "")
		userFilter = strings.ReplaceAll(userFilter, "%s", "*")
	}

	countRequest := ldap.NewSearchRequest(
		a.config.BaseDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0,
		0,
		false,
		userFilter,
		[]string{"dn"},
		nil,
	)

	countResult, err := conn.Search(countRequest)
	userCount := 0
	if err == nil {
		userCount = len(countResult.Entries)
	}

	return &idp.TestResult{
		Success: true,
		Message: "LDAP connection successful",
		Details: map[string]any{
			"server_url": a.config.ServerURL,
			"base_dn":    a.config.BaseDN,
			"user_count": userCount,
		},
	}, nil
}

// GetConfig 返回 LDAP 配置
func (a *Authenticator) GetConfig() *idp.LDAPConfig {
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

// getAttr 返回属性名，如果为空则返回默认值
func (a *Authenticator) getAttr(attr, defaultAttr string) string {
	if attr != "" {
		return attr
	}
	return defaultAttr
}
