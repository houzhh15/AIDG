package documents

import (
	"fmt"
	"strings"
)

// DiffType 差异类型
type DiffType string

const (
	DiffTypeAdd    DiffType = "add"
	DiffTypeDelete DiffType = "delete"
	DiffTypeModify DiffType = "modify"
	DiffTypeEqual  DiffType = "equal"
)

// DiffLine 行级差异
type DiffLine struct {
	Type    DiffType `json:"type"`
	LineNum int      `json:"line_num"`
	Content string   `json:"content"`
	OldLine int      `json:"old_line,omitempty"`
	NewLine int      `json:"new_line,omitempty"`
}

// DiffResult 差异结果
type DiffResult struct {
	FromVersion int         `json:"from_version"`
	ToVersion   int         `json:"to_version"`
	Lines       []DiffLine  `json:"lines"`
	Summary     DiffSummary `json:"summary"`
}

// DiffSummary 差异摘要
type DiffSummary struct {
	Added    int `json:"added"`
	Deleted  int `json:"deleted"`
	Modified int `json:"modified"`
	Total    int `json:"total"`
}

// ContentDiffer 内容差异比较器
type ContentDiffer struct{}

// NewContentDiffer 创建内容差异比较器
func NewContentDiffer() *ContentDiffer {
	return &ContentDiffer{}
}

// CompareContent 比较两个版本的内容
func (cd *ContentDiffer) CompareContent(oldContent, newContent string, fromVersion, toVersion int) *DiffResult {
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	diffs := cd.computeDiff(oldLines, newLines)
	summary := cd.computeSummary(diffs)

	return &DiffResult{
		FromVersion: fromVersion,
		ToVersion:   toVersion,
		Lines:       diffs,
		Summary:     summary,
	}
}

// computeDiff 计算行级差异（简化的LCS算法）
func (cd *ContentDiffer) computeDiff(oldLines, newLines []string) []DiffLine {
	var result []DiffLine

	// 使用简化的diff算法（类似git diff的行级比较）
	oldIndex := 0
	newIndex := 0
	lineNum := 1

	for oldIndex < len(oldLines) || newIndex < len(newLines) {
		if oldIndex >= len(oldLines) {
			// 只有新行剩余
			result = append(result, DiffLine{
				Type:    DiffTypeAdd,
				LineNum: lineNum,
				Content: newLines[newIndex],
				NewLine: newIndex + 1,
			})
			newIndex++
		} else if newIndex >= len(newLines) {
			// 只有旧行剩余
			result = append(result, DiffLine{
				Type:    DiffTypeDelete,
				LineNum: lineNum,
				Content: oldLines[oldIndex],
				OldLine: oldIndex + 1,
			})
			oldIndex++
		} else if oldLines[oldIndex] == newLines[newIndex] {
			// 行相同
			result = append(result, DiffLine{
				Type:    DiffTypeEqual,
				LineNum: lineNum,
				Content: oldLines[oldIndex],
				OldLine: oldIndex + 1,
				NewLine: newIndex + 1,
			})
			oldIndex++
			newIndex++
		} else {
			// 查找下一个匹配行
			matchFound := false

			// 在接下来的几行中寻找匹配
			for i := 1; i <= 3 && newIndex+i < len(newLines); i++ {
				if oldLines[oldIndex] == newLines[newIndex+i] {
					// 找到匹配，前面的新行是新增的
					for j := 0; j < i; j++ {
						result = append(result, DiffLine{
							Type:    DiffTypeAdd,
							LineNum: lineNum,
							Content: newLines[newIndex+j],
							NewLine: newIndex + j + 1,
						})
						lineNum++
					}
					newIndex += i
					matchFound = true
					break
				}
			}

			if !matchFound {
				// 在接下来的几行中寻找旧行
				for i := 1; i <= 3 && oldIndex+i < len(oldLines); i++ {
					if oldLines[oldIndex+i] == newLines[newIndex] {
						// 找到匹配，前面的旧行是删除的
						for j := 0; j < i; j++ {
							result = append(result, DiffLine{
								Type:    DiffTypeDelete,
								LineNum: lineNum,
								Content: oldLines[oldIndex+j],
								OldLine: oldIndex + j + 1,
							})
							lineNum++
						}
						oldIndex += i
						matchFound = true
						break
					}
				}
			}

			if !matchFound {
				// 没找到匹配，标记为修改
				result = append(result, DiffLine{
					Type:    DiffTypeModify,
					LineNum: lineNum,
					Content: fmt.Sprintf("- %s\n+ %s", oldLines[oldIndex], newLines[newIndex]),
					OldLine: oldIndex + 1,
					NewLine: newIndex + 1,
				})
				oldIndex++
				newIndex++
			}
		}
		lineNum++
	}

	return result
}

// computeSummary 计算差异摘要
func (cd *ContentDiffer) computeSummary(diffs []DiffLine) DiffSummary {
	summary := DiffSummary{}

	for _, diff := range diffs {
		switch diff.Type {
		case DiffTypeAdd:
			summary.Added++
		case DiffTypeDelete:
			summary.Deleted++
		case DiffTypeModify:
			summary.Modified++
		}
	}

	summary.Total = summary.Added + summary.Deleted + summary.Modified
	return summary
}
