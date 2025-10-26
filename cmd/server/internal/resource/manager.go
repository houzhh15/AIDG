package resource

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ResourceManager 资源管理器 (内存+JSON持久化)
type ResourceManager struct {
	users     map[string]*UserResourceCollection // key: username
	mu        sync.RWMutex
	dataDir   string        // 数据目录路径
	saveQueue chan string   // 异步保存队列
	stopSave  chan struct{} // 停止保存信号
}

// UserResourceCollection 用户的资源集合
type UserResourceCollection struct {
	Username  string      `json:"username"`
	Resources []*Resource `json:"resources"`
	UpdatedAt time.Time   `json:"updated_at"`
}

// Resource 资源对象
type Resource struct {
	ResourceID  string    `json:"resource_id"`
	ProjectID   string    `json:"project_id"`
	TaskID      string    `json:"task_id,omitempty"`
	URI         string    `json:"uri"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	MimeType    string    `json:"mime_type"`
	Visibility  string    `json:"visibility"` // "private" or "public"
	AutoAdded   bool      `json:"auto_added"`
	Content     string    `json:"content,omitempty"` // 自定义资源内容
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NewResourceManager 创建资源管理器
// 参数:
//   - dataDir: 数据目录路径
//
// 返回:
//   - *ResourceManager: 初始化后的资源管理器实例
//
// 功能:
//   - 初始化内存数据结构和通道
//   - 启动异步保存和定期保存协程
func NewResourceManager(dataDir string) *ResourceManager {
	rm := &ResourceManager{
		users:     make(map[string]*UserResourceCollection),
		dataDir:   dataDir,
		saveQueue: make(chan string, 100),
		stopSave:  make(chan struct{}),
	}

	// 启动异步保存协程
	go rm.asyncSaveWorker()

	// 启动定期保存协程 (每5分钟)
	go rm.periodicSaveWorker(5 * time.Minute)

	return rm
}

// asyncSaveWorker 异步保存协程
// 功能:
//   - 从保存队列中接收用户名
//   - 调用 saveUserResources 保存到文件
//   - 失败时记录日志但不阻塞
func (rm *ResourceManager) asyncSaveWorker() {
	for {
		select {
		case username := <-rm.saveQueue:
			if err := rm.saveUserResources(username); err != nil {
				// 记录日志,但不阻塞
				fmt.Printf("ERROR: failed to save resources for %s: %v\n", username, err)
			}
		case <-rm.stopSave:
			return
		}
	}
}

// periodicSaveWorker 定期保存协程
// 参数:
//   - interval: 保存间隔
//
// 功能:
//   - 定期调用 saveAll 保存所有用户资源
func (rm *ResourceManager) periodicSaveWorker(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rm.saveAll()
		case <-rm.stopSave:
			return
		}
	}
}

// saveUserResources 保存单个用户的资源到文件
// 参数:
//   - username: 用户名
//
// 返回:
//   - error: 保存失败时返回错误
//
// 功能:
//   - 读取用户资源集合
//   - 序列化为 JSON
//   - 写入文件 data/users/{username}/resources.json
func (rm *ResourceManager) saveUserResources(username string) error {
	rm.mu.RLock()
	collection, ok := rm.users[username]
	rm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("user not found")
	}

	resourceFile := filepath.Join(rm.dataDir, "users", username, "resources.json")
	dir := filepath.Dir(resourceFile)

	// 创建目录
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// 序列化
	data, err := json.MarshalIndent(collection, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal resources: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(resourceFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write resources file: %w", err)
	}

	return nil
}

// saveAll 保存所有用户的资源
// 功能:
//   - 遍历所有用户
//   - 调用 saveUserResources 逐个保存
func (rm *ResourceManager) saveAll() {
	rm.mu.RLock()
	usernames := make([]string, 0, len(rm.users))
	for username := range rm.users {
		usernames = append(usernames, username)
	}
	rm.mu.RUnlock()

	for _, username := range usernames {
		if err := rm.saveUserResources(username); err != nil {
			fmt.Printf("ERROR: failed to save resources for %s: %v\n", username, err)
		}
	}
}

// Shutdown 优雅关闭
// 功能:
//   - 关闭停止信号通道
//   - 调用 saveAll 最后一次保存
func (rm *ResourceManager) Shutdown() {
	close(rm.stopSave)
	rm.saveAll() // 最后一次保存
}

// LoadAll 从磁盘加载所有用户的资源
// 返回:
//   - error: 读取或解析失败时返回错误
//
// 功能:
//   - 遍历 data/users 目录
//   - 读取每个用户的 resources.json 文件
//   - 解析填充 users map
func (rm *ResourceManager) LoadAll() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// 遍历 data/users 目录
	usersDir := filepath.Join(rm.dataDir, "users")
	entries, err := os.ReadDir(usersDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 目录不存在,跳过
		}
		return fmt.Errorf("failed to read users directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		username := entry.Name()
		resourceFile := filepath.Join(usersDir, username, "resources.json")

		data, err := os.ReadFile(resourceFile)
		if err != nil {
			if os.IsNotExist(err) {
				continue // 文件不存在,跳过
			}
			return fmt.Errorf("failed to read resources file for %s: %w", username, err)
		}

		var collection UserResourceCollection
		if err := json.Unmarshal(data, &collection); err != nil {
			return fmt.Errorf("failed to parse resources file for %s: %w", username, err)
		}

		rm.users[username] = &collection
	}

	return nil
}

// GetUserResources 获取用户的资源列表
// 参数:
//   - username: 用户名
//   - projectID: 项目ID (可选,传空字符串表示不过滤)
//
// 返回:
//   - []*Resource: 资源切片
//   - error: 查询失败时返回错误
//
// 功能:
//   - 根据 username 查询用户资源集合
//   - 如果指定 projectID,过滤匹配的资源
//   - 使用读锁保护并发安全
func (rm *ResourceManager) GetUserResources(username string, projectID string) ([]*Resource, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	collection, ok := rm.users[username]
	if !ok {
		return []*Resource{}, nil
	}

	// 过滤 (如果指定了 projectID)
	result := []*Resource{}
	for _, res := range collection.Resources {
		if projectID == "" || res.ProjectID == projectID {
			result = append(result, res)
		}
	}

	return result, nil
}

// AddResource 添加资源
// 参数:
//   - username: 用户名
//   - res: 资源对象
//
// 返回:
//   - error: 添加失败时返回错误
//
// 功能:
//   - 将资源添加到用户集合
//   - 更新 UpdatedAt 时间戳
//   - 触发异步保存
func (rm *ResourceManager) AddResource(username string, res *Resource) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	collection, ok := rm.users[username]
	if !ok {
		collection = &UserResourceCollection{
			Username:  username,
			Resources: []*Resource{},
			UpdatedAt: time.Now(),
		}
		rm.users[username] = collection
	}

	collection.Resources = append(collection.Resources, res)
	collection.UpdatedAt = time.Now()

	// 触发异步保存
	select {
	case rm.saveQueue <- username:
	default:
		// 队列满,忽略 (定期保存会兜底)
	}

	return nil
}

// DeleteResource 删除资源
// 参数:
//   - username: 用户名
//   - resourceID: 资源ID
//
// 返回:
//   - error: 用户不存在或资源未找到时返回错误
//
// 功能:
//   - 从用户集合中删除指定资源
//   - 更新 UpdatedAt 时间戳
//   - 触发异步保存
func (rm *ResourceManager) DeleteResource(username, resourceID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	collection, ok := rm.users[username]
	if !ok {
		return fmt.Errorf("user not found")
	}

	newResources := []*Resource{}
	found := false
	for _, res := range collection.Resources {
		if res.ResourceID == resourceID {
			found = true
			continue
		}
		newResources = append(newResources, res)
	}

	if !found {
		return fmt.Errorf("resource not found")
	}

	collection.Resources = newResources
	collection.UpdatedAt = time.Now()

	// 触发异步保存
	select {
	case rm.saveQueue <- username:
	default:
	}

	return nil
}

// UpdateResource 更新资源
// 参数:
//   - username: 用户名
//   - resourceID: 资源ID
//   - updates: 要更新的资源字段
//
// 返回:
//   - error: 用户不存在或资源未找到时返回错误
//
// 功能:
//   - 查找并更新指定资源的字段
//   - 更新 UpdatedAt 时间戳
//   - 触发异步保存
func (rm *ResourceManager) UpdateResource(username, resourceID string, updates *Resource) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	collection, ok := rm.users[username]
	if !ok {
		return fmt.Errorf("user not found")
	}

	found := false
	for _, res := range collection.Resources {
		if res.ResourceID == resourceID {
			// 只更新允许修改的字段
			if updates.Name != "" {
				res.Name = updates.Name
			}
			if updates.Description != "" {
				res.Description = updates.Description
			}
			if updates.Content != "" {
				res.Content = updates.Content
			}
			if updates.Visibility != "" {
				res.Visibility = updates.Visibility
			}
			if updates.ProjectID != "" {
				res.ProjectID = updates.ProjectID
			}
			if updates.TaskID != "" {
				res.TaskID = updates.TaskID
			}

			res.UpdatedAt = time.Now()
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("resource not found")
	}

	collection.UpdatedAt = time.Now()

	// 触发异步保存
	select {
	case rm.saveQueue <- username:
	default:
	}

	return nil
}

// ClearAutoAddedResources 清除自动添加的资源
// 参数:
//   - username: 用户名
//
// 返回:
//   - error: 清除失败时返回错误
//
// 功能:
//   - 过滤删除 AutoAdded 为 true 的资源
//   - 更新 UpdatedAt 时间戳
//   - 触发异步保存
func (rm *ResourceManager) ClearAutoAddedResources(username string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	collection, ok := rm.users[username]
	if !ok {
		return nil
	}

	newResources := []*Resource{}
	for _, res := range collection.Resources {
		if !res.AutoAdded {
			newResources = append(newResources, res)
		}
	}

	collection.Resources = newResources
	collection.UpdatedAt = time.Now()

	// 触发异步保存
	select {
	case rm.saveQueue <- username:
	default:
	}

	return nil
}
