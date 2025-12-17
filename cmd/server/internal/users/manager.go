package users

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Scope definitions
const (
	// 项目文档权限（新增）- 涵盖所有项目级文档
	ScopeProjectDocRead  = "project.doc.read"  // 项目文档读取（特性列表、架构设计、技术设计、文档管理、项目状态等）
	ScopeProjectDocWrite = "project.doc.write" // 项目文档编辑（特性列表、架构设计、技术设计、文档管理、项目状态等）
	ScopeProjectAdmin    = "project.admin"     // 项目全局管理（创建/全局配置）
	ScopeProjectDelete   = "project.delete"    // 项目删除（允许删除项目和任务）

	// 任务权限
	ScopeTaskRead        = "task.read"
	ScopeTaskWrite       = "task.write"
	ScopeTaskPlanApprove = "task.plan.approve"

	// 会议权限
	ScopeMeetingRead  = "meeting.read"
	ScopeMeetingWrite = "meeting.write"

	// 用户管理权限
	ScopeUserManage = "user.manage"

	// 身份源管理权限
	ScopeIdpRead  = "idp.read"  // 查看身份源配置
	ScopeIdpWrite = "idp.write" // 管理身份源配置

	// 以下权限已弃用，保留用于向后兼容，将在下一版本移除
	// Deprecated: 使用 ScopeProjectDocRead 代替
	ScopeFeatureRead = "feature.read"
	// Deprecated: 使用 ScopeProjectDocWrite 代替
	ScopeFeatureWrite = "feature.write"
	// Deprecated: 使用 ScopeProjectDocRead 代替
	ScopeArchRead = "architecture.read"
	// Deprecated: 使用 ScopeProjectDocWrite 代替
	ScopeArchWrite = "architecture.write"
	// Deprecated: 使用 ScopeProjectDocRead 代替
	ScopeTechRead = "tech.read"
	// Deprecated: 使用 ScopeProjectDocWrite 代替
	ScopeTechWrite = "tech.write"
)

var allScopes = []string{
	ScopeProjectDocRead, ScopeProjectDocWrite, ScopeProjectAdmin, ScopeProjectDelete,
	ScopeTaskRead, ScopeTaskWrite, ScopeTaskPlanApprove,
	ScopeMeetingRead, ScopeMeetingWrite,
	ScopeUserManage,
	ScopeIdpRead, ScopeIdpWrite,
	// 保留旧权限用于向后兼容
	ScopeFeatureRead, ScopeFeatureWrite,
	ScopeArchRead, ScopeArchWrite,
	ScopeTechRead, ScopeTechWrite,
}

// User source constants
const (
	SourceLocal    = "local"
	SourceExternal = "external"
)

// User 数据模型
// Password 存储哈希 (sha256(hex))
type User struct {
	Username    string     `json:"username"`
	Password    string     `json:"password_hash"`
	Scopes      []string   `json:"scopes"`
	Source      string     `json:"source,omitempty"`      // local | external
	IdpID       string     `json:"idp_id,omitempty"`      // 关联的身份源ID
	ExternalID  string     `json:"external_id,omitempty"` // 外部系统用户ID
	Email       string     `json:"email,omitempty"`
	Fullname    string     `json:"fullname,omitempty"`
	Disabled    bool       `json:"disabled,omitempty"` // 是否禁用
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
	SyncedAt    *time.Time `json:"synced_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// Claims 自定义 JWT claims (无过期，交给外部策略控制)
type Claims struct {
	Username string   `json:"username"`
	Scopes   []string `json:"scopes"`
	jwt.RegisteredClaims
}

// Manager 管理用户及 JWT
// 简易文件存储 users/users.json

type Manager struct {
	mu        sync.RWMutex
	users     map[string]*User
	secretKey []byte
	storePath string
}

// NewManager 创建管理器，secret 用于 JWT 签名
func NewManager(storeDir string, secret []byte) (*Manager, error) {
	if len(secret) == 0 {
		return nil, errors.New("secret key required")
	}
	m := &Manager{users: map[string]*User{}, secretKey: secret, storePath: filepath.Join(storeDir, "users.json")}
	if err := m.load(); err != nil {
		return nil, err
	}
	return m, nil
}

// hashPassword 简单 sha256；生产系统应使用 bcrypt/argon2
func hashPassword(pw string) string {
	s := sha256.Sum256([]byte(pw))
	return hex.EncodeToString(s[:])
}

// load 从文件读取
func (m *Manager) load() error {
	b, err := os.ReadFile(m.storePath)
	if err != nil {
		return nil // first run
	}
	var arr []*User
	if err := json.Unmarshal(b, &arr); err != nil {
		return err
	}
	for _, u := range arr {
		m.users[u.Username] = u
	}
	return nil
}

// save 写入文件（全量）
func (m *Manager) save() error {
	arr := []*User{}
	for _, u := range m.users {
		arr = append(arr, u)
	}
	b, _ := json.MarshalIndent(arr, "", "  ")
	if err := os.MkdirAll(filepath.Dir(m.storePath), 0755); err != nil {
		return err
	}
	return os.WriteFile(m.storePath, b, 0644)
}

// EnsureDefaultAdmin 如果没有用户则创建 admin 默认用户
func (m *Manager) EnsureDefaultAdmin(defaultPassword string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.users) > 0 {
		return nil
	}
	now := time.Now()
	m.users["admin"] = &User{Username: "admin", Password: hashPassword(defaultPassword), Scopes: allScopes, CreatedAt: now, UpdatedAt: now}
	return m.save()
}

// CreateUser 创建用户（用户名唯一）
func (m *Manager) CreateUser(username, password string, scopes []string) (*User, error) {
	if username == "" {
		return nil, errors.New("username required")
	}
	if password == "" {
		password = "neteye@123"
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.users[username]; exists {
		return nil, errors.New("user exists")
	}
	now := time.Now()
	u := &User{Username: username, Password: hashPassword(password), Scopes: dedupScopes(scopes), CreatedAt: now, UpdatedAt: now}
	m.users[username] = u
	return u, m.save()
}

// GetUser 获取单个
func (m *Manager) GetUser(username string) (*User, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[username]
	if !ok {
		return nil, false
	}
	// 复制避免外部修改
	copyU := *u
	copyU.Password = ""
	return &copyU, true
}

// ListUsers 返回所有用户（隐藏密码）
func (m *Manager) ListUsers() []*User {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := []*User{}
	for _, u := range m.users {
		cpy := *u
		cpy.Password = ""
		out = append(out, &cpy)
	}
	return out
}

// UpdateUser 更新 scopes
func (m *Manager) UpdateUser(username string, scopes []string) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[username]
	if !ok {
		return nil, errors.New("not found")
	}
	u.Scopes = dedupScopes(scopes)
	u.UpdatedAt = time.Now()
	if err := m.save(); err != nil {
		return nil, err
	}
	cpy := *u
	cpy.Password = ""
	return &cpy, nil
}

// DeleteUser 删除
func (m *Manager) DeleteUser(username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[username]; !ok {
		return errors.New("not found")
	}
	delete(m.users, username)
	return m.save()
}

// ChangePassword 修改密码
func (m *Manager) ChangePassword(username, oldPassword, newPassword string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[username]
	if !ok {
		return errors.New("not found")
	}
	if u.Password != hashPassword(oldPassword) {
		return errors.New("old password incorrect")
	}
	u.Password = hashPassword(newPassword)
	u.UpdatedAt = time.Now()
	return m.save()
}

// Authenticate 验证用户名密码
func (m *Manager) Authenticate(username, password string) (*User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[username]
	if !ok {
		return nil, errors.New("invalid credentials")
	}
	// 检查用户是否被禁用
	if u.Disabled {
		return nil, errors.New("user is disabled")
	}
	if u.Password != hashPassword(password) {
		return nil, errors.New("invalid credentials")
	}
	cpy := *u
	cpy.Password = ""
	return &cpy, nil
}

// GenerateToken 永久有效（不设置 exp），并记录 LastLoginAt
func (m *Manager) GenerateToken(username string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[username]
	if !ok {
		return "", errors.New("not found")
	}
	// 记录最后登录时间
	now := time.Now()
	u.LastLoginAt = &now
	u.UpdatedAt = now
	if err := m.save(); err != nil {
		// 保存失败不影响 token 生成，仅记录日志
		// TODO: 添加日志记录
	}
	claims := Claims{Username: u.Username, Scopes: u.Scopes, RegisteredClaims: jwt.RegisteredClaims{IssuedAt: jwt.NewNumericDate(time.Now())}}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(m.secretKey)
}

// ParseToken 验证并返回 claims
func (m *Manager) ParseToken(tokenStr string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) { return m.secretKey, nil })
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok {
		return nil, errors.New("invalid claims")
	}
	return claims, nil
}

// HasScope 判断用户是否具有 scope
func HasScope(scopes []string, required string) bool {
	for _, s := range scopes {
		if s == required {
			return true
		}
	}
	return false
}

// dedupScopes 去重并过滤非法 scope
func dedupScopes(in []string) []string {
	m := map[string]struct{}{}
	valid := map[string]struct{}{}
	for _, s := range allScopes {
		valid[s] = struct{}{}
	}
	out := []string{}
	for _, s := range in {
		if _, ok := valid[s]; ok {
			if _, seen := m[s]; !seen {
				m[s] = struct{}{}
				out = append(out, s)
			}
		}
	}
	return out
}

// Debug helper
func (m *Manager) DebugString() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return fmt.Sprintf("users:%d", len(m.users))
}

// ==================== 外部用户支持方法 ====================

// FindOrCreateExternalUser 查找或创建外部用户
// 如果用户不存在且 autoCreate 为 true，则创建新用户
func (m *Manager) FindOrCreateExternalUser(extID, username, email, fullname, idpID string, defaultScopes []string, autoCreate bool) (*User, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 先按 idpID 和 externalID 查找
	for _, u := range m.users {
		if u.IdpID == idpID && u.ExternalID == extID {
			// 更新用户信息
			changed := false
			if email != "" && u.Email != email {
				u.Email = email
				changed = true
			}
			if fullname != "" && u.Fullname != fullname {
				u.Fullname = fullname
				changed = true
			}
			now := time.Now()
			u.SyncedAt = &now
			u.UpdatedAt = now
			if changed {
				if err := m.save(); err != nil {
					return nil, false, err
				}
			}
			cpy := *u
			cpy.Password = ""
			return &cpy, false, nil // 返回现有用户
		}
	}

	// 检查用户名是否已存在（可能是本地用户）
	if existing, exists := m.users[username]; exists {
		// LDAP 优先策略：如果是本地用户（Source为空或"local"），将其转换为 LDAP 用户
		if existing.Source == SourceLocal || existing.Source == "" {
			now := time.Now()
			existing.Password = "" // 清除本地密码
			existing.Source = SourceExternal
			existing.IdpID = idpID
			existing.ExternalID = extID
			if email != "" {
				existing.Email = email
			}
			if fullname != "" {
				existing.Fullname = fullname
			}
			existing.SyncedAt = &now
			existing.UpdatedAt = now
			// 保留现有权限，不覆盖
			if err := m.save(); err != nil {
				return nil, false, fmt.Errorf("failed to convert local user to LDAP user: %w", err)
			}
			cpy := *existing
			cpy.Password = ""
			return &cpy, false, nil // 返回转换后的用户
		}
		// 如果是同一个 IdP 的外部用户，允许登录并更新信息
		if existing.Source == SourceExternal && existing.IdpID == idpID {
			now := time.Now()
			existing.ExternalID = extID // 更新 External ID（可能 LDAP DN 变化了）
			if email != "" {
				existing.Email = email
			}
			if fullname != "" {
				existing.Fullname = fullname
			}
			existing.SyncedAt = &now
			existing.UpdatedAt = now
			if err := m.save(); err != nil {
				return nil, false, err
			}
			cpy := *existing
			cpy.Password = ""
			return &cpy, false, nil
		}
		// 如果是其他 IdP 的外部用户，返回错误
		return nil, false, fmt.Errorf("username %s already exists with different identity provider", username)
	}

	// 不存在，检查是否自动创建
	if !autoCreate {
		return nil, false, errors.New("user not found and auto-create is disabled")
	}

	// 创建新用户
	now := time.Now()
	u := &User{
		Username:   username,
		Password:   "", // 外部用户无本地密码
		Scopes:     dedupScopes(defaultScopes),
		Source:     SourceExternal,
		IdpID:      idpID,
		ExternalID: extID,
		Email:      email,
		Fullname:   fullname,
		SyncedAt:   &now,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	m.users[username] = u
	if err := m.save(); err != nil {
		delete(m.users, username)
		return nil, false, err
	}

	cpy := *u
	return &cpy, true, nil // 返回新创建的用户
}

// UpdateExternalUserInfo 更新外部用户信息
func (m *Manager) UpdateExternalUserInfo(username, email, fullname string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	u, ok := m.users[username]
	if !ok {
		return errors.New("not found")
	}

	if u.Source != SourceExternal {
		return errors.New("can only update external users")
	}

	changed := false
	if email != "" && u.Email != email {
		u.Email = email
		changed = true
	}
	if fullname != "" && u.Fullname != fullname {
		u.Fullname = fullname
		changed = true
	}

	if changed {
		now := time.Now()
		u.SyncedAt = &now
		u.UpdatedAt = now
		return m.save()
	}

	return nil
}

// DisableUser 禁用用户
func (m *Manager) DisableUser(username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	u, ok := m.users[username]
	if !ok {
		return errors.New("not found")
	}

	if u.Disabled {
		return nil // 已禁用
	}

	u.Disabled = true
	u.UpdatedAt = time.Now()
	return m.save()
}

// EnableUser 启用用户
func (m *Manager) EnableUser(username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	u, ok := m.users[username]
	if !ok {
		return errors.New("not found")
	}

	if !u.Disabled {
		return nil // 已启用
	}

	u.Disabled = false
	u.UpdatedAt = time.Now()
	return m.save()
}

// FindByExternalID 按外部 ID 查找用户
func (m *Manager) FindByExternalID(idpID, externalID string) (*User, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, u := range m.users {
		if u.IdpID == idpID && u.ExternalID == externalID {
			cpy := *u
			cpy.Password = ""
			return &cpy, true
		}
	}

	return nil, false
}

// ListExternalUsers 列出指定身份源的所有外部用户
func (m *Manager) ListExternalUsers(idpID string) []*User {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var users []*User
	for _, u := range m.users {
		if u.Source == SourceExternal && u.IdpID == idpID {
			cpy := *u
			cpy.Password = ""
			users = append(users, &cpy)
		}
	}

	return users
}

// IsExternalUser 检查用户是否为外部用户
func (m *Manager) IsExternalUser(username string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, ok := m.users[username]
	if !ok {
		return false
	}
	return u.Source == SourceExternal
}
