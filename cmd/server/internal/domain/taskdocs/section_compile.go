package taskdocs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CompileSections 将章节文件拼接成完整的 Markdown 文档
func CompileSections(meta *SectionMeta, sectionsDir string) (string, error) {
	var builder strings.Builder

	// 按 order 排序（sections 应该已经排序）
	for _, section := range meta.Sections {
		// 1. 写入标题
		builder.WriteString(section.Title)
		builder.WriteString("\n\n")

		// 2. 读取章节内容
		filePath := filepath.Join(sectionsDir, section.File)
		content, err := os.ReadFile(filePath)
		if err != nil {
			return "", fmt.Errorf("read section %s: %w", section.ID, err)
		}

		// 3. 写入内容
		builder.Write(content)
		builder.WriteString("\n\n")
	}

	return strings.TrimSpace(builder.String()), nil
}

// ValidateSections 验证章节完整性
func ValidateSections(meta *SectionMeta, sectionsDir string) error {
	for _, section := range meta.Sections {
		filePath := filepath.Join(sectionsDir, section.File)

		// 检查文件是否存在
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			return fmt.Errorf("section file not found: %s", section.File)
		}

		// 验证内容哈希
		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("read section %s: %w", section.ID, err)
		}

		actualHash := hashContent(string(content))
		if actualHash != section.Hash {
			return fmt.Errorf("section %s hash mismatch: expected %s, got %s",
				section.ID, section.Hash, actualHash)
		}
	}

	return nil
}

// CompileSectionsIncremental 增量拼接章节（仅重新编译变化的部分）
// 返回完整的文档内容
func CompileSectionsIncremental(meta *SectionMeta, sectionsDir string, changedSectionIDs []string) (string, error) {
	// 目前简化实现，直接调用完整拼接
	// TODO: 优化为真正的增量拼接
	return CompileSections(meta, sectionsDir)
}

// WriteSectionFile 写入章节文件
func WriteSectionFile(sectionsDir string, section Section, content string) error {
	// 确保目录存在
	if err := os.MkdirAll(sectionsDir, 0755); err != nil {
		return fmt.Errorf("create sections dir: %w", err)
	}

	// 写入文件
	filePath := filepath.Join(sectionsDir, section.File)
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("write section file: %w", err)
	}

	return nil
}

// ReadSectionFile 读取章节文件内容
func ReadSectionFile(sectionsDir string, section Section) (string, error) {
	filePath := filepath.Join(sectionsDir, section.File)
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read section file: %w", err)
	}
	return string(content), nil
}

// DeleteSectionFile 删除章节文件
func DeleteSectionFile(sectionsDir string, section Section) error {
	filePath := filepath.Join(sectionsDir, section.File)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete section file: %w", err)
	}
	return nil
}
