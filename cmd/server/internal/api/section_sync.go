package api

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/taskdocs"
)

var (
	// sectionMutex 用于防止并发修改sections.json
	sectionMutex sync.Mutex

	// sectionFilePattern 匹配section_xxx.md文件名
	sectionFilePattern = regexp.MustCompile(`^section_(\d{3})\.md$`)
)

// SyncSectionsAfterSwitch 同步章节文件与sections.json元数据
// docPath: 文档目录路径，如 data/projects/{projectId}/tasks/{taskId}/docs/{docType}
func SyncSectionsAfterSwitch(docPath string) error {
	// 加锁防止并发修改
	sectionMutex.Lock()
	defer sectionMutex.Unlock()

	sectionsDir := filepath.Join(docPath, "sections")
	sectionsJSONPath := filepath.Join(sectionsDir, "sections.json") // sections.json在sections/目录下

	// 检查sections目录是否存在
	if _, err := os.Stat(sectionsDir); os.IsNotExist(err) {
		// 如果sections目录不存在，不需要同步
		return nil
	}

	// 读取现有sections.json
	var sectionMeta taskdocs.SectionMeta
	if data, err := os.ReadFile(sectionsJSONPath); err == nil {
		if err := json.Unmarshal(data, &sectionMeta); err != nil {
			return fmt.Errorf("failed to parse sections.json: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read sections.json: %w", err)
	}

	// 读取sections目录下的所有文件
	entries, err := os.ReadDir(sectionsDir)
	if err != nil {
		return fmt.Errorf("failed to read sections directory: %w", err)
	}

	// 构建文件名到Section的映射
	fileToSection := make(map[string]*taskdocs.Section)
	for i := range sectionMeta.Sections {
		fileToSection[sectionMeta.Sections[i].File] = &sectionMeta.Sections[i]
	}

	// 构建实际存在的文件集合
	actualFiles := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() || !sectionFilePattern.MatchString(entry.Name()) {
			continue
		}
		actualFiles[entry.Name()] = true

		// 如果是新文件，需要添加到sections中
		if _, exists := fileToSection[entry.Name()]; !exists {
			filePath := filepath.Join(sectionsDir, entry.Name())
			section, err := parseSectionFile(filePath, entry.Name())
			if err != nil {
				// 跳过无法解析的文件
				continue
			}
			sectionMeta.Sections = append(sectionMeta.Sections, section)
		}
	}

	// 删除不存在的文件对应的section记录
	filteredSections := make([]taskdocs.Section, 0, len(sectionMeta.Sections))
	for _, section := range sectionMeta.Sections {
		if actualFiles[section.File] {
			// 递增版本号
			section.Hash = computeSectionHash(filepath.Join(sectionsDir, section.File))
			filteredSections = append(filteredSections, section)
		}
	}
	sectionMeta.Sections = filteredSections

	// 重新编号order
	for i := range sectionMeta.Sections {
		sectionMeta.Sections[i].Order = i + 1
	}

	// 按order排序
	sort.Slice(sectionMeta.Sections, func(i, j int) bool {
		return sectionMeta.Sections[i].Order < sectionMeta.Sections[j].Order
	})

	// 更新版本和时间戳
	sectionMeta.Version++
	sectionMeta.UpdatedAt = time.Now()
	sectionMeta.ETag = generateETag(sectionMeta.Sections)

	// 写回sections.json
	data, err := json.MarshalIndent(sectionMeta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal sections.json: %w", err)
	}

	if err := os.WriteFile(sectionsJSONPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write sections.json: %w", err)
	}

	return nil
}

// parseSectionFile 解析section文件获取标题等信息
func parseSectionFile(filePath, fileName string) (taskdocs.Section, error) {
	var section taskdocs.Section
	section.File = fileName

	// 从文件名提取ID
	matches := sectionFilePattern.FindStringSubmatch(fileName)
	if len(matches) >= 2 {
		section.ID = fmt.Sprintf("section_%s", matches[1])
	}

	// 读取文件第一行作为标题
	file, err := os.Open(filePath)
	if err != nil {
		return section, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// 解析Markdown标题
		if strings.HasPrefix(line, "#") {
			// 计算#号数量
			hashCount := 0
			for _, ch := range line {
				if ch == '#' {
					hashCount++
				} else {
					break
				}
			}
			section.Level = hashCount
			// 去除#号和空格，获取纯标题文本
			section.Title = strings.TrimSpace(strings.TrimLeft(line, "#"))
			break
		}
	}

	if section.Title == "" {
		section.Title = section.ID
		section.Level = 1
	}

	section.Hash = computeSectionHash(filePath)
	section.Children = []string{}

	return section, nil
}

// computeSectionHash 计算section文件的哈希值
func computeSectionHash(filePath string) string {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	// 使用SHA256计算哈希（取前16字节）
	h := sha256.New()
	h.Write(content)
	return fmt.Sprintf("sha256:%x", h.Sum(nil)[:16])
}

// generateETag 生成ETag
func generateETag(sections []taskdocs.Section) string {
	h := sha256.New()
	for _, s := range sections {
		h.Write([]byte(s.ID + s.Hash))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
