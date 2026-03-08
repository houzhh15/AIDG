package executionplan

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

var (
	// ErrPlanExists 表示执行计划文件已存在且不允许覆盖。
	ErrPlanExists = errors.New("execution plan already exists")
)

// ExecutionPlanRepository 定义执行计划的读写接口。
type ExecutionPlanRepository interface {
	Read(ctx context.Context) (string, error)
	Write(ctx context.Context, content string) error
}

// TemplateOptions 包含模板生成的配置选项。
type TemplateOptions struct {
	// Force 为 true 时强制覆盖已有文件。
	Force bool
	// Assignee 预留字段，用于未来自定义描述。
	Assignee string
}

// TemplateGenerator 负责生成执行计划的默认模板。
type TemplateGenerator struct{}

// NewTemplateGenerator 返回一个新的模板生成器实例。
func NewTemplateGenerator() *TemplateGenerator {
	return &TemplateGenerator{}
}

// Ensure 确保执行计划文件存在，若不存在或 force 标志为 true 则生成默认模板。
func (g *TemplateGenerator) Ensure(ctx context.Context, repo ExecutionPlanRepository, taskID string, opts TemplateOptions) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// 检查文件是否已存在
	_, err := repo.Read(ctx)
	if err == nil {
		// 文件存在
		if !opts.Force {
			return ErrPlanExists
		}
		// Force 模式，继续覆盖
	} else if !errors.Is(err, ErrPlanNotFound) {
		// 其他错误（非文件不存在）
		return fmt.Errorf("failed to check existing plan: %w", err)
	}

	// 生成默认模板内容
	content, err := g.RenderDefault(taskID, time.Now())
	if err != nil {
		return fmt.Errorf("failed to render default template: %w", err)
	}

	// 写入文件
	if err := repo.Write(ctx, content); err != nil {
		return fmt.Errorf("failed to write template: %w", err)
	}

	return nil
}

// RenderDefault 生成符合规范的默认执行计划 Markdown 内容。
func (g *TemplateGenerator) RenderDefault(taskID string, now time.Time) (string, error) {
	planID := uuid.New().String()
	timestamp := now.Format(time.RFC3339)

	template := fmt.Sprintf(`---
plan_id: "%s"
task_id: "%s"
status: "Draft"
created_at: "%s"
updated_at: "%s"
dependencies: []
---
- [ ] step-01: 请用正式计划替换此示例步骤。这是一个占位步骤，用于演示执行计划的格式。请根据实际任务需求编写具体的执行步骤。 priority:medium
`, planID, taskID, timestamp, timestamp)

	return template, nil
}
