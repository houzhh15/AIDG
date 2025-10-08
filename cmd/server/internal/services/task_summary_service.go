package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/houzhh15-hub/AIDG/cmd/server/internal/models"
	"github.com/houzhh15-hub/AIDG/cmd/server/internal/util"
)

// 错误定义
var (
	ErrSummaryNotFound = errors.New("SUMMARY_NOT_FOUND")
	ErrContentEmpty    = errors.New("CONTENT_EMPTY")
	ErrInvalidTaskID   = errors.New("INVALID_TASK_ID")
)

// TaskSummaryService 任务总结服务接口
type TaskSummaryService interface {
	// AddSummary 添加任务总结
	AddSummary(ctx context.Context, projectID, taskID, creator string, time time.Time, content string) (*models.TaskSummary, error)

	// UpdateSummary 更新任务总结
	UpdateSummary(ctx context.Context, projectID, taskID, summaryID string, update *models.TaskSummaryUpdate) error

	// DeleteSummary 删除任务总结
	DeleteSummary(ctx context.Context, projectID, taskID, summaryID string) error

	// GetSummaries 获取任务的所有总结
	GetSummaries(ctx context.Context, projectID, taskID string) ([]*models.TaskSummary, error)

	// GetSummariesByWeekRange 按周范围获取任务总结
	GetSummariesByWeekRange(ctx context.Context, projectID, taskID, startWeek, endWeek string) ([]*models.TaskSummary, error)

	// GetSummariesAcrossTasksByWeek 跨任务按周范围检索总结
	GetSummariesAcrossTasksByWeek(ctx context.Context, projectID, startWeek, endWeek string, taskIDs []string) ([]*models.TaskSummary, error)
}

// taskSummaryService 任务总结服务实现
type taskSummaryService struct {
	mu             sync.RWMutex
	dataRoot       string // 数据根目录
	versionEnabled bool   // 是否启用版本管理
}

// NewTaskSummaryService 创建任务总结服务实例
func NewTaskSummaryService(dataRoot string) TaskSummaryService {
	return &taskSummaryService{
		dataRoot:       dataRoot,
		versionEnabled: true,
	}
}

// AddSummary 添加任务总结
func (s *taskSummaryService) AddSummary(ctx context.Context, projectID, taskID, creator string, time time.Time, content string) (*models.TaskSummary, error) {
	if content == "" {
		return nil, ErrContentEmpty
	}
	if taskID == "" {
		return nil, ErrInvalidTaskID
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 读取现有总结
	summaries, err := s.loadSummariesUnsafe(projectID, taskID)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load summaries: %w", err)
	}

	// 如果文件不存在，初始化
	if summaries == nil {
		summaries = &models.TaskSummaries{
			Version:   0,
			UpdatedAt: time,
			Summaries: []*models.TaskSummary{},
		}
	}

	// 创建新总结
	summary := &models.TaskSummary{
		ID:         fmt.Sprintf("summary_%d", time.Unix()),
		Time:       time,
		WeekNumber: util.GetWeekNumber(time),
		Content:    content,
		Creator:    creator,
		CreatedAt:  time,
		UpdatedAt:  time,
	}

	// 添加到列表
	summaries.Summaries = append(summaries.Summaries, summary)
	summaries.Version++
	summaries.UpdatedAt = time

	// 按时间倒序排序
	sort.Slice(summaries.Summaries, func(i, j int) bool {
		return summaries.Summaries[i].Time.After(summaries.Summaries[j].Time)
	})

	// 保存
	if err := s.saveSummariesUnsafe(projectID, taskID, summaries); err != nil {
		return nil, fmt.Errorf("save summaries: %w", err)
	}

	return summary, nil
}

// UpdateSummary 更新任务总结
func (s *taskSummaryService) UpdateSummary(ctx context.Context, projectID, taskID, summaryID string, update *models.TaskSummaryUpdate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 读取现有总结
	summaries, err := s.loadSummariesUnsafe(projectID, taskID)
	if err != nil {
		return fmt.Errorf("load summaries: %w", err)
	}

	// 查找要更新的总结
	found := false
	for _, summary := range summaries.Summaries {
		if summary.ID == summaryID {
			if update.Time != nil {
				summary.Time = *update.Time
				summary.WeekNumber = util.GetWeekNumber(*update.Time)
			}
			if update.Content != nil {
				if *update.Content == "" {
					return ErrContentEmpty
				}
				summary.Content = *update.Content
			}
			summary.UpdatedAt = time.Now()
			found = true
			break
		}
	}

	if !found {
		return ErrSummaryNotFound
	}

	// 重新排序
	sort.Slice(summaries.Summaries, func(i, j int) bool {
		return summaries.Summaries[i].Time.After(summaries.Summaries[j].Time)
	})

	// 更新版本
	summaries.Version++
	summaries.UpdatedAt = time.Now()

	// 保存
	if err := s.saveSummariesUnsafe(projectID, taskID, summaries); err != nil {
		return fmt.Errorf("save summaries: %w", err)
	}

	return nil
}

// DeleteSummary 删除任务总结
func (s *taskSummaryService) DeleteSummary(ctx context.Context, projectID, taskID, summaryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 读取现有总结
	summaries, err := s.loadSummariesUnsafe(projectID, taskID)
	if err != nil {
		return fmt.Errorf("load summaries: %w", err)
	}

	// 查找并删除
	found := false
	newSummaries := make([]*models.TaskSummary, 0, len(summaries.Summaries))
	for _, summary := range summaries.Summaries {
		if summary.ID != summaryID {
			newSummaries = append(newSummaries, summary)
		} else {
			found = true
		}
	}

	if !found {
		return ErrSummaryNotFound
	}

	summaries.Summaries = newSummaries
	summaries.Version++
	summaries.UpdatedAt = time.Now()

	// 保存
	if err := s.saveSummariesUnsafe(projectID, taskID, summaries); err != nil {
		return fmt.Errorf("save summaries: %w", err)
	}

	return nil
}

// GetSummaries 获取任务的所有总结
func (s *taskSummaryService) GetSummaries(ctx context.Context, projectID, taskID string) ([]*models.TaskSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summaries, err := s.loadSummariesUnsafe(projectID, taskID)
	if err != nil {
		if os.IsNotExist(err) {
			return []*models.TaskSummary{}, nil
		}
		return nil, fmt.Errorf("load summaries: %w", err)
	}

	return summaries.Summaries, nil
}

// GetSummariesByWeekRange 按周范围获取任务总结
func (s *taskSummaryService) GetSummariesByWeekRange(ctx context.Context, projectID, taskID, startWeek, endWeek string) ([]*models.TaskSummary, error) {
	allSummaries, err := s.GetSummaries(ctx, projectID, taskID)
	if err != nil {
		return nil, err
	}

	// 过滤周范围
	result := make([]*models.TaskSummary, 0)
	for _, summary := range allSummaries {
		if util.IsWeekInRange(summary.WeekNumber, startWeek, endWeek) {
			result = append(result, summary)
		}
	}

	return result, nil
}

// GetSummariesAcrossTasksByWeek 跨任务按周范围检索总结
func (s *taskSummaryService) GetSummariesAcrossTasksByWeek(ctx context.Context, projectID, startWeek, endWeek string, taskIDs []string) ([]*models.TaskSummary, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	results := make([]*models.TaskSummary, 0)

	// 并发查询每个任务
	for _, taskID := range taskIDs {
		wg.Add(1)
		go func(tid string) {
			defer wg.Done()

			summaries, err := s.GetSummariesByWeekRange(ctx, projectID, tid, startWeek, endWeek)
			if err != nil {
				// 记录错误但不中断其他任务的查询
				fmt.Fprintf(os.Stderr, "Failed to get summaries for task %s: %v\n", tid, err)
				return
			}

			mu.Lock()
			results = append(results, summaries...)
			mu.Unlock()
		}(taskID)
	}

	wg.Wait()

	// 按时间倒序排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Time.After(results[j].Time)
	})

	return results, nil
}

// loadSummariesUnsafe 加载任务总结（不加锁，内部使用）
func (s *taskSummaryService) loadSummariesUnsafe(projectID, taskID string) (*models.TaskSummaries, error) {
	filePath := s.getSummariesPath(projectID, taskID)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var summaries models.TaskSummaries
	if err := json.Unmarshal(data, &summaries); err != nil {
		return nil, fmt.Errorf("unmarshal summaries: %w", err)
	}

	return &summaries, nil
}

// saveSummariesUnsafe 保存任务总结（不加锁，内部使用）
func (s *taskSummaryService) saveSummariesUnsafe(projectID, taskID string, summaries *models.TaskSummaries) error {
	filePath := s.getSummariesPath(projectID, taskID)

	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create directory: %w", err)
	}

	// 序列化为JSON
	data, err := json.MarshalIndent(summaries, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal summaries: %w", err)
	}

	// 写入文件
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	// 创建版本快照
	if s.versionEnabled {
		if err := s.createVersionSnapshotUnsafe(projectID, taskID, data); err != nil {
			// 版本快照失败不应中断主流程
			fmt.Fprintf(os.Stderr, "Failed to create version snapshot: %v\n", err)
		}
	}

	return nil
}

// getSummariesPath 获取总结文件路径
func (s *taskSummaryService) getSummariesPath(projectID, taskID string) string {
	return filepath.Join(s.dataRoot, "projects", projectID, "tasks", taskID, "summaries.json")
}

// createVersionSnapshotUnsafe 创建版本快照（不加锁，内部使用）
func (s *taskSummaryService) createVersionSnapshotUnsafe(projectID, taskID string, data []byte) error {
	versionDir := filepath.Join(s.dataRoot, "projects", projectID, "tasks", taskID, ".versions")
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		return err
	}

	timestamp := time.Now().Format("20060102_150405")
	versionFile := filepath.Join(versionDir, fmt.Sprintf("summaries_%s.json", timestamp))

	return os.WriteFile(versionFile, data, 0644)
}
