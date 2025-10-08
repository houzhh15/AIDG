package documents

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func setupTestDir(t *testing.T) string {
	dir := filepath.Join(os.TempDir(), "documents_test_"+time.Now().Format("20060102_150405"))
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	t.Cleanup(func() {
		os.RemoveAll(dir)
	})

	return dir
}

func TestIndexManager_Initialize(t *testing.T) {
	testDir := setupTestDir(t)

	manager := NewIndexManager(testDir)
	err := manager.Load()
	if err != nil {
		t.Fatalf("Failed to initialize index manager: %v", err)
	}

	// 验证索引文件是否创建
	indexPath := filepath.Join(testDir, "documents_index.json")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Errorf("Index file was not created")
	}

	// 验证初始状态
	if len(manager.DocMeta) != 0 {
		t.Errorf("Expected empty DocMeta, got %d items", len(manager.DocMeta))
	}
}

func TestDocumentTreeManager_CreateNode(t *testing.T) {
	testDir := setupTestDir(t)

	manager := NewDocumentTreeManager(testDir)
	err := manager.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize tree manager: %v", err)
	}

	// 创建根节点
	req := CreateNodeRequest{
		Title:   "Test Root",
		Type:    TypeArchitecture,
		Content: "# Test Root Document\n\nThis is a test.",
	}

	node, err := manager.CreateNode(req)
	if err != nil {
		t.Fatalf("Failed to create root node: %v", err)
	}

	// 验证节点属性
	if node.Title != "Test Root" {
		t.Errorf("Expected title 'Test Root', got '%s'", node.Title)
	}
	if node.Type != TypeArchitecture {
		t.Errorf("Expected type %s, got %s", TypeArchitecture, node.Type)
	}
	if node.Level != 1 {
		t.Errorf("Expected level 1, got %d", node.Level)
	}
	if node.ParentID != nil {
		t.Errorf("Expected nil parent for root node, got %v", node.ParentID)
	}

	// 验证文档文件是否创建
	docPath := filepath.Join(testDir, node.ID+".md")
	if _, err := os.Stat(docPath); os.IsNotExist(err) {
		t.Errorf("Document file was not created")
	}
}

func TestDocumentTreeManager_CreateChildNode(t *testing.T) {
	testDir := setupTestDir(t)

	manager := NewDocumentTreeManager(testDir)
	err := manager.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize tree manager: %v", err)
	}

	// 创建根节点
	rootReq := CreateNodeRequest{
		Title:   "Root",
		Type:    TypeArchitecture,
		Content: "Root content",
	}

	rootNode, err := manager.CreateNode(rootReq)
	if err != nil {
		t.Fatalf("Failed to create root node: %v", err)
	}

	// 创建子节点
	childReq := CreateNodeRequest{
		ParentID: &rootNode.ID,
		Title:    "Child",
		Type:     TypeTechDesign,
		Content:  "Child content",
	}

	childNode, err := manager.CreateNode(childReq)
	if err != nil {
		t.Fatalf("Failed to create child node: %v", err)
	}

	// 验证子节点属性
	if childNode.ParentID == nil || *childNode.ParentID != rootNode.ID {
		t.Errorf("Expected parent ID %s, got %v", rootNode.ID, childNode.ParentID)
	}
	if childNode.Level != 2 {
		t.Errorf("Expected level 2, got %d", childNode.Level)
	}
}

func TestDocumentTreeManager_HierarchyLimits(t *testing.T) {
	testDir := setupTestDir(t)

	manager := NewDocumentTreeManager(testDir)
	err := manager.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize tree manager: %v", err)
	}

	// 创建深层嵌套到达极限
	var currentParent *string
	for level := 1; level <= MaxLevel; level++ {
		req := CreateNodeRequest{
			ParentID: currentParent,
			Title:    fmt.Sprintf("Level %d", level),
			Type:     TypeBackground,
			Content:  "Content",
		}

		node, err := manager.CreateNode(req)
		if err != nil {
			t.Fatalf("Failed to create node at level %d: %v", level, err)
		}

		currentParent = &node.ID
	}

	// 尝试创建超出限制的层级
	req := CreateNodeRequest{
		ParentID: currentParent,
		Title:    "Over Limit",
		Type:     TypeBackground,
		Content:  "Should fail",
	}

	_, err = manager.CreateNode(req)
	if err != ErrHierarchyOverflow {
		t.Errorf("Expected ErrHierarchyOverflow, got %v", err)
	}
}

func TestDocumentTreeManager_GetTree(t *testing.T) {
	testDir := setupTestDir(t)

	manager := NewDocumentTreeManager(testDir)
	err := manager.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize tree manager: %v", err)
	}

	// 创建测试树结构
	// Root
	//  ├── Child1
	//  └── Child2
	//      └── Grandchild

	rootReq := CreateNodeRequest{
		Title: "Root", Type: TypeArchitecture, Content: "Root",
	}
	root, _ := manager.CreateNode(rootReq)

	child1Req := CreateNodeRequest{
		ParentID: &root.ID, Title: "Child1", Type: TypeTechDesign, Content: "Child1",
	}
	_, _ = manager.CreateNode(child1Req)

	child2Req := CreateNodeRequest{
		ParentID: &root.ID, Title: "Child2", Type: TypeTechDesign, Content: "Child2",
	}
	child2, _ := manager.CreateNode(child2Req)

	grandchildReq := CreateNodeRequest{
		ParentID: &child2.ID, Title: "Grandchild", Type: TypeBackground, Content: "Grandchild",
	}
	_, _ = manager.CreateNode(grandchildReq)

	// 获取完整树（深度3）
	tree, err := manager.GetTree(&root.ID, 3)
	if err != nil {
		t.Fatalf("Failed to get tree: %v", err)
	}

	// 验证树结构
	if tree.Node.ID != root.ID {
		t.Errorf("Expected root ID %s, got %s", root.ID, tree.Node.ID)
	}

	if len(tree.Children) != 2 {
		t.Errorf("Expected 2 children, got %d", len(tree.Children))
	}

	// 检查Child2是否有子节点
	var child2Tree *DocumentTreeDTO
	for _, child := range tree.Children {
		if child.Node.Title == "Child2" {
			child2Tree = child
			break
		}
	}

	if child2Tree == nil {
		t.Errorf("Child2 not found in tree")
	} else if len(child2Tree.Children) != 1 {
		t.Errorf("Expected Child2 to have 1 child, got %d", len(child2Tree.Children))
	}
}

func TestDocumentTreeManager_GetTreeWithoutRootIncludesAllRoots(t *testing.T) {
	testDir := setupTestDir(t)

	manager := NewDocumentTreeManager(testDir)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize tree manager: %v", err)
	}

	root1, err := manager.CreateNode(CreateNodeRequest{
		Title:   "RootOne",
		Type:    TypeArchitecture,
		Content: "Root 1",
	})
	if err != nil {
		t.Fatalf("Failed to create first root node: %v", err)
	}

	root2, err := manager.CreateNode(CreateNodeRequest{
		Title:   "RootTwo",
		Type:    TypeTechDesign,
		Content: "Root 2",
	})
	if err != nil {
		t.Fatalf("Failed to create second root node: %v", err)
	}

	tree, err := manager.GetTree(nil, 3)
	if err != nil {
		t.Fatalf("Failed to get tree without root: %v", err)
	}

	if tree.Node.ID != "virtual_root" {
		t.Fatalf("Expected virtual_root node id, got %s", tree.Node.ID)
	}

	if len(tree.Children) != 2 {
		t.Fatalf("Expected 2 root children, got %d", len(tree.Children))
	}

	found := map[string]bool{
		root1.ID: false,
		root2.ID: false,
	}

	for _, child := range tree.Children {
		found[child.Node.ID] = true
	}

	for id, ok := range found {
		if !ok {
			t.Errorf("Expected root node %s to be present under virtual root", id)
		}
	}
}

func TestDocumentTreeManager_UpdateContent(t *testing.T) {
	testDir := setupTestDir(t)

	manager := NewDocumentTreeManager(testDir)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize tree manager: %v", err)
	}

	node, err := manager.CreateNode(CreateNodeRequest{
		Title:   "Updatable Doc",
		Type:    TypeTechDesign,
		Content: "Initial content",
	})
	if err != nil {
		t.Fatalf("Failed to create document: %v", err)
	}

	originalVersion := node.Version
	updatedContent := "Updated content"

	newVersion, err := manager.UpdateContent(node.ID, updatedContent, originalVersion)
	if err != nil {
		t.Fatalf("UpdateContent returned error: %v", err)
	}

	if newVersion != originalVersion+1 {
		t.Fatalf("expected version %d, got %d", originalVersion+1, newVersion)
	}

	content, meta, err := manager.GetContent(node.ID)
	if err != nil {
		t.Fatalf("GetContent returned error: %v", err)
	}

	if content != updatedContent {
		t.Fatalf("expected content %q, got %q", updatedContent, content)
	}

	if meta.Version != newVersion {
		t.Fatalf("metadata version not updated: expected %d, got %d", newVersion, meta.Version)
	}

	if _, err := manager.UpdateContent(node.ID, "outdated", originalVersion); !errors.Is(err, ErrVersionMismatch) {
		t.Fatalf("expected ErrVersionMismatch, got %v", err)
	}
}

func TestDocumentTreeManager_VersionHistoryIncludesCurrent(t *testing.T) {
	testDir := setupTestDir(t)

	manager := NewDocumentTreeManager(testDir)
	if err := manager.Initialize(); err != nil {
		t.Fatalf("Failed to initialize tree manager: %v", err)
	}

	initialContent := "Initial content"
	node, err := manager.CreateNode(CreateNodeRequest{
		Title:   "History Doc",
		Type:    TypeTechDesign,
		Content: initialContent,
	})
	if err != nil {
		t.Fatalf("Failed to create document: %v", err)
	}

	history, err := manager.GetVersionHistory(node.ID, 10)
	if err != nil {
		t.Fatalf("GetVersionHistory returned error: %v", err)
	}

	if len(history) == 0 || history[0].Version != node.Version {
		t.Fatalf("expected current version %d at top of history, got %+v", node.Version, history)
	}

	newContent := "Updated content"
	newVersion, err := manager.UpdateContent(node.ID, newContent, node.Version)
	if err != nil {
		t.Fatalf("UpdateContent returned error: %v", err)
	}

	history, err = manager.GetVersionHistory(node.ID, 10)
	if err != nil {
		t.Fatalf("GetVersionHistory returned error after update: %v", err)
	}

	if len(history) < 2 {
		t.Fatalf("expected at least 2 history entries, got %d", len(history))
	}

	if history[0].Version != newVersion {
		t.Fatalf("expected latest history entry version %d, got %d", newVersion, history[0].Version)
	}

	if history[1].Version != newVersion-1 {
		t.Fatalf("expected previous history entry version %d, got %d", newVersion-1, history[1].Version)
	}

	currentContent, err := manager.GetVersionContent(node.ID, newVersion)
	if err != nil {
		t.Fatalf("GetVersionContent for current version returned error: %v", err)
	}
	if currentContent != newContent {
		t.Fatalf("expected current version content %q, got %q", newContent, currentContent)
	}

	previousContent, err := manager.GetVersionContent(node.ID, newVersion-1)
	if err != nil {
		t.Fatalf("GetVersionContent for previous version returned error: %v", err)
	}
	if previousContent != initialContent {
		t.Fatalf("expected previous version content %q, got %q", initialContent, previousContent)
	}
}

func TestIndexManager_ConcurrentAccess(t *testing.T) {
	testDir := setupTestDir(t)

	manager := NewIndexManager(testDir)
	err := manager.Load()
	if err != nil {
		t.Fatalf("Failed to load index: %v", err)
	}

	// 并发添加节点
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer func() { done <- true }()

			meta := &DocMetaEntry{
				ID:        fmt.Sprintf("node_%d", id),
				Title:     fmt.Sprintf("Node %d", id),
				Type:      TypeBackground,
				Level:     1,
				Version:   1,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			err := manager.AddNode(meta)
			if err != nil {
				t.Errorf("Failed to add node %d: %v", id, err)
			}
		}(i)
	}

	// 等待所有goroutine完成
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// 验证所有节点都添加成功
	if len(manager.DocMeta) != numGoroutines {
		t.Errorf("Expected %d nodes, got %d", numGoroutines, len(manager.DocMeta))
	}
}
