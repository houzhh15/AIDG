package executionplan

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var (
	// ErrInvalidIdentifier 表示项目或任务标识不合法。
	ErrInvalidIdentifier = errors.New("invalid identifier")
	// ErrProjectNotFound 表示项目目录不存在。
	ErrProjectNotFound = errors.New("project not found")
	// ErrTaskNotFound 表示任务目录不存在。
	ErrTaskNotFound = errors.New("task not found")
	// ErrPlanNotFound 表示执行计划文件不存在。
	ErrPlanNotFound = errors.New("execution plan not found")
)

const (
	defaultProjectsRoot = "projects"
	executionPlanFile   = "execution_plan.md"
	temporaryFileSuffix = ".tmp"
)

// FileRepository 实现 services.ExecutionPlanRepository，基于文件系统读写 plan。
type FileRepository struct {
	planPath string
	taskDir  string
}

// NewFileRepository 返回一个指向指定项目与任务的文件仓库。
func NewFileRepository(projectsRoot, projectID, taskID string) (*FileRepository, error) {
	sanitizedProject, err := sanitizeSegment(projectID)
	if err != nil {
		return nil, err
	}
	sanitizedTask, err := sanitizeSegment(taskID)
	if err != nil {
		return nil, err
	}

	root := projectsRoot
	if strings.TrimSpace(root) == "" {
		root = defaultProjectsRoot
	}

	projectPath := filepath.Join(root, sanitizedProject)
	if err := ensureDirectory(projectPath, ErrProjectNotFound); err != nil {
		return nil, err
	}

	taskDir := filepath.Join(projectPath, "tasks", sanitizedTask)
	if err := ensureDirectory(taskDir, ErrTaskNotFound); err != nil {
		return nil, err
	}

	planPath := filepath.Join(taskDir, executionPlanFile)
	return &FileRepository{planPath: planPath, taskDir: taskDir}, nil
}

// Read 读取并返回执行计划的完整内容。
func (r *FileRepository) Read(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	data, err := os.ReadFile(r.planPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", ErrPlanNotFound
		}
		return "", err
	}
	return string(data), nil
}

// Write 覆盖写入执行计划内容（原子写）。
func (r *FileRepository) Write(ctx context.Context, content string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := ensureDirectory(r.taskDir, ErrTaskNotFound); err != nil {
		return err
	}

	tmpPath := r.planPath + temporaryFileSuffix
	if err := os.WriteFile(tmpPath, []byte(content), 0o644); err != nil {
		return err
	}

	if err := os.Rename(tmpPath, r.planPath); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return nil
}

func sanitizeSegment(seg string) (string, error) {
	trimmed := strings.TrimSpace(seg)
	if trimmed == "" {
		return "", fmt.Errorf("%w: empty", ErrInvalidIdentifier)
	}
	if strings.Contains(trimmed, "..") {
		return "", fmt.Errorf("%w: contains '..'", ErrInvalidIdentifier)
	}
	if strings.ContainsAny(trimmed, "/\\") {
		return "", fmt.Errorf("%w: contains path separator", ErrInvalidIdentifier)
	}
	return trimmed, nil
}

func ensureDirectory(path string, notFoundErr error) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return notFoundErr
		}
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %s is not a directory", notFoundErr, path)
	}
	return nil
}
