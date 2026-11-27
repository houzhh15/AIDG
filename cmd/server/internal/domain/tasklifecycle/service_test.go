package tasklifecycle

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/taskdocs"
)

func rawFields(t *testing.T, value map[string]any) map[string]json.RawMessage {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]json.RawMessage
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func newTestService(t *testing.T) (*Service, string) {
	t.Helper()
	root := t.TempDir()
	project := "project-one"
	if err := os.MkdirAll(filepath.Join(root, project), 0o755); err != nil {
		t.Fatal(err)
	}
	return NewService(root, taskdocs.NewDocService()), project
}

func createTask(t *testing.T, service *Service, project string) map[string]any {
	t.Helper()
	task, err := service.Create(project, rawFields(t, map[string]any{
		"name":        "task",
		"reviewed_by": "spoofed-client",
		"reviewer":    "spoofed-reviewer",
		"creator":     "spoofed-creator",
		"modifier":    "spoofed-modifier",
		"extension":   map[string]any{"nested": []any{"kept", 7}},
	}))
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestCreateDefaultsAndHasNoExecutionArtifacts(t *testing.T) {
	service, project := newTestService(t)
	task := createTask(t, service, project)
	if task["status"] != StatusTodo || int(task["revision"].(float64)) != 1 {
		t.Fatalf("unexpected task lifecycle fields: %#v", task)
	}
	for _, field := range []string{"reviewed_by", "reviewer", "creator", "modifier"} {
		if _, spoofed := task[field]; spoofed {
			t.Fatalf("client supplied identity field %s became authoritative: %#v", field, task)
		}
	}
	taskID := task["id"].(string)
	for _, forbidden := range []string{
		filepath.Join(service.root, project, "tasks", taskID, "execution-plan"),
		filepath.Join(service.root, project, "current-task.json"),
		filepath.Join(service.root, project, "workflow"),
	} {
		if _, err := os.Stat(forbidden); !os.IsNotExist(err) {
			t.Fatalf("unexpected creation side effect %s", forbidden)
		}
	}
	if _, err := os.Stat(filepath.Join(service.root, project, "tasks", taskID)); !os.IsNotExist(err) {
		t.Fatalf("task creation must not create a task directory")
	}

	_, err := service.Create(project, rawFields(t, map[string]any{"status": "in_progress"}))
	if !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("legacy status accepted: %v", err)
	}
}

func TestTransitionMatrix(t *testing.T) {
	statuses := []string{StatusTodo, StatusInProgress, StatusReview, StatusCompleted}
	allowed := map[[2]string]bool{
		{StatusTodo, StatusInProgress}:   true,
		{StatusInProgress, StatusReview}: true,
		{StatusReview, StatusInProgress}: true,
		{StatusReview, StatusCompleted}:  true,
	}
	for _, from := range statuses {
		for _, to := range statuses {
			if got := CanTransition(from, to); got != allowed[[2]string{from, to}] {
				t.Errorf("CanTransition(%q,%q)=%v", from, to, got)
			}
		}
	}
}

func TestUpdatePreservesUnknownFieldsAndRejectsCompletion(t *testing.T) {
	service, project := newTestService(t)
	task := createTask(t, service, project)
	taskID := task["id"].(string)
	revision := 1
	updated, err := service.Update(project, taskID, rawFields(t, map[string]any{"status": StatusInProgress, "name": "renamed"}), &revision)
	if err != nil {
		t.Fatal(err)
	}
	if updated["extension"].(map[string]any)["nested"].([]any)[0] != "kept" {
		t.Fatalf("unknown field lost: %#v", updated)
	}
	stale := 1
	if _, err = service.Update(project, taskID, rawFields(t, map[string]any{"name": "stale"}), &stale); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale update error=%v", err)
	}
	if _, err = service.Update(project, taskID, rawFields(t, map[string]any{"status": StatusReview}), nil); err != nil {
		t.Fatal(err)
	}
	if _, err = service.Update(project, taskID, rawFields(t, map[string]any{"status": StatusCompleted}), nil); !errors.Is(err, ErrReviewRequired) {
		t.Fatalf("direct completion error=%v", err)
	}
	observed, err := service.Get(project, taskID)
	if err != nil || observed["status"] != StatusReview {
		t.Fatalf("illegal transition changed task: %#v, %v", observed, err)
	}
}

func TestReadinessIsDerivedAndDoesNotChangeStatus(t *testing.T) {
	service, project := newTestService(t)
	task := createTask(t, service, project)
	taskID := task["id"].(string)
	projectPath, _ := service.projectPath(project)
	docs := taskdocs.NewDocService()
	for slot, content := range map[string]string{"requirements": "requirements", "design": "  \n", "test": "tests"} {
		path := filepath.Join(projectPath, "tasks", taskID, "docs", slot)
		if _, _, _, err := docs.AppendWithPath(path, content, "replace_full", "tester", "test", nil); err != nil {
			t.Fatal(err)
		}
	}
	observed, err := service.Get(project, taskID)
	if err != nil {
		t.Fatal(err)
	}
	if observed["ready_count"].(int) != 2 || observed["status"] != StatusTodo {
		t.Fatalf("readiness changed lifecycle: %#v", observed)
	}
}

func TestReviewGateAndAuthenticatedActor(t *testing.T) {
	service, project := newTestService(t)
	task := createTask(t, service, project)
	taskID := task["id"].(string)
	if _, err := service.Update(project, taskID, rawFields(t, map[string]any{"status": StatusInProgress}), nil); err != nil {
		t.Fatal(err)
	}
	request := ReviewRequest{TargetType: "代码", TargetPath: "cmd/server", Criteria: "all tests pass"}
	current, review, err := service.RequestReview(project, taskID, "authenticated-user", request)
	if err != nil {
		t.Fatal(err)
	}
	if current["status"] != StatusReview || !review.Exists || reviewVerdict(review.Content) != "" {
		t.Fatalf("unexpected requested review: %#v %#v", current, review)
	}
	current, review, err = service.SetReviewResult(project, taskID, "reviewer-one", ReviewResultRequest{
		Issues: "- fix one issue", Result: ReviewNotPassed,
	})
	if err != nil {
		t.Fatal(err)
	}
	if current["status"] != StatusReview || current["reviewed_by"] != "reviewer-one" || reviewVerdict(review.Content) != ReviewNotPassed {
		t.Fatalf("failed review changed lifecycle incorrectly: %#v %s", current, review.Content)
	}
	if _, err = service.Update(project, taskID, rawFields(t, map[string]any{"status": StatusInProgress}), nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err = service.RequestReview(project, taskID, "authenticated-user", request); err != nil {
		t.Fatal(err)
	}
	current, review, err = service.SetReviewResult(project, taskID, "reviewer-two", ReviewResultRequest{
		Result: ReviewPassed, ChangeSummary: "implemented lifecycle", Verification: "go test passed",
	})
	if err != nil {
		t.Fatal(err)
	}
	if current["status"] != StatusCompleted || current["reviewed_by"] != "reviewer-two" || reviewVerdict(review.Content) != ReviewPassed {
		t.Fatalf("passed review did not complete atomically: %#v %s", current, review.Content)
	}
	if !strings.Contains(review.Content, "## 执行结果") || !strings.Contains(review.Content, "### 变更摘要") || !strings.Contains(review.Content, "### 验证") {
		t.Fatalf("finish fields missing from review: %s", review.Content)
	}
	if _, err := os.Stat(filepath.Join(service.root, project, ".task-review-transaction.json")); !os.IsNotExist(err) {
		t.Fatalf("completed transaction journal remains: %v", err)
	}
}

func TestReviewRequiresIdentityAndExistingRequest(t *testing.T) {
	service, project := newTestService(t)
	task := createTask(t, service, project)
	taskID := task["id"].(string)
	if _, err := service.Update(project, taskID, rawFields(t, map[string]any{"status": StatusInProgress}), nil); err != nil {
		t.Fatal(err)
	}
	request := ReviewRequest{TargetType: "代码", TargetPath: "cmd/server", Criteria: "test"}
	if _, _, err := service.RequestReview(project, taskID, "", request); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("anonymous review request error=%v", err)
	}
	if _, err := service.Update(project, taskID, rawFields(t, map[string]any{"status": StatusReview}), nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.SetReviewResult(project, taskID, "reviewer", ReviewResultRequest{Result: ReviewPassed}); !errors.Is(err, ErrReviewNotFound) {
		t.Fatalf("missing review error=%v", err)
	}
}

func TestHistoricalReadDoesNotWriteBack(t *testing.T) {
	service, project := newTestService(t)
	path, err := service.tasksPath(project)
	if err != nil {
		t.Fatal(err)
	}
	original := []byte(`[{"id":"legacy","status":"in_progress","unknown":{"x":1}}]`)
	if err := os.WriteFile(path, original, 0o644); err != nil {
		t.Fatal(err)
	}
	observed, err := service.Get(project, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if observed["status"] != StatusInProgress {
		t.Fatalf("legacy equivalent not displayed canonically: %#v", observed)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(original) {
		t.Fatalf("read modified historical storage: %q", after)
	}
}

func TestPassedReviewRequiresExecutionEvidence(t *testing.T) {
	service, project := newTestService(t)
	task := createTask(t, service, project)
	taskID := task["id"].(string)
	if _, err := service.Update(project, taskID, rawFields(t, map[string]any{"status": StatusInProgress}), nil); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.RequestReview(project, taskID, "requester", ReviewRequest{TargetType: "代码", TargetPath: "cmd/server", Criteria: "test"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.SetReviewResult(project, taskID, "reviewer", ReviewResultRequest{Result: ReviewPassed}); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("passed review without evidence error=%v", err)
	}
	observed, err := service.Get(project, taskID)
	if err != nil || observed["status"] != StatusReview {
		t.Fatalf("invalid pass changed lifecycle: %#v %v", observed, err)
	}
}

func preparePendingReview(t *testing.T, reviewAlreadyApplied, taskAlreadyApplied bool) (*Service, string, string, string) {
	t.Helper()
	service, project := newTestService(t)
	task := createTask(t, service, project)
	taskID := task["id"].(string)
	if _, err := service.Update(project, taskID, rawFields(t, map[string]any{"status": StatusInProgress}), nil); err != nil {
		t.Fatal(err)
	}
	tasksPath, _ := service.tasksPath(project)
	tasks, _, err := readTasks(tasksPath)
	if err != nil {
		t.Fatal(err)
	}
	_, rawTask := findTask(tasks, taskID)
	_ = setRaw(rawTask, "status", StatusReview)
	_ = setRaw(rawTask, "revision", 3)
	targetTasks, err := encodeTasks(tasks)
	if err != nil {
		t.Fatal(err)
	}
	content, err := formatReviewRequest("代码", "cmd/server", "test")
	if err != nil {
		t.Fatal(err)
	}
	tx := reviewTransaction{Protocol: 1, TaskID: taskID, TargetTasks: targetTasks, ReviewContent: content, ExpectedReviewVersion: 0, Actor: "reviewer"}
	if err = service.persistReviewTransaction(tasksPath, tx); err != nil {
		t.Fatal(err)
	}
	projectPath, _ := service.projectPath(project)
	reviewPath := filepath.Join(projectPath, "tasks", taskID, "docs", "review")
	if reviewAlreadyApplied {
		expected := 0
		if _, _, _, err = service.docs.AppendWithPath(reviewPath, content, "replace_full", "reviewer", "review_api", &expected); err != nil {
			t.Fatal(err)
		}
	}
	if taskAlreadyApplied {
		if err = atomicWrite(tasksPath, targetTasks); err != nil {
			t.Fatal(err)
		}
	}
	return service, project, taskID, content
}

func TestReviewTransactionRecoversReviewThenTaskGap(t *testing.T) {
	service, project, taskID, content := preparePendingReview(t, true, false)
	if _, err := service.Get(project, taskID); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("ordinary read recovered pending transaction: %v", err)
	}
	if _, err := service.Update(project, taskID, map[string]json.RawMessage{}, nil); err != nil {
		t.Fatalf("explicit write did not recover transaction: %v", err)
	}
	observed, err := service.Get(project, taskID)
	if err != nil {
		t.Fatal(err)
	}
	if observed["status"] != StatusReview {
		t.Fatalf("task state not recovered: %#v", observed)
	}
	review, err := service.Review(project, taskID)
	if err != nil || review.Content != content {
		t.Fatalf("review mismatch after recovery: %#v %v", review, err)
	}
}

func TestReviewTransactionRecoversTaskThenReviewGap(t *testing.T) {
	service, project, taskID, content := preparePendingReview(t, false, true)
	if _, err := service.Review(project, taskID); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("ordinary review read recovered pending transaction: %v", err)
	}
	if _, err := service.Update(project, taskID, map[string]json.RawMessage{}, nil); err != nil {
		t.Fatalf("explicit write did not recover transaction: %v", err)
	}
	review, err := service.Review(project, taskID)
	if err != nil || review.Content != content {
		t.Fatalf("review not recovered: %#v %v", review, err)
	}
	observed, err := service.Get(project, taskID)
	if err != nil || observed["status"] != StatusReview {
		t.Fatalf("task mismatch after recovery: %#v %v", observed, err)
	}
}

func TestImportLifecycleCompletesWithPassedReviewAndProvenance(t *testing.T) {
	service, project := newTestService(t)
	task := createTask(t, service, project)
	taskID := task["id"].(string)
	revision := 1
	review := "# 审核文档\n\n## 审核请求\n\n### 目标\n- 类型: 代码\n- 路径: src\n\n### 标准\n通过测试\n\n## 执行结果\n\n### 变更摘要\n已完成实现\n\n### 验证\n全部测试通过\n\n### 遗留问题\n_无_\n\n## 审核结果\n\n### 问题列表\n_暂无_\n\n### 结果\n通过\n"
	value := LifecycleImport{
		Status: StatusCompleted,
		Review: LifecycleReview{Content: review, Verdict: ReviewPassed, ChangeSummary: "已完成实现", Verification: "全部测试通过", Issues: "_暂无_"},
		Source: LifecycleSource{System: "mote", ProjectID: "local-project", TaskID: "local-task", Actor: "mote"},
	}
	value.EvidenceHash, _ = LifecycleEvidenceHash(value)
	updated, err := service.ImportLifecycle(project, taskID, "remote-admin", &revision, value)
	if err != nil {
		t.Fatal(err)
	}
	if updated["status"] != StatusCompleted || int(updated["revision"].(float64)) != 2 || updated["reviewed_by"] != "remote-admin" {
		t.Fatalf("unexpected imported lifecycle: %#v", updated)
	}
	if updated["sync_lifecycle_evidence_hash"] != value.EvidenceHash {
		t.Fatalf("evidence hash not persisted: %#v", updated)
	}
	exported, _, err := taskdocs.ExportWithPath(filepath.Join(service.root, project, "tasks", taskID, "docs", "review"))
	if err != nil || exported != review {
		t.Fatalf("review not imported atomically: exported=%+v err=%v", exported, err)
	}
}

func TestImportLifecycleRejectsCompletionWithoutReviewAndTamperedEvidence(t *testing.T) {
	service, project := newTestService(t)
	task := createTask(t, service, project)
	taskID := task["id"].(string)
	revision := 1
	value := LifecycleImport{Status: StatusCompleted, Source: LifecycleSource{System: "mote", ProjectID: "p", TaskID: "t", Actor: "mote"}}
	value.EvidenceHash, _ = LifecycleEvidenceHash(value)
	if _, err := service.ImportLifecycle(project, taskID, "remote-admin", &revision, value); !errors.Is(err, ErrReviewRequired) {
		t.Fatalf("expected review gate, got %v", err)
	}
	value.Status = StatusInProgress
	value.EvidenceHash = "sha256:tampered"
	if _, err := service.ImportLifecycle(project, taskID, "remote-admin", &revision, value); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected evidence validation, got %v", err)
	}
	duplicate := LifecycleImport{
		Status: StatusCompleted,
		Review: LifecycleReview{Content: "# 审核文档\n\n### 变更摘要\n可信摘要\n\n### 验证\n可信验证\n\n### 结果\n通过\n\n### 变更摘要\n伪造摘要\n", Verdict: ReviewPassed, ChangeSummary: "可信摘要", Verification: "可信验证"},
		Source: LifecycleSource{System: "mote", ProjectID: "p", TaskID: "t", Actor: "mote"},
	}
	duplicate.EvidenceHash, _ = LifecycleEvidenceHash(duplicate)
	if _, err := service.ImportLifecycle(project, taskID, "remote-admin", &revision, duplicate); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected duplicate section rejection, got %v", err)
	}
	current, err := service.Get(project, taskID)
	if err != nil || current["status"] != StatusTodo || int(current["revision"].(float64)) != 1 {
		t.Fatalf("failed import changed task: %#v err=%v", current, err)
	}
}

func TestStrictLifecycleSectionAllowsHistoricalH2BodyHeadings(t *testing.T) {
	content := "### 变更摘要\n\n## T05 / DTO 扩展\n- 已完成\n\n### 验证\n测试通过\n"
	summary, err := strictLifecycleSection(content, "变更摘要")
	if err != nil {
		t.Fatal(err)
	}
	if summary != "## T05 / DTO 扩展\n- 已完成" {
		t.Fatalf("historical H2 body was truncated: %q", summary)
	}
}
