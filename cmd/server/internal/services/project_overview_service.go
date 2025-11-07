package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/projects"
)

// ProjectOverviewService 项目概述服务接口
type ProjectOverviewService interface {
	// GetProjectOverview 获取项目概述（基本信息 + 统计指标）
	GetProjectOverview(ctx context.Context, projectID string, projectReg *projects.ProjectRegistry) (*ProjectOverview, error)

	// UpdateProjectMetadata 更新项目元数据
	UpdateProjectMetadata(ctx context.Context, projectID string, metadata map[string]interface{}) error
}

// ProjectOverview 项目概述数据
type ProjectOverview struct {
	BasicInfo BasicInfo      `json:"basic_info"`
	Metrics   ProjectMetrics `json:"metrics"`
}

// BasicInfo 项目基本信息
type BasicInfo struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ProductLine string    `json:"product_line"`
	Description string    `json:"description,omitempty"`
	Owner       string    `json:"owner,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// projectOverviewService 项目概述服务实现
type projectOverviewService struct {
	mu           sync.RWMutex
	dataRoot     string
	statsService StatisticsService
}

// NewProjectOverviewService 创建项目概述服务实例
func NewProjectOverviewService(dataRoot string, statsService StatisticsService) ProjectOverviewService {
	return &projectOverviewService{
		dataRoot:     dataRoot,
		statsService: statsService,
	}
}

// GetProjectOverview 获取项目概述
func (s *projectOverviewService) GetProjectOverview(ctx context.Context, projectID string, projectReg *projects.ProjectRegistry) (*ProjectOverview, error) {
	// 获取项目基本信息
	project := projectReg.Get(projectID)
	if project == nil {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}

	basicInfo := BasicInfo{
		ID:          project.ID,
		Name:        project.Name,
		ProductLine: project.ProductLine,
		CreatedAt:   project.CreatedAt,
		UpdatedAt:   project.UpdatedAt,
	}

	// 读取扩展元数据
	metadata, err := s.loadMetadata(projectID)
	if err == nil {
		if desc, ok := metadata["description"].(string); ok {
			basicInfo.Description = desc
		}
		if owner, ok := metadata["owner"].(string); ok {
			basicInfo.Owner = owner
		}
	}

	// 获取统计指标
	metrics, err := s.statsService.CalculateProjectMetrics(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("calculate metrics: %w", err)
	}

	overview := &ProjectOverview{
		BasicInfo: basicInfo,
		Metrics:   *metrics,
	}

	return overview, nil
}

// UpdateProjectMetadata 更新项目元数据
func (s *projectOverviewService) UpdateProjectMetadata(ctx context.Context, projectID string, metadata map[string]interface{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 读取现有元数据
	existing, err := s.loadMetadataUnsafe(projectID)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load existing metadata: %w", err)
	}

	// 如果文件不存在，初始化
	if existing == nil {
		existing = make(map[string]interface{})
	}

	// 更新字段
	for key, value := range metadata {
		// 只允许更新特定字段
		switch key {
		case "description", "owner", "start_date", "estimated_end_date":
			existing[key] = value
		default:
			// 忽略不允许更新的字段
			continue
		}
	}

	// 更新时间戳
	existing["updated_at"] = time.Now().Format(time.RFC3339)

	// 保存元数据
	if err := s.saveMetadataUnsafe(projectID, existing); err != nil {
		return fmt.Errorf("save metadata: %w", err)
	}

	return nil
}

// loadMetadata 加载项目元数据
func (s *projectOverviewService) loadMetadata(projectID string) (map[string]interface{}, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loadMetadataUnsafe(projectID)
}

// loadMetadataUnsafe 加载项目元数据（不加锁，内部使用）
func (s *projectOverviewService) loadMetadataUnsafe(projectID string) (map[string]interface{}, error) {
	metadataPath := s.getMetadataPath(projectID)

	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return nil, err
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}

	return metadata, nil
}

// saveMetadataUnsafe 保存项目元数据（不加锁，内部使用）
func (s *projectOverviewService) saveMetadataUnsafe(projectID string, metadata map[string]interface{}) error {
	metadataPath := s.getMetadataPath(projectID)

	// 确保目录存在
	dir := filepath.Dir(metadataPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// 序列化为JSON
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

// getMetadataPath 获取元数据文件路径
func (s *projectOverviewService) getMetadataPath(projectID string) string {
	return filepath.Join(s.dataRoot, "projects", projectID, "metadata.json")
}
