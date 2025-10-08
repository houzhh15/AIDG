package services

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/houzhh15-hub/AIDG/cmd/server/internal/models"

	"gopkg.in/yaml.v3"
)

// 错误定义
var (
	// ErrPlanNotLoaded 表示当前内存中尚未加载执行计划。
	ErrPlanNotLoaded = errors.New("EXECUTION_PLAN_NOT_LOADED")
	// ErrPlanNotReady 表示计划状态尚未允许导航。
	ErrPlanNotReady = errors.New("EXECUTION_PLAN_NOT_READY")
	// ErrNotImplemented 表示功能尚未实现，等待后续步骤补充。
	ErrNotImplemented = errors.New("NOT_IMPLEMENTED")
	// ErrInvalidPlanFormat 表示执行计划文件格式不合法。
	ErrInvalidPlanFormat = errors.New("INVALID_EXECUTION_PLAN_FORMAT")
	// ErrDuplicateStepID 表示存在重复的步骤 ID。
	ErrDuplicateStepID = errors.New("DUPLICATE_STEP_ID")
	// ErrStepNotFound 表示要更新的步骤在计划中不存在。
	ErrStepNotFound = errors.New("STEP_NOT_FOUND")
)

// ExecutionPlanRepository 抽象底层存储读写逻辑，后续可对接文件或数据库。
type ExecutionPlanRepository interface {
	Read(ctx context.Context) (string, error)
	Write(ctx context.Context, content string) error
}

// ExecutionPlanService 定义执行计划业务核心能力。
type ExecutionPlanService interface {
	// Load 读取并解析执行计划，返回最新结构体表示。
	Load(ctx context.Context) (*models.ExecutionPlan, error)
	// UpdatePlan 覆盖式更新执行计划内容。
	UpdatePlan(ctx context.Context, content string) (*models.ExecutionPlan, error)
	// UpdateStepStatus 更新指定步骤的状态及输出。
	UpdateStepStatus(ctx context.Context, stepID string, status models.StepStatus, output string) (*models.ExecutionPlan, error)
	// UpdatePlanStatus 更新计划的全局状态（如 Approved, Rejected）。
	UpdatePlanStatus(ctx context.Context, status models.PlanStatus) (*models.ExecutionPlan, error)
	// GetNextExecutableStep 根据依赖关系和优先级返回下一可执行步骤。
	GetNextExecutableStep(ctx context.Context) (*models.Step, error)
	// CurrentPlan 返回内存中的计划副本。
	CurrentPlan() (*models.ExecutionPlan, error)
}

// executionPlanService 为 ExecutionPlanService 的默认实现骨架。
type executionPlanService struct {
	mu         sync.RWMutex
	repository ExecutionPlanRepository
	plan       *models.ExecutionPlan
}

// NewExecutionPlanService 创建 ExecutionPlanService 实例。
func NewExecutionPlanService(repo ExecutionPlanRepository) ExecutionPlanService {
	return &executionPlanService{repository: repo}
}

func (s *executionPlanService) Load(ctx context.Context) (*models.ExecutionPlan, error) {
	if s.repository == nil {
		return nil, fmt.Errorf("load execution plan: repository not configured")
	}

	raw, err := s.repository.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("load execution plan: read repository: %w", err)
	}

	plan, err := parseExecutionPlan(raw)
	if err != nil {
		return nil, fmt.Errorf("load execution plan: %w", err)
	}

	s.mu.Lock()
	s.plan = plan
	s.mu.Unlock()

	return plan, nil
}

func (s *executionPlanService) UpdatePlan(ctx context.Context, content string) (*models.ExecutionPlan, error) {
	if s.repository == nil {
		return nil, fmt.Errorf("update execution plan: repository not configured")
	}

	if err := s.repository.Write(ctx, content); err != nil {
		return nil, fmt.Errorf("update execution plan: write repository: %w", err)
	}

	plan, err := parseExecutionPlan(content)
	if err != nil {
		return nil, fmt.Errorf("update execution plan: %w", err)
	}

	s.mu.Lock()
	s.plan = plan
	s.mu.Unlock()

	return plan, nil
}

func (s *executionPlanService) UpdateStepStatus(ctx context.Context, stepID string, status models.StepStatus, output string) (*models.ExecutionPlan, error) {
	if s.repository == nil {
		return nil, fmt.Errorf("update step status: repository not configured")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.plan == nil {
		return nil, ErrPlanNotLoaded
	}

	plan := s.plan
	steps, index := flattenAndIndexSteps(plan.Steps)
	target, ok := index[stepID]
	if !ok {
		return nil, fmt.Errorf("update step status: step %s: %w", stepID, ErrStepNotFound)
	}

	now := time.Now().UTC()
	target.UpdatedAt = now
	target.Status = status
	target.Output = output

	switch status {
	case models.StepStatusInProgress:
		if target.StartedAt == nil {
			started := now
			target.StartedAt = &started
		}
		target.CompletedAt = nil
	case models.StepStatusSucceeded:
		if target.StartedAt == nil {
			started := now
			target.StartedAt = &started
		}
		completed := now
		target.CompletedAt = &completed
	case models.StepStatusFailed, models.StepStatusCancelled:
		completed := now
		target.CompletedAt = &completed
	default:
		target.StartedAt = nil
		target.CompletedAt = nil
	}

	recalculatePlanStatus(plan, steps)
	plan.UpdatedAt = now

	content, err := renderExecutionPlan(plan)
	if err != nil {
		return nil, fmt.Errorf("update step status: %w", err)
	}

	if err := s.repository.Write(ctx, content); err != nil {
		return nil, fmt.Errorf("update step status: write repository: %w", err)
	}

	updatedPlan, err := parseExecutionPlan(content)
	if err != nil {
		return nil, fmt.Errorf("update step status: %w", err)
	}

	s.plan = updatedPlan
	return updatedPlan, nil
}

// UpdatePlanStatus 更新执行计划的全局状态（如 Approved, Rejected）
func (s *executionPlanService) UpdatePlanStatus(ctx context.Context, status models.PlanStatus) (*models.ExecutionPlan, error) {
	if s.repository == nil {
		return nil, fmt.Errorf("update plan status: repository not configured")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.plan == nil {
		return nil, ErrPlanNotLoaded
	}

	plan := s.plan
	now := time.Now().UTC()
	plan.Status = status
	plan.UpdatedAt = now

	content, err := renderExecutionPlan(plan)
	if err != nil {
		return nil, fmt.Errorf("update plan status: %w", err)
	}

	if err := s.repository.Write(ctx, content); err != nil {
		return nil, fmt.Errorf("update plan status: write repository: %w", err)
	}

	updatedPlan, err := parseExecutionPlan(content)
	if err != nil {
		return nil, fmt.Errorf("update plan status: %w", err)
	}

	s.plan = updatedPlan
	return updatedPlan, nil
}

func (s *executionPlanService) GetNextExecutableStep(ctx context.Context) (*models.Step, error) {
	s.mu.RLock()
	plan := s.plan
	s.mu.RUnlock()

	if plan == nil {
		return nil, ErrPlanNotLoaded
	}

	if plan.Status != models.PlanStatusApproved && plan.Status != models.PlanStatusExecuting {
		return nil, fmt.Errorf("plan status %s does not allow navigation: %w", plan.Status, ErrPlanNotReady)
	}

	steps, index := flattenAndIndexSteps(plan.Steps)
	if len(steps) == 0 {
		return nil, nil
	}

	dependencyMap := buildDependencyMap(plan.Dependencies)

	var candidates []*models.Step
	for _, step := range steps {
		if step == nil {
			continue
		}
		if step.Status != models.StepStatusPending {
			continue
		}
		if !dependenciesSatisfied(step.ID, dependencyMap, index) {
			continue
		}
		candidates = append(candidates, step)
	}

	if len(candidates) == 0 {
		return nil, nil
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		lw, rw := priorityWeight(left.Priority), priorityWeight(right.Priority)
		if lw != rw {
			return lw > rw
		}
		return compareStepIDs(left.ID, right.ID) < 0
	})

	return candidates[0], nil
}

func (s *executionPlanService) CurrentPlan() (*models.ExecutionPlan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.plan == nil {
		return nil, ErrPlanNotLoaded
	}

	return s.plan, nil
}

var (
	stepLinePattern    = regexp.MustCompile(`^\s*-\s*\[([ xX\-\>\!\~])\]\s*(.+)$`)
	stepHeaderPattern  = regexp.MustCompile(`^#{1,6}\s+step-(\d+):\s*(.+)$`)
	attributePattern   = regexp.MustCompile(`(\w+):("[^"]+"|\S+)`)
	stepNumericPattern = regexp.MustCompile(`\d+`)
)

type frontMatter struct {
	PlanID       string             `yaml:"plan_id"`
	TaskID       string             `yaml:"task_id"`
	Status       string             `yaml:"status"`
	CreatedAt    string             `yaml:"created_at"`
	UpdatedAt    string             `yaml:"updated_at"`
	Dependencies []dependencyRecord `yaml:"dependencies"`
	Metadata     map[string]any
}

type frontMatterOutput struct {
	PlanID       string             `yaml:"plan_id"`
	TaskID       string             `yaml:"task_id"`
	Status       string             `yaml:"status"`
	CreatedAt    string             `yaml:"created_at"`
	UpdatedAt    string             `yaml:"updated_at"`
	Dependencies []dependencyRecord `yaml:"dependencies,omitempty"`
	Extra        map[string]any     `yaml:",inline"`
}

type dependencyRecord struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}

func parseExecutionPlan(content string) (*models.ExecutionPlan, error) {
	fm, body, err := splitFrontMatter(content)
	if err != nil {
		return nil, err
	}

	meta, err := parseFrontMatter(fm)
	if err != nil {
		return nil, err
	}

	createdAt, err := parseTimeValue(meta.CreatedAt)
	if err != nil {
		return nil, err
	}
	updatedAt, err := parseTimeValue(meta.UpdatedAt)
	if err != nil {
		return nil, err
	}

	steps, stepIndex, err := parseSteps(body, updatedAt)
	if err != nil {
		// 当解析失败时，创建一个包含原始内容的fallback步骤
		fallbackStep := &models.Step{
			ID:          "content-view",
			Status:      models.StepStatusPending,
			Description: "执行计划内容（点击编辑）\n\n" + body,
			UpdatedAt:   updatedAt,
		}
		steps = []*models.Step{fallbackStep}
		stepIndex = map[string]*models.Step{"content-view": fallbackStep}
	}

	// 如果解析成功但没有步骤，也提供原始内容视图
	if len(steps) == 0 {
		fallbackStep := &models.Step{
			ID:          "content-view",
			Status:      models.StepStatusPending,
			Description: "执行计划内容（点击编辑）\n\n" + body,
			UpdatedAt:   updatedAt,
		}
		steps = []*models.Step{fallbackStep}
		stepIndex = map[string]*models.Step{"content-view": fallbackStep}
	}

	dependencies := make([]models.Dependency, 0, len(meta.Dependencies))
	for _, dep := range meta.Dependencies {
		if dep.Source == "" || dep.Target == "" {
			return nil, fmt.Errorf("dependency must contain source and target: %w", ErrInvalidPlanFormat)
		}
		if _, ok := stepIndex[dep.Source]; !ok {
			return nil, fmt.Errorf("dependency references unknown step %q: %w", dep.Source, ErrInvalidPlanFormat)
		}
		if _, ok := stepIndex[dep.Target]; !ok {
			return nil, fmt.Errorf("dependency references unknown step %q: %w", dep.Target, ErrInvalidPlanFormat)
		}
		dependencies = append(dependencies, models.Dependency{Source: dep.Source, Target: dep.Target})
	}

	plan := &models.ExecutionPlan{
		PlanID:       meta.PlanID,
		TaskID:       meta.TaskID,
		Status:       models.PlanStatus(meta.Status),
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
		Dependencies: dependencies,
		Steps:        steps,
		RawContent:   content,
		Metadata:     meta.Metadata,
	}

	return plan, nil
}

func splitFrontMatter(content string) (string, string, error) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.TrimPrefix(normalized, "\ufeff")
	normalized = strings.TrimLeft(normalized, "\n \t")

	if !strings.HasPrefix(normalized, "---\n") {
		return "", "", fmt.Errorf("missing front matter opening delimiter: %w", ErrInvalidPlanFormat)
	}

	trimmed := strings.TrimPrefix(normalized, "---\n")
	parts := strings.SplitN(trimmed, "\n---\n", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("missing front matter closing delimiter: %w", ErrInvalidPlanFormat)
	}

	return parts[0], parts[1], nil
}

func parseFrontMatter(fm string) (*frontMatter, error) {
	raw := make(map[string]any)
	if err := yaml.Unmarshal([]byte(fm), &raw); err != nil {
		return nil, fmt.Errorf("parse front matter: %w", err)
	}

	var meta frontMatter
	if err := yaml.Unmarshal([]byte(fm), &meta); err != nil {
		return nil, fmt.Errorf("parse front matter: %w", err)
	}

	// 验证并修正状态值 - 容错机制避免LLM生成不稳定状态
	meta.Status = validateAndFixPlanStatus(meta.Status)

	extra := make(map[string]any)
	for key, value := range raw {
		switch key {
		case "plan_id", "task_id", "status", "created_at", "updated_at", "dependencies":
			continue
		default:
			extra[key] = value
		}
	}
	if len(extra) > 0 {
		meta.Metadata = extra
	}

	return &meta, nil
}

// validateAndFixPlanStatus 验证计划状态，如果不符合预期则自动修正为 Pending Approval
func validateAndFixPlanStatus(status string) string {
	// 定义所有有效的计划状态
	validStatuses := map[string]bool{
		"Draft":            true,
		"Pending Approval": true,
		"Approved":         true,
		"Rejected":         true,
		"Executing":        true,
		"Completed":        true,
		"Failed":           true,
	}

	// 如果状态为空或不在有效状态列表中，则默认为待审批状态
	if status == "" || !validStatuses[status] {
		// 常见的非标准状态映射
		switch strings.ToLower(strings.TrimSpace(status)) {
		case "in-progress", "in_progress", "inprogress", "running":
			return "Executing"
		case "done", "finished", "complete":
			return "Completed"
		case "pending", "waiting", "review":
			return "Pending Approval"
		case "draft", "new":
			return "Draft"
		case "approved", "ok", "ready":
			return "Approved"
		case "rejected", "denied", "cancelled":
			return "Rejected"
		case "failed", "error", "failure":
			return "Failed"
		default:
			// 对于完全无法识别的状态，默认设为待审批
			return "Pending Approval"
		}
	}

	return status
}

func parseSteps(body string, updatedAt time.Time) ([]*models.Step, map[string]*models.Step, error) {
	lines := strings.Split(body, "\n")
	defaultUpdated := updatedAt
	if defaultUpdated.IsZero() {
		defaultUpdated = time.Now().UTC()
	}

	var (
		steps []*models.Step
		stack []stackEntry
		index = make(map[string]*models.Step)
	)

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !stepLinePattern.MatchString(line) && !stepHeaderPattern.MatchString(line) {
			continue
		}

		indent := leadingIndent(line)

		// 收集步骤的多行描述
		stepContent := line
		j := i + 1
		stepIndent := indent
		isMarkdownHeader := stepHeaderPattern.MatchString(line)

		// 查找下一个步骤开始的位置，收集中间的所有缩进内容
		for j < len(lines) {
			nextLine := lines[j]
			nextTrimmed := strings.TrimSpace(nextLine)

			// 空行跳过并收集
			if nextTrimmed == "" {
				stepContent += "\n" + nextLine
				j++
				continue
			}

			// 如果遇到新的步骤行，停止收集
			if stepLinePattern.MatchString(nextLine) || stepHeaderPattern.MatchString(nextLine) {
				break
			}

			// 如果遇到新的 Markdown 章节标题（如 ### 业务逻辑迁移阶段），停止收集
			if strings.HasPrefix(nextTrimmed, "###") && !strings.HasPrefix(nextTrimmed, "####") {
				break
			}

			// 对于 Markdown 标题格式的步骤，收集所有非标题行的内容
			if isMarkdownHeader {
				stepContent += "\n" + nextLine
			} else {
				// 对于复选框格式，按缩进和内容类型判断
				nextIndent := leadingIndent(nextLine)

				// 收集缩进更大的行，或者步骤属性行（以 "- **" 开头的行）
				if nextIndent > stepIndent || strings.HasPrefix(nextTrimmed, "- **") {
					stepContent += "\n" + nextLine
				} else {
					// 如果缩进不够且不是属性行，说明不属于当前步骤
					break
				}
			}

			j++
		}

		// 解析包含多行内容的步骤
		step, err := parseStepLineWithContent(stepContent, defaultUpdated)
		if err != nil {
			return nil, nil, err
		}

		if _, exists := index[step.ID]; exists {
			return nil, nil, fmt.Errorf("duplicate step id %q: %w", step.ID, ErrDuplicateStepID)
		}
		index[step.ID] = step

		for len(stack) > 0 && indent <= stack[len(stack)-1].indent {
			stack = stack[:len(stack)-1]
		}

		if len(stack) == 0 {
			steps = append(steps, step)
		} else {
			parent := stack[len(stack)-1].step
			parent.SubSteps = append(parent.SubSteps, step)
		}

		stack = append(stack, stackEntry{indent: indent, step: step})

		// 更新循环索引，跳过已处理的行
		i = j - 1
	}

	return steps, index, nil
}

func parseStepLineWithContent(content string, defaultUpdated time.Time) (*models.Step, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return nil, fmt.Errorf("empty step content: %w", ErrInvalidPlanFormat)
	}

	// 第一行必须是步骤行
	firstLine := lines[0]

	var marker, body, id, firstLineDesc string
	var attrs map[string]string
	var err error

	// 尝试匹配复选框格式
	if matches := stepLinePattern.FindStringSubmatch(firstLine); len(matches) == 3 {
		marker = matches[1]
		body = matches[2]
		id, firstLineDesc, attrs, err = parseStepBody(body)
		if err != nil {
			return nil, err
		}
	} else if matches := stepHeaderPattern.FindStringSubmatch(firstLine); len(matches) == 3 {
		// 匹配 Markdown 标题格式
		stepNum := matches[1]
		title := matches[2]

		id = "step-" + stepNum
		firstLineDesc = title

		// 从标题中提取状态（如果有 ✅ 等标记）
		if strings.Contains(title, "✅") {
			marker = "x"
			firstLineDesc = strings.TrimSpace(strings.ReplaceAll(title, "✅", ""))
		} else {
			marker = " " // 默认为未完成状态
		}

		attrs = make(map[string]string)
	} else {
		return nil, fmt.Errorf("invalid step line format: %w", ErrInvalidPlanFormat)
	}

	// 构建完整的描述，包含多行内容
	var fullDescription strings.Builder
	fullDescription.WriteString(firstLineDesc)

	// 添加后续行的内容，并对 Markdown 格式进行特殊处理
	if len(lines) > 1 {
		for i := 1; i < len(lines); i++ {
			line := lines[i]
			trimmed := strings.TrimSpace(line)

			// 对于 Markdown 格式，尝试从后续行提取状态信息
			if stepHeaderPattern.MatchString(lines[0]) {
				if strings.HasPrefix(trimmed, "- **状态**:") {
					// 提取状态信息
					statusLine := strings.TrimPrefix(trimmed, "- **状态**:")
					statusValue := strings.TrimSpace(statusLine)
					if statusValue == "succeeded" {
						marker = "x"
					} else if statusValue == "in-progress" {
						marker = ">"
					} else if statusValue == "failed" {
						marker = "!"
					} else {
						marker = " "
					}
				}
			}

			// 保持原有的缩进格式
			fullDescription.WriteString("\n")
			fullDescription.WriteString(line)
		}
	}

	step := &models.Step{
		ID:          id,
		Status:      statusFromMarker(marker),
		Description: fullDescription.String(),
		UpdatedAt:   defaultUpdated,
	}

	if len(attrs) > 0 {
		applyStepAttributes(step, attrs)
	}

	if step.Description == "" {
		step.Description = id
	}

	return step, nil
}

func parseStepBody(body string) (string, string, map[string]string, error) {
	parts := strings.SplitN(body, ":", 2)
	if len(parts) != 2 {
		return "", "", nil, fmt.Errorf("step line missing description delimiter: %w", ErrInvalidPlanFormat)
	}

	stepID := strings.TrimSpace(parts[0])
	if stepID == "" {
		return "", "", nil, fmt.Errorf("step id cannot be empty: %w", ErrInvalidPlanFormat)
	}

	rest := strings.TrimSpace(parts[1])
	description, attrs := extractAttributes(rest)

	return stepID, description, attrs, nil
}

func extractAttributes(text string) (string, map[string]string) {
	attributes := make(map[string]string)
	matches := attributePattern.FindAllStringSubmatch(text, -1)
	cleaned := text
	for _, match := range matches {
		key := strings.ToLower(match[1])
		value := match[2]
		value = strings.Trim(value, "\"")
		attributes[key] = value
		cleaned = strings.ReplaceAll(cleaned, match[0], "")
	}

	return strings.TrimSpace(cleaned), attributes
}

func applyStepAttributes(step *models.Step, attrs map[string]string) {
	for key, value := range attrs {
		switch key {
		case "priority":
			step.Priority = priorityFromString(value)
		case "status":
			step.Status = statusFromString(value)
		case "description":
			step.Description = value
		case "output":
			step.Output = value
		case "started_at":
			if t, err := parseTimeValue(value); err == nil && !t.IsZero() {
				step.StartedAt = &t
			}
		case "completed_at":
			if t, err := parseTimeValue(value); err == nil && !t.IsZero() {
				step.CompletedAt = &t
			}
		case "updated_at":
			if t, err := parseTimeValue(value); err == nil && !t.IsZero() {
				step.UpdatedAt = t
			}
		default:
			if step.Metadata == nil {
				step.Metadata = make(map[string]any)
			}
			step.Metadata[key] = value
		}
	}
}

func statusFromMarker(marker string) models.StepStatus {
	switch strings.ToLower(marker) {
	case "x":
		return models.StepStatusSucceeded
	case "-":
		return models.StepStatusCancelled
	case ">", "~":
		return models.StepStatusInProgress
	case "!":
		return models.StepStatusFailed
	default:
		return models.StepStatusPending
	}
}

func statusFromString(value string) models.StepStatus {
	switch strings.ToLower(value) {
	case "pending":
		return models.StepStatusPending
	case "in-progress":
		return models.StepStatusInProgress
	case "succeeded":
		return models.StepStatusSucceeded
	case "failed":
		return models.StepStatusFailed
	case "cancelled":
		return models.StepStatusCancelled
	default:
		return models.StepStatusPending
	}
}

func priorityFromString(value string) models.StepPriority {
	switch strings.ToLower(value) {
	case "high":
		return models.StepPriorityHigh
	case "medium":
		return models.StepPriorityMedium
	case "low":
		return models.StepPriorityLow
	default:
		return models.StepPriority(strings.ToLower(value))
	}
}

func parseTimeValue(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}

	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", value, err)
	}

	return t, nil
}

type stackEntry struct {
	indent int
	step   *models.Step
}

func leadingIndent(line string) int {
	count := 0
	for _, r := range line {
		switch r {
		case ' ':
			count++
		case '\t':
			count += 4
		default:
			return count
		}
	}
	return count
}

func renderExecutionPlan(plan *models.ExecutionPlan) (string, error) {
	if plan == nil {
		return "", fmt.Errorf("render execution plan: plan is nil")
	}

	fm := frontMatterOutput{
		PlanID:    plan.PlanID,
		TaskID:    plan.TaskID,
		Status:    string(plan.Status),
		CreatedAt: plan.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: plan.UpdatedAt.UTC().Format(time.RFC3339),
	}

	if len(plan.Dependencies) > 0 {
		deps := make([]dependencyRecord, 0, len(plan.Dependencies))
		for _, dep := range plan.Dependencies {
			deps = append(deps, dependencyRecord{Source: dep.Source, Target: dep.Target})
		}
		fm.Dependencies = deps
	}

	if len(plan.Metadata) > 0 {
		extra := make(map[string]any, len(plan.Metadata))
		for key, value := range plan.Metadata {
			extra[key] = value
		}
		fm.Extra = extra
	}

	yamlBytes, err := yaml.Marshal(&fm)
	if err != nil {
		return "", fmt.Errorf("marshal front matter: %w", err)
	}

	var body strings.Builder
	renderSteps(plan.Steps, &body, 0)

	front := string(yamlBytes)
	if !strings.HasSuffix(front, "\n") {
		front += "\n"
	}

	var builder strings.Builder
	builder.WriteString("---\n")
	builder.WriteString(front)
	builder.WriteString("---\n")
	builder.WriteString(body.String())

	return builder.String(), nil
}

func renderSteps(steps []*models.Step, builder *strings.Builder, indentLevel int) {
	indent := strings.Repeat("    ", indentLevel)
	for _, step := range steps {
		if step == nil {
			continue
		}
		description := step.Description
		if description == "" {
			description = step.ID
		}

		builder.WriteString(indent)
		builder.WriteString("- [")
		builder.WriteString(markerFromStatus(step.Status))
		builder.WriteString("] ")
		builder.WriteString(step.ID)
		builder.WriteString(": ")
		builder.WriteString(description)

		if attrs := formatStepAttributes(step); attrs != "" {
			builder.WriteByte(' ')
			builder.WriteString(attrs)
		}
		builder.WriteByte('\n')

		if len(step.SubSteps) > 0 {
			renderSteps(step.SubSteps, builder, indentLevel+1)
		}
	}
}

func formatStepAttributes(step *models.Step) string {
	var attrs []string

	if step.Priority != "" {
		attrs = append(attrs, fmt.Sprintf("priority:%s", formatAttributeValue(string(step.Priority))))
	}
	if step.Output != "" {
		attrs = append(attrs, fmt.Sprintf("output:%s", formatAttributeValue(step.Output)))
	}
	if step.StartedAt != nil && !step.StartedAt.IsZero() {
		attrs = append(attrs, fmt.Sprintf("started_at:%s", formatAttributeValue(step.StartedAt.UTC().Format(time.RFC3339))))
	}
	if step.CompletedAt != nil && !step.CompletedAt.IsZero() {
		attrs = append(attrs, fmt.Sprintf("completed_at:%s", formatAttributeValue(step.CompletedAt.UTC().Format(time.RFC3339))))
	}
	if len(step.Metadata) > 0 {
		keys := make([]string, 0, len(step.Metadata))
		for key := range step.Metadata {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			value := fmt.Sprint(step.Metadata[key])
			attrs = append(attrs, fmt.Sprintf("%s:%s", key, formatAttributeValue(value)))
		}
	}

	return strings.Join(attrs, " ")
}

func formatAttributeValue(value string) string {
	if needsQuote(value) {
		// 只转义双引号，不转义换行符和反斜杠，避免双重转义问题
		escaped := strings.ReplaceAll(value, `"`, `\"`)
		return `"` + escaped + `"`
	}
	return value
}

func needsQuote(value string) bool {
	if value == "" {
		return true
	}
	return strings.ContainsAny(value, " \t\n\r:\"'")
}

func markerFromStatus(status models.StepStatus) string {
	switch status {
	case models.StepStatusSucceeded:
		return "x"
	case models.StepStatusInProgress:
		return ">"
	case models.StepStatusFailed:
		return "!"
	case models.StepStatusCancelled:
		return "-"
	default:
		return " "
	}
}

func recalculatePlanStatus(plan *models.ExecutionPlan, steps []*models.Step) {
	if plan == nil {
		return
	}
	if plan.Status == models.PlanStatusRejected {
		return
	}

	var (
		hasPending    bool
		hasInProgress bool
		hasFailed     bool
		hasSucceeded  bool
	)

	for _, step := range steps {
		if step == nil {
			continue
		}
		switch step.Status {
		case models.StepStatusPending:
			hasPending = true
		case models.StepStatusInProgress:
			hasInProgress = true
		case models.StepStatusFailed, models.StepStatusCancelled:
			hasFailed = true
		case models.StepStatusSucceeded:
			hasSucceeded = true
		}
	}

	switch {
	case hasFailed:
		plan.Status = models.PlanStatusFailed
		return
	case hasInProgress:
		plan.Status = models.PlanStatusExecuting
		return
	case hasPending:
		if plan.Status == models.PlanStatusCompleted || plan.Status == models.PlanStatusFailed || plan.Status == models.PlanStatusExecuting {
			plan.Status = models.PlanStatusApproved
		}
	default:
		if hasSucceeded {
			plan.Status = models.PlanStatusCompleted
		}
	}

	if hasSucceeded && (plan.Status == models.PlanStatusApproved || plan.Status == models.PlanStatusPendingApproval || plan.Status == models.PlanStatusDraft) {
		plan.Status = models.PlanStatusExecuting
	}
}

func flattenAndIndexSteps(steps []*models.Step) ([]*models.Step, map[string]*models.Step) {
	result := make([]*models.Step, 0)
	index := make(map[string]*models.Step)

	var walk func(nodes []*models.Step)
	walk = func(nodes []*models.Step) {
		for _, step := range nodes {
			if step == nil {
				continue
			}
			result = append(result, step)
			index[step.ID] = step
			if len(step.SubSteps) > 0 {
				walk(step.SubSteps)
			}
		}
	}

	walk(steps)
	return result, index
}

func buildDependencyMap(deps []models.Dependency) map[string][]string {
	if len(deps) == 0 {
		return nil
	}

	dependencyMap := make(map[string][]string)
	for _, dep := range deps {
		if dep.Source == "" || dep.Target == "" {
			continue
		}
		dependencyMap[dep.Source] = append(dependencyMap[dep.Source], dep.Target)
	}

	return dependencyMap
}

func dependenciesSatisfied(stepID string, dependencyMap map[string][]string, index map[string]*models.Step) bool {
	if len(dependencyMap) == 0 {
		return true
	}

	prerequisites := dependencyMap[stepID]
	if len(prerequisites) == 0 {
		return true
	}

	for _, prerequisite := range prerequisites {
		target, ok := index[prerequisite]
		if !ok {
			return false
		}
		if target.Status != models.StepStatusSucceeded {
			return false
		}
	}

	return true
}

func priorityWeight(priority models.StepPriority) int {
	switch priority {
	case models.StepPriorityHigh:
		return 3
	case models.StepPriorityMedium:
		return 2
	case models.StepPriorityLow:
		return 1
	case "":
		return 0
	default:
		return 0
	}
}

func compareStepIDs(a, b string) int {
	aKey := stepIDKey(a)
	bKey := stepIDKey(b)

	maxLen := len(aKey)
	if len(bKey) > maxLen {
		maxLen = len(bKey)
	}

	for i := 0; i < maxLen; i++ {
		var ai, bi int
		if i < len(aKey) {
			ai = aKey[i]
		}
		if i < len(bKey) {
			bi = bKey[i]
		}

		if ai != bi {
			if ai < bi {
				return -1
			}
			return 1
		}
	}

	return strings.Compare(strings.ToLower(a), strings.ToLower(b))
}

func stepIDKey(id string) []int {
	matches := stepNumericPattern.FindAllString(id, -1)
	if len(matches) == 0 {
		return []int{0}
	}

	key := make([]int, 0, len(matches))
	for _, match := range matches {
		value, err := strconv.Atoi(match)
		if err != nil {
			continue
		}
		key = append(key, value)
	}

	if len(key) == 0 {
		return []int{0}
	}

	return key
}

// HasEditPermission 检查用户是否有权限编辑执行计划。
// 权限条件：
// 1. 用户是任务的 Assignee
// 2. 用户具有 execution_plan.edit 或 task.write 权限
// 注意：项目 Owner/Admin 的检查应在调用层（Handler）基于项目元数据进行。
func HasEditPermission(userID string, taskAssignee string, userScopes []string) bool {
	// 1. 检查是否为任务负责人
	if userID != "" && taskAssignee != "" && userID == taskAssignee {
		return true
	}

	// 2. 检查是否具有 task.write 权限（包含执行计划编辑权限）
	for _, scope := range userScopes {
		if scope == "task.write" || scope == "execution_plan.edit" {
			return true
		}
	}

	return false
}

// IsEditableState 判断执行计划是否处于可编辑状态。
// 可编辑状态：Pending Approval、Rejected、Approved（有条件）、Executing（有条件）、Completed
// 不可编辑状态：Failed
func IsEditableState(status models.PlanStatus) bool {
	switch status {
	case models.PlanStatusPendingApproval, models.PlanStatusRejected:
		// 审批前和被拒绝后可以无条件编辑
		return true
	case models.PlanStatusApproved, models.PlanStatusExecuting, models.PlanStatusCompleted:
		// 已批准、执行中或已完成可以有条件编辑（仅 pending 状态的步骤）
		return true
	case models.PlanStatusFailed:
		// 失败状态不允许编辑
		return false
	default:
		// 未知状态默认不允许编辑
		return false
	}
}

// ProtectExecutedSteps 检查新计划是否修改了已执行或执行中的步骤。
// 在 Approved 或 Executing 状态下，只允许修改 pending 状态的步骤。
// 返回 error 如果检测到对已执行步骤的修改。
func ProtectExecutedSteps(currentPlan, newPlan *models.ExecutionPlan) error {
	if currentPlan == nil || newPlan == nil {
		return nil
	}

	// 只有在 Approved 或 Executing 状态下才需要保护已执行步骤
	if currentPlan.Status != models.PlanStatusApproved && currentPlan.Status != models.PlanStatusExecuting {
		return nil
	}

	// 构建当前计划中已执行步骤的映射
	executedSteps := make(map[string]*models.Step)
	currentSteps, _ := flattenAndIndexSteps(currentPlan.Steps)
	for _, step := range currentSteps {
		if step != nil && step.Status != models.StepStatusPending {
			executedSteps[step.ID] = step
		}
	}

	// 检查新计划中是否修改了已执行步骤
	newSteps, _ := flattenAndIndexSteps(newPlan.Steps)
	for _, newStep := range newSteps {
		if newStep == nil {
			continue
		}

		oldStep, exists := executedSteps[newStep.ID]
		if !exists {
			// 新步骤或仍为 pending 状态的步骤，允许修改
			continue
		}

		// 检查关键字段是否被修改
		if oldStep.Description != newStep.Description {
			return fmt.Errorf(
				"cannot modify step '%s': already executed or in-progress (status: %s)",
				newStep.ID,
				oldStep.Status,
			)
		}

		// 优先级的修改也应该被禁止
		if oldStep.Priority != newStep.Priority {
			return fmt.Errorf(
				"cannot modify priority of step '%s': already executed or in-progress (status: %s)",
				newStep.ID,
				oldStep.Status,
			)
		}
	}

	// 检查是否删除了已执行的步骤
	newStepIDs := make(map[string]bool)
	for _, step := range newSteps {
		if step != nil {
			newStepIDs[step.ID] = true
		}
	}

	for stepID := range executedSteps {
		if !newStepIDs[stepID] {
			return fmt.Errorf(
				"cannot delete step '%s': already executed or in-progress",
				stepID,
			)
		}
	}

	return nil
}

// UpdatePlanContent 是一个辅助函数，用于安全地更新执行计划内容。
// 它整合了权限检查、状态检查、格式校验、依赖图验证和步骤保护。
//
// 参数：
//   - svc: ExecutionPlanService 实例
//   - ctx: 上下文
//   - content: 新的计划内容（完整的 Markdown 文档）
//   - userID: 当前用户ID
//   - taskAssignee: 任务负责人
//   - userScopes: 用户权限范围
//
// 返回：
//   - 更新后的执行计划
//   - 错误（权限不足、状态不允许、格式错误、依赖图不合法或步骤保护失败）
func UpdatePlanContent(
	svc ExecutionPlanService,
	ctx context.Context,
	content string,
	userID string,
	taskAssignee string,
	userScopes []string,
) (*models.ExecutionPlan, error) {
	// 1. 权限检查
	if !HasEditPermission(userID, taskAssignee, userScopes) {
		return nil, fmt.Errorf("user %s does not have permission to edit execution plan", userID)
	}

	// 2. 加载当前计划
	currentPlan, err := svc.CurrentPlan()
	if err != nil {
		if errors.Is(err, ErrPlanNotLoaded) {
			// 如果计划尚未加载，尝试加载一次
			currentPlan, err = svc.Load(ctx)
			if err != nil {
				return nil, fmt.Errorf("update plan content: load current plan: %w", err)
			}
		} else {
			return nil, fmt.Errorf("update plan content: get current plan: %w", err)
		}
	}

	// 3. 状态检查
	if !IsEditableState(currentPlan.Status) {
		return nil, fmt.Errorf("plan in '%s' state cannot be edited", currentPlan.Status)
	}

	// 4. 解析新内容
	newPlan, err := parseExecutionPlan(content)
	if err != nil {
		return nil, fmt.Errorf("update plan content: invalid plan format: %w", err)
	}

	// 5. 依赖图合法性校验
	if err := validateDependencyGraph(newPlan); err != nil {
		return nil, fmt.Errorf("update plan content: %w", err)
	}

	// 6. 步骤状态保护（如果当前计划已部分执行）
	if currentPlan.Status == models.PlanStatusApproved || currentPlan.Status == models.PlanStatusExecuting {
		if err := ProtectExecutedSteps(currentPlan, newPlan); err != nil {
			return nil, fmt.Errorf("update plan content: %w", err)
		}
	}

	// 7. 更新计划
	updatedPlan, err := svc.UpdatePlan(ctx, content)
	if err != nil {
		return nil, fmt.Errorf("update plan content: %w", err)
	}

	return updatedPlan, nil
}

// validateDependencyGraph 验证依赖图的合法性，检测循环依赖。
func validateDependencyGraph(plan *models.ExecutionPlan) error {
	if plan == nil || len(plan.Dependencies) == 0 {
		return nil
	}

	// 构建步骤索引
	steps, stepIndex := flattenAndIndexSteps(plan.Steps)
	if len(steps) == 0 {
		return nil
	}

	// 构建依赖图（邻接表）
	graph := make(map[string][]string)
	inDegree := make(map[string]int)

	// 初始化所有步骤的入度为 0
	for _, step := range steps {
		if step != nil {
			inDegree[step.ID] = 0
		}
	}

	// 构建图并计算入度
	for _, dep := range plan.Dependencies {
		// 验证依赖引用的步骤是否存在
		if _, ok := stepIndex[dep.Source]; !ok {
			return fmt.Errorf("dependency source step '%s' not found: %w", dep.Source, ErrInvalidPlanFormat)
		}
		if _, ok := stepIndex[dep.Target]; !ok {
			return fmt.Errorf("dependency target step '%s' not found: %w", dep.Target, ErrInvalidPlanFormat)
		}

		graph[dep.Target] = append(graph[dep.Target], dep.Source)
		inDegree[dep.Source]++
	}

	// 使用拓扑排序检测循环依赖
	queue := make([]string, 0)
	for stepID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, stepID)
		}
	}

	processed := 0
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		processed++

		for _, dependent := range graph[current] {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	// 如果处理的步骤数少于总步骤数，说明存在循环依赖
	if processed < len(inDegree) {
		return fmt.Errorf("circular dependency detected in execution plan: %w", ErrInvalidPlanFormat)
	}

	return nil
}
