package documents

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// SnapshotManager 快照管理器
type SnapshotManager struct {
	baseDir string // 项目文档目录
}

// SnapshotMeta 快照元数据
type SnapshotMeta struct {
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
}

// NewSnapshotManager 创建快照管理器
func NewSnapshotManager(baseDir string) *SnapshotManager {
	return &SnapshotManager{
		baseDir: baseDir,
	}
}

// CreateSnapshot 创建文档快照
func (sm *SnapshotManager) CreateSnapshot(nodeID string, version int, content string) error {
	// 创建快照目录结构 .history/{node_id}/
	historyDir := filepath.Join(sm.baseDir, ".history", nodeID)
	if err := os.MkdirAll(historyDir, 0755); err != nil {
		return fmt.Errorf("failed to create history directory: %w", err)
	}

	// 写入快照文件 {version}.md
	snapshotPath := filepath.Join(historyDir, fmt.Sprintf("%d.md", version))
	if err := os.WriteFile(snapshotPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write snapshot file: %w", err)
	}

	// 更新快照索引 snapshots.json
	indexPath := filepath.Join(historyDir, "snapshots.json")
	snapshots, err := sm.loadSnapshotIndex(indexPath)
	if err != nil {
		// 首次创建索引
		snapshots = []SnapshotMeta{}
	}

	// 添加新快照到索引
	stat, _ := os.Stat(snapshotPath)
	newSnapshot := SnapshotMeta{
		Version:   version,
		CreatedAt: time.Now(),
		Path:      snapshotPath,
		Size:      stat.Size(),
	}

	// 检查是否已存在该版本
	found := false
	for i, snap := range snapshots {
		if snap.Version == version {
			snapshots[i] = newSnapshot
			found = true
			break
		}
	}
	if !found {
		snapshots = append(snapshots, newSnapshot)
	}

	// 按版本排序
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].Version > snapshots[j].Version
	})

	// 写入索引文件
	return sm.saveSnapshotIndex(indexPath, snapshots)
}

// ListSnapshots 列出文档的快照历史
func (sm *SnapshotManager) ListSnapshots(nodeID string, limit int) ([]SnapshotMeta, error) {
	historyDir := filepath.Join(sm.baseDir, ".history", nodeID)
	indexPath := filepath.Join(historyDir, "snapshots.json")

	snapshots, err := sm.loadSnapshotIndex(indexPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []SnapshotMeta{}, nil
		}
		return nil, fmt.Errorf("failed to load snapshot index: %w", err)
	}

	// 应用限制
	if limit > 0 && len(snapshots) > limit {
		snapshots = snapshots[:limit]
	}

	return snapshots, nil
}

// GetSnapshot 获取特定版本的快照内容
func (sm *SnapshotManager) GetSnapshot(nodeID string, version int) (string, error) {
	historyDir := filepath.Join(sm.baseDir, ".history", nodeID)
	snapshotPath := filepath.Join(historyDir, fmt.Sprintf("%d.md", version))

	content, err := os.ReadFile(snapshotPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("snapshot version %d not found for node %s", version, nodeID)
		}
		return "", fmt.Errorf("failed to read snapshot: %w", err)
	}

	return string(content), nil
}

// CleanupSnapshots 清理旧快照（保留最新N个版本）
func (sm *SnapshotManager) CleanupSnapshots(nodeID string, keepCount int) error {
	historyDir := filepath.Join(sm.baseDir, ".history", nodeID)
	indexPath := filepath.Join(historyDir, "snapshots.json")

	snapshots, err := sm.loadSnapshotIndex(indexPath)
	if err != nil {
		return err
	}

	if len(snapshots) <= keepCount {
		return nil // 无需清理
	}

	// 删除多余的快照文件
	toDelete := snapshots[keepCount:]
	for _, snap := range toDelete {
		if err := os.Remove(snap.Path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete snapshot file %s: %w", snap.Path, err)
		}
	}

	// 更新索引
	snapshots = snapshots[:keepCount]
	return sm.saveSnapshotIndex(indexPath, snapshots)
}

// loadSnapshotIndex 加载快照索引
func (sm *SnapshotManager) loadSnapshotIndex(indexPath string) ([]SnapshotMeta, error) {
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return nil, err
	}

	var snapshots []SnapshotMeta
	if err := json.Unmarshal(data, &snapshots); err != nil {
		return nil, fmt.Errorf("failed to parse snapshot index: %w", err)
	}

	return snapshots, nil
}

// saveSnapshotIndex 保存快照索引
func (sm *SnapshotManager) saveSnapshotIndex(indexPath string, snapshots []SnapshotMeta) error {
	data, err := json.MarshalIndent(snapshots, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot index: %w", err)
	}

	// 原子写入
	tempPath := indexPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write temp snapshot index: %w", err)
	}

	if err := os.Rename(tempPath, indexPath); err != nil {
		os.Remove(tempPath) // 清理临时文件
		return fmt.Errorf("failed to rename snapshot index: %w", err)
	}

	return nil
}
