package documents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// IndexManager 索引管理器
type IndexManager struct {
	projectDir       string
	docIndexPath     string
	relIndexPath     string
	refIndexPath     string
	DocMeta          map[string]*DocMetaEntry `json:"documents"`
	ParentChildren   map[string][]string      `json:"parent_children"`
	Relationships    map[string]*Relationship `json:"relationships"`
	ReferencesByTask map[string][]*Reference  `json:"references_by_task"`
	ReferencesByDoc  map[string][]*Reference  `json:"references_by_doc"`
	mu               sync.RWMutex
	version          int
}

// NewIndexManager 创建新的索引管理器
func NewIndexManager(projectDir string) *IndexManager {
	docIndexPath := filepath.Join(projectDir, "documents_index.json")
	relIndexPath := filepath.Join(projectDir, "relationships_index.json")
	refIndexPath := filepath.Join(projectDir, "references_index.json")

	return &IndexManager{
		projectDir:       projectDir,
		docIndexPath:     docIndexPath,
		relIndexPath:     relIndexPath,
		refIndexPath:     refIndexPath,
		DocMeta:          make(map[string]*DocMetaEntry),
		ParentChildren:   make(map[string][]string),
		Relationships:    make(map[string]*Relationship),
		ReferencesByTask: make(map[string][]*Reference),
		ReferencesByDoc:  make(map[string][]*Reference),
		version:          1,
	}
}

// Load 加载索引文件
func (im *IndexManager) Load() error {
	im.mu.Lock()
	defer im.mu.Unlock()

	// 确保项目目录存在
	if err := os.MkdirAll(im.projectDir, 0755); err != nil {
		return fmt.Errorf("failed to create project directory: %w", err)
	}

	// 检查索引文件是否存在
	if _, err := os.Stat(im.docIndexPath); os.IsNotExist(err) {
		// 创建空索引
		return im.createEmptyIndex()
	}

	// 加载现有索引
	file, err := os.Open(im.docIndexPath)
	if err != nil {
		return fmt.Errorf("failed to open documents index: %w", err)
	}
	defer file.Close()

	var index DocumentsIndex
	if err := json.NewDecoder(file).Decode(&index); err != nil {
		return fmt.Errorf("failed to decode documents index: %w", err)
	}

	im.DocMeta = index.Documents
	if im.DocMeta == nil {
		im.DocMeta = make(map[string]*DocMetaEntry)
	}
	im.version = index.Version

	// 加载关系索引
	if err := im.loadRelationships(); err != nil {
		return fmt.Errorf("failed to load relationships: %w", err)
	}

	// 加载引用索引
	if err := im.loadReferences(); err != nil {
		return fmt.Errorf("failed to load references: %w", err)
	}

	// 重建父子关系映射
	im.rebuildParentChildrenMap()

	return nil
}

// createEmptyIndex 创建空索引文件
func (im *IndexManager) createEmptyIndex() error {
	index := DocumentsIndex{
		Documents: make(map[string]*DocMetaEntry),
		Version:   1,
		UpdatedAt: time.Now(),
	}

	return im.writeIndex(index)
}

// rebuildParentChildrenMap 重建父子关系映射
func (im *IndexManager) rebuildParentChildrenMap() {
	im.ParentChildren = make(map[string][]string)

	for nodeID, meta := range im.DocMeta {
		if meta.ParentID != nil {
			parentID := *meta.ParentID
			im.ParentChildren[parentID] = append(im.ParentChildren[parentID], nodeID)
		} else {
			im.ParentChildren["virtual_root"] = append(im.ParentChildren["virtual_root"], nodeID)
		}
	}

	// 按position排序每个父节点的子节点
	for parentID := range im.ParentChildren {
		im.sortChildrenByPosition(parentID)
	}
}

// sortChildrenByPosition 按position排序子节点
func (im *IndexManager) sortChildrenByPosition(parentID string) {
	children, exists := im.ParentChildren[parentID]
	if !exists || len(children) <= 1 {
		return
	}

	filtered := make([]string, 0, len(children))
	for _, childID := range children {
		if meta, ok := im.DocMeta[childID]; ok && meta != nil {
			filtered = append(filtered, childID)
		}
	}

	if len(filtered) == 0 {
		delete(im.ParentChildren, parentID)
		return
	}

	if len(filtered) == 1 {
		im.ParentChildren[parentID] = filtered
		return
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		left := im.DocMeta[filtered[i]]
		right := im.DocMeta[filtered[j]]

		switch {
		case left == nil && right == nil:
			return filtered[i] < filtered[j]
		case left == nil:
			return false
		case right == nil:
			return true
		case left.Position == right.Position:
			return filtered[i] < filtered[j]
		default:
			return left.Position < right.Position
		}
	})

	im.ParentChildren[parentID] = filtered
}

// FlushDocuments 原子性写入文档索引
func (im *IndexManager) FlushDocuments() error {
	im.mu.Lock()
	defer im.mu.Unlock()

	return im.flushDocumentsLocked()
}

func (im *IndexManager) flushDocumentsLocked() error {
	im.version++
	index := DocumentsIndex{
		Documents: im.DocMeta,
		Version:   im.version,
		UpdatedAt: time.Now(),
	}

	return im.writeIndex(index)
}

// writeIndex 写入索引文件（原子操作）
func (im *IndexManager) writeIndex(index DocumentsIndex) error {
	// 写临时文件
	tempPath := im.docIndexPath + ".tmp"

	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp index file: %w", err)
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(index); err != nil {
		file.Close()
		os.Remove(tempPath)
		return fmt.Errorf("failed to encode index: %w", err)
	}

	if err := file.Close(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// 原子替换
	if err := os.Rename(tempPath, im.docIndexPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// AddNode 添加节点到索引
func (im *IndexManager) AddNode(meta *DocMetaEntry) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	im.DocMeta[meta.ID] = meta

	// 更新父子关系
	if meta.ParentID != nil {
		parentID := *meta.ParentID
		im.ParentChildren[parentID] = append(im.ParentChildren[parentID], meta.ID)
		im.sortChildrenByPosition(parentID)
	} else {
		// 根文档添加到虚拟根节点
		im.ParentChildren["virtual_root"] = append(im.ParentChildren["virtual_root"], meta.ID)
		im.sortChildrenByPosition("virtual_root")
	}

	return nil
}

// GetNode 获取节点
func (im *IndexManager) GetNode(nodeID string) (*DocMetaEntry, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	node, exists := im.DocMeta[nodeID]
	if !exists {
		return nil, ErrNodeNotFound
	}

	return node, nil
}

// GetChildren 获取子节点ID列表
func (im *IndexManager) GetChildren(nodeID string) []string {
	im.mu.RLock()
	defer im.mu.RUnlock()

	children, exists := im.ParentChildren[nodeID]
	if !exists {
		return []string{}
	}

	// 返回副本
	result := make([]string, len(children))
	copy(result, children)
	return result
}

// ListAllDocuments 获取所有文档ID列表
func (im *IndexManager) ListAllDocuments() ([]string, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	var docIDs []string
	for docID := range im.DocMeta {
		docIDs = append(docIDs, docID)
	}

	return docIDs, nil
}

// ValidateChildrenLimit 验证子节点数量限制
func (im *IndexManager) ValidateChildrenLimit(parentID string) error {
	children := im.GetChildren(parentID)
	if len(children) >= MaxChildrenPerNode {
		return ErrChildrenLimitReached
	}
	return nil
}

// loadRelationships 加载关系索引
func (im *IndexManager) loadRelationships() error {
	if _, err := os.Stat(im.relIndexPath); os.IsNotExist(err) {
		return im.createEmptyRelationships()
	}

	file, err := os.Open(im.relIndexPath)
	if err != nil {
		return fmt.Errorf("failed to open relationships index: %w", err)
	}
	defer file.Close()

	var index RelationshipsIndex
	if err := json.NewDecoder(file).Decode(&index); err != nil {
		return fmt.Errorf("failed to decode relationships index: %w", err)
	}

	im.Relationships = index.Relationships
	if im.Relationships == nil {
		im.Relationships = make(map[string]*Relationship)
	}

	return nil
}

// createEmptyRelationships 创建空关系索引
func (im *IndexManager) createEmptyRelationships() error {
	index := RelationshipsIndex{
		Relationships: make(map[string]*Relationship),
		Version:       1,
		UpdatedAt:     time.Now(),
	}
	return im.writeRelationships(index)
}

// loadReferences 加载引用索引
func (im *IndexManager) loadReferences() error {
	if _, err := os.Stat(im.refIndexPath); os.IsNotExist(err) {
		return im.createEmptyReferences()
	}

	file, err := os.Open(im.refIndexPath)
	if err != nil {
		return fmt.Errorf("failed to open references index: %w", err)
	}
	defer file.Close()

	var index ReferencesIndex
	if err := json.NewDecoder(file).Decode(&index); err != nil {
		return fmt.Errorf("failed to decode references index: %w", err)
	}

	// 重建引用映射
	im.rebuildReferenceMaps(index.References)

	return nil
}

// createEmptyReferences 创建空引用索引
func (im *IndexManager) createEmptyReferences() error {
	index := ReferencesIndex{
		References: make(map[string]*Reference),
		Version:    1,
		UpdatedAt:  time.Now(),
	}
	return im.writeReferences(index)
}

// rebuildReferenceMaps 重建引用映射
func (im *IndexManager) rebuildReferenceMaps(references map[string]*Reference) {
	im.ReferencesByTask = make(map[string][]*Reference)
	im.ReferencesByDoc = make(map[string][]*Reference)

	for _, ref := range references {
		im.ReferencesByTask[ref.TaskID] = append(im.ReferencesByTask[ref.TaskID], ref)
		im.ReferencesByDoc[ref.DocumentID] = append(im.ReferencesByDoc[ref.DocumentID], ref)
	}
}

// FlushRelationships 原子性写入关系索引
func (im *IndexManager) FlushRelationships() error {
	im.mu.Lock()
	defer im.mu.Unlock()

	return im.flushRelationshipsLocked()
}

func (im *IndexManager) flushRelationshipsLocked() error {
	index := RelationshipsIndex{
		Relationships: im.Relationships,
		Version:       im.version + 1,
		UpdatedAt:     time.Now(),
	}

	return im.writeRelationships(index)
}

// writeRelationships 写入关系索引文件（原子操作）
func (im *IndexManager) writeRelationships(index RelationshipsIndex) error {
	tempPath := im.relIndexPath + ".tmp"

	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp relationships file: %w", err)
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(index); err != nil {
		file.Close()
		os.Remove(tempPath)
		return fmt.Errorf("failed to encode relationships: %w", err)
	}

	if err := file.Close(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tempPath, im.relIndexPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// FlushReferences 原子性写入引用索引
func (im *IndexManager) FlushReferences() error {
	im.mu.Lock()
	defer im.mu.Unlock()

	return im.flushReferencesLocked()
}

func (im *IndexManager) flushReferencesLocked() error {
	// 从映射重建references map
	allRefs := make(map[string]*Reference)
	for _, refs := range im.ReferencesByTask {
		for _, ref := range refs {
			allRefs[ref.ID] = ref
		}
	}

	index := ReferencesIndex{
		References: allRefs,
		Version:    im.version + 1,
		UpdatedAt:  time.Now(),
	}

	return im.writeReferences(index)
}

// writeReferences 写入引用索引文件（原子操作）
func (im *IndexManager) writeReferences(index ReferencesIndex) error {
	tempPath := im.refIndexPath + ".tmp"

	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp references file: %w", err)
	}

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(index); err != nil {
		file.Close()
		os.Remove(tempPath)
		return fmt.Errorf("failed to encode references: %w", err)
	}

	if err := file.Close(); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Rename(tempPath, im.refIndexPath); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}
