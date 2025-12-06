package idp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager 管理身份源配置的生命周期
type Manager struct {
	mu       sync.RWMutex
	idps     map[string]*IdentityProvider
	storeDir string
	crypto   *Crypto
}

// NewManager 创建身份源管理器
// storeDir: 配置文件存储目录（如 identity_providers/）
func NewManager(storeDir string) (*Manager, error) {
	m := &Manager{
		idps:     make(map[string]*IdentityProvider),
		storeDir: storeDir,
		crypto:   NewCrypto(),
	}

	// 确保存储目录存在
	if err := os.MkdirAll(storeDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create store directory: %w", err)
	}

	// 加载现有配置
	if err := m.load(); err != nil {
		return nil, fmt.Errorf("failed to load identity providers: %w", err)
	}

	return m, nil
}

// load 从存储目录加载所有身份源配置
func (m *Manager) load() error {
	entries, err := os.ReadDir(m.storeDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 目录不存在，首次运行
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(m.storeDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue // 跳过读取失败的文件
		}

		var idp IdentityProvider
		if err := json.Unmarshal(data, &idp); err != nil {
			continue // 跳过解析失败的文件
		}

		m.idps[idp.ID] = &idp
	}

	return nil
}

// save 保存单个身份源配置到文件
func (m *Manager) save(idp *IdentityProvider) error {
	// 加密敏感字段
	idpCopy := *idp
	if err := m.encryptSensitiveFields(&idpCopy); err != nil {
		return err
	}

	data, err := json.MarshalIndent(&idpCopy, "", "  ")
	if err != nil {
		return err
	}

	filePath := filepath.Join(m.storeDir, idp.ID+".json")
	return os.WriteFile(filePath, data, 0644)
}

// encryptSensitiveFields 加密配置中的敏感字段
func (m *Manager) encryptSensitiveFields(idp *IdentityProvider) error {
	if m.crypto == nil {
		return nil // 未配置加密，跳过
	}

	switch idp.Type {
	case TypeOIDC:
		config, err := idp.GetOIDCConfig()
		if err != nil {
			return err
		}
		encrypted, err := m.crypto.Encrypt(config.ClientSecret)
		if err != nil {
			return err
		}
		config.ClientSecret = encrypted
		return idp.SetConfig(config)

	case TypeLDAP:
		config, err := idp.GetLDAPConfig()
		if err != nil {
			return err
		}
		encrypted, err := m.crypto.Encrypt(config.BindPassword)
		if err != nil {
			return err
		}
		config.BindPassword = encrypted
		return idp.SetConfig(config)
	}

	return nil
}

// decryptSensitiveFields 解密配置中的敏感字段
func (m *Manager) decryptSensitiveFields(idp *IdentityProvider) error {
	if m.crypto == nil {
		return nil // 未配置加密，跳过
	}

	switch idp.Type {
	case TypeOIDC:
		config, err := idp.GetOIDCConfig()
		if err != nil {
			return err
		}
		decrypted, err := m.crypto.Decrypt(config.ClientSecret)
		if err != nil {
			return err
		}
		config.ClientSecret = decrypted
		return idp.SetConfig(config)

	case TypeLDAP:
		config, err := idp.GetLDAPConfig()
		if err != nil {
			return err
		}
		decrypted, err := m.crypto.Decrypt(config.BindPassword)
		if err != nil {
			return err
		}
		config.BindPassword = decrypted
		return idp.SetConfig(config)
	}

	return nil
}

// DecryptSecret 解密敏感字符串（公开方法，供 API 层调用）
func (m *Manager) DecryptSecret(ciphertext string) (string, error) {
	if m.crypto == nil {
		return ciphertext, nil // 未配置加密，原样返回
	}
	return m.crypto.Decrypt(ciphertext)
}

// generateID 生成唯一的身份源 ID
func (m *Manager) generateID() string {
	return fmt.Sprintf("idp_%d", time.Now().UnixNano())
}

// Create 创建新的身份源配置
func (m *Manager) Create(idp *IdentityProvider) (*IdentityProvider, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证必填字段
	if idp.Name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrConfigInvalid)
	}
	if idp.Type != TypeOIDC && idp.Type != TypeLDAP {
		return nil, fmt.Errorf("%w: invalid type, must be OIDC or LDAP", ErrConfigInvalid)
	}

	// 生成 ID
	idp.ID = m.generateID()

	// 设置默认值
	if idp.Status == "" {
		idp.Status = StatusEnabled
	}

	now := time.Now()
	idp.CreatedAt = now
	idp.UpdatedAt = now

	// 保存到文件
	if err := m.save(idp); err != nil {
		return nil, err
	}

	// 添加到内存
	m.idps[idp.ID] = idp

	return idp, nil
}

// Get 获取身份源配置
func (m *Manager) Get(id string) (*IdentityProvider, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	idp, ok := m.idps[id]
	if !ok {
		return nil, ErrNotFound
	}

	// 返回副本，避免外部修改
	copy := *idp
	copy.Config = append([]byte(nil), idp.Config...)

	// 解密敏感字段
	if err := m.decryptSensitiveFields(&copy); err != nil {
		return nil, err
	}

	return &copy, nil
}

// GetForAuth 获取用于认证的身份源配置（仅返回启用的）
func (m *Manager) GetForAuth(id string) (*IdentityProvider, error) {
	idp, err := m.Get(id)
	if err != nil {
		return nil, err
	}

	if idp.Status != StatusEnabled {
		return nil, ErrDisabled
	}

	return idp, nil
}

// List 列出所有身份源配置
func (m *Manager) List() []*IdentityProvider {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]*IdentityProvider, 0, len(m.idps))
	for _, idp := range m.idps {
		copy := *idp
		copy.Config = append([]byte(nil), idp.Config...)
		list = append(list, &copy)
	}

	// 按 Priority 排序
	sort.Slice(list, func(i, j int) bool {
		return list[i].Priority < list[j].Priority
	})

	return list
}

// ListEnabled 列出所有启用的身份源配置
func (m *Manager) ListEnabled() []*IdentityProvider {
	all := m.List()
	enabled := make([]*IdentityProvider, 0)
	for _, idp := range all {
		if idp.Status == StatusEnabled {
			enabled = append(enabled, idp)
		}
	}
	return enabled
}

// ListPublic 列出公开信息（用于登录页显示）
func (m *Manager) ListPublic() []*PublicInfo {
	enabled := m.ListEnabled()
	public := make([]*PublicInfo, 0, len(enabled))
	for _, idp := range enabled {
		public = append(public, idp.ToPublicInfo())
	}
	return public
}

// Update 更新身份源配置
func (m *Manager) Update(id string, updates *IdentityProvider) (*IdentityProvider, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.idps[id]
	if !ok {
		return nil, ErrNotFound
	}

	// 更新字段（保留 ID 和 CreatedAt）
	if updates.Name != "" {
		existing.Name = updates.Name
	}
	if updates.Type != "" && updates.Type != existing.Type {
		return nil, fmt.Errorf("%w: cannot change type", ErrConfigInvalid)
	}
	if updates.Status != "" {
		existing.Status = updates.Status
	}
	if updates.Priority != 0 {
		existing.Priority = updates.Priority
	}
	if len(updates.Config) > 0 {
		existing.Config = updates.Config
	}
	if updates.Sync != nil {
		existing.Sync = updates.Sync
	}

	existing.UpdatedAt = time.Now()

	// 保存到文件
	if err := m.save(existing); err != nil {
		return nil, err
	}

	return existing, nil
}

// Delete 删除身份源配置
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.idps[id]; !ok {
		return ErrNotFound
	}

	// 删除文件
	filePath := filepath.Join(m.storeDir, id+".json")
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return err
	}

	// 从内存删除
	delete(m.idps, id)

	return nil
}

// TestConnection 测试身份源连接（不保存配置）
func (m *Manager) TestConnection(idp *IdentityProvider) (*TestResult, error) {
	// 根据类型创建认证器并测试
	// 此方法将在 step-03 和 step-04 中实现具体逻辑
	return &TestResult{
		Success: false,
		Message: "test connection not implemented for type: " + idp.Type,
	}, nil
}

// GetCrypto 返回加密器实例（供认证器使用）
func (m *Manager) GetCrypto() *Crypto {
	return m.crypto
}

// Count 返回身份源数量
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.idps)
}
