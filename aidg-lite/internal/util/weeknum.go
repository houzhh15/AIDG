package util

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// GetWeekNumber 计算ISO 8601周编号
// 返回格式: "2025-05" (表示2025年第5周)
func GetWeekNumber(t time.Time) string {
	year, week := t.ISOWeek()
	return fmt.Sprintf("%d-%02d", year, week)
}

// ParseWeekNumber 解析周编号
// 输入格式: "2025-05"
// 返回: year, week, error
func ParseWeekNumber(weekNum string) (year int, week int, err error) {
	parts := strings.Split(weekNum, "-")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid week number format: %s (expected YYYY-WW)", weekNum)
	}

	year, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid year in week number: %s", parts[0])
	}

	week, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid week in week number: %s", parts[1])
	}

	if week < 1 || week > 53 {
		return 0, 0, fmt.Errorf("week number out of range: %d (expected 1-53)", week)
	}

	return year, week, nil
}

// IsWeekInRange 判断周是否在范围内
// weekNum: 要判断的周编号 (如: "2025-05")
// start: 起始周编号 (如: "2025-01")
// end: 结束周编号 (如: "2025-10")
// 返回: 是否在范围内
func IsWeekInRange(weekNum, start, end string) bool {
	return weekNum >= start && weekNum <= end
}

// GetCurrentWeek 获取当前周编号
// 返回格式: "2025-05"
func GetCurrentWeek() string {
	return GetWeekNumber(time.Now())
}

// GetWeekRange 获取周的日期范围（周一到周日）
// weekNum: 周编号 (如: "2025-05")
// 返回: 周一日期, 周日日期, error
func GetWeekRange(weekNum string) (start, end time.Time, err error) {
	year, week, err := ParseWeekNumber(weekNum)
	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	// 计算该年第一个周四
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)

	// 找到第一个周一
	weekday := int(jan4.Weekday())
	if weekday == 0 {
		weekday = 7 // Sunday is 0, convert to 7 for calculation
	}
	monday := jan4.AddDate(0, 0, -(weekday - 1))

	// 计算目标周的周一
	start = monday.AddDate(0, 0, (week-1)*7)
	end = start.AddDate(0, 0, 6) // 周日 = 周一 + 6天

	return start, end, nil
}

// FormatWeekRange 格式化周范围为字符串
// weekNum: 周编号 (如: "2025-05")
// 返回格式: "01/29-02/04" (月/日-月/日)
func FormatWeekRange(weekNum string) (string, error) {
	start, end, err := GetWeekRange(weekNum)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%02d/%02d-%02d/%02d",
		start.Month(), start.Day(),
		end.Month(), end.Day()), nil
}
