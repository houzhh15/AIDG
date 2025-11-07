package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/houzhh15/AIDG/cmd/server/internal/models"
	"github.com/houzhh15/AIDG/cmd/server/internal/util"
)

// 错误定义
var (
	ErrProgressNotFound  = errors.New("PROGRESS_NOT_FOUND")
	ErrInvalidWeekNumber = errors.New("INVALID_WEEK_NUMBER")
	ErrInvalidMarkdown   = errors.New("INVALID_MARKDOWN")
)

// ProgressService 进展管理服务接口
type ProgressService interface {
	// GetWeekProgress 获取周进展（包含季度、月、周）
	GetWeekProgress(ctx context.Context, projectID, weekNumber string) (*models.WeekProgress, error)

	// UpdateWeekProgress 更新周进展
	UpdateWeekProgress(ctx context.Context, projectID, weekNumber string, quarterSummary, monthSummary, weekSummary *string) error

	// GetYearProgress 获取年度进展
	GetYearProgress(ctx context.Context, projectID string, year int) (*models.YearProgress, error)
}

// progressService 进展管理服务实现
type progressService struct {
	mu       sync.RWMutex
	dataRoot string
}

// NewProgressService 创建进展管理服务实例
func NewProgressService(dataRoot string) ProgressService {
	return &progressService{
		dataRoot: dataRoot,
	}
}

// GetWeekProgress 获取周进展
func (s *progressService) GetWeekProgress(ctx context.Context, projectID, weekNumber string) (*models.WeekProgress, error) {
	// 解析周编号
	year, week, err := util.ParseWeekNumber(weekNumber)
	if err != nil {
		return nil, ErrInvalidWeekNumber
	}

	// 计算周范围
	weekRange, err := util.FormatWeekRange(weekNumber)
	if err != nil {
		return nil, fmt.Errorf("format week range: %w", err)
	}

	// 计算季度和月份
	quarter := getQuarterFromWeek(year, week)
	month := getMonthFromWeek(year, week)

	s.mu.RLock()
	defer s.mu.RUnlock()

	// 读取季度总结
	quarterSummary, _ := s.readQuarterSummaryUnsafe(projectID, year, quarter)

	// 读取月总结
	monthSummary, _ := s.readMonthSummaryUnsafe(projectID, year, month)

	// 读取周总结
	weekSummary, _ := s.readWeekSummaryUnsafe(projectID, year, quarter, month, weekNumber)

	progress := &models.WeekProgress{
		Year:       year,
		WeekNumber: weekNumber,
		WeekRange:  weekRange,
		Quarter: &models.Quarter{
			Quarter:       fmt.Sprintf("Q%d", quarter),
			QuarterNumber: quarter,
			Summary:       quarterSummary,
		},
		Month: &models.Month{
			Month:       fmt.Sprintf("%02d", month),
			MonthNumber: month,
			Name:        fmt.Sprintf("%d年%d月", year, month),
			Summary:     monthSummary,
		},
		Week: &models.Week{
			WeekNumber:    weekNumber,
			WeekNumberInt: week,
			Range:         weekRange,
			Summary:       weekSummary,
		},
	}

	return progress, nil
}

// UpdateWeekProgress 更新周进展
func (s *progressService) UpdateWeekProgress(ctx context.Context, projectID, weekNumber string, quarterSummary, monthSummary, weekSummary *string) error {
	// 解析周编号
	year, week, err := util.ParseWeekNumber(weekNumber)
	if err != nil {
		return ErrInvalidWeekNumber
	}

	quarter := getQuarterFromWeek(year, week)
	month := getMonthFromWeek(year, week)

	s.mu.Lock()
	defer s.mu.Unlock()

	// 更新季度总结
	if quarterSummary != nil {
		if err := s.writeQuarterSummaryUnsafe(projectID, year, quarter, *quarterSummary); err != nil {
			return fmt.Errorf("write quarter summary: %w", err)
		}
	}

	// 更新月总结
	if monthSummary != nil {
		if err := s.writeMonthSummaryUnsafe(projectID, year, month, *monthSummary); err != nil {
			return fmt.Errorf("write month summary: %w", err)
		}
	}

	// 更新周总结
	if weekSummary != nil {
		if err := s.writeWeekSummaryUnsafe(projectID, year, quarter, month, weekNumber, *weekSummary); err != nil {
			return fmt.Errorf("write week summary: %w", err)
		}
	}

	return nil
}

// GetYearProgress 获取年度进展
func (s *progressService) GetYearProgress(ctx context.Context, projectID string, year int) (*models.YearProgress, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	yearProgress := &models.YearProgress{
		Year:     year,
		Quarters: make([]*models.Quarter, 0),
	}

	// 遍历4个季度
	for q := 1; q <= 4; q++ {
		quarterSummary, _ := s.readQuarterSummaryUnsafe(projectID, year, q)

		quarter := &models.Quarter{
			Quarter:       fmt.Sprintf("Q%d", q),
			QuarterNumber: q,
			Summary:       quarterSummary,
			Months:        make([]*models.Month, 0),
		}

		// 获取该季度的月份
		months := getMonthsInQuarter(q)
		for _, m := range months {
			monthSummary, _ := s.readMonthSummaryUnsafe(projectID, year, m)

			month := &models.Month{
				Month:       fmt.Sprintf("%02d", m),
				MonthNumber: m,
				Name:        fmt.Sprintf("%d年%d月", year, m),
				Summary:     monthSummary,
				Weeks:       make([]*models.Week, 0),
			}

			// 读取该月的周总结
			weeks, _ := s.listWeeksInMonthUnsafe(projectID, year, q, m)
			for _, weekNum := range weeks {
				weekSummary, _ := s.readWeekSummaryUnsafe(projectID, year, q, m, weekNum)
				weekRange, _ := util.FormatWeekRange(weekNum)
				_, weekInt, _ := util.ParseWeekNumber(weekNum)

				week := &models.Week{
					WeekNumber:    weekNum,
					WeekNumberInt: weekInt,
					Range:         weekRange,
					Summary:       weekSummary,
				}
				month.Weeks = append(month.Weeks, week)
			}

			quarter.Months = append(quarter.Months, month)
		}

		yearProgress.Quarters = append(yearProgress.Quarters, quarter)
	}

	return yearProgress, nil
}

// 辅助函数：根据周数计算季度
func getQuarterFromWeek(year, week int) int {
	// 简化计算：根据周数估算季度
	if week <= 13 {
		return 1
	} else if week <= 26 {
		return 2
	} else if week <= 39 {
		return 3
	}
	return 4
}

// 辅助函数：根据周数计算月份
func getMonthFromWeek(year, week int) int {
	// 简化计算：周数除以4.33约等于月份
	start, _, _ := util.GetWeekRange(fmt.Sprintf("%d-%02d", year, week))
	return int(start.Month())
}

// 辅助函数：获取季度包含的月份
func getMonthsInQuarter(quarter int) []int {
	switch quarter {
	case 1:
		return []int{1, 2, 3}
	case 2:
		return []int{4, 5, 6}
	case 3:
		return []int{7, 8, 9}
	case 4:
		return []int{10, 11, 12}
	default:
		return []int{}
	}
}

// 文件路径辅助函数
func (s *progressService) getProgressDir(projectID string, year int) string {
	return filepath.Join(s.dataRoot, "projects", projectID, "progress", strconv.Itoa(year))
}

func (s *progressService) getQuarterDir(projectID string, year, quarter int) string {
	return filepath.Join(s.getProgressDir(projectID, year), fmt.Sprintf("Q%d", quarter))
}

func (s *progressService) getMonthDir(projectID string, year, month int) string {
	quarter := (month-1)/3 + 1
	return filepath.Join(s.getQuarterDir(projectID, year, quarter), fmt.Sprintf("%02d", month))
}

// 读写操作（不加锁，内部使用）
func (s *progressService) readQuarterSummaryUnsafe(projectID string, year, quarter int) (string, error) {
	path := filepath.Join(s.getQuarterDir(projectID, year, quarter), "summary.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *progressService) writeQuarterSummaryUnsafe(projectID string, year, quarter int, content string) error {
	dir := s.getQuarterDir(projectID, year, quarter)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "summary.md")
	return os.WriteFile(path, []byte(content), 0644)
}

func (s *progressService) readMonthSummaryUnsafe(projectID string, year, month int) (string, error) {
	path := filepath.Join(s.getMonthDir(projectID, year, month), "summary.md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *progressService) writeMonthSummaryUnsafe(projectID string, year, month int, content string) error {
	dir := s.getMonthDir(projectID, year, month)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, "summary.md")
	return os.WriteFile(path, []byte(content), 0644)
}

func (s *progressService) readWeekSummaryUnsafe(projectID string, year, quarter, month int, weekNumber string) (string, error) {
	// 从周编号中提取周数
	_, week, _ := util.ParseWeekNumber(weekNumber)
	path := filepath.Join(s.getMonthDir(projectID, year, month), fmt.Sprintf("week_%02d.md", week))
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *progressService) writeWeekSummaryUnsafe(projectID string, year, quarter, month int, weekNumber, content string) error {
	dir := s.getMonthDir(projectID, year, month)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	_, week, _ := util.ParseWeekNumber(weekNumber)
	path := filepath.Join(dir, fmt.Sprintf("week_%02d.md", week))
	return os.WriteFile(path, []byte(content), 0644)
}

func (s *progressService) listWeeksInMonthUnsafe(projectID string, year, quarter, month int) ([]string, error) {
	dir := s.getMonthDir(projectID, year, month)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	weeks := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".md" && entry.Name() != "summary.md" {
			// 从文件名提取周数
			var weekNum int
			if _, err := fmt.Sscanf(entry.Name(), "week_%02d.md", &weekNum); err == nil {
				weeks = append(weeks, fmt.Sprintf("%d-%02d", year, weekNum))
			}
		}
	}

	return weeks, nil
}

// saveMetadataUnsafe 保存元数据（版本管理）
func (s *progressService) saveMetadataUnsafe(projectID string, year int, metadata *models.ProgressMetadata) error {
	dir := s.getProgressDir(projectID, year)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(dir, "metadata.json")
	return os.WriteFile(path, data, 0644)
}
