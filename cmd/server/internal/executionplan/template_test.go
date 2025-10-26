package executionplan

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderDefault(t *testing.T) {
	generator := NewTemplateGenerator()
	taskID := "task_test_123"
	now := time.Date(2025, 10, 26, 12, 0, 0, 0, time.UTC)

	content, err := generator.RenderDefault(taskID, now)
	require.NoError(t, err)
	require.NotEmpty(t, content)

	// 测试内容包含 YAML frontmatter 分隔符
	assert.Contains(t, content, "---\n")
	assert.True(t, strings.Count(content, "---\n") >= 2, "should have YAML frontmatter delimiters")

	// 测试必需字段存在
	assert.Contains(t, content, "plan_id:")
	assert.Contains(t, content, `task_id: "task_test_123"`)
	assert.Contains(t, content, `status: "Draft"`)
	assert.Contains(t, content, "created_at:")
	assert.Contains(t, content, "updated_at:")
	assert.Contains(t, content, "dependencies: []")

	// 测试时间格式为 RFC3339
	expectedTime := now.Format(time.RFC3339)
	assert.Contains(t, content, expectedTime)

	// 测试示例步骤格式
	assert.Contains(t, content, "- [ ] step-01:")
	assert.Contains(t, content, "请用正式计划替换此示例步骤")
	assert.Contains(t, content, "priority:medium")
}

func TestRenderDefault_UUIDFormat(t *testing.T) {
	generator := NewTemplateGenerator()

	// 生成两次，确保 plan_id 不同
	content1, err1 := generator.RenderDefault("task1", time.Now())
	content2, err2 := generator.RenderDefault("task2", time.Now())

	require.NoError(t, err1)
	require.NoError(t, err2)
	assert.NotEqual(t, content1, content2, "each template should have unique plan_id")

	// 检查 UUID 格式（简单检查，包含连字符）
	assert.Contains(t, content1, "plan_id: \"")
	lines := strings.Split(content1, "\n")
	var planIDLine string
	for _, line := range lines {
		if strings.HasPrefix(line, "plan_id:") {
			planIDLine = line
			break
		}
	}
	assert.Contains(t, planIDLine, "-", "plan_id should be UUID format with hyphens")
}

// mockRepository 用于测试的内存仓库。
type mockRepository struct {
	content   string
	readErr   error
	writeErr  error
	writeCall bool
}

func (m *mockRepository) Read(ctx context.Context) (string, error) {
	if m.readErr != nil {
		return "", m.readErr
	}
	return m.content, nil
}

func (m *mockRepository) Write(ctx context.Context, content string) error {
	m.writeCall = true
	if m.writeErr != nil {
		return m.writeErr
	}
	m.content = content
	return nil
}

func TestEnsure_FileNotExists(t *testing.T) {
	generator := NewTemplateGenerator()
	repo := &mockRepository{readErr: ErrPlanNotFound}
	ctx := context.Background()

	err := generator.Ensure(ctx, repo, "task_123", TemplateOptions{Force: false})
	require.NoError(t, err)
	assert.True(t, repo.writeCall, "should write template when file not exists")
	assert.NotEmpty(t, repo.content)
	assert.Contains(t, repo.content, "task_123")
}

func TestEnsure_FileExists_NoForce(t *testing.T) {
	generator := NewTemplateGenerator()
	repo := &mockRepository{content: "existing content"}
	ctx := context.Background()

	err := generator.Ensure(ctx, repo, "task_456", TemplateOptions{Force: false})
	assert.ErrorIs(t, err, ErrPlanExists, "should return ErrPlanExists when file exists and force=false")
	assert.False(t, repo.writeCall, "should not write when file exists and force=false")
	assert.Equal(t, "existing content", repo.content, "content should remain unchanged")
}

func TestEnsure_FileExists_WithForce(t *testing.T) {
	generator := NewTemplateGenerator()
	repo := &mockRepository{content: "old content"}
	ctx := context.Background()

	err := generator.Ensure(ctx, repo, "task_789", TemplateOptions{Force: true})
	require.NoError(t, err)
	assert.True(t, repo.writeCall, "should write template when force=true")
	assert.NotEqual(t, "old content", repo.content, "content should be overwritten")
	assert.Contains(t, repo.content, "task_789")
}

func TestEnsure_ContextCancelled(t *testing.T) {
	generator := NewTemplateGenerator()
	repo := &mockRepository{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	err := generator.Ensure(ctx, repo, "task_999", TemplateOptions{})
	assert.Error(t, err)
	assert.False(t, repo.writeCall, "should not write when context cancelled")
}
