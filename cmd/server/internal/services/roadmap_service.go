package services

import (
	"encoding/json"
	"fmt"
	"github.com/houzhh15/AIDG/cmd/server/internal/models"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// RoadmapService Roadmap管理服务
type RoadmapService struct {
	baseDir string
	mu      sync.RWMutex
}

// NewRoadmapService 创建Roadmap服务实例
func NewRoadmapService(baseDir string) *RoadmapService {
	return &RoadmapService{
		baseDir: baseDir,
	}
}

// getRoadmapPath 获取roadmap.json文件路径
func (s *RoadmapService) getRoadmapPath(projectID string) string {
	return filepath.Join(s.baseDir, "projects", projectID, "roadmap.json")
}

// GetRoadmap 获取项目Roadmap（对外接口，带锁）
func (s *RoadmapService) GetRoadmap(projectID string) (*models.Roadmap, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.getRoadmapUnsafe(projectID)
}

// getRoadmapUnsafe 获取项目Roadmap（内部方法，不加锁）
// 警告：调用此方法前必须已持有锁
func (s *RoadmapService) getRoadmapUnsafe(projectID string) (*models.Roadmap, error) {
	path := s.getRoadmapPath(projectID)

	// 检查文件是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// 返回空的Roadmap
		return &models.Roadmap{
			Version:   0,
			UpdatedAt: time.Now(),
			Nodes:     []*models.RoadmapNode{},
		}, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取roadmap文件失败: %w", err)
	}

	var roadmap models.Roadmap
	if err := json.Unmarshal(data, &roadmap); err != nil {
		return nil, fmt.Errorf("解析roadmap文件失败: %w", err)
	}

	return &roadmap, nil
}

// saveRoadmap 保存Roadmap到文件
func (s *RoadmapService) saveRoadmap(projectID string, roadmap *models.Roadmap) error {
	path := s.getRoadmapPath(projectID)

	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 序列化为JSON
	data, err := json.MarshalIndent(roadmap, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化roadmap失败: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入roadmap文件失败: %w", err)
	}

	return nil
}

// AddNode 添加Roadmap节点
func (s *RoadmapService) AddNode(projectID string, nodeCreate *models.RoadmapNodeCreate) (*models.RoadmapNode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 验证输入
	if err := validateNodeCreate(nodeCreate); err != nil {
		return nil, err
	}

	// 读取当前Roadmap（使用不加锁的内部方法）
	roadmap, err := s.getRoadmapUnsafe(projectID)
	if err != nil {
		return nil, err
	}

	// 创建新节点
	node := &models.RoadmapNode{
		ID:          fmt.Sprintf("node_%d", time.Now().Unix()),
		Date:        nodeCreate.Date,
		Goal:        nodeCreate.Goal,
		Description: nodeCreate.Description,
		Status:      nodeCreate.Status,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 添加节点
	roadmap.Nodes = append(roadmap.Nodes, node)
	roadmap.Version++
	roadmap.UpdatedAt = time.Now()

	// 保存
	if err := s.saveRoadmap(projectID, roadmap); err != nil {
		return nil, err
	}

	return node, nil
}

// UpdateNode 更新Roadmap节点(支持乐观锁)
func (s *RoadmapService) UpdateNode(projectID, nodeID string, nodeUpdate *models.RoadmapNodeUpdate, expectedVersion int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 读取当前Roadmap（使用不加锁的内部方法）
	roadmap, err := s.getRoadmapUnsafe(projectID)
	if err != nil {
		return err
	}

	// 版本检查（乐观锁）
	if roadmap.Version != expectedVersion {
		return fmt.Errorf("版本冲突: 期望版本 %d, 实际版本 %d", expectedVersion, roadmap.Version)
	}

	// 查找节点
	found := false
	for i, node := range roadmap.Nodes {
		if node.ID == nodeID {
			// 更新字段
			if nodeUpdate.Date != nil {
				node.Date = *nodeUpdate.Date
			}
			if nodeUpdate.Goal != nil {
				node.Goal = *nodeUpdate.Goal
			}
			if nodeUpdate.Description != nil {
				node.Description = *nodeUpdate.Description
			}
			if nodeUpdate.Status != nil {
				if err := validateStatus(*nodeUpdate.Status); err != nil {
					return err
				}
				node.Status = *nodeUpdate.Status
			}
			node.UpdatedAt = time.Now()
			roadmap.Nodes[i] = node
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("节点不存在: %s", nodeID)
	}

	// 更新版本号
	roadmap.Version++
	roadmap.UpdatedAt = time.Now()

	// 保存
	if err := s.saveRoadmap(projectID, roadmap); err != nil {
		return err
	}

	return nil
}

// DeleteNode 删除Roadmap节点
func (s *RoadmapService) DeleteNode(projectID, nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 读取当前Roadmap（使用不加锁的内部方法）
	roadmap, err := s.getRoadmapUnsafe(projectID)
	if err != nil {
		return err
	}

	// 查找并删除节点
	found := false
	newNodes := make([]*models.RoadmapNode, 0, len(roadmap.Nodes))
	for _, node := range roadmap.Nodes {
		if node.ID == nodeID {
			found = true
			continue
		}
		newNodes = append(newNodes, node)
	}

	if !found {
		return fmt.Errorf("节点不存在: %s", nodeID)
	}

	roadmap.Nodes = newNodes
	roadmap.Version++
	roadmap.UpdatedAt = time.Now()

	// 保存
	if err := s.saveRoadmap(projectID, roadmap); err != nil {
		return err
	}

	return nil
}

// validateNodeCreate 验证创建节点的输入
func validateNodeCreate(node *models.RoadmapNodeCreate) error {
	if node.Goal == "" {
		return fmt.Errorf("目标不能为空")
	}
	if len(node.Goal) > 50 {
		return fmt.Errorf("目标长度不能超过50字")
	}
	if len(node.Description) > 500 {
		return fmt.Errorf("描述长度不能超过500字")
	}

	// 验证日期格式
	if _, err := time.Parse("2006-01-02", node.Date); err != nil {
		return fmt.Errorf("日期格式错误，应为YYYY-MM-DD: %w", err)
	}

	// 验证状态值
	if err := validateStatus(node.Status); err != nil {
		return err
	}

	return nil
}

// validateStatus 验证状态值
func validateStatus(status string) error {
	validStatuses := map[string]bool{
		"completed":   true,
		"in-progress": true,
		"todo":        true,
	}

	if !validStatuses[status] {
		return fmt.Errorf("无效的状态值: %s，应为 completed, in-progress 或 todo", status)
	}

	return nil
}
