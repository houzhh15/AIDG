// Package docslot provides migration utilities for converting legacy document storage
// formats to the new unified chunk-based format.
//
// Migration Flow:
// 1. Scan legacy document directories for existing files
// 2. For each legacy file found, create corresponding docs/{slot}/ directory structure
// 3. Import content as initial chunk with proper metadata
// 4. Optionally archive or remove legacy files after successful migration
//
// Supported Migrations:
// - Project: docs/feature_list.md → docs/feature_list/
// - Project: docs/architecture_design.md → docs/architecture_design/
// - Meeting: feature_list.md → docs/feature_list/
// - Meeting: architecture_new.md → docs/architecture_design/
// - Meeting: polish_all.md → docs/polish/
// - Meeting: meeting_summary.md → docs/summary/
// - Meeting: topic.md → docs/topic/
package docslot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/taskdocs"
)

// MigrationResult 迁移结果
type MigrationResult struct {
	Scope     DocumentScope `json:"scope"`
	ScopeID   string        `json:"scope_id"`
	SlotKey   string        `json:"slot_key"`
	Migrated  bool          `json:"migrated"`
	Source    string        `json:"source"`
	Target    string        `json:"target"`
	Error     string        `json:"error,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
}

// MigrationSummary 迁移汇总
type MigrationSummary struct {
	TotalScanned  int                `json:"total_scanned"`
	TotalMigrated int                `json:"total_migrated"`
	TotalSkipped  int                `json:"total_skipped"`
	TotalFailed   int                `json:"total_failed"`
	Results       []*MigrationResult `json:"results"`
	StartTime     time.Time          `json:"start_time"`
	EndTime       time.Time          `json:"end_time"`
}

// MigrationConfig 迁移配置
type MigrationConfig struct {
	DryRun        bool   // 是否仅预览，不实际迁移
	ArchiveLegacy bool   // 是否归档旧文件（重命名为 .legacy）
	RemoveLegacy  bool   // 是否删除旧文件（ArchiveLegacy 优先）
	DefaultUser   string // 默认用户名
	DefaultSource string // 默认来源标识
}

// DefaultMigrationConfig 默认迁移配置
func DefaultMigrationConfig() MigrationConfig {
	return MigrationConfig{
		DryRun:        false,
		ArchiveLegacy: true,
		RemoveLegacy:  false,
		DefaultUser:   "migration",
		DefaultSource: "legacy_migration",
	}
}

// Migrator 迁移器
type Migrator struct {
	basePath   string
	docService *taskdocs.DocService
	config     MigrationConfig
}

// NewMigrator 创建迁移器
func NewMigrator(basePath string, config MigrationConfig) *Migrator {
	return &Migrator{
		basePath:   basePath,
		docService: taskdocs.NewDocService(),
		config:     config,
	}
}

// legacyMapping 定义旧文件到新槽位的映射
type legacyMapping struct {
	LegacyPath string        // 相对路径
	SlotKey    string        // 新槽位名称
	Scope      DocumentScope // 作用域
	GlobPath   bool          // 是否使用 glob 匹配
}

// projectLegacyMappings 项目文档迁移映射
// 优先检查根目录的旧文件，如果不存在再检查 docs/ 目录
var projectLegacyMappings = []legacyMapping{
	// 根目录旧文件（优先）
	{LegacyPath: "feature_list.md", SlotKey: "feature_list", Scope: ScopeProject},
	{LegacyPath: "architecture_new.md", SlotKey: "architecture_design", Scope: ScopeProject},
	{LegacyPath: "tech_design_v1.md", SlotKey: "tech_design", Scope: ScopeProject},
	// docs/ 目录旧文件（备选）
	{LegacyPath: "docs/feature_list.md", SlotKey: "feature_list", Scope: ScopeProject},
	{LegacyPath: "docs/architecture_design.md", SlotKey: "architecture_design", Scope: ScopeProject},
	{LegacyPath: "docs/tech_design.md", SlotKey: "tech_design", Scope: ScopeProject},
}

// meetingLegacyMappings 会议文档迁移映射
var meetingLegacyMappings = []legacyMapping{
	{LegacyPath: "feature_list.md", SlotKey: "feature_list", Scope: ScopeMeeting},
	{LegacyPath: "architecture_new.md", SlotKey: "architecture_design", Scope: ScopeMeeting},
	{LegacyPath: "polish_all.md", SlotKey: "polish", Scope: ScopeMeeting},
	{LegacyPath: "meeting_summary.md", SlotKey: "summary", Scope: ScopeMeeting},
	{LegacyPath: "topic.md", SlotKey: "topic", Scope: ScopeMeeting},
}

// MigrateProject 迁移单个项目的文档
func (m *Migrator) MigrateProject(projectID string) ([]*MigrationResult, error) {
	projectDir := filepath.Join(m.basePath, projectID)
	if _, err := os.Stat(projectDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("project directory not found: %s", projectID)
	}

	var results []*MigrationResult

	for _, mapping := range projectLegacyMappings {
		result := m.migrateFile(projectDir, projectID, mapping)
		results = append(results, result)
	}

	return results, nil
}

// MigrateMeeting 迁移单个会议的文档
func (m *Migrator) MigrateMeeting(meetingDir string, meetingID string) ([]*MigrationResult, error) {
	if _, err := os.Stat(meetingDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("meeting directory not found: %s", meetingDir)
	}

	var results []*MigrationResult

	for _, mapping := range meetingLegacyMappings {
		result := m.migrateFile(meetingDir, meetingID, mapping)
		results = append(results, result)
	}

	return results, nil
}

// migrateFile 迁移单个文件
func (m *Migrator) migrateFile(baseDir, scopeID string, mapping legacyMapping) *MigrationResult {
	result := &MigrationResult{
		Scope:     mapping.Scope,
		ScopeID:   scopeID,
		SlotKey:   mapping.SlotKey,
		Timestamp: time.Now(),
	}

	legacyPath := filepath.Join(baseDir, mapping.LegacyPath)
	result.Source = legacyPath

	// 检查旧文件是否存在
	content, err := os.ReadFile(legacyPath)
	if os.IsNotExist(err) {
		result.Migrated = false
		result.Error = "legacy file not found (skipped)"
		return result
	}
	if err != nil {
		result.Migrated = false
		result.Error = fmt.Sprintf("failed to read legacy file: %v", err)
		return result
	}

	// 计算新的目标路径
	var targetDir string
	if mapping.Scope == ScopeProject {
		targetDir = filepath.Join(baseDir, "docs", mapping.SlotKey)
	} else {
		targetDir = filepath.Join(baseDir, "docs", mapping.SlotKey)
	}
	result.Target = targetDir

	// 检查目标是否已存在
	chunksPath := filepath.Join(targetDir, "chunks.jsonl")
	if _, err := os.Stat(chunksPath); err == nil {
		result.Migrated = false
		result.Error = "target already migrated (skipped)"
		return result
	}

	// 干运行模式
	if m.config.DryRun {
		result.Migrated = false
		result.Error = "dry run - would migrate"
		return result
	}

	// 创建目标目录
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		result.Migrated = false
		result.Error = fmt.Sprintf("failed to create target directory: %v", err)
		return result
	}

	// 导入内容作为初始 chunk
	if err := m.importAsChunk(targetDir, string(content)); err != nil {
		result.Migrated = false
		result.Error = fmt.Sprintf("failed to import content: %v", err)
		return result
	}

	// 处理旧文件
	if m.config.ArchiveLegacy {
		archivePath := legacyPath + ".legacy"
		if err := os.Rename(legacyPath, archivePath); err != nil {
			// 不视为失败，只记录
			result.Error = fmt.Sprintf("migrated but failed to archive: %v", err)
		}
	} else if m.config.RemoveLegacy {
		if err := os.Remove(legacyPath); err != nil {
			result.Error = fmt.Sprintf("migrated but failed to remove: %v", err)
		}
	}

	result.Migrated = true
	return result
}

// importAsChunk 将内容导入为初始 chunk
func (m *Migrator) importAsChunk(targetDir, content string) error {
	// 创建初始 chunk
	chunk := taskdocs.DocChunk{
		Sequence:  1,
		Timestamp: time.Now(),
		Op:        "replace",
		Content:   content,
		User:      m.config.DefaultUser,
		Source:    m.config.DefaultSource,
		Hash:      generateHash(content),
		Active:    true,
	}

	// 写入 chunks.jsonl
	chunksPath := filepath.Join(targetDir, "chunks.jsonl")
	f, err := os.Create(chunksPath)
	if err != nil {
		return fmt.Errorf("failed to create chunks file: %w", err)
	}
	defer f.Close()

	encoder := json.NewEncoder(f)
	if err := encoder.Encode(chunk); err != nil {
		return fmt.Errorf("failed to write chunk: %w", err)
	}

	// 创建 meta.json
	meta := taskdocs.DocMeta{
		Version:      1,
		LastSequence: 1,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		DocType:      "legacy_migration",
		ChunkCount:   1,
		DeletedCount: 0,
		ETag:         generateETag(content),
	}

	metaPath := filepath.Join(targetDir, "meta.json")
	metaData, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal meta: %w", err)
	}
	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return fmt.Errorf("failed to write meta: %w", err)
	}

	// 创建 compiled.md
	compiledPath := filepath.Join(targetDir, "compiled.md")
	if err := os.WriteFile(compiledPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write compiled: %w", err)
	}

	return nil
}

// MigrateAllProjects 迁移所有项目
func (m *Migrator) MigrateAllProjects() (*MigrationSummary, error) {
	summary := &MigrationSummary{
		StartTime: time.Now(),
	}

	entries, err := os.ReadDir(m.basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read projects directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectID := entry.Name()
		results, err := m.MigrateProject(projectID)
		if err != nil {
			continue // 跳过无效目录
		}

		for _, r := range results {
			summary.TotalScanned++
			if r.Migrated {
				summary.TotalMigrated++
			} else if r.Error == "" || r.Error == "legacy file not found (skipped)" || r.Error == "target already migrated (skipped)" {
				summary.TotalSkipped++
			} else {
				summary.TotalFailed++
			}
			summary.Results = append(summary.Results, r)
		}
	}

	summary.EndTime = time.Now()
	return summary, nil
}

// generateChunkID 生成 chunk ID
func generateChunkID() string {
	return fmt.Sprintf("chunk_%d", time.Now().UnixNano())
}

// generateHash 生成内容哈希
func generateHash(content string) string {
	// 简单实现：使用内容长度哈希
	return fmt.Sprintf("hash_%x", len(content))
}

// generateETag 生成 ETag
func generateETag(content string) string {
	// 简单实现：使用内容长度和时间戳
	return fmt.Sprintf("%d-%d", len(content), time.Now().Unix())
}

// CheckMigrationStatus 检查迁移状态
func (m *Migrator) CheckMigrationStatus(scope DocumentScope, scopeID, slotKey string) (bool, error) {
	var basePath string
	switch scope {
	case ScopeProject:
		basePath = filepath.Join(m.basePath, scopeID, "docs", slotKey)
	case ScopeMeeting:
		// 会议需要知道完整的输出目录
		return false, fmt.Errorf("meeting migration status check requires full path")
	default:
		return false, fmt.Errorf("invalid scope: %s", scope)
	}

	chunksPath := filepath.Join(basePath, "chunks.jsonl")
	_, err := os.Stat(chunksPath)
	return err == nil, nil
}
