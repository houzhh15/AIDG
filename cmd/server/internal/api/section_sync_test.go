package api

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/taskdocs"
)

func TestSyncSectionsAfterSwitch_NewSection(t *testing.T) {
	tmpDir := t.TempDir()
	sectionsDir := filepath.Join(tmpDir, "sections")
	if err := os.MkdirAll(sectionsDir, 0755); err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}

	// 初始化sections.json
	meta := taskdocs.SectionMeta{
		Version:   1,
		UpdatedAt: time.Now(),
		RootLevel: 1,
		Sections:  []taskdocs.Section{},
		ETag:      "initial",
	}
	metaFile := filepath.Join(sectionsDir, "sections.json")
	if err := saveSectionMeta(metaFile, &meta); err != nil {
		t.Fatalf("保存初始metadata失败: %v", err)
	}

	// 创建新的section文件
	sectionFile := filepath.Join(sectionsDir, "section_001.md")
	content := `# 新章节标题

这是章节内容。`
	if err := os.WriteFile(sectionFile, []byte(content), 0644); err != nil {
		t.Fatalf("创建section文件失败: %v", err)
	}

	// 执行同步
	if err := SyncSectionsAfterSwitch(tmpDir); err != nil {
		t.Fatalf("同步失败: %v", err)
	}

	// 验证sections.json已更新
	newMeta, err := loadSectionMeta(metaFile)
	if err != nil {
		t.Fatalf("加载metadata失败: %v", err)
	}

	if len(newMeta.Sections) != 1 {
		t.Errorf("期望1个section，得到%d", len(newMeta.Sections))
	}

	if newMeta.Version != 2 {
		t.Errorf("期望version=2，得到%d", newMeta.Version)
	}

	if newMeta.Sections[0].Title != "新章节标题" {
		t.Errorf("标题不匹配: %s", newMeta.Sections[0].Title)
	}

	if newMeta.Sections[0].File != "section_001.md" {
		t.Errorf("文件名不匹配: %s", newMeta.Sections[0].File)
	}
}

func TestSyncSectionsAfterSwitch_DeleteSection(t *testing.T) {
	tmpDir := t.TempDir()
	sectionsDir := filepath.Join(tmpDir, "sections")
	if err := os.MkdirAll(sectionsDir, 0755); err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}

	// 初始化sections.json，包含两个section
	meta := taskdocs.SectionMeta{
		Version:   1,
		UpdatedAt: time.Now(),
		RootLevel: 1,
		Sections: []taskdocs.Section{
			{ID: "001", Title: "Section 1", File: "section_001.md", Level: 1, Order: 1},
			{ID: "002", Title: "Section 2", File: "section_002.md", Level: 1, Order: 2},
		},
		ETag: "initial",
	}
	metaFile := filepath.Join(sectionsDir, "sections.json")
	if err := saveSectionMeta(metaFile, &meta); err != nil {
		t.Fatalf("保存初始metadata失败: %v", err)
	}

	// 只创建section_001.md，不创建section_002.md（模拟删除）
	sectionFile := filepath.Join(sectionsDir, "section_001.md")
	content := `# Section 1

内容1`
	if err := os.WriteFile(sectionFile, []byte(content), 0644); err != nil {
		t.Fatalf("创建section文件失败: %v", err)
	}

	// 执行同步
	if err := SyncSectionsAfterSwitch(tmpDir); err != nil {
		t.Fatalf("同步失败: %v", err)
	}

	// 验证sections.json已更新，section_002已移除
	newMeta, err := loadSectionMeta(metaFile)
	if err != nil {
		t.Fatalf("加载metadata失败: %v", err)
	}

	if len(newMeta.Sections) != 1 {
		t.Errorf("期望1个section（section_002应被删除），得到%d", len(newMeta.Sections))
	}

	if newMeta.Version != 2 {
		t.Errorf("期望version=2，得到%d", newMeta.Version)
	}

	if newMeta.Sections[0].File != "section_001.md" {
		t.Errorf("剩余section应为section_001.md，得到%s", newMeta.Sections[0].File)
	}
}

func TestSyncSectionsAfterSwitch_EmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	sectionsDir := filepath.Join(tmpDir, "sections")
	if err := os.MkdirAll(sectionsDir, 0755); err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}

	// 初始化sections.json，包含一个section
	meta := taskdocs.SectionMeta{
		Version:   1,
		UpdatedAt: time.Now(),
		RootLevel: 1,
		Sections: []taskdocs.Section{
			{ID: "001", Title: "Section 1", File: "section_001.md", Level: 1, Order: 1},
		},
		ETag: "initial",
	}
	metaFile := filepath.Join(sectionsDir, "sections.json")
	if err := saveSectionMeta(metaFile, &meta); err != nil {
		t.Fatalf("保存初始metadata失败: %v", err)
	}

	// 不创建任何section文件（模拟空目录）

	// 执行同步
	if err := SyncSectionsAfterSwitch(tmpDir); err != nil {
		t.Fatalf("同步失败: %v", err)
	}

	// 验证sections.json已更新，所有section已移除
	newMeta, err := loadSectionMeta(metaFile)
	if err != nil {
		t.Fatalf("加载metadata失败: %v", err)
	}

	if len(newMeta.Sections) != 0 {
		t.Errorf("期望0个section（所有section应被删除），得到%d", len(newMeta.Sections))
	}

	if newMeta.Version != 2 {
		t.Errorf("期望version=2，得到%d", newMeta.Version)
	}
}

func TestSyncSectionsAfterSwitch_NoMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	sectionsDir := filepath.Join(tmpDir, "sections")
	if err := os.MkdirAll(sectionsDir, 0755); err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}

	// 不创建sections.json，只创建section文件
	sectionFile := filepath.Join(sectionsDir, "section_001.md")
	content := `# 新章节

内容`
	if err := os.WriteFile(sectionFile, []byte(content), 0644); err != nil {
		t.Fatalf("创建section文件失败: %v", err)
	}

	// 执行同步
	if err := SyncSectionsAfterSwitch(tmpDir); err != nil {
		t.Fatalf("同步失败: %v", err)
	}

	// 验证sections.json已创建
	metaFile := filepath.Join(sectionsDir, "sections.json")
	newMeta, err := loadSectionMeta(metaFile)
	if err != nil {
		t.Fatalf("加载metadata失败: %v", err)
	}

	if len(newMeta.Sections) != 1 {
		t.Errorf("期望1个section，得到%d", len(newMeta.Sections))
	}

	if newMeta.Version != 1 {
		t.Errorf("期望version=1（新创建），得到%d", newMeta.Version)
	}

	if newMeta.Sections[0].Title != "新章节" {
		t.Errorf("标题不匹配: %s", newMeta.Sections[0].Title)
	}
}

func TestSyncSectionsAfterSwitch_Concurrent(t *testing.T) {
	tmpDir := t.TempDir()
	sectionsDir := filepath.Join(tmpDir, "sections")
	if err := os.MkdirAll(sectionsDir, 0755); err != nil {
		t.Fatalf("创建测试目录失败: %v", err)
	}

	// 初始化sections.json
	meta := taskdocs.SectionMeta{
		Version:   1,
		UpdatedAt: time.Now(),
		RootLevel: 1,
		Sections:  []taskdocs.Section{},
		ETag:      "initial",
	}
	metaFile := filepath.Join(sectionsDir, "sections.json")
	if err := saveSectionMeta(metaFile, &meta); err != nil {
		t.Fatalf("保存初始metadata失败: %v", err)
	}

	// 创建10个section文件
	for i := 1; i <= 10; i++ {
		sectionFile := filepath.Join(sectionsDir, fmt.Sprintf("section_%03d.md", i))
		content := fmt.Sprintf("# Section %d\n\n内容 %d", i, i)
		if err := os.WriteFile(sectionFile, []byte(content), 0644); err != nil {
			t.Fatalf("创建section文件失败: %v", err)
		}
	}

	// 并发执行同步
	var wg sync.WaitGroup
	errChan := make(chan error, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := SyncSectionsAfterSwitch(tmpDir); err != nil {
				errChan <- err
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// 检查是否有错误
	for err := range errChan {
		t.Errorf("并发同步失败: %v", err)
	}

	// 验证sections.json状态一致
	newMeta, err := loadSectionMeta(metaFile)
	if err != nil {
		t.Fatalf("加载metadata失败: %v", err)
	}

	if len(newMeta.Sections) != 10 {
		t.Errorf("期望10个section，得到%d", len(newMeta.Sections))
	}

	// Version应该在2-6之间（初始1 + 5次并发同步，但可能有些goroutine看到相同状态）
	if newMeta.Version < 2 || newMeta.Version > 6 {
		t.Errorf("期望version在2-6之间，得到%d", newMeta.Version)
	}
}

// 辅助函数：保存section metadata
func saveSectionMeta(filePath string, meta *taskdocs.SectionMeta) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

// 辅助函数：加载section metadata
func loadSectionMeta(filePath string) (*taskdocs.SectionMeta, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var meta taskdocs.SectionMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, err
	}

	return &meta, nil
}
