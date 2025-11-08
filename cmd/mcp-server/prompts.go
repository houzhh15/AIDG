package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ===== 核心数据结构 =====

// PromptArgument 定义提示词模版的参数
type PromptArgument struct {
	Name        string `json:"name"`                  // 参数名称
	Description string `json:"description,omitempty"` // 参数描述
	Required    bool   `json:"required"`              // 是否必填
}

// PromptMetadata 定义提示词模版的元数据（用于 prompts/list 响应）
type PromptMetadata struct {
	Name        string           `json:"name"`                  // 模版名称
	Description string           `json:"description,omitempty"` // 模版描述
	Arguments   []PromptArgument `json:"arguments,omitempty"`   // 参数列表
	Scope       string           `json:"scope,omitempty"`       // 作用域：global/project/personal
	ProjectID   string           `json:"project_id,omitempty"`  // 项目ID（仅 scope=project 时有值）
}

// PromptTemplate 定义完整的提示词模版对象
type PromptTemplate struct {
	Name        string           `json:"name"`        // 模版名称
	Description string           `json:"description"` // 模版描述
	Arguments   []PromptArgument `json:"arguments"`   // 参数定义
	Content     string           `json:"content"`     // 模版内容（Markdown）
	FilePath    string           `json:"file_path"`   // 文件路径（用于日志和调试）
	Scope       string           `json:"scope"`       // 作用域：global/project/personal
	ProjectID   string           `json:"project_id"`  // 项目ID（仅 scope=project 时有值）
}

// MessageContent 定义 MCP 消息内容
type MessageContent struct {
	Type string `json:"type"` // 内容类型，通常为 "text"
	Text string `json:"text"` // 文本内容
}

// PromptMessage 定义 MCP 提示词消息
type PromptMessage struct {
	Role    string         `json:"role"`    // 角色，通常为 "user"
	Content MessageContent `json:"content"` // 消息内容
}

// PromptResult 定义 prompts/get 接口的响应结果
type PromptResult struct {
	Description string          `json:"description,omitempty"` // 模版描述
	Messages    []PromptMessage `json:"messages"`              // 消息列表
}

// ===== 模版解析引擎 =====

// parseTemplate 解析单个模版文件
// 支持 YAML Frontmatter 和纯 Markdown 两种格式
func parseTemplate(filePath string) (*PromptTemplate, error) {
	// 读取文件内容
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	// 检查文件大小（超过 100KB 记录警告）
	const maxTemplateSize = 100 * 1024 // 100KB
	if len(content) > maxTemplateSize {
		log.Printf("⚠️  [PROMPTS] 模版文件过大: %s (%d bytes)", filePath, len(content))
	}

	text := string(content)

	// 尝试提取 YAML Frontmatter
	meta, body := extractFrontmatter(text)

	// 初始化模版对象
	template := &PromptTemplate{
		FilePath: filePath,
		Content:  body, // 模版正文
	}

	// 从 Frontmatter 提取元数据
	if meta != nil {
		if name, ok := meta["name"].(string); ok {
			template.Name = name
		}
		if desc, ok := meta["description"].(string); ok {
			template.Description = desc
		}
		if args, ok := meta["arguments"].([]interface{}); ok {
			template.Arguments = parseArguments(args)
		}
	}

	// 兜底方案：如果没有提取到名称，使用文件名
	if template.Name == "" {
		template.Name = extractNameFromFilename(filePath)
	}

	// 如果没有从 Frontmatter 获取到参数，从内容中提取占位符
	if len(template.Arguments) == 0 {
		placeholders := extractPlaceholders(body)
		for _, p := range placeholders {
			template.Arguments = append(template.Arguments, PromptArgument{
				Name:     p,
				Required: false, // 默认为可选参数
			})
		}
	}

	// 如果没有从 Frontmatter 获取到名称，尝试从第一个 Markdown heading 提取
	if template.Name == "" {
		template.Name = extractNameFromMarkdown(body)
	}

	return template, nil
}

// extractFrontmatter 提取 YAML Frontmatter
// 返回元数据 map 和去除 Frontmatter 后的正文
func extractFrontmatter(content string) (map[string]interface{}, string) {
	// 检查是否以 --- 开头
	if !strings.HasPrefix(content, "---\n") && !strings.HasPrefix(content, "---\r\n") {
		return nil, content
	}

	// 查找第二个 ---
	lines := strings.Split(content, "\n")
	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIdx = i
			break
		}
	}

	if endIdx == -1 {
		return nil, content // 没有找到结束标记
	}

	// 提取 YAML 部分和正文部分
	yamlText := strings.Join(lines[1:endIdx], "\n")
	body := strings.Join(lines[endIdx+1:], "\n")

	// 简单的 YAML 解析（仅支持本需求的子集）
	meta := parseSimpleYAML(yamlText)

	return meta, strings.TrimSpace(body)
}

// parseSimpleYAML 简单的 YAML 子集解析器
// 仅支持 name, description 字符串和 arguments 数组
func parseSimpleYAML(yamlText string) map[string]interface{} {
	result := make(map[string]interface{})
	lines := strings.Split(yamlText, "\n")

	var currentKey string
	var arrayItems []interface{}
	var currentItem map[string]interface{}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// 顶层键值对
		if !strings.HasPrefix(line, " ") && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])

			// 保存之前的数组
			if currentKey == "arguments" && len(arrayItems) > 0 {
				if currentItem != nil {
					arrayItems = append(arrayItems, currentItem)
				}
				result[currentKey] = arrayItems
				arrayItems = nil
				currentItem = nil
			}

			currentKey = key

			if value != "" && value != "[]" {
				// 简单字符串值（去除引号）
				value = strings.Trim(value, `"'`)
				result[key] = value
			} else if key == "arguments" {
				// 数组开始
				arrayItems = []interface{}{}
			}
		} else if strings.HasPrefix(trimmed, "- ") {
			// 数组项
			if currentItem != nil {
				arrayItems = append(arrayItems, currentItem)
			}
			currentItem = make(map[string]interface{})

			// 处理同一行的 name: value 形式
			itemLine := strings.TrimPrefix(trimmed, "- ")
			if strings.Contains(itemLine, ":") {
				parts := strings.SplitN(itemLine, ":", 2)
				itemKey := strings.TrimSpace(parts[0])
				itemValue := strings.TrimSpace(parts[1])
				itemValue = strings.Trim(itemValue, `"'`)
				currentItem[itemKey] = itemValue
			}
		} else if strings.Contains(trimmed, ":") && currentItem != nil {
			// 数组项的子属性
			parts := strings.SplitN(trimmed, ":", 2)
			itemKey := strings.TrimSpace(parts[0])
			itemValue := strings.TrimSpace(parts[1])
			itemValue = strings.Trim(itemValue, `"'`)

			// 处理布尔值
			if itemValue == "true" {
				currentItem[itemKey] = true
			} else if itemValue == "false" {
				currentItem[itemKey] = false
			} else {
				currentItem[itemKey] = itemValue
			}
		}
	}

	// 保存最后的数组项
	if currentKey == "arguments" {
		if currentItem != nil {
			arrayItems = append(arrayItems, currentItem)
		}
		if len(arrayItems) > 0 {
			result[currentKey] = arrayItems
		}
	}

	return result
}

// parseArguments 将解析出的参数列表转换为 PromptArgument 数组
func parseArguments(args []interface{}) []PromptArgument {
	result := []PromptArgument{}

	for _, arg := range args {
		if argMap, ok := arg.(map[string]interface{}); ok {
			pa := PromptArgument{}

			if name, ok := argMap["name"].(string); ok {
				pa.Name = name
			}
			if desc, ok := argMap["description"].(string); ok {
				pa.Description = desc
			}
			if req, ok := argMap["required"].(bool); ok {
				pa.Required = req
			}

			if pa.Name != "" {
				result = append(result, pa)
			}
		}
	}

	return result
}

// extractPlaceholders 从内容中提取所有 {{key}} 占位符
func extractPlaceholders(content string) []string {
	// 简单的正则匹配实现（不依赖 regexp 包以提升性能）
	var placeholders []string
	seen := make(map[string]bool)

	// 手动扫描 {{...}} 模式
	for i := 0; i < len(content)-3; i++ {
		if content[i] == '{' && content[i+1] == '{' {
			// 找到开始标记
			endIdx := i + 2
			for endIdx < len(content)-1 {
				if content[endIdx] == '}' && content[endIdx+1] == '}' {
					// 找到结束标记
					key := content[i+2 : endIdx]
					key = strings.TrimSpace(key)

					// 验证是否是有效的标识符（字母、数字、下划线）
					if isValidPlaceholder(key) && !seen[key] {
						placeholders = append(placeholders, key)
						seen[key] = true
					}

					i = endIdx + 1
					break
				}
				endIdx++
			}
		}
	}

	return placeholders
}

// isValidPlaceholder 验证占位符是否是有效的标识符
func isValidPlaceholder(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// extractNameFromFilename 从文件名提取模版名称
func extractNameFromFilename(filePath string) string {
	base := filepath.Base(filePath)
	// 移除 .prompt.md 扩展名
	name := strings.TrimSuffix(base, ".prompt.md")
	name = strings.TrimSuffix(name, ".md") // 兼容 .md 后缀
	return name
}

// extractNameFromMarkdown 从 Markdown 第一个标题提取名称
func extractNameFromMarkdown(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	return ""
}

// ===== 缓存管理 =====

// PromptCache 提示词模版缓存
type PromptCache struct {
	prompts    map[string]*PromptTemplate // 模版名称 -> 模版对象
	lastLoaded time.Time                  // 最后加载时间
	dirMtime   time.Time                  // 目录修改时间快照
	cacheTTL   time.Duration              // 缓存过期时间
	mu         sync.RWMutex               // 读写锁
}

// newPromptCache 创建新的缓存实例
func newPromptCache(ttl time.Duration) *PromptCache {
	return &PromptCache{
		prompts:  make(map[string]*PromptTemplate),
		cacheTTL: ttl,
	}
}

// isValid 检查缓存是否有效
func (pc *PromptCache) isValid(dirPath string) bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	// 检查是否已加载
	if pc.lastLoaded.IsZero() {
		return false
	}

	// 检查是否超过 TTL
	if pc.cacheTTL > 0 && time.Since(pc.lastLoaded) > pc.cacheTTL {
		return false
	}

	// 检查目录修改时间
	info, err := os.Stat(dirPath)
	if err != nil {
		return false
	}

	return !info.ModTime().After(pc.dirMtime)
}

// set 更新缓存（写锁保护）
func (pc *PromptCache) set(prompts map[string]*PromptTemplate, dirPath string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.prompts = prompts
	pc.lastLoaded = time.Now()

	// 记录目录修改时间
	if info, err := os.Stat(dirPath); err == nil {
		pc.dirMtime = info.ModTime()
	}
}

// get 获取单个模版（读锁保护）
func (pc *PromptCache) get(name string) (*PromptTemplate, bool) {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	template, exists := pc.prompts[name]
	return template, exists
}

// list 获取所有模版元数据（读锁保护）
func (pc *PromptCache) list() []PromptMetadata {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	result := make([]PromptMetadata, 0, len(pc.prompts))
	for _, template := range pc.prompts {
		result = append(result, PromptMetadata{
			Name:        template.Name,
			Description: template.Description,
			Arguments:   template.Arguments,
		})
	}

	return result
}

// ===== 模版管理器 =====

// PromptManager 提示词模版管理器
type PromptManager struct {
	cache               *PromptCache
	promptsDir          string
	projectsRoot        string                     // 项目根目录（用于加载项目 Prompts）
	dynamicPromptsCache map[string]*PromptTemplate // 动态 Prompts 缓存
	cacheTTL            time.Duration              // 缓存有效期
	lastCacheUpdate     time.Time                  // 上次缓存更新时间
	triggerFilePath     string                     // MCP 通知触发文件路径（step-06）
	mu                  sync.RWMutex
}

// NewPromptManager 创建模版管理器实例
func NewPromptManager() *PromptManager {
	promptsDir := getPromptsDir()
	cacheTTL := getPromptsCacheTTL()
	projectsRoot := getProjectsRoot()

	pm := &PromptManager{
		cache:               newPromptCache(cacheTTL),
		promptsDir:          promptsDir,
		projectsRoot:        projectsRoot,
		dynamicPromptsCache: make(map[string]*PromptTemplate),
		cacheTTL:            cacheTTL,
		triggerFilePath:     filepath.Join(projectsRoot, ".prompts_changed"), // step-06
	}

	// 验证目录
	if validatePromptsDir(promptsDir) {
		log.Printf("✅ [PROMPTS] 模版目录: %s", promptsDir)
	} else {
		log.Printf("⚠️  [PROMPTS] 模版目录不可用，将返回空模版列表")
	}

	return pm
}

// ensureCacheValid 确保缓存有效（Double-Checked Locking 模式）
func (pm *PromptManager) ensureCacheValid() error {
	// step-06: 检查触发文件是否存在（优先级最高）
	if pm.checkAndConsumeTriggerFile() {
		log.Printf("📢 [PROMPTS] 检测到外部通知触发文件，强制刷新缓存")
		pm.mu.Lock()
		defer pm.mu.Unlock()
		return pm.reloadPrompts()
	}

	// 第一次检查（读锁，快速路径）
	if pm.cache.isValid(pm.promptsDir) {
		return nil
	}

	// 第二次检查（写锁，慢速路径）
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 再次检查，防止其他 goroutine 已刷新
	if pm.cache.isValid(pm.promptsDir) {
		return nil
	}

	// 执行刷新
	log.Printf("🔄 [PROMPTS] 检测到模版变更或缓存失效，重新加载缓存")
	return pm.reloadPrompts()
}

// checkAndConsumeTriggerFile 检查并消费触发文件（step-06）
// 如果触发文件存在，删除它并返回 true
func (pm *PromptManager) checkAndConsumeTriggerFile() bool {
	if pm.triggerFilePath == "" {
		return false
	}

	// 检查文件是否存在
	if _, err := os.Stat(pm.triggerFilePath); os.IsNotExist(err) {
		return false
	}

	// 删除触发文件（消费通知）
	if err := os.Remove(pm.triggerFilePath); err != nil {
		log.Printf("⚠️  [PROMPTS] 删除触发文件失败: %v", err)
		return false
	}

	log.Printf("✅ [PROMPTS] 已消费外部通知触发文件: %s", pm.triggerFilePath)
	return true
}

// reloadPrompts 重新加载所有模版（需要调用者持有写锁）
func (pm *PromptManager) reloadPrompts() error {
	prompts, err := pm.loadPrompts(pm.promptsDir)
	if err != nil {
		return fmt.Errorf("加载模版失败: %w", err)
	}

	pm.cache.set(prompts, pm.promptsDir)
	log.Printf("✅ [PROMPTS] 已加载 %d 个提示词模版", len(prompts))

	return nil
}

// loadPrompts 扫描目录并加载所有 .prompt.md 文件
func (pm *PromptManager) loadPrompts(dirPath string) (map[string]*PromptTemplate, error) {
	// 检查目录是否存在
	if !validatePromptsDir(dirPath) {
		return make(map[string]*PromptTemplate), nil // 返回空 map，不报错
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	prompts := make(map[string]*PromptTemplate)

	for _, entry := range entries {
		// 只处理 .prompt.md 文件
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".prompt.md") {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())

		// 安全性检查
		if err := validateTemplatePath(dirPath, filePath); err != nil {
			log.Printf("⚠️  [PROMPTS] 跳过不安全的文件路径: %s (%v)", filePath, err)
			continue
		}

		// 解析模版
		template, err := parseTemplate(filePath)
		if err != nil {
			log.Printf("⚠️  [PROMPTS] 跳过无法解析的文件: %s (%v)", filePath, err)
			continue // 单个文件失败不影响其他模版
		}

		// 检查名称冲突
		if _, exists := prompts[template.Name]; exists {
			log.Printf("⚠️  [PROMPTS] 模版名称冲突，跳过: %s (文件: %s)", template.Name, filePath)
			continue
		}

		prompts[template.Name] = template
	}

	return prompts, nil
}

// ===== 对外接口方法 =====

// ListPrompts 返回所有可用模版的元数据列表
func (pm *PromptManager) ListPrompts() ([]PromptMetadata, error) {
	// 确保缓存有效
	if err := pm.ensureCacheValid(); err != nil {
		return nil, err
	}

	// 从缓存获取列表
	list := pm.cache.list()

	// 按名称字母顺序排序
	sortPromptMetadata(list)

	return list, nil
}

// GetPrompt 获取指定模版并替换参数
func (pm *PromptManager) GetPrompt(name string, args map[string]string) (*PromptResult, error) {
	// 确保缓存有效
	if err := pm.ensureCacheValid(); err != nil {
		return nil, err
	}

	// 先从静态缓存查找
	template, exists := pm.cache.get(name)

	// 如果静态缓存没有，尝试从动态缓存查找
	if !exists {
		pm.mu.RLock()
		template, exists = pm.dynamicPromptsCache[name]
		pm.mu.RUnlock()
	}

	if !exists {
		return nil, fmt.Errorf("模版不存在: %s", name)
	}

	// 验证必填参数
	if err := validateArguments(template, args); err != nil {
		return nil, err
	}

	// 参数替换
	content := replaceParameters(template.Content, args)

	// 构造 MCP 响应
	result := &PromptResult{
		Description: template.Description,
		Messages: []PromptMessage{
			{
				Role: "user",
				Content: MessageContent{
					Type: "text",
					Text: content,
				},
			},
		},
	}

	log.Printf("🔧 [PROMPTS] prompts/get - name: %s, args: %v", name, args)

	return result, nil
}

// sortPromptMetadata 按名称字母顺序排序
func sortPromptMetadata(list []PromptMetadata) {
	// 简单的冒泡排序（模版数量不多）
	n := len(list)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if list[j].Name > list[j+1].Name {
				list[j], list[j+1] = list[j+1], list[j]
			}
		}
	}
}

// validateArguments 验证必填参数是否都已提供
func validateArguments(template *PromptTemplate, args map[string]string) error {
	for _, arg := range template.Arguments {
		if arg.Required {
			if _, exists := args[arg.Name]; !exists {
				return fmt.Errorf("缺少必填参数: %s", arg.Name)
			}
		}
	}
	return nil
}

// replaceParameters 替换模版中的参数占位符
func replaceParameters(content string, args map[string]string) string {
	result := content

	// 遍历所有参数，替换对应的占位符
	for key, value := range args {
		placeholder := "{{" + key + "}}"
		result = strings.ReplaceAll(result, placeholder, value)
	}

	// 对于未提供的可选参数，替换为空字符串
	// 扫描剩余的占位符
	for i := 0; i < len(result)-3; i++ {
		if result[i] == '{' && result[i+1] == '{' {
			endIdx := i + 2
			for endIdx < len(result)-1 {
				if result[endIdx] == '}' && result[endIdx+1] == '}' {
					key := result[i+2 : endIdx]
					key = strings.TrimSpace(key)

					// 如果是有效的标识符且未提供值，替换为空字符串
					if isValidPlaceholder(key) {
						placeholder := result[i : endIdx+2]
						result = strings.ReplaceAll(result, placeholder, "")
						// 重新开始扫描，因为字符串已改变
						i = -1
						break
					}

					i = endIdx + 1
					break
				}
				endIdx++
			}
		}
	}

	return result
}

// getPromptsDir 读取并解析 MCP_PROMPTS_DIR 环境变量
// 返回最终的绝对路径
func getPromptsDir() string {
	dir := os.Getenv("MCP_PROMPTS_DIR")
	if dir == "" {
		dir = "./prompts" // 默认值
	}
	return resolvePromptsDir(dir)
}

// getPromptsCacheTTL 读取 MCP_PROMPTS_CACHE_TTL 环境变量
// 返回缓存过期时间（分钟），默认 5 分钟
func getPromptsCacheTTL() time.Duration {
	ttlStr := os.Getenv("MCP_PROMPTS_CACHE_TTL")
	if ttlStr == "" {
		return 5 * time.Minute // 默认 5 分钟
	}

	var minutes int
	if _, err := fmt.Sscanf(ttlStr, "%d", &minutes); err != nil {
		log.Printf("⚠️  [PROMPTS] 无效的 MCP_PROMPTS_CACHE_TTL 值: %s，使用默认值 5 分钟", ttlStr)
		return 5 * time.Minute
	}

	if minutes <= 0 {
		return 0 // 禁用缓存
	}

	return time.Duration(minutes) * time.Minute
}

// getProjectsRoot 读取项目根目录路径
func getProjectsRoot() string {
	root := os.Getenv("PROJECTS_ROOT")
	if root == "" {
		root = "./data" // 默认值：数据根目录（不是 ./data/projects）
	}
	return filepath.Clean(root)
}

// resolvePromptsDir 解析模版目录路径
// 支持相对路径和绝对路径
func resolvePromptsDir(dir string) string {
	// 如果是绝对路径，直接返回
	if filepath.IsAbs(dir) {
		return filepath.Clean(dir)
	}

	// 相对路径：基于当前工作目录解析
	wd, err := os.Getwd()
	if err != nil {
		log.Printf("❌ [PROMPTS] 无法获取当前工作目录: %v", err)
		return dir
	}

	absPath := filepath.Join(wd, dir)
	return filepath.Clean(absPath)
}

// validatePromptsDir 验证模版目录是否存在且可访问
// 如果目录不存在或无法访问，记录 ERROR 日志但不阻止服务启动
func validatePromptsDir(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("❌ [PROMPTS] 模版目录不存在: %s", dir)
		} else {
			log.Printf("❌ [PROMPTS] 模版目录无法访问: %s (%v)", dir, err)
		}
		return false
	}

	if !info.IsDir() {
		log.Printf("❌ [PROMPTS] 路径不是目录: %s", dir)
		return false
	}

	// 尝试读取目录以检查权限
	_, err = os.ReadDir(dir)
	if err != nil {
		log.Printf("❌ [PROMPTS] 模版目录无读权限: %s (%v)", dir, err)
		return false
	}

	return true
}

// validateTemplatePath 验证模版文件路径安全性
// 防止路径遍历攻击（如 .. 等）
func validateTemplatePath(basePath, filePath string) error {
	absBase, err := filepath.Abs(basePath)
	if err != nil {
		return fmt.Errorf("无法解析基础路径: %w", err)
	}

	absFile, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("无法解析文件路径: %w", err)
	}

	// 检查文件路径是否在基础路径下
	if !strings.HasPrefix(absFile, absBase) {
		return fmt.Errorf("路径遍历攻击检测: %s 不在 %s 目录下", filePath, basePath)
	}

	return nil
}

// ===== 动态 Prompts 加载（三层架构）=====

// LoadDynamicPrompts 加载三层 Prompts（全局、项目、个人）
// 参数：username（用户名）、projectID（项目ID）、taskID（任务ID，预留）
func (pm *PromptManager) LoadDynamicPrompts(username, projectID, taskID string) ([]*PromptTemplate, error) {
	var allPrompts []*PromptTemplate

	// 1. 加载全局 Prompts（{projectsRoot}/prompts/global/）
	globalDir := filepath.Join(pm.projectsRoot, "prompts", "global")
	if globalPrompts, err := pm.loadPromptsFromJSONDir(globalDir); err == nil {
		allPrompts = append(allPrompts, globalPrompts...)
		log.Printf("📁 [PROMPTS] 全局 Prompts: %d 个 (目录: %s)", len(globalPrompts), globalDir)
	} else {
		log.Printf("⚠️  [PROMPTS] 加载全局 Prompts 失败: %v (目录: %s)", err, globalDir)
	}

	// 2. 加载个人 Prompts（{projectsRoot}/users/{username}/prompts/）
	if username != "" {
		userDir := filepath.Join(pm.projectsRoot, "users", username, "prompts")
		if userPrompts, err := pm.loadPromptsFromJSONDir(userDir); err == nil {
			allPrompts = append(allPrompts, userPrompts...)
			log.Printf("📁 [PROMPTS] 用户 %s Prompts: %d 个 (目录: %s)", username, len(userPrompts), userDir)
		} else {
			log.Printf("⚠️  [PROMPTS] 加载用户 %s 的 Prompts 失败: %v (目录: %s)", username, err, userDir)
		}
	}

	// 3. 加载项目 Prompts（{projectsRoot}/projects/{projectID}/prompts/）
	if projectID != "" {
		projectDir := filepath.Join(pm.projectsRoot, "projects", projectID, "prompts")
		if projectPrompts, err := pm.loadPromptsFromJSONDir(projectDir); err == nil {
			allPrompts = append(allPrompts, projectPrompts...)
			log.Printf("📁 [PROMPTS] 项目 %s Prompts: %d 个 (目录: %s)", projectID, len(projectPrompts), projectDir)
		} else {
			log.Printf("⚠️  [PROMPTS] 加载项目 %s 的 Prompts 失败: %v (目录: %s)", projectID, err, projectDir)
		}
	}

	log.Printf("✅ [PROMPTS] 动态加载完成: 全局+用户+项目 共 %d 个 Prompts (username=%s, projectID=%s)",
		len(allPrompts), username, projectID)
	return allPrompts, nil
}

// loadPromptsFromJSONDir 从指定目录加载所有 JSON 格式的 Prompts
func (pm *PromptManager) loadPromptsFromJSONDir(dirPath string) ([]*PromptTemplate, error) {
	// 检查目录是否存在
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return []*PromptTemplate{}, nil // 目录不存在不报错，返回空列表
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	var prompts []*PromptTemplate
	for _, entry := range entries {
		// 只处理 .json 文件
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		filePath := filepath.Join(dirPath, entry.Name())

		// 读取 JSON 文件并解析为 Prompt 结构
		content, err := os.ReadFile(filePath)
		if err != nil {
			log.Printf("⚠️  [PROMPTS] 读取文件失败: %s (%v)", filePath, err)
			continue
		}

		// 简单的 JSON 解析（复用现有的 Prompt 结构）
		var prompt struct {
			PromptID    string `json:"prompt_id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Content     string `json:"content"`
			Scope       string `json:"scope"`      // 新增：scope 字段
			ProjectID   string `json:"project_id"` // 新增：project_id 字段
			Arguments   []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Required    bool   `json:"required"`
			} `json:"arguments"`
		}

		// 解析 JSON
		if err := json.Unmarshal(content, &prompt); err != nil {
			log.Printf("⚠️  [PROMPTS] JSON 解析失败: %s (%v)", filePath, err)
			continue
		}

		// 转换为 PromptTemplate 结构
		template := &PromptTemplate{
			Name:        prompt.Name,
			Description: prompt.Description,
			Content:     prompt.Content,
			FilePath:    filePath,
			Scope:       prompt.Scope,     // 新增：设置 scope
			ProjectID:   prompt.ProjectID, // 新增：设置 project_id
		}

		for _, arg := range prompt.Arguments {
			template.Arguments = append(template.Arguments, PromptArgument{
				Name:        arg.Name,
				Description: arg.Description,
				Required:    arg.Required,
			})
		}

		prompts = append(prompts, template)
	}

	return prompts, nil
}

// InvalidateCache 缓存失效（被变更通知调用时清空缓存）
func (pm *PromptManager) InvalidateCache() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	pm.dynamicPromptsCache = make(map[string]*PromptTemplate)
	pm.lastCacheUpdate = time.Time{} // 重置为零值
	log.Printf("🔄 [PROMPTS] 动态缓存已失效，下次查询将重新加载")
}

// GetUserPrompts 获取用户可见的 Prompts 列表（合并静态+动态）
func (pm *PromptManager) GetUserPrompts(username, projectID, taskID string) ([]PromptMetadata, error) {
	// step-06: 优先检查触发文件（外部通知）
	triggerFileExists := false
	if pm.triggerFilePath != "" {
		if _, err := os.Stat(pm.triggerFilePath); err == nil {
			triggerFileExists = true
			// 删除触发文件（消费通知）
			if err := os.Remove(pm.triggerFilePath); err != nil {
				log.Printf("⚠️  [PROMPTS] 删除触发文件失败: %v", err)
			} else {
				log.Printf("✅ [PROMPTS] 检测到外部通知触发文件，强制刷新动态 Prompts 缓存")
			}
		}
	}

	// 检查缓存是否有效
	pm.mu.RLock()
	cacheValid := !triggerFileExists && pm.cacheTTL > 0 && !pm.lastCacheUpdate.IsZero() && time.Since(pm.lastCacheUpdate) < pm.cacheTTL
	pm.mu.RUnlock()

	// 缓存失效或触发文件存在，重新加载
	if !cacheValid {
		pm.mu.Lock()
		// 双重检查
		if triggerFileExists || pm.cacheTTL == 0 || pm.lastCacheUpdate.IsZero() || time.Since(pm.lastCacheUpdate) >= pm.cacheTTL {
			dynamicPrompts, err := pm.LoadDynamicPrompts(username, projectID, taskID)
			if err != nil {
				pm.mu.Unlock()
				return nil, fmt.Errorf("加载动态 Prompts 失败: %w", err)
			}

			// 更新缓存
			pm.dynamicPromptsCache = make(map[string]*PromptTemplate)
			for _, p := range dynamicPrompts {
				pm.dynamicPromptsCache[p.Name] = p
			}
			pm.lastCacheUpdate = time.Now()

			if triggerFileExists {
				log.Printf("📢 [PROMPTS] 动态 Prompts 缓存已刷新（触发器驱动）")
			}
		}
		pm.mu.Unlock()
	}

	// 合并静态模板（预置 Prompts）
	staticList, err := pm.ListPrompts()
	if err != nil {
		return nil, fmt.Errorf("获取静态 Prompts 失败: %w", err)
	}

	// 合并动态 Prompts
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	seen := make(map[string]bool)
	var result []PromptMetadata

	// 先添加静态 Prompts
	for _, meta := range staticList {
		result = append(result, meta)
		seen[meta.Name] = true
	}

	// 再添加动态 Prompts（去重）
	for _, template := range pm.dynamicPromptsCache {
		if !seen[template.Name] {
			result = append(result, PromptMetadata{
				Name:        template.Name,
				Description: template.Description,
				Arguments:   template.Arguments,
				Scope:       template.Scope,     // 新增：传递 scope
				ProjectID:   template.ProjectID, // 新增：传递 project_id
			})
			seen[template.Name] = true
		}
	}

	// 排序
	sortPromptMetadata(result)

	return result, nil
}
