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

	// 任务权限
	ScopeTaskRead        = "task.read"
	ScopeTaskWrite       = "task.write"
	ScopeTaskPlanApprove = "task.plan.approve"

	// 会议权限
	ScopeMeetingRead  = "meeting.read"
	ScopeMeetingWrite = "meeting.write"

	// 用户管理权限
	ScopeUserManage = "user.manage"

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
	ScopeProjectDocRead, ScopeProjectDocWrite, ScopeProjectAdmin,
	ScopeTaskRead, ScopeTaskWrite, ScopeTaskPlanApprove,
	ScopeMeetingRead, ScopeMeetingWrite,
	ScopeUserManage,
	// 保留旧权限用于向后兼容
	ScopeFeatureRead, ScopeFeatureWrite,
	ScopeArchRead, ScopeArchWrite,
	ScopeTechRead, ScopeTechWrite,
}

// User 数据模型
// Password 存储哈希 (sha256(hex))
type User struct {
	Username  string    `json:"username"`
	Password  string    `json:"password_hash"`
	Scopes    []string  `json:"scopes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
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
	if u.Password != hashPassword(password) {
		return nil, errors.New("invalid credentials")
	}
	cpy := *u
	cpy.Password = ""
	return &cpy, nil
}

// GenerateToken 永久有效（不设置 exp）
func (m *Manager) GenerateToken(username string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.users[username]
	if !ok {
		return "", errors.New("not found")
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
