package documents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

// DocumentTreeManager 文档树管理器
type DocumentTreeManager struct {
	index           *IndexManager
	projectDir      string
	snapshotManager *SnapshotManager
}

// NewDocumentTreeManager 创建文档树管理器
func NewDocumentTreeManager(projectDir string) *DocumentTreeManager {
	return &DocumentTreeManager{
		index:           NewIndexManager(projectDir),
		projectDir:      projectDir,
		snapshotManager: NewSnapshotManager(projectDir),
	}
}

// Initialize 初始化管理器
func (m *DocumentTreeManager) Initialize() error {
	return m.index.Load()
}

// CreateNode 创建节点
func (m *DocumentTreeManager) CreateNode(req CreateNodeRequest) (*DocMetaEntry, error) {
	// 验证父节点存在（如果指定了父节点）
	var level int = 1
	if req.ParentID != nil {
		parent, err := m.index.GetNode(*req.ParentID)
		if err != nil {
			return nil, fmt.Errorf("parent node not found: %w", err)
		}

		level = parent.Level + 1

		// 验证层级限制
		if level > MaxLevel {
			return nil, ErrHierarchyOverflow
		}

		// 验证子节点数量限制
		if err := m.index.ValidateChildrenLimit(*req.ParentID); err != nil {
			return nil, err
		}
	}

	// 计算position（在同级中的位置）
	var siblings []string
	if req.ParentID != nil {
		siblings = m.index.GetChildren(*req.ParentID)
	} else {
		// 根文档获取虚拟根的子节点
		siblings = m.index.GetChildren("virtual_root")
	}
	position := len(siblings)

	// 创建节点元数据
	nodeID := "doc_" + uuid.New().String()
	now := time.Now()

	meta := &DocMetaEntry{
		ID:        nodeID,
		ParentID:  req.ParentID,
		Title:     req.Title,
		Type:      req.Type,
		Level:     level,
		Position:  position,
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// 验证层级
	if err := meta.ValidateLevel(); err != nil {
		return nil, err
	}

	// 添加到索引
	if err := m.index.AddNode(meta); err != nil {
		return nil, fmt.Errorf("failed to add node to index: %w", err)
	}

	// 创建文档内容文件
	if err := m.createDocumentFile(nodeID, req.Content); err != nil {
		return nil, fmt.Errorf("failed to create document file: %w", err)
	}

	// 保存索引
	if err := m.index.FlushDocuments(); err != nil {
		return nil, fmt.Errorf("failed to flush index: %w", err)
	}

	return meta, nil
}

// MoveNode 移动节点
func (m *DocumentTreeManager) MoveNode(nodeID string, req MoveNodeRequest) error {
	// 获取当前节点
	node, err := m.index.GetNode(nodeID)
	if err != nil {
		return err
	}

	// 验证新父节点存在（如果指定）
	var newLevel int = 1
	if req.NewParentID != nil {
		newParent, err := m.index.GetNode(*req.NewParentID)
		if err != nil {
			return fmt.Errorf("new parent node not found: %w", err)
		}

		newLevel = newParent.Level + 1

		// 验证层级限制
		if newLevel > MaxLevel {
			return ErrHierarchyOverflow
		}

		// 验证不能移动到自己的子节点
		if m.isDescendant(nodeID, *req.NewParentID) {
			return ErrCircularDependency
		}

		// 验证新父节点的子节点数量限制
		if err := m.index.ValidateChildrenLimit(*req.NewParentID); err != nil {
			return err
		}
	}

	// 更新节点信息
	node.ParentID = req.NewParentID
	node.Level = newLevel
	node.Position = req.Position
	node.UpdatedAt = time.Now()

	// 递归更新子节点的层级
	if err := m.updateChildrenLevel(nodeID, newLevel); err != nil {
		return err
	}

	// 重建父子关系映射
	m.index.rebuildParentChildrenMap()

	// 保存索引
	return m.index.FlushDocuments()
}

// UpdateNode 更新节点元数据（如标题、类型）
func (m *DocumentTreeManager) UpdateNode(nodeID string, req UpdateNodeRequest) (*DocMetaEntry, error) {
	if req.Title == nil && req.Type == nil {
		return nil, fmt.Errorf("no fields to update")
	}

	m.index.mu.Lock()
	node, exists := m.index.DocMeta[nodeID]
	if !exists {
		m.index.mu.Unlock()
		return nil, ErrNodeNotFound
	}

	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			m.index.mu.Unlock()
			return nil, fmt.Errorf("title cannot be empty")
		}
		node.Title = title
	}

	if req.Type != nil {
		node.Type = *req.Type
	}

	node.UpdatedAt = time.Now()
	m.index.mu.Unlock()

	if err := m.index.FlushDocuments(); err != nil {
		return nil, fmt.Errorf("failed to flush index: %w", err)
	}

	return node, nil
}

// GetTree 获取文档树
func (m *DocumentTreeManager) GetTree(rootID *string, depth int) (*DocumentTreeDTO, error) {
	// 如果未指定根节点，获取所有根节点
	if rootID == nil {
		return m.getRootTrees(depth)
	}

	// 获取指定根节点的树
	root, err := m.index.GetNode(*rootID)
	if err != nil {
		return nil, err
	}

	return m.buildTree(root, depth)
}

// DeleteNode 删除节点
func (m *DocumentTreeManager) DeleteNode(nodeID string, cascade bool) error {
	// 获取节点
	node, err := m.index.GetNode(nodeID)
	if err != nil {
		return err
	}

	// 获取子节点
	children := m.index.GetChildren(nodeID)

	if len(children) > 0 && !cascade {
		return fmt.Errorf("node has children, use cascade=true to delete")
	}

	// 递归删除子节点（如果cascade=true）
	if cascade {
		for _, childID := range children {
			if err := m.DeleteNode(childID, true); err != nil {
				return fmt.Errorf("failed to delete child node %s: %w", childID, err)
			}
		}
	}

	// 从索引中删除
	m.index.mu.Lock()
	delete(m.index.DocMeta, nodeID)

	// 从父节点的子列表中移除
	if node.ParentID != nil {
		parentChildren := m.index.ParentChildren[*node.ParentID]
		for i, childID := range parentChildren {
			if childID == nodeID {
				m.index.ParentChildren[*node.ParentID] = append(parentChildren[:i], parentChildren[i+1:]...)
				break
			}
		}
	}
	m.index.mu.Unlock()

	// 删除文档文件
	if err := m.deleteDocumentFile(nodeID); err != nil {
		// 日志错误但继续执行
		fmt.Printf("Warning: failed to delete document file for node %s: %v\n", nodeID, err)
	}

	// 保存索引
	return m.index.FlushDocuments()
}

// 辅助方法

// createDocumentFile 创建文档内容文件
func (m *DocumentTreeManager) createDocumentFile(nodeID, content string) error {
	docPath := filepath.Join(m.projectDir, nodeID+".md")
	return writeFileAtomic(docPath, []byte(content))
}

// deleteDocumentFile 删除文档文件
func (m *DocumentTreeManager) deleteDocumentFile(nodeID string) error {
	docPath := filepath.Join(m.projectDir, nodeID+".md")
	return deleteFileIfExists(docPath)
}

// isDescendant 检查target是否是nodeID的后代
func (m *DocumentTreeManager) isDescendant(nodeID, target string) bool {
	children := m.index.GetChildren(nodeID)
	for _, childID := range children {
		if childID == target {
			return true
		}
		if m.isDescendant(childID, target) {
			return true
		}
	}
	return false
}

// updateChildrenLevel 递归更新子节点层级
func (m *DocumentTreeManager) updateChildrenLevel(nodeID string, parentLevel int) error {
	children := m.index.GetChildren(nodeID)
	for _, childID := range children {
		child, err := m.index.GetNode(childID)
		if err != nil {
			return err
		}

		child.Level = parentLevel + 1
		child.UpdatedAt = time.Now()

		// 验证层级限制
		if err := child.ValidateLevel(); err != nil {
			return err
		}

		// 递归更新
		if err := m.updateChildrenLevel(childID, child.Level); err != nil {
			return err
		}
	}
	return nil
}

// getRootTrees 获取所有根节点树
func (m *DocumentTreeManager) getRootTrees(depth int) (*DocumentTreeDTO, error) {
	// 创建虚拟根节点
	virtualRoot := &DocMetaEntry{
		ID:    "virtual_root",
		Title: "Root",
		Level: 0,
	}

	return m.buildTree(virtualRoot, depth)
}

// buildTree 构建文档树
func (m *DocumentTreeManager) buildTree(node *DocMetaEntry, depth int) (*DocumentTreeDTO, error) {
	result := &DocumentTreeDTO{
		Node: node,
	}

	// 如果深度为0，不获取子节点
	if depth == 0 {
		return result, nil
	}

	// 获取子节点
	childrenIDs := m.index.GetChildren(node.ID)
	if len(childrenIDs) == 0 {
		return result, nil
	}

	// 构建子树
	for _, childID := range childrenIDs {
		child, err := m.index.GetNode(childID)
		if err != nil {
			continue // 跳过损坏的节点
		}

		childTree, err := m.buildTree(child, depth-1)
		if err != nil {
			return nil, err
		}

		result.Children = append(result.Children, childTree)
	}

	return result, nil
}

// UpdateContent 更新文档内容（带版本控制）
func (m *DocumentTreeManager) UpdateContent(nodeID, content string, clientVersion int) (int, error) {
	m.index.mu.Lock()
	defer m.index.mu.Unlock()

	// 检查节点是否存在
	meta, exists := m.index.DocMeta[nodeID]
	if !exists {
		return 0, ErrNodeNotFound
	}

	// 版本检查
	if meta.Version != clientVersion {
		return 0, ErrVersionMismatch
	}

	nextVersion := meta.Version + 1

	// 读取当前内容用于版本快照
	contentPath := filepath.Join(m.projectDir, fmt.Sprintf("%s.md", nodeID))
	var previousContent string
	if data, err := os.ReadFile(contentPath); err == nil {
		previousContent = string(data)
	} else if os.IsNotExist(err) {
		previousContent = ""
	} else {
		return 0, fmt.Errorf("failed to read existing content: %w", err)
	}

	// 创建快照（在更新前保存旧版本内容）
	if err := m.snapshotManager.CreateSnapshot(nodeID, clientVersion, previousContent); err != nil {
		// 快照失败不应阻止更新，只记录错误
		fmt.Printf("Warning: failed to create snapshot for %s version %d: %v\n", nodeID, clientVersion, err)
	}

	// 写入内容文件
	if err := os.WriteFile(contentPath, []byte(content), 0644); err != nil {
		return 0, fmt.Errorf("failed to write content file: %w", err)
	}

	// 更新版本和时间戳
	meta.Version = nextVersion
	meta.UpdatedAt = time.Now()

	// 刷新索引
	if err := m.index.flushDocumentsLocked(); err != nil {
		return 0, fmt.Errorf("failed to flush index: %w", err)
	}

	return nextVersion, nil
}

// GetContent 获取文档内容和元数据
func (m *DocumentTreeManager) GetContent(nodeID string) (string, *DocMetaEntry, error) {
	meta, err := m.index.GetNode(nodeID)
	if err != nil {
		return "", nil, err
	}

	contentPath := filepath.Join(m.projectDir, fmt.Sprintf("%s.md", nodeID))
	content, err := os.ReadFile(contentPath)
	if err != nil {
		if os.IsNotExist(err) {
			// 文件不存在，返回空内容
			return "", meta, nil
		}
		return "", nil, fmt.Errorf("failed to read content file: %w", err)
	}

	return string(content), meta, nil
}

// GetVersionHistory 获取文档版本历史
// 返回值始终包含当前版本的快照元信息（即使尚未生成实际快照），
// 以保证客户端可以看到并访问最新版本的内容。
func (m *DocumentTreeManager) GetVersionHistory(nodeID string, limit int) ([]SnapshotMeta, error) {
	snapshots, err := m.snapshotManager.ListSnapshots(nodeID, limit)
	if err != nil {
		return nil, err
	}

	meta, err := m.index.GetNode(nodeID)
	if err != nil {
		if err == ErrNodeNotFound {
			return snapshots, nil
		}
		return nil, err
	}

	hasCurrent := false
	for _, snap := range snapshots {
		if snap.Version == meta.Version {
			hasCurrent = true
			break
		}
	}

	if !hasCurrent {
		contentPath := filepath.Join(m.projectDir, fmt.Sprintf("%s.md", nodeID))
		var size int64
		if info, err := os.Stat(contentPath); err == nil {
			size = info.Size()
		}

		currentSnapshot := SnapshotMeta{
			Version:   meta.Version,
			CreatedAt: meta.UpdatedAt,
			Path:      contentPath,
			Size:      size,
		}

		snapshots = append([]SnapshotMeta{currentSnapshot}, snapshots...)

		if limit > 0 && len(snapshots) > limit {
			snapshots = snapshots[:limit]
		}
	}

	return snapshots, nil
}

// GetVersionContent 获取特定版本的内容
func (m *DocumentTreeManager) GetVersionContent(nodeID string, version int) (string, error) {
	meta, err := m.index.GetNode(nodeID)
	if err != nil {
		return "", err
	}

	if meta.Version == version {
		contentPath := filepath.Join(m.projectDir, fmt.Sprintf("%s.md", nodeID))
		data, err := os.ReadFile(contentPath)
		if err != nil {
			if os.IsNotExist(err) {
				return "", nil
			}
			return "", fmt.Errorf("failed to read current version content: %w", err)
		}
		return string(data), nil
	}

	return m.snapshotManager.GetSnapshot(nodeID, version)
}

// CompareVersions 比较两个版本的差异
func (m *DocumentTreeManager) CompareVersions(nodeID string, fromVersion, toVersion int) (*DiffResult, error) {
	differ := NewContentDiffer()

	// 获取两个版本的内容
	var fromContent, toContent string
	var err error

	if fromVersion == 0 {
		// 从当前版本比较
		fromContent, _, err = m.GetContent(nodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to get current content: %w", err)
		}
	} else {
		fromContent, err = m.GetVersionContent(nodeID, fromVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to get version %d content: %w", fromVersion, err)
		}
	}

	if toVersion == 0 {
		// 与当前版本比较
		toContent, _, err = m.GetContent(nodeID)
		if err != nil {
			return nil, fmt.Errorf("failed to get current content: %w", err)
		}
	} else {
		toContent, err = m.GetVersionContent(nodeID, toVersion)
		if err != nil {
			return nil, fmt.Errorf("failed to get version %d content: %w", toVersion, err)
		}
	}

	return differ.CompareContent(fromContent, toContent, fromVersion, toVersion), nil
}

// AnalyzeImpact 分析节点影响范围
func (m *DocumentTreeManager) AnalyzeImpact(nodeID string, modes []AnalysisMode) (*ImpactResult, error) {
	analyzer := NewImpactAnalyzer(m.index)
	return analyzer.Analyze(nodeID, modes)
}
