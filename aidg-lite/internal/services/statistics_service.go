package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ProjectMetrics 项目核心指标
type ProjectMetrics struct {
	TotalTasks       int     `json:"total_tasks"`
	CompletedTasks   int     `json:"completed_tasks"`
	InProgressTasks  int     `json:"in_progress_tasks"`
	TodoTasks        int     `json:"todo_tasks"`
	CompletionRate   float64 `json:"completion_rate"`
	StartDate        string  `json:"start_date,omitempty"`
	EstimatedEndDate string  `json:"estimated_end_date,omitempty"`
}

// TaskDistribution 任务状态分布
type TaskDistribution struct {
	Total        int                `json:"total"`
	Completed    int                `json:"completed"`
	InProgress   int                `json:"in_progress"`
	Todo         int                `json:"todo"`
	Distribution map[string]float64 `json:"distribution"` // 百分比分布
	Trend        *TaskTrend         `json:"trend,omitempty"`
}

// TaskTrend 任务趋势
type TaskTrend struct {
	CompletedThisWeek int `json:"completed_this_week"`
	CompletedLastWeek int `json:"completed_last_week"`
}

// StatisticsService 统计计算服务接口
type StatisticsService interface {
	// CalculateProjectMetrics 计算项目核心指标
	CalculateProjectMetrics(ctx context.Context, projectID string) (*ProjectMetrics, error)

	// GetTaskStatusDistribution 获取任务状态分布
	GetTaskStatusDistribution(ctx context.Context, projectID string) (*TaskDistribution, error)
}

// cacheEntry 缓存条目
type cacheEntry struct {
	data      interface{}
	expiresAt time.Time
}

// statisticsService 统计计算服务实现
type statisticsService struct {
	mu       sync.RWMutex
	dataRoot string
	cache    map[string]*cacheEntry // 缓存，5分钟TTL
}

// NewStatisticsService 创建统计计算服务实例
func NewStatisticsService(dataRoot string) StatisticsService {
	return &statisticsService{
		dataRoot: dataRoot,
		cache:    make(map[string]*cacheEntry),
	}
}

// CalculateProjectMetrics 计算项目核心指标
func (s *statisticsService) CalculateProjectMetrics(ctx context.Context, projectID string) (*ProjectMetrics, error) {
	// 尝试从缓存获取
	cacheKey := fmt.Sprintf("metrics:%s", projectID)
	s.mu.RLock()
	if entry, found := s.cache[cacheKey]; found && time.Now().Before(entry.expiresAt) {
		s.mu.RUnlock()
		return entry.data.(*ProjectMetrics), nil
	}
	s.mu.RUnlock()

	// 读取任务列表
	tasks, err := s.loadTasks(projectID)
	if err != nil {
		return nil, fmt.Errorf("load tasks: %w", err)
	}

	// 统计任务状态
	metrics := &ProjectMetrics{
		TotalTasks:      len(tasks),
		CompletedTasks:  0,
		InProgressTasks: 0,
		TodoTasks:       0,
	}

	for _, task := range tasks {
		status, ok := task["status"].(string)
		if !ok {
			status = "todo" // 默认状态
		}

		switch status {
		case "completed":
			metrics.CompletedTasks++
		case "in-progress":
			metrics.InProgressTasks++
		case "todo":
			metrics.TodoTasks++
		default:
			// 其他状态也计入todo
			metrics.TodoTasks++
		}
	}

	// 计算完成率
	if metrics.TotalTasks > 0 {
		metrics.CompletionRate = float64(metrics.CompletedTasks) / float64(metrics.TotalTasks) * 100
	}

	// 读取项目元数据（如果存在）
	metadata, err := s.loadProjectMetadata(projectID)
	if err == nil {
		if startDate, ok := metadata["start_date"].(string); ok {
			metrics.StartDate = startDate
		}
		if endDate, ok := metadata["estimated_end_date"].(string); ok {
			metrics.EstimatedEndDate = endDate
		}
	}

	// 存入缓存
	s.mu.Lock()
	s.cache[cacheKey] = &cacheEntry{
		data:      metrics,
		expiresAt: time.Now().Add(5 * time.Minute),
	}
	s.mu.Unlock()

	return metrics, nil
}

// GetTaskStatusDistribution 获取任务状态分布
func (s *statisticsService) GetTaskStatusDistribution(ctx context.Context, projectID string) (*TaskDistribution, error) {
	// 尝试从缓存获取
	cacheKey := fmt.Sprintf("distribution:%s", projectID)
	s.mu.RLock()
	if entry, found := s.cache[cacheKey]; found && time.Now().Before(entry.expiresAt) {
		s.mu.RUnlock()
		return entry.data.(*TaskDistribution), nil
	}
	s.mu.RUnlock()

	// 读取任务列表
	tasks, err := s.loadTasks(projectID)
	if err != nil {
		return nil, fmt.Errorf("load tasks: %w", err)
	}

	// 统计任务状态
	distribution := &TaskDistribution{
		Total:        len(tasks),
		Completed:    0,
		InProgress:   0,
		Todo:         0,
		Distribution: make(map[string]float64),
	}

	for _, task := range tasks {
		status, ok := task["status"].(string)
		if !ok {
			status = "todo"
		}

		switch status {
		case "completed":
			distribution.Completed++
		case "in-progress":
			distribution.InProgress++
		case "todo":
			distribution.Todo++
		default:
			distribution.Todo++
		}
	}

	// 计算百分比分布
	if distribution.Total > 0 {
		distribution.Distribution["completed"] = float64(distribution.Completed) / float64(distribution.Total) * 100
		distribution.Distribution["in_progress"] = float64(distribution.InProgress) / float64(distribution.Total) * 100
		distribution.Distribution["todo"] = float64(distribution.Todo) / float64(distribution.Total) * 100
	}

	// 计算趋势：统计本周和上周完成的任务数
	distribution.Trend = s.calculateTaskTrend(tasks)

	// 存入缓存
	s.mu.Lock()
	s.cache[cacheKey] = &cacheEntry{
		data:      distribution,
		expiresAt: time.Now().Add(5 * time.Minute),
	}
	s.mu.Unlock()

	return distribution, nil
}

// loadTasks 加载项目的所有任务
func (s *statisticsService) loadTasks(projectID string) ([]map[string]interface{}, error) {
	tasksFile := filepath.Join(s.dataRoot, projectID, "tasks.json")
	fmt.Printf("[DEBUG] statisticsService.loadTasks: projectID=%s, tasksFile=%s\n", projectID, tasksFile)

	// 如果文件不存在，返回空列表
	if _, err := os.Stat(tasksFile); os.IsNotExist(err) {
		fmt.Printf("[DEBUG] tasks file not found: %s\n", tasksFile)
		return []map[string]interface{}{}, nil
	}

	data, err := os.ReadFile(tasksFile)
	if err != nil {
		return nil, fmt.Errorf("read tasks file: %w", err)
	}

	var tasks []map[string]interface{}
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("unmarshal tasks: %w", err)
	}

	fmt.Printf("[DEBUG] loaded %d tasks from %s\n", len(tasks), tasksFile)
	return tasks, nil
}

// loadProjectMetadata 加载项目元数据
func (s *statisticsService) loadProjectMetadata(projectID string) (map[string]interface{}, error) {
	metadataFile := filepath.Join(s.dataRoot, projectID, "metadata.json")
	fmt.Printf("[DEBUG] statisticsService.loadProjectMetadata: projectID=%s, metadataFile=%s\n", projectID, metadataFile)

	// 如果文件不存在，返回空map
	if _, err := os.Stat(metadataFile); os.IsNotExist(err) {
		fmt.Printf("[DEBUG] metadata file not found: %s\n", metadataFile)
		return make(map[string]interface{}), nil
	}

	data, err := os.ReadFile(metadataFile)
	if err != nil {
		return nil, fmt.Errorf("read metadata file: %w", err)
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("unmarshal metadata: %w", err)
	}

	fmt.Printf("[DEBUG] loaded metadata from %s\n", metadataFile)
	return metadata, nil
}

// calculateTaskTrend 计算任务完成趋势
func (s *statisticsService) calculateTaskTrend(tasks []map[string]interface{}) *TaskTrend {
	trend := &TaskTrend{
		CompletedThisWeek: 0,
		CompletedLastWeek: 0,
	}

	now := time.Now()

	// 计算本周的起始时间（周一 00:00:00）
	weekday := int(now.Weekday())
	if weekday == 0 {
		weekday = 7 // 周日算作第7天
	}
	thisWeekStart := now.AddDate(0, 0, -(weekday - 1))
	thisWeekStart = time.Date(thisWeekStart.Year(), thisWeekStart.Month(), thisWeekStart.Day(), 0, 0, 0, 0, thisWeekStart.Location())

	// 计算上周的起始和结束时间
	lastWeekStart := thisWeekStart.AddDate(0, 0, -7)
	lastWeekEnd := thisWeekStart

	fmt.Printf("[DEBUG] calculateTaskTrend: now=%s, thisWeekStart=%s, lastWeekStart=%s\n",
		now.Format("2006-01-02"), thisWeekStart.Format("2006-01-02"), lastWeekStart.Format("2006-01-02"))

	for _, task := range tasks {
		// 只统计已完成的任务
		status, ok := task["status"].(string)
		if !ok || status != "completed" {
			continue
		}

		// 获取任务的更新时间
		updatedAtStr, ok := task["updated_at"].(string)
		if !ok || updatedAtStr == "" {
			continue
		}

		// 解析时间
		updatedAt, err := time.Parse(time.RFC3339, updatedAtStr)
		if err != nil {
			// 尝试其他格式
			updatedAt, err = time.Parse("2006-01-02T15:04:05-07:00", updatedAtStr)
			if err != nil {
				fmt.Printf("[DEBUG] failed to parse updated_at: %s, error: %v\n", updatedAtStr, err)
				continue
			}
		}

		// 判断是否在本周完成
		if updatedAt.After(thisWeekStart) || updatedAt.Equal(thisWeekStart) {
			trend.CompletedThisWeek++
			fmt.Printf("[DEBUG] task completed this week: id=%v, updated_at=%s\n", task["id"], updatedAtStr)
		} else if (updatedAt.After(lastWeekStart) || updatedAt.Equal(lastWeekStart)) && updatedAt.Before(lastWeekEnd) {
			// 判断是否在上周完成
			trend.CompletedLastWeek++
			fmt.Printf("[DEBUG] task completed last week: id=%v, updated_at=%s\n", task["id"], updatedAtStr)
		}
	}

	fmt.Printf("[DEBUG] trend result: thisWeek=%d, lastWeek=%d\n", trend.CompletedThisWeek, trend.CompletedLastWeek)
	return trend
}
