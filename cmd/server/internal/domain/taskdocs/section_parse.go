package taskdocs

import (
	"fmt"
	"strings"
	"time"
)

// ParseSections 从 compiled.md 的内容解析出章节结构
func ParseSections(compiledContent string) (*SectionMeta, error) {
	if strings.TrimSpace(compiledContent) == "" {
		// 空文档返回空的章节元数据
		return &SectionMeta{
			Version:   1,
			UpdatedAt: time.Now(),
			RootLevel: 1,
			Sections:  []Section{},
			ETag:      generateETag([]Section{}),
		}, nil
	}

	// 1. 检测根标题等级
	rootLevel := detectRootLevel(compiledContent)

	// 2. 按行解析，识别标题
	lines := strings.Split(compiledContent, "\n")
	var sections []Section
	var currentSection *Section
	var contentBuffer strings.Builder
	sectionCounter := 1
	inCodeBlock := false // 标记是否在代码块内

	for _, line := range lines {
		// 检测代码块边界
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			// 代码块标记行也作为内容的一部分
			if currentSection != nil {
				contentBuffer.WriteString(line + "\n")
			}
			continue
		}

		// 在代码块内，不识别标题
		if inCodeBlock {
			if currentSection != nil {
				contentBuffer.WriteString(line + "\n")
			}
			continue
		}

		if isHeading(line) {
			// 保存上一个章节
			if currentSection != nil {
				content := strings.TrimSpace(contentBuffer.String())
				currentSection.Hash = hashContent(content)
				sections = append(sections, *currentSection)
				contentBuffer.Reset()
			}

			// 创建新章节
			level := getHeadingLevel(line)
			if level >= rootLevel {
				currentSection = &Section{
					ID:       fmt.Sprintf("section_%03d", sectionCounter),
					Title:    line,
					Level:    level,
					Order:    sectionCounter,
					File:     fmt.Sprintf("section_%03d.md", sectionCounter),
					Children: []string{},
				}
				sectionCounter++
			}
		} else if currentSection != nil {
			contentBuffer.WriteString(line + "\n")
		}
	}

	// 保存最后一个章节
	if currentSection != nil {
		content := strings.TrimSpace(contentBuffer.String())
		currentSection.Hash = hashContent(content)
		sections = append(sections, *currentSection)
	}

	// 3. 构建层级关系
	buildHierarchy(sections)

	return &SectionMeta{
		Version:   1,
		UpdatedAt: time.Now(),
		RootLevel: rootLevel,
		Sections:  sections,
		ETag:      generateETag(sections),
	}, nil
}

// detectRootLevel 检测根标题等级
func detectRootLevel(content string) int {
	lines := strings.Split(content, "\n")
	inCodeBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// 检测代码块边界
		if strings.HasPrefix(trimmed, "```") {
			inCodeBlock = !inCodeBlock
			continue
		}
		// 在代码块外才检测标题
		if !inCodeBlock && isHeading(line) {
			return getHeadingLevel(line)
		}
	}
	return 1 // 默认为 # 级别
}

// isHeading 判断是否为标题行
func isHeading(line string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "#") {
		return false
	}

	// 检查 # 后面是否有空格（标准 Markdown 格式）
	// 或者整行都是 # （也算标题）
	for i, ch := range trimmed {
		if ch != '#' {
			return ch == ' '
		}
		if i >= 6 {
			// Markdown 最多支持 6 级标题
			return false
		}
	}
	return true
}

// getHeadingLevel 获取标题等级
func getHeadingLevel(line string) int {
	trimmed := strings.TrimSpace(line)
	count := 0
	for _, ch := range trimmed {
		if ch == '#' {
			count++
		} else {
			break
		}
	}
	if count > 6 {
		count = 6 // Markdown 最多 6 级
	}
	return count
}

// buildHierarchy 构建层级关系
func buildHierarchy(sections []Section) {
	// 🔧 修复：先清空所有章节的 Children 和 ParentID，避免累加
	for i := range sections {
		sections[i].Children = []string{}
		sections[i].ParentID = nil
	}

	stack := []*Section{}

	for i := range sections {
		sec := &sections[i]

		// 弹出比当前等级高或相等的章节
		for len(stack) > 0 && stack[len(stack)-1].Level >= sec.Level {
			stack = stack[:len(stack)-1]
		}

		// 设置父子关系
		if len(stack) > 0 {
			parent := stack[len(stack)-1]
			sec.ParentID = &parent.ID
			parent.Children = append(parent.Children, sec.ID)
		}

		stack = append(stack, sec)
	}
}

// extractSectionContent 从完整文档中提取特定章节的内容（不含标题，不含子章节）
// 返回：章节内容, 处理的末尾字节位置
func extractSectionContent(compiledContent string, section Section) (string, int) {
	lines := strings.Split(compiledContent, "\n")
	var contentBuffer strings.Builder
	inSection := false
	inCodeBlock := false
	currentPos := 0 // 当前处理的字节位置

	for _, line := range lines {
		lineStartPos := currentPos
		currentPos += len(line) + 1 // +1 for \n

		trimmed := strings.TrimSpace(line)

		// 检测代码块边界
		if strings.HasPrefix(trimmed, "```") {
			if inSection {
				contentBuffer.WriteString(line + "\n")
				inCodeBlock = !inCodeBlock
			}
			continue
		}

		// 只在代码块外检测标题
		if !inCodeBlock && isHeading(line) {
			if line == section.Title && !inSection {
				inSection = true
				continue // 跳过标题行
			}

			// 遇到任何标题都停止提取（保证不包含子章节）
			if inSection {
				return strings.TrimSpace(contentBuffer.String()), lineStartPos
			}
		}

		// 收集章节内容
		if inSection {
			contentBuffer.WriteString(line + "\n")
		}
	}

	// 如果到文档末尾仍未找到下一个标题
	if inSection {
		return strings.TrimSpace(contentBuffer.String()), len(compiledContent)
	}

	// 未找到章节
	return "", 0
}
