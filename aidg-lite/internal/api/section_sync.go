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

	"github.com/houzhh15/aidg-lite/internal/domain/taskdocs"
)

var (
	sectionMutex       sync.Mutex
	sectionFilePattern = regexp.MustCompile(`^section_(\d{3})\.md$`)
)

// SyncSectionsAfterSwitch 同步章节文件与sections.json元数据
func SyncSectionsAfterSwitch(docPath string) error {
	sectionMutex.Lock()
	defer sectionMutex.Unlock()

	sectionsDir := filepath.Join(docPath, "sections")
	sectionsJSONPath := filepath.Join(sectionsDir, "sections.json")

	if _, err := os.Stat(sectionsDir); os.IsNotExist(err) {
		return nil
	}

	var sectionMeta taskdocs.SectionMeta
	if data, err := os.ReadFile(sectionsJSONPath); err == nil {
		if err := json.Unmarshal(data, &sectionMeta); err != nil {
			return fmt.Errorf("failed to parse sections.json: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read sections.json: %w", err)
	}

	entries, err := os.ReadDir(sectionsDir)
	if err != nil {
		return fmt.Errorf("failed to read sections directory: %w", err)
	}

	fileToSection := make(map[string]*taskdocs.Section)
	for i := range sectionMeta.Sections {
		fileToSection[sectionMeta.Sections[i].File] = &sectionMeta.Sections[i]
	}

	actualFiles := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() || !sectionFilePattern.MatchString(entry.Name()) {
			continue
		}
		actualFiles[entry.Name()] = true
		if _, exists := fileToSection[entry.Name()]; !exists {
			filePath := filepath.Join(sectionsDir, entry.Name())
			section, err := parseSectionFile(filePath, entry.Name())
			if err != nil {
				continue
			}
			sectionMeta.Sections = append(sectionMeta.Sections, section)
		}
	}

	filteredSections := make([]taskdocs.Section, 0, len(sectionMeta.Sections))
	for _, section := range sectionMeta.Sections {
		if actualFiles[section.File] {
			section.Hash = computeSectionHash(filepath.Join(sectionsDir, section.File))
			filteredSections = append(filteredSections, section)
		}
	}
	sectionMeta.Sections = filteredSections

	for i := range sectionMeta.Sections {
		sectionMeta.Sections[i].Order = i + 1
	}

	sort.Slice(sectionMeta.Sections, func(i, j int) bool {
		return sectionMeta.Sections[i].Order < sectionMeta.Sections[j].Order
	})

	sectionMeta.Version++
	sectionMeta.UpdatedAt = time.Now()
	sectionMeta.ETag = generateETag(sectionMeta.Sections)

	data, err := json.MarshalIndent(sectionMeta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal sections.json: %w", err)
	}

	if err := os.WriteFile(sectionsJSONPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write sections.json: %w", err)
	}

	return nil
}

func parseSectionFile(filePath, fileName string) (taskdocs.Section, error) {
	var section taskdocs.Section
	section.File = fileName

	matches := sectionFilePattern.FindStringSubmatch(fileName)
	if len(matches) >= 2 {
		section.ID = fmt.Sprintf("section_%s", matches[1])
	}

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
		if strings.HasPrefix(line, "#") {
			hashCount := 0
			for _, ch := range line {
				if ch == '#' {
					hashCount++
				} else {
					break
				}
			}
			section.Level = hashCount
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

func computeSectionHash(filePath string) string {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	h := sha256.New()
	h.Write(content)
	return fmt.Sprintf("sha256:%x", h.Sum(nil)[:16])
}

func generateETag(sections []taskdocs.Section) string {
	h := sha256.New()
	for _, s := range sections {
		h.Write([]byte(s.ID + s.Hash))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
