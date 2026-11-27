package tasklifecycle

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/taskdocs"
)

const maxReviewBytes = 256 * 1024

type Service struct {
	root string
	docs *taskdocs.DocService
}

func NewService(projectsRoot string, docs *taskdocs.DocService) *Service {
	if docs == nil {
		docs = taskdocs.NewDocService()
	}
	return &Service{root: projectsRoot, docs: docs}
}

var projectLocks = struct {
	sync.Mutex
	m map[string]*sync.Mutex
}{m: make(map[string]*sync.Mutex)}

func lockProject(path string) func() {
	projectLocks.Lock()
	lock := projectLocks.m[path]
	if lock == nil {
		lock = &sync.Mutex{}
		projectLocks.m[path] = lock
	}
	projectLocks.Unlock()
	lock.Lock()
	return lock.Unlock
}

func validateID(value string) error {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 200 || value == "." || value == ".." || filepath.Base(value) != value || strings.ContainsAny(value, "/\\\x00") {
		return ErrInvalidID
	}
	return nil
}

func (s *Service) projectPath(projectID string) (string, error) {
	if err := validateID(projectID); err != nil {
		return "", err
	}
	root, err := filepath.Abs(s.root)
	if err != nil {
		return "", err
	}
	rootEval, err := evalExisting(root)
	if err != nil {
		return "", err
	}
	project := filepath.Join(root, projectID)
	projectEval, err := evalExisting(project)
	if err != nil {
		return "", err
	}
	if !within(rootEval, projectEval) {
		return "", ErrInvalidID
	}
	return projectEval, nil
}

func evalExisting(path string) (string, error) {
	clean := filepath.Clean(path)
	current := clean
	var suffix []string
	for {
		resolved, err := filepath.EvalSymlinks(current)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return filepath.Clean(resolved), nil
		}
		if !os.IsNotExist(err) {
			return "", err
		}
		parent := filepath.Dir(current)
		if parent == current {
			return clean, nil
		}
		suffix = append(suffix, filepath.Base(current))
		current = parent
	}
}

func within(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) && !filepath.IsAbs(rel)
}

func (s *Service) tasksPath(projectID string) (string, error) {
	project, err := s.projectPath(projectID)
	if err != nil {
		return "", err
	}
	return filepath.Join(project, "tasks.json"), nil
}

func readTasks(path string) ([]map[string]json.RawMessage, []byte, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return []map[string]json.RawMessage{}, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	var tasks []map[string]json.RawMessage
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, nil, fmt.Errorf("decode tasks: %w", err)
	}
	return tasks, data, nil
}

func encodeTasks(tasks []map[string]json.RawMessage) ([]byte, error) {
	data, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tasks-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() { _ = os.Remove(name) }()
	if err = tmp.Chmod(0o644); err == nil {
		_, err = tmp.Write(data)
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func rawString(task map[string]json.RawMessage, key string) string {
	var value string
	_ = json.Unmarshal(task[key], &value)
	return value
}

func rawInt(task map[string]json.RawMessage, key string) int {
	var value int
	_ = json.Unmarshal(task[key], &value)
	return value
}

func setRaw(task map[string]json.RawMessage, key string, value any) error {
	data, err := json.Marshal(value)
	if err == nil {
		task[key] = data
	}
	return err
}

func findTask(tasks []map[string]json.RawMessage, taskID string) (int, map[string]json.RawMessage) {
	for i, task := range tasks {
		if rawString(task, "id") == taskID {
			return i, task
		}
	}
	return -1, nil
}

func taskView(task map[string]json.RawMessage) (map[string]any, error) {
	data, err := json.Marshal(task)
	if err != nil {
		return nil, err
	}
	var view map[string]any
	if err := json.Unmarshal(data, &view); err != nil {
		return nil, err
	}
	if status, _ := view["status"].(string); status != "" {
		switch status {
		case "pending":
			view["status"] = StatusTodo
		case "in_progress":
			view["status"] = StatusInProgress
		case "done":
			view["status"] = StatusCompleted
		case "blocked":
			view["status"] = StatusReview
		}
	}
	return view, nil
}

func (s *Service) Create(projectID string, fields map[string]json.RawMessage) (map[string]any, error) {
	path, err := s.tasksPath(projectID)
	if err != nil {
		return nil, err
	}
	unlock := lockProject(path)
	defer unlock()
	if err = s.recoverReviewTransaction(projectID, path); err != nil {
		return nil, err
	}
	tasks, _, err := readTasks(path)
	if err != nil {
		return nil, err
	}
	if raw, supplied := fields["status"]; supplied {
		var status string
		if json.Unmarshal(raw, &status) != nil || status != StatusTodo {
			return nil, ErrInvalidStatus
		}
	}
	now := time.Now().UTC()
	task := make(map[string]json.RawMessage, len(fields)+6)
	for key, value := range fields {
		switch key {
		case "id", "revision", "created_at", "updated_at", "creator", "created_by", "modifier", "modified_by", "reviewer", "reviewed_by", "reviewed_at":
			continue
		default:
			task[key] = append(json.RawMessage(nil), value...)
		}
	}
	id := fmt.Sprintf("task_%d", now.UnixNano())
	if err = setRaw(task, "id", id); err != nil {
		return nil, err
	}
	_ = setRaw(task, "status", StatusTodo)
	_ = setRaw(task, "revision", 1)
	_ = setRaw(task, "created_at", now.Format(time.RFC3339Nano))
	_ = setRaw(task, "updated_at", now.Format(time.RFC3339Nano))
	tasks = append(tasks, task)
	encoded, err := encodeTasks(tasks)
	if err != nil {
		return nil, err
	}
	if err = atomicWrite(path, encoded); err != nil {
		return nil, err
	}
	return taskView(task)
}

func (s *Service) List(projectID string) ([]map[string]any, error) {
	path, err := s.tasksPath(projectID)
	if err != nil {
		return nil, err
	}
	unlock := lockProject(path)
	defer unlock()
	if err := ensureNoPendingReviewTransaction(path); err != nil {
		return nil, err
	}
	tasks, _, err := readTasks(path)
	if err != nil {
		return nil, err
	}
	views := make([]map[string]any, 0, len(tasks))
	for _, task := range tasks {
		view, err := taskView(task)
		if err != nil {
			return nil, err
		}
		readiness, err := s.readiness(projectID, rawString(task, "id"))
		if err != nil {
			return nil, err
		}
		view["ready_count"] = readiness.ReadyCount
		view["constraint_readiness"] = readiness
		views = append(views, view)
	}
	return views, nil
}

func (s *Service) Get(projectID, taskID string) (map[string]any, error) {
	if err := validateID(taskID); err != nil {
		return nil, err
	}
	path, err := s.tasksPath(projectID)
	if err != nil {
		return nil, err
	}
	unlock := lockProject(path)
	defer unlock()
	if err := ensureNoPendingReviewTransaction(path); err != nil {
		return nil, err
	}
	tasks, _, err := readTasks(path)
	if err != nil {
		return nil, err
	}
	_, task := findTask(tasks, taskID)
	if task == nil {
		return nil, ErrNotFound
	}
	view, err := taskView(task)
	if err != nil {
		return nil, err
	}
	readiness, err := s.readiness(projectID, taskID)
	if err != nil {
		return nil, err
	}
	view["ready_count"] = readiness.ReadyCount
	view["constraint_readiness"] = readiness
	return view, nil
}

func (s *Service) readiness(projectID, taskID string) (Readiness, error) {
	if err := validateID(taskID); err != nil {
		return Readiness{}, err
	}
	project, err := s.projectPath(projectID)
	if err != nil {
		return Readiness{}, err
	}
	result := Readiness{}
	for slot, ready := range map[string]*bool{
		"requirements": &result.Requirements,
		"design":       &result.Design,
		"test":         &result.Test,
	} {
		content, _, err := taskdocs.ExportReadOnlyWithPath(filepath.Join(project, "tasks", taskID, "docs", slot))
		if err != nil {
			return Readiness{}, fmt.Errorf("read %s: %w", slot, err)
		}
		*ready = strings.TrimSpace(content) != ""
		if *ready {
			result.ReadyCount++
		}
	}
	return result, nil
}

func (s *Service) Update(projectID, taskID string, fields map[string]json.RawMessage, expectedRevision *int) (map[string]any, error) {
	if err := validateID(taskID); err != nil {
		return nil, err
	}
	path, err := s.tasksPath(projectID)
	if err != nil {
		return nil, err
	}
	unlock := lockProject(path)
	defer unlock()
	if err := s.recoverReviewTransaction(projectID, path); err != nil {
		return nil, err
	}
	tasks, _, err := readTasks(path)
	if err != nil {
		return nil, err
	}
	index, task := findTask(tasks, taskID)
	if task == nil {
		return nil, ErrNotFound
	}
	currentRevision := rawInt(task, "revision")
	if currentRevision == 0 {
		currentRevision = 1
	}
	if expectedRevision != nil && *expectedRevision != currentRevision {
		return nil, ErrVersionConflict
	}
	if raw, ok := fields["status"]; ok {
		var next string
		if json.Unmarshal(raw, &next) != nil || !ValidStatus(next) {
			return nil, ErrInvalidStatus
		}
		current := rawString(task, "status")
		if !ValidStatus(current) || !CanTransition(current, next) {
			return nil, ErrInvalidTransition
		}
		if next == StatusCompleted {
			return nil, ErrReviewRequired
		}
	}
	for key, value := range fields {
		switch key {
		case "id", "created_at", "revision", "creator", "created_by", "modifier", "modified_by", "reviewer", "reviewed_by", "reviewed_at":
			continue
		default:
			task[key] = append(json.RawMessage(nil), value...)
		}
	}
	_ = setRaw(task, "revision", currentRevision+1)
	_ = setRaw(task, "updated_at", time.Now().UTC().Format(time.RFC3339Nano))
	tasks[index] = task
	encoded, err := encodeTasks(tasks)
	if err != nil {
		return nil, err
	}
	if err = atomicWrite(path, encoded); err != nil {
		return nil, err
	}
	return taskView(task)
}

func (s *Service) Delete(projectID, taskID string) error {
	if err := validateID(taskID); err != nil {
		return err
	}
	path, err := s.tasksPath(projectID)
	if err != nil {
		return err
	}
	unlock := lockProject(path)
	defer unlock()
	if err = s.recoverReviewTransaction(projectID, path); err != nil {
		return err
	}
	tasks, _, err := readTasks(path)
	if err != nil {
		return err
	}
	index, _ := findTask(tasks, taskID)
	if index < 0 {
		return ErrNotFound
	}
	tasks = append(tasks[:index], tasks[index+1:]...)
	encoded, err := encodeTasks(tasks)
	if err != nil {
		return err
	}
	project, err := s.projectPath(projectID)
	if err != nil {
		return err
	}
	taskDir := filepath.Join(project, "tasks", taskID)
	resolved, err := evalExisting(taskDir)
	if err != nil {
		return err
	}
	if !within(project, resolved) {
		return ErrInvalidID
	}
	if err = atomicWrite(path, encoded); err != nil {
		return err
	}
	// Task metadata is the authority. A failed best-effort directory cleanup
	// leaves unreachable historical files rather than resurrecting the task.
	_ = os.RemoveAll(taskDir)
	return nil
}

func formatReviewRequest(targetType, targetPath, criteria string) (string, error) {
	if targetType != "文件" && targetType != "AIDG文档" && targetType != "代码" {
		return "", ErrInvalidInput
	}
	if strings.TrimSpace(targetPath) == "" || strings.TrimSpace(criteria) == "" || strings.ContainsAny(targetType+targetPath, "\r\n") {
		return "", ErrInvalidInput
	}
	content := fmt.Sprintf("# 审核文档\n\n## 审核请求\n\n### 目标\n- 类型: %s\n- 路径: %s\n\n### 标准\n%s\n\n## 审核结果\n\n### 问题列表\n_暂无_\n\n### 结果\n", targetType, targetPath, criteria)
	if len(content) > maxReviewBytes || !utf8.ValidString(content) {
		return "", ErrInvalidInput
	}
	return content, nil
}

func reviewVerdict(content string) string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) != "### 结果" {
			continue
		}
		for _, candidate := range lines[i+1:] {
			candidate = strings.TrimSpace(candidate)
			if candidate == "" {
				continue
			}
			if candidate == ReviewPassed || candidate == ReviewNotPassed {
				return candidate
			}
			return ""
		}
	}
	return ""
}

func updateReviewContent(content string, req ReviewResultRequest) (string, error) {
	if req.Result != ReviewPassed && req.Result != ReviewNotPassed {
		return "", ErrInvalidInput
	}
	issues := strings.TrimSpace(req.Issues)
	if issues == "" {
		issues = "_暂无_"
	}
	if issues != "_暂无_" {
		for _, line := range strings.Split(issues, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "- ") {
				return "", ErrInvalidInput
			}
		}
	}
	resultSection := strings.LastIndex(content, "## 审核结果")
	issuesIndex := strings.LastIndex(content, "### 问题列表")
	resultIndex := strings.LastIndex(content, "### 结果")
	if resultSection < 0 || issuesIndex < resultSection || resultIndex < issuesIndex || !strings.Contains(content, "## 审核请求") {
		return "", ErrInvalidInput
	}
	prefix := content[:resultSection]
	if executionIndex := strings.Index(prefix, "## 执行结果"); executionIndex >= 0 {
		prefix = prefix[:executionIndex]
	}
	oldIssues := strings.TrimSpace(content[issuesIndex+len("### 问题列表") : resultIndex])
	oldResult := strings.TrimSpace(content[resultIndex+len("### 结果"):])
	history := ""
	if historyIndex := strings.Index(content[resultSection:issuesIndex], "### 审核历史"); historyIndex >= 0 {
		history = strings.TrimSpace(content[resultSection+historyIndex+len("### 审核历史") : issuesIndex])
	}
	if oldResult == ReviewPassed || oldResult == ReviewNotPassed {
		round := strings.Count(history, "#### 第") + 1
		entry := fmt.Sprintf("#### 第%d次审核\n问题: %s\n结果: %s", round, defaultText(oldIssues), oldResult)
		if history != "" {
			history += "\n\n"
		}
		history += entry
	}
	if strings.TrimSpace(req.ChangeSummary) != "" || strings.TrimSpace(req.Verification) != "" {
		prefix += fmt.Sprintf("## 执行结果\n\n### 变更摘要\n%s\n\n### 验证\n%s\n\n", defaultText(req.ChangeSummary), defaultText(req.Verification))
	}
	next := prefix + "## 审核结果\n\n"
	if history != "" {
		next += "### 审核历史\n" + history + "\n\n"
	}
	next += "### 问题列表\n" + issues + "\n\n### 结果\n" + req.Result + "\n"
	if len(next) > maxReviewBytes || !utf8.ValidString(next) {
		return "", ErrInvalidInput
	}
	return next, nil
}

func defaultText(value string) string {
	if strings.TrimSpace(value) == "" {
		return "_未提供_"
	}
	return value
}

func ensureNoPendingReviewTransaction(tasksPath string) error {
	_, err := os.Stat(reviewTransactionPath(tasksPath))
	if err == nil {
		return ErrVersionConflict
	}
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

type reviewTransaction struct {
	Protocol              int    `json:"protocol"`
	TaskID                string `json:"task_id"`
	TargetTasks           []byte `json:"target_tasks"`
	ReviewContent         string `json:"review_content"`
	ExpectedReviewVersion int    `json:"expected_review_version"`
	Actor                 string `json:"actor"`
}

func reviewTransactionPath(tasksPath string) string {
	return filepath.Join(filepath.Dir(tasksPath), ".task-review-transaction.json")
}

func (s *Service) persistReviewTransaction(path string, tx reviewTransaction) error {
	data, err := json.MarshalIndent(tx, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(reviewTransactionPath(path), append(data, '\n'))
}

func (s *Service) recoverReviewTransaction(projectID, tasksPath string) error {
	transactionPath := reviewTransactionPath(tasksPath)
	data, err := os.ReadFile(transactionPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var tx reviewTransaction
	if json.Unmarshal(data, &tx) != nil || tx.Protocol != 1 || validateID(tx.TaskID) != nil || tx.Actor == "" || len(tx.ReviewContent) == 0 || !utf8.ValidString(tx.ReviewContent) {
		return errors.New("invalid review transaction")
	}
	var target []map[string]json.RawMessage
	if json.Unmarshal(tx.TargetTasks, &target) != nil {
		return errors.New("invalid target task state")
	}
	project, err := s.projectPath(projectID)
	if err != nil {
		return err
	}
	reviewPath := filepath.Join(project, "tasks", tx.TaskID, "docs", "review")
	expected := tx.ExpectedReviewVersion
	content, meta, err := taskdocs.ExportWithPath(reviewPath)
	if err != nil {
		return err
	}
	if content != tx.ReviewContent {
		if meta.Version != expected {
			return ErrVersionConflict
		}
		if _, _, _, err = s.docs.AppendWithPath(reviewPath, tx.ReviewContent, "replace_full", tx.Actor, "review_api", &expected); err != nil {
			if err.Error() == "version_mismatch" {
				return ErrVersionConflict
			}
			return err
		}
	}
	current, err := os.ReadFile(tasksPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if !bytes.Equal(current, tx.TargetTasks) {
		if err = atomicWrite(tasksPath, tx.TargetTasks); err != nil {
			return err
		}
	}
	if err = os.Remove(transactionPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (s *Service) RequestReview(projectID, taskID, actor string, req ReviewRequest) (map[string]any, *ReviewDocument, error) {
	if err := validateID(taskID); err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(actor) == "" {
		return nil, nil, ErrUnauthorized
	}
	content, err := formatReviewRequest(req.TargetType, req.TargetPath, req.Criteria)
	if err != nil {
		return nil, nil, err
	}
	return s.commitReview(projectID, taskID, actor, content, req.ExpectedRevision, false, "")
}

func (s *Service) RequestReviewContent(projectID, taskID, actor, content string, expectedRevision *int) (map[string]any, *ReviewDocument, error) {
	if err := validateID(taskID); err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(actor) == "" {
		return nil, nil, ErrUnauthorized
	}
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if len(content) == 0 || len(content) > maxReviewBytes || !utf8.ValidString(content) {
		return nil, nil, ErrInvalidInput
	}
	typePrefix, pathPrefix := "- 类型: ", "- 路径: "
	typeIndex, pathIndex := strings.Index(content, typePrefix), strings.Index(content, pathPrefix)
	criteriaHeader, resultHeader := "### 标准\n", "\n\n## 审核结果"
	criteriaIndex, resultIndex := strings.Index(content, criteriaHeader), strings.Index(content, resultHeader)
	if typeIndex < 0 || pathIndex < 0 || criteriaIndex < 0 || resultIndex <= criteriaIndex {
		return nil, nil, ErrInvalidInput
	}
	targetType := strings.TrimSpace(strings.SplitN(content[typeIndex+len(typePrefix):], "\n", 2)[0])
	targetPath := strings.TrimSpace(strings.SplitN(content[pathIndex+len(pathPrefix):], "\n", 2)[0])
	criteria := content[criteriaIndex+len(criteriaHeader) : resultIndex]
	canonical, err := formatReviewRequest(targetType, targetPath, criteria)
	if err != nil || content != canonical {
		return nil, nil, ErrInvalidInput
	}
	return s.commitReview(projectID, taskID, actor, content, expectedRevision, false, "")
}

func (s *Service) SetReviewResult(projectID, taskID, actor string, req ReviewResultRequest) (map[string]any, *ReviewDocument, error) {
	if err := validateID(taskID); err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(actor) == "" {
		return nil, nil, ErrUnauthorized
	}
	path, err := s.tasksPath(projectID)
	if err != nil {
		return nil, nil, err
	}
	unlock := lockProject(path)
	defer unlock()
	if err = s.recoverReviewTransaction(projectID, path); err != nil {
		return nil, nil, err
	}
	project, err := s.projectPath(projectID)
	if err != nil {
		return nil, nil, err
	}
	content, _, err := taskdocs.ExportWithPath(filepath.Join(project, "tasks", taskID, "docs", "review"))
	if err != nil {
		return nil, nil, err
	}
	if strings.TrimSpace(content) == "" {
		return nil, nil, ErrReviewNotFound
	}
	if req.Result == ReviewPassed && (strings.TrimSpace(req.ChangeSummary) == "" || strings.TrimSpace(req.Verification) == "") {
		return nil, nil, ErrInvalidInput
	}
	next, err := updateReviewContent(content, req)
	if err != nil {
		return nil, nil, err
	}
	return s.commitReviewLocked(projectID, taskID, actor, next, req.ExpectedRevision, true, req.Result, path)
}

// LifecycleEvidenceHash returns the canonical digest for imported lifecycle
// facts. Provenance timestamps and the authenticated AIDG actor are excluded so
// retries and round trips preserve the same business-fact hash.
func LifecycleEvidenceHash(value LifecycleImport) (string, error) {
	value.EvidenceHash = ""
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func isLifecycleContractHeading(line string) bool {
	switch line {
	case "## 审核请求", "## 执行结果", "## 审核结果",
		"### 目标", "### 标准", "### 变更摘要", "### 验证", "### 遗留问题", "### 问题列表", "### 结果":
		return true
	default:
		return false
	}
}

func strictLifecycleSection(content, header string) (string, error) {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	marker := "### " + header
	start := -1
	for index, line := range lines {
		if strings.TrimSpace(line) != marker {
			continue
		}
		if start >= 0 {
			return "", ErrInvalidInput
		}
		start = index + 1
	}
	if start < 0 {
		return "", ErrInvalidInput
	}
	end := len(lines)
	for index := start; index < len(lines); index++ {
		if isLifecycleContractHeading(strings.TrimSpace(lines[index])) {
			end = index
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n")), nil
}

// ImportLifecycle atomically imports a non-session lifecycle fact. It is only
// reachable from the privileged sync publish resource; normal Update continues
// to reject completed and therefore cannot bypass the review gate.
func (s *Service) ImportLifecycle(projectID, taskID, actor string, expectedRevision *int, value LifecycleImport) (map[string]any, error) {
	if validateID(taskID) != nil || strings.TrimSpace(actor) == "" {
		return nil, ErrUnauthorized
	}
	if !ValidStatus(value.Status) || value.Source.System != "mote" || strings.TrimSpace(value.Source.ProjectID) == "" || strings.TrimSpace(value.Source.TaskID) == "" || value.Source.Actor != "mote" {
		return nil, ErrInvalidInput
	}
	if len(value.Review.Content) > maxReviewBytes || !utf8.ValidString(value.Review.Content) {
		return nil, ErrInvalidInput
	}
	expectedHash, err := LifecycleEvidenceHash(value)
	if err != nil || value.EvidenceHash != expectedHash {
		return nil, ErrInvalidInput
	}
	if value.Status == StatusCompleted {
		if strings.TrimSpace(value.Review.Content) == "" {
			return nil, ErrReviewRequired
		}
		verdict, verdictErr := strictLifecycleSection(value.Review.Content, "结果")
		summary, summaryErr := strictLifecycleSection(value.Review.Content, "变更摘要")
		verification, verificationErr := strictLifecycleSection(value.Review.Content, "验证")
		if verdictErr != nil || summaryErr != nil || verificationErr != nil {
			return nil, ErrInvalidInput
		}
		if value.Review.Verdict != ReviewPassed || verdict != ReviewPassed {
			return nil, ErrReviewRequired
		}
		if summary == "" || summary == "_未提供_" || verification == "" || verification == "_未提供_" || summary != strings.TrimSpace(value.Review.ChangeSummary) || verification != strings.TrimSpace(value.Review.Verification) {
			return nil, ErrInvalidInput
		}
	} else if value.Status == StatusReview {
		if strings.TrimSpace(value.Review.Content) == "" {
			return nil, ErrReviewRequired
		}
		verdict, verdictErr := strictLifecycleSection(value.Review.Content, "结果")
		if verdictErr != nil || verdict != "" || value.Review.Verdict != "" {
			return nil, ErrInvalidInput
		}
	} else if value.Review.Content != "" && value.Review.Verdict != "" {
		verdict, verdictErr := strictLifecycleSection(value.Review.Content, "结果")
		if verdictErr != nil || verdict != value.Review.Verdict || verdict == ReviewPassed {
			return nil, ErrInvalidInput
		}
	}

	path, err := s.tasksPath(projectID)
	if err != nil {
		return nil, err
	}
	unlock := lockProject(path)
	defer unlock()
	if err = s.recoverReviewTransaction(projectID, path); err != nil {
		return nil, err
	}
	tasks, _, err := readTasks(path)
	if err != nil {
		return nil, err
	}
	index, task := findTask(tasks, taskID)
	if task == nil {
		return nil, ErrNotFound
	}
	currentRevision := rawInt(task, "revision")
	if currentRevision == 0 {
		currentRevision = 1
	}
	if expectedRevision != nil && *expectedRevision != currentRevision {
		return nil, ErrVersionConflict
	}
	now := time.Now().UTC()
	_ = setRaw(task, "status", value.Status)
	_ = setRaw(task, "reviewed_by", actor)
	_ = setRaw(task, "reviewed_at", now.Format(time.RFC3339Nano))
	_ = setRaw(task, "sync_lifecycle_source", value.Source)
	_ = setRaw(task, "sync_lifecycle_evidence_hash", value.EvidenceHash)
	_ = setRaw(task, "revision", currentRevision+1)
	_ = setRaw(task, "updated_at", now.Format(time.RFC3339Nano))
	tasks[index] = task
	targetTasks, err := encodeTasks(tasks)
	if err != nil {
		return nil, err
	}
	if value.Review.Content == "" {
		if err = atomicWrite(path, targetTasks); err != nil {
			return nil, err
		}
	} else {
		project, pathErr := s.projectPath(projectID)
		if pathErr != nil {
			return nil, pathErr
		}
		reviewPath := filepath.Join(project, "tasks", taskID, "docs", "review")
		_, meta, exportErr := taskdocs.ExportWithPath(reviewPath)
		if exportErr != nil {
			return nil, exportErr
		}
		tx := reviewTransaction{Protocol: 1, TaskID: taskID, TargetTasks: targetTasks, ReviewContent: value.Review.Content, ExpectedReviewVersion: meta.Version, Actor: actor}
		if err = s.persistReviewTransaction(path, tx); err != nil {
			return nil, err
		}
		if err = s.recoverReviewTransaction(projectID, path); err != nil {
			return nil, err
		}
	}
	return taskView(task)
}
func (s *Service) commitReview(projectID, taskID, actor, content string, expectedRevision *int, result bool, verdict string) (map[string]any, *ReviewDocument, error) {
	path, err := s.tasksPath(projectID)
	if err != nil {
		return nil, nil, err
	}
	unlock := lockProject(path)
	defer unlock()
	if err = s.recoverReviewTransaction(projectID, path); err != nil {
		return nil, nil, err
	}
	return s.commitReviewLocked(projectID, taskID, actor, content, expectedRevision, result, verdict, path)
}

func (s *Service) commitReviewLocked(projectID, taskID, actor, content string, expectedRevision *int, result bool, verdict, tasksPath string) (map[string]any, *ReviewDocument, error) {
	tasks, _, err := readTasks(tasksPath)
	if err != nil {
		return nil, nil, err
	}
	index, task := findTask(tasks, taskID)
	if task == nil {
		return nil, nil, ErrNotFound
	}
	currentRevision := rawInt(task, "revision")
	if currentRevision == 0 {
		currentRevision = 1
	}
	if expectedRevision != nil && *expectedRevision != currentRevision {
		return nil, nil, ErrVersionConflict
	}
	currentStatus := rawString(task, "status")
	if !result {
		if currentStatus != StatusInProgress || !CanTransition(currentStatus, StatusReview) {
			return nil, nil, ErrInvalidTransition
		}
		_ = setRaw(task, "status", StatusReview)
	} else {
		if currentStatus != StatusReview {
			return nil, nil, ErrInvalidTransition
		}
		if verdict == ReviewPassed {
			_ = setRaw(task, "status", StatusCompleted)
		} else if verdict != ReviewNotPassed {
			return nil, nil, ErrInvalidInput
		}
	}
	now := time.Now().UTC()
	_ = setRaw(task, "reviewed_by", actor)
	_ = setRaw(task, "reviewed_at", now.Format(time.RFC3339Nano))
	_ = setRaw(task, "revision", currentRevision+1)
	_ = setRaw(task, "updated_at", now.Format(time.RFC3339Nano))
	tasks[index] = task
	targetTasks, err := encodeTasks(tasks)
	if err != nil {
		return nil, nil, err
	}
	project, err := s.projectPath(projectID)
	if err != nil {
		return nil, nil, err
	}
	reviewPath := filepath.Join(project, "tasks", taskID, "docs", "review")
	_, meta, err := taskdocs.ExportWithPath(reviewPath)
	if err != nil {
		return nil, nil, err
	}
	tx := reviewTransaction{Protocol: 1, TaskID: taskID, TargetTasks: targetTasks, ReviewContent: content, ExpectedReviewVersion: meta.Version, Actor: actor}
	if err = s.persistReviewTransaction(tasksPath, tx); err != nil {
		return nil, nil, err
	}
	if err = s.recoverReviewTransaction(projectID, tasksPath); err != nil {
		return nil, nil, err
	}
	view, err := taskView(task)
	if err != nil {
		return nil, nil, err
	}
	review, err := s.reviewUnlocked(projectID, taskID)
	return view, review, err
}

func (s *Service) Review(projectID, taskID string) (*ReviewDocument, error) {
	if err := validateID(taskID); err != nil {
		return nil, err
	}
	path, err := s.tasksPath(projectID)
	if err != nil {
		return nil, err
	}
	unlock := lockProject(path)
	defer unlock()
	if err = ensureNoPendingReviewTransaction(path); err != nil {
		return nil, err
	}
	return s.reviewUnlocked(projectID, taskID)
}

func (s *Service) reviewUnlocked(projectID, taskID string) (*ReviewDocument, error) {
	project, err := s.projectPath(projectID)
	if err != nil {
		return nil, err
	}
	content, meta, err := taskdocs.ExportReadOnlyWithPath(filepath.Join(project, "tasks", taskID, "docs", "review"))
	if err != nil {
		return nil, err
	}
	return &ReviewDocument{Content: content, Version: meta.Version, ETag: meta.ETag, UpdatedAt: meta.UpdatedAt, Exists: content != "" || meta.Version > 0}, nil
}

func ContentETag(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:])
}
