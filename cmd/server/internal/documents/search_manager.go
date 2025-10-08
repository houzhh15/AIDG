package documents

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// SearchManager 负责文档全文搜索
type SearchManager struct {
	mu         sync.RWMutex
	indexMgr   *IndexManager
	projectDir string
}

// NewSearchManager 创建搜索管理器
func NewSearchManager(indexMgr *IndexManager, projectDir string) *SearchManager {
	return &SearchManager{
		indexMgr:   indexMgr,
		projectDir: projectDir,
	}
}

// SearchDocument 搜索文档内容结构
type SearchDocument struct {
	Title     string                 `json:"title"`
	Content   string                 `json:"content"`
	Type      DocumentType           `json:"type"`
	Metadata  map[string]interface{} `json:"metadata"`
	CreatedAt time.Time              `json:"created_at"`
	UpdatedAt time.Time              `json:"updated_at"`
}

// SearchResult 搜索结果
type SearchResult struct {
	DocumentID     string                 `json:"document_id"`
	Title          string                 `json:"title"`
	Content        string                 `json:"content"`
	Score          int                    `json:"score"`
	TitleMatches   []MatchHighlight       `json:"title_matches"`
	ContentMatches []MatchHighlight       `json:"content_matches"`
	Metadata       map[string]interface{} `json:"metadata"`
	CreatedAt      time.Time              `json:"created_at"`
	UpdatedAt      time.Time              `json:"updated_at"`
}

// MatchHighlight 匹配高亮信息
type MatchHighlight struct {
	Start  int    `json:"start"`
	End    int    `json:"end"`
	Text   string `json:"text"`
	Before string `json:"before"`
	After  string `json:"after"`
}

// SearchOptions 搜索选项
type SearchOptions struct {
	Query         string   `json:"query"`
	CaseSensitive bool     `json:"case_sensitive"`
	WholeWord     bool     `json:"whole_word"`
	UseRegex      bool     `json:"use_regex"`
	MaxResults    int      `json:"max_results"`
	DocumentTypes []string `json:"document_types"`
	ContextChars  int      `json:"context_chars"`
}

// SearchDocuments 执行文档搜索
func (sm *SearchManager) SearchDocuments(options SearchOptions) ([]SearchResult, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if strings.TrimSpace(options.Query) == "" {
		return []SearchResult{}, nil
	}

	// 设置默认值
	if options.MaxResults <= 0 {
		options.MaxResults = 50
	}
	if options.ContextChars <= 0 {
		options.ContextChars = 100
	}

	// 准备搜索正则表达式
	pattern, err := sm.prepareSearchPattern(options)
	if err != nil {
		return nil, fmt.Errorf("prepare search pattern: %w", err)
	}

	// 获取所有文档
	docs, err := sm.indexMgr.ListAllDocuments()
	if err != nil {
		return nil, fmt.Errorf("list documents: %w", err)
	}

	// 执行搜索
	var results []SearchResult
	for _, docID := range docs {
		result, err := sm.searchInDocument(docID, pattern, options)
		if err != nil {
			continue // 跳过有问题的文档
		}
		if result != nil {
			results = append(results, *result)
		}
	}

	// 按评分排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// 限制结果数量
	if len(results) > options.MaxResults {
		results = results[:options.MaxResults]
	}

	return results, nil
}

// prepareSearchPattern 准备搜索模式
func (sm *SearchManager) prepareSearchPattern(options SearchOptions) (*regexp.Regexp, error) {
	query := options.Query

	if !options.UseRegex {
		// 转义正则表达式特殊字符
		query = regexp.QuoteMeta(query)
	}

	if options.WholeWord {
		query = `\b` + query + `\b`
	}

	flags := ""
	if !options.CaseSensitive {
		flags += "(?i)"
	}

	pattern := flags + query
	return regexp.Compile(pattern)
}

// searchInDocument 在单个文档中搜索
func (sm *SearchManager) searchInDocument(docID string, pattern *regexp.Regexp, options SearchOptions) (*SearchResult, error) {
	// 获取文档元数据
	meta, err := sm.indexMgr.GetNode(docID)
	if err != nil {
		return nil, err
	}

	// 读取文档内容
	contentPath := filepath.Join(sm.projectDir, fmt.Sprintf("%s.md", docID))
	contentBytes, err := os.ReadFile(contentPath)
	var content string
	if err != nil {
		if os.IsNotExist(err) {
			content = "" // 文件不存在，使用空内容
		} else {
			return nil, fmt.Errorf("read content file: %w", err)
		}
	} else {
		content = string(contentBytes)
	}

	// 构建搜索文档对象
	doc := SearchDocument{
		Title:     meta.Title,
		Content:   content,
		Type:      meta.Type,
		Metadata:  make(map[string]interface{}),
		CreatedAt: meta.CreatedAt,
		UpdatedAt: meta.UpdatedAt,
	}

	// 检查文档类型过滤
	if len(options.DocumentTypes) > 0 {
		typeMatch := false
		for _, docType := range options.DocumentTypes {
			if docType == string(doc.Type) {
				typeMatch = true
				break
			}
		}
		if !typeMatch {
			return nil, nil
		}
	}

	// 搜索标题和内容
	titleMatches := sm.findMatches(doc.Title, pattern, options.ContextChars)
	contentMatches := sm.findMatches(doc.Content, pattern, options.ContextChars)

	// 如果没有匹配，返回nil
	if len(titleMatches) == 0 && len(contentMatches) == 0 {
		return nil, nil
	}

	// 计算评分
	score := sm.calculateScore(titleMatches, contentMatches, doc.Title, doc.Content)

	return &SearchResult{
		DocumentID:     docID,
		Title:          doc.Title,
		Content:        sm.truncateContent(doc.Content, 500),
		Score:          score,
		TitleMatches:   titleMatches,
		ContentMatches: contentMatches,
		Metadata:       doc.Metadata,
		CreatedAt:      doc.CreatedAt,
		UpdatedAt:      doc.UpdatedAt,
	}, nil
}

// findMatches 在文本中查找匹配
func (sm *SearchManager) findMatches(text string, pattern *regexp.Regexp, contextChars int) []MatchHighlight {
	matches := pattern.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return nil
	}

	var highlights []MatchHighlight
	for _, match := range matches {
		start, end := match[0], match[1]

		// 计算上下文范围
		beforeStart := max(0, start-contextChars)
		afterEnd := min(len(text), end+contextChars)

		highlights = append(highlights, MatchHighlight{
			Start:  start,
			End:    end,
			Text:   text[start:end],
			Before: text[beforeStart:start],
			After:  text[end:afterEnd],
		})
	}

	return highlights
}

// calculateScore 计算搜索评分
func (sm *SearchManager) calculateScore(titleMatches, contentMatches []MatchHighlight, title, content string) int {
	score := 0

	// 标题匹配权重更高
	score += len(titleMatches) * 10

	// 内容匹配基础分
	score += len(contentMatches) * 2

	// 标题长度权重（越短权重越高）
	if len(title) > 0 && len(titleMatches) > 0 {
		titleBonus := max(1, 100/len(title))
		score += titleBonus
	}

	// 内容匹配密度加分
	if len(content) > 0 && len(contentMatches) > 0 {
		density := len(contentMatches) * 1000 / len(content)
		score += density
	}

	return score
}

// truncateContent 截断内容
func (sm *SearchManager) truncateContent(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}

	truncated := content[:maxLen]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > maxLen*2/3 {
		truncated = truncated[:lastSpace]
	}

	return truncated + "..."
}

// GetSearchSuggestions 获取搜索建议
func (sm *SearchManager) GetSearchSuggestions(query string, limit int) ([]string, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if limit <= 0 {
		limit = 10
	}

	query = strings.ToLower(strings.TrimSpace(query))
	if len(query) < 2 {
		return []string{}, nil
	}

	// 获取所有文档
	docs, err := sm.indexMgr.ListAllDocuments()
	if err != nil {
		return nil, err
	}

	suggestions := make(map[string]int)

	for _, docID := range docs {
		// 获取文档元数据
		meta, err := sm.indexMgr.GetNode(docID)
		if err != nil {
			continue
		}

		// 读取文档内容
		contentPath := filepath.Join(sm.projectDir, fmt.Sprintf("%s.md", docID))
		contentBytes, err := os.ReadFile(contentPath)
		var content string
		if err != nil {
			if os.IsNotExist(err) {
				content = ""
			} else {
				continue
			}
		} else {
			content = string(contentBytes)
		}

		// 从标题中提取词汇
		sm.extractSuggestions(meta.Title, query, suggestions)

		// 从内容中提取词汇（限制长度避免性能问题）
		if len(content) > 5000 {
			content = content[:5000]
		}
		sm.extractSuggestions(content, query, suggestions)
	}

	// 转换为排序的建议列表
	type suggestion struct {
		text  string
		count int
	}

	var suggestionList []suggestion
	for text, count := range suggestions {
		suggestionList = append(suggestionList, suggestion{text, count})
	}

	sort.Slice(suggestionList, func(i, j int) bool {
		return suggestionList[i].count > suggestionList[j].count
	})

	var result []string
	for i, s := range suggestionList {
		if i >= limit {
			break
		}
		result = append(result, s.text)
	}

	return result, nil
}

// extractSuggestions 从文本中提取建议
func (sm *SearchManager) extractSuggestions(text, query string, suggestions map[string]int) {
	text = strings.ToLower(text)
	words := regexp.MustCompile(`\b\w{3,}\b`).FindAllString(text, -1)

	for _, word := range words {
		if strings.Contains(word, query) && word != query {
			suggestions[word]++
		}
	}
}

// 辅助函数
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
