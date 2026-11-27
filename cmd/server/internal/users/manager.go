package users

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const (
	maxUserStoreBytes = 8 << 20
	maxUsers          = 10000
	bcryptPrefix      = "$aidg-bcrypt-sha256$v1$"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrUserExists         = errors.New("user already exists")
	ErrUsernameRequired   = errors.New("username required")
	ErrPasswordRequired   = errors.New("password required")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserDisabled       = errors.New("user disabled")
	ErrExternalUser       = errors.New("external user has no local password")
	ErrCredentialUpgrade  = errors.New("credential hash upgrade failed")
	ErrInvalidUserStore   = errors.New("invalid user store")
)

var legacyPasswordPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type persistError struct {
	err       error
	committed bool
}

func (e *persistError) Error() string { return e.err.Error() }
func (e *persistError) Unwrap() error { return e.err }

// Scope definitions
const (
	// 项目文档权限（新增）- 涵盖所有项目级文档
	ScopeProjectDocRead  = "project.doc.read"  // 项目文档读取（特性列表、架构设计、技术设计、文档管理、项目状态等）
	ScopeProjectDocWrite = "project.doc.write" // 项目文档编辑（特性列表、架构设计、技术设计、文档管理、项目状态等）
	ScopeProjectAdmin    = "project.admin"     // 项目全局管理（创建/全局配置）
	ScopeProjectDelete   = "project.delete"    // 项目删除（允许删除项目和任务）

	// 任务权限
	ScopeTaskRead            = "task.read"
	ScopeTaskWrite           = "task.write"
	ScopeTaskPlanApprove     = "task.plan.approve"
	ScopeTaskLifecycleImport = "task.lifecycle.import"

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
	ScopeTaskRead, ScopeTaskWrite, ScopeTaskPlanApprove, ScopeTaskLifecycleImport,
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

func legacyPasswordHash(password string) string {
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
}

func hashPassword(password string) (string, error) {
	digest := legacyPasswordHash(password)
	hash, err := bcrypt.GenerateFromPassword([]byte(digest), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return bcryptPrefix + string(hash), nil
}

func verifyPassword(stored, password string) (bool, bool) {
	if legacyPasswordPattern.MatchString(stored) {
		return stored == legacyPasswordHash(password), true
	}
	if !strings.HasPrefix(stored, bcryptPrefix) {
		return false, false
	}
	err := bcrypt.CompareHashAndPassword([]byte(strings.TrimPrefix(stored, bcryptPrefix)), []byte(legacyPasswordHash(password)))
	return err == nil, false
}

func (m *Manager) load() error {
	info, err := os.Lstat(m.storePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidUserStore, err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("%w: users.json must be a regular file", ErrInvalidUserStore)
	}
	file, err := os.Open(m.storePath)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidUserStore, err)
	}
	defer file.Close()
	reader := bufio.NewReader(io.LimitReader(file, maxUserStoreBytes+1))
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var users []*User
	if err := decoder.Decode(&users); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidUserStore, err)
	}
	if len(users) > maxUsers {
		return fmt.Errorf("%w: too many users", ErrInvalidUserStore)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return fmt.Errorf("%w: trailing data", ErrInvalidUserStore)
	}
	if buffered, _ := io.ReadAll(reader); len(buffered) > 0 || info.Size() > maxUserStoreBytes {
		return fmt.Errorf("%w: file exceeds size limit", ErrInvalidUserStore)
	}
	loaded := make(map[string]*User, len(users))
	for _, user := range users {
		if err := validateStoredUser(user); err != nil {
			return err
		}
		if _, exists := loaded[user.Username]; exists {
			return fmt.Errorf("%w: duplicate username", ErrInvalidUserStore)
		}
		loaded[user.Username] = cloneUser(user)
	}
	m.users = loaded
	return nil
}

func validateStoredUser(user *User) error {
	if user == nil || !validUsername(user.Username) {
		return fmt.Errorf("%w: invalid username", ErrInvalidUserStore)
	}
	if user.Source == "" {
		user.Source = SourceLocal
	}
	if user.Source == SourceLocal {
		if !legacyPasswordPattern.MatchString(user.Password) && !validBcryptHash(user.Password) {
			return fmt.Errorf("%w: invalid local password hash", ErrInvalidUserStore)
		}
	} else if user.Source != SourceExternal {
		return fmt.Errorf("%w: invalid user source", ErrInvalidUserStore)
	}
	for _, scope := range user.Scopes {
		if !validScope(scope) {
			return fmt.Errorf("%w: invalid scope", ErrInvalidUserStore)
		}
	}
	return nil
}

func validBcryptHash(hash string) bool {
	if !strings.HasPrefix(hash, bcryptPrefix) {
		return false
	}
	_, err := bcrypt.Cost([]byte(strings.TrimPrefix(hash, bcryptPrefix)))
	return err == nil
}

func validUsername(username string) bool {
	if username == "" || len(username) > 256 {
		return false
	}
	for _, r := range username {
		if r == 0 || unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func validScope(scope string) bool {
	for _, allowed := range allScopes {
		if scope == allowed {
			return true
		}
	}
	return false
}

func cloneUser(user *User) *User {
	if user == nil {
		return nil
	}
	copy := *user
	copy.Scopes = append([]string(nil), user.Scopes...)
	return &copy
}

func cloneUsers(users map[string]*User) map[string]*User {
	copy := make(map[string]*User, len(users))
	for username, user := range users {
		copy[username] = cloneUser(user)
	}
	return copy
}

func (m *Manager) persist(users map[string]*User) error {
	usernames := make([]string, 0, len(users))
	for username := range users {
		usernames = append(usernames, username)
	}
	sort.Strings(usernames)
	items := make([]*User, 0, len(users))
	for _, username := range usernames {
		items = append(items, users[username])
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(m.storePath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	if existing, err := os.ReadFile(m.storePath); err == nil {
		if err := writeAtomicFile(m.storePath+".bak", existing, 0o600); err != nil {
			return fmt.Errorf("write user backup: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return writeAtomicFile(m.storePath, data, 0o600)
}

func writeAtomicFile(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if info, err := os.Lstat(path); err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		return fmt.Errorf("refusing to replace non-regular file")
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := os.CreateTemp(dir, ".users-tmp-*")
	if err != nil {
		return err
	}
	tempPath := file.Name()
	defer os.Remove(tempPath)
	if err := file.Chmod(mode); err != nil {
		file.Close()
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	directory, err := os.Open(dir)
	if err != nil {
		return &persistError{err: err, committed: true}
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return &persistError{err: err, committed: true}
	}
	return nil
}

func (m *Manager) commit(users map[string]*User) error {
	if err := m.persist(users); err != nil {
		var persistence *persistError
		if errors.As(err, &persistence) && persistence.committed {
			m.users = users
		}
		return err
	}
	m.users = users
	return nil
}
func (m *Manager) save() error {
	return m.persist(m.users)
}

func (m *Manager) HasUsers() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.users) > 0
}

// EnsureDefaultAdmin creates the initial administrator only with an explicit password.
func (m *Manager) EnsureDefaultAdmin(defaultPassword string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.users) > 0 {
		return nil
	}
	if defaultPassword == "" {
		return ErrPasswordRequired
	}
	hash, err := hashPassword(defaultPassword)
	if err != nil {
		return err
	}
	now := time.Now()
	next := cloneUsers(m.users)
	next["admin"] = &User{Username: "admin", Password: hash, Scopes: append([]string(nil), allScopes...), CreatedAt: now, UpdatedAt: now, Source: SourceLocal}
	return m.commit(next)
}

func (m *Manager) CreateUser(username, password string, scopes []string) (*User, error) {
	if !validUsername(username) {
		return nil, ErrUsernameRequired
	}
	if password == "" {
		return nil, ErrPasswordRequired
	}
	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.users[username]; exists {
		return nil, ErrUserExists
	}
	now := time.Now()
	u := &User{Username: username, Password: hash, Scopes: dedupScopes(scopes), CreatedAt: now, UpdatedAt: now, Source: SourceLocal}
	next := cloneUsers(m.users)
	next[username] = u
	if err := m.commit(next); err != nil {
		return nil, err
	}
	result := cloneUser(u)
	result.Password = ""
	return result, nil
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
	if _, ok := m.users[username]; !ok {
		return nil, ErrUserNotFound
	}
	next := cloneUsers(m.users)
	next[username].Scopes = dedupScopes(scopes)
	next[username].UpdatedAt = time.Now()
	if err := m.commit(next); err != nil {
		return nil, err
	}
	result := cloneUser(next[username])
	result.Password = ""
	return result, nil
}

// DeleteUser 删除
func (m *Manager) DeleteUser(username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[username]; !ok {
		return ErrUserNotFound
	}
	next := cloneUsers(m.users)
	delete(next, username)
	return m.commit(next)
}

// ChangePassword 修改密码
func (m *Manager) ChangePassword(username, oldPassword, newPassword string) error {
	if newPassword == "" {
		return ErrPasswordRequired
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[username]
	if !ok {
		return ErrUserNotFound
	}
	if u.Source == SourceExternal {
		return ErrExternalUser
	}
	valid, _ := verifyPassword(u.Password, oldPassword)
	if !valid {
		return ErrInvalidCredentials
	}
	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	next := cloneUsers(m.users)
	next[username].Password = hash
	next[username].UpdatedAt = time.Now()
	return m.commit(next)
}

// Authenticate 验证用户名密码
func (m *Manager) Authenticate(username, password string) (*User, error) {
	m.mu.RLock()
	u, ok := m.users[username]
	if !ok || u.Source == SourceExternal {
		m.mu.RUnlock()
		return nil, ErrInvalidCredentials
	}
	stored := u.Password
	disabled := u.Disabled
	m.mu.RUnlock()
	if disabled {
		return nil, ErrUserDisabled
	}
	valid, legacy := verifyPassword(stored, password)
	if !valid {
		return nil, ErrInvalidCredentials
	}
	var upgradedHash string
	if legacy {
		var err error
		upgradedHash, err = hashPassword(password)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCredentialUpgrade, err)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	current, exists := m.users[username]
	if !exists || current.Disabled || current.Source == SourceExternal {
		return nil, ErrInvalidCredentials
	}
	validCurrent, currentLegacy := verifyPassword(current.Password, password)
	if !validCurrent {
		return nil, ErrInvalidCredentials
	}
	if legacy && currentLegacy {
		next := cloneUsers(m.users)
		next[username].Password = upgradedHash
		next[username].UpdatedAt = time.Now()
		if err := m.commit(next); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCredentialUpgrade, err)
		}
		current = next[username]
	}
	result := cloneUser(current)
	result.Password = ""
	return result, nil
}

// GenerateToken 永久有效（不设置 exp），并记录 LastLoginAt
func (m *Manager) GenerateToken(username string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[username]
	if !ok {
		return "", ErrUserNotFound
	}
	now := time.Now()
	next := cloneUsers(m.users)
	next[username].LastLoginAt = &now
	next[username].UpdatedAt = now
	if err := m.commit(next); err != nil {
		return "", err
	}
	u = next[username]
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
	if !validUsername(username) {
		return nil, false, ErrUsernameRequired
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	next := cloneUsers(m.users)
	now := time.Now()
	for _, user := range next {
		if user.IdpID == idpID && user.ExternalID == extID {
			if email != "" {
				user.Email = email
			}
			if fullname != "" {
				user.Fullname = fullname
			}
			user.SyncedAt, user.UpdatedAt = &now, now
			if err := m.commit(next); err != nil {
				return nil, false, err
			}
			result := cloneUser(user)
			result.Password = ""
			return result, false, nil
		}
	}
	if existing, exists := next[username]; exists {
		if existing.Source == SourceLocal || existing.Source == "" {
			existing.Password = ""
			existing.Source = SourceExternal
			existing.IdpID, existing.ExternalID = idpID, extID
		} else if existing.Source == SourceExternal && existing.IdpID == idpID {
			existing.ExternalID = extID
		} else {
			return nil, false, fmt.Errorf("username %s already exists with different identity provider", username)
		}
		if email != "" {
			existing.Email = email
		}
		if fullname != "" {
			existing.Fullname = fullname
		}
		existing.SyncedAt, existing.UpdatedAt = &now, now
		if err := m.commit(next); err != nil {
			return nil, false, err
		}
		result := cloneUser(existing)
		result.Password = ""
		return result, false, nil
	}
	if !autoCreate {
		return nil, false, ErrUserNotFound
	}
	user := &User{Username: username, Scopes: dedupScopes(defaultScopes), Source: SourceExternal, IdpID: idpID, ExternalID: extID,
		Email: email, Fullname: fullname, SyncedAt: &now, CreatedAt: now, UpdatedAt: now}
	next[username] = user
	if err := m.commit(next); err != nil {
		return nil, false, err
	}
	return cloneUser(user), true, nil
}

// UpdateExternalUserInfo 更新外部用户信息
func (m *Manager) UpdateExternalUserInfo(username, email, fullname string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[username]
	if !ok {
		return ErrUserNotFound
	}
	if u.Source != SourceExternal {
		return ErrExternalUser
	}
	next := cloneUsers(m.users)
	updated := next[username]
	changed := false
	if email != "" && updated.Email != email {
		updated.Email = email
		changed = true
	}
	if fullname != "" && updated.Fullname != fullname {
		updated.Fullname = fullname
		changed = true
	}
	if !changed {
		return nil
	}
	now := time.Now()
	updated.SyncedAt, updated.UpdatedAt = &now, now
	return m.commit(next)
}

// DisableUser 禁用用户
func (m *Manager) DisableUser(username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[username]
	if !ok {
		return ErrUserNotFound
	}
	if u.Disabled {
		return nil
	}
	next := cloneUsers(m.users)
	next[username].Disabled = true
	next[username].UpdatedAt = time.Now()
	return m.commit(next)
}

// EnableUser 启用用户
func (m *Manager) EnableUser(username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[username]
	if !ok {
		return ErrUserNotFound
	}
	if !u.Disabled {
		return nil
	}
	next := cloneUsers(m.users)
	next[username].Disabled = false
	next[username].UpdatedAt = time.Now()
	return m.commit(next)
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
