package migration

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/houzhh15/AIDG/cmd/server/internal/config"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/projects"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/taskdocs"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/tasklifecycle"
)

type fileSnapshot struct {
	Hash    string
	Mode    os.FileMode
	ModTime time.Time
}

func snapshotTree(t *testing.T, root string) map[string]fileSnapshot {
	t.Helper()
	result := map[string]fileSnapshot{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		entry := fileSnapshot{Mode: info.Mode(), ModTime: info.ModTime()}
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			entry.Hash = hashBytes(data)
		}
		result[rel] = entry
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func newFixture(t *testing.T, status string) (*Service, *projects.ProjectRegistry, string) {
	t.Helper()
	root := t.TempDir()
	previousConfig := config.GlobalConfig
	config.GlobalConfig = &config.Config{Data: config.DataConfig{ProjectsDir: root}}
	t.Cleanup(func() { config.GlobalConfig = previousConfig })
	projects.InitPaths()
	reg := projects.NewProjectRegistry()
	created := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	reg.Set(&projects.Project{ID: "project-a", Name: "project-a", SchemaVersion: 0, CreatedAt: created, UpdatedAt: created})
	if err := projects.SaveProjects(reg); err != nil {
		t.Fatal(err)
	}
	projectDir := filepath.Join(root, "project-a")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tasks := []map[string]any{{
		"id":         "task-a",
		"name":       "legacy",
		"status":     status,
		"revision":   7,
		"updated_at": "2025-01-02T03:04:05Z",
		"extension":  map[string]any{"nested": []any{float64(1), map[string]any{"future": true}}, "null_value": nil},
	}}
	data, _ := json.MarshalIndent(tasks, "", "  ")
	if err := os.WriteFile(filepath.Join(projectDir, "tasks.json"), append(data, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
	return NewService(root, reg), reg, projectDir
}

func TestAnalyzeDryRunHasNoPersistentSideEffects(t *testing.T) {
	service, _, projectDir := newFixture(t, "in_progress")
	if err := os.WriteFile(filepath.Join(projectDir, "feature_list.md"), []byte("# Legacy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshotTree(t, service.Root)
	plan, err := service.AnalyzeProject("project-a")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.PlannedChanges != 3 { // task status, document, schema marker
		t.Fatalf("unexpected plan: %+v", plan)
	}
	after := snapshotTree(t, service.Root)
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("dry-run changed tree\nbefore=%#v\nafter=%#v", before, after)
	}
}

func TestExecutePreservesLegacyAndUnknownTaskFieldsAndIsIdempotent(t *testing.T) {
	service, reg, projectDir := newFixture(t, "done")
	legacy := []byte("# Legacy feature list\n")
	if err := os.WriteFile(filepath.Join(projectDir, "feature_list.md"), legacy, 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err := service.AnalyzeProject("project-a")
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Execute(plan, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.Changed != 3 {
		t.Fatalf("changed=%d", result.Summary.Changed)
	}
	if reg.Get("project-a").SchemaVersion != projects.CurrentSchemaVersion {
		t.Fatal("schema version was not advanced")
	}
	legacyAfter, err := os.ReadFile(filepath.Join(projectDir, "feature_list.md"))
	if err != nil || !reflect.DeepEqual(legacy, legacyAfter) {
		t.Fatal("legacy file was changed or removed")
	}
	compiled, _, err := taskdocs.ExportReadOnlyWithPath(filepath.Join(projectDir, "docs", "feature_list"))
	if err != nil || compiled != string(legacy) {
		t.Fatalf("canonical document mismatch: %q, %v", compiled, err)
	}
	var tasks []map[string]json.RawMessage
	data, _ := os.ReadFile(filepath.Join(projectDir, "tasks.json"))
	if err := json.Unmarshal(data, &tasks); err != nil {
		t.Fatal(err)
	}
	status, _ := rawString(tasks[0], "status")
	if status != "completed" {
		t.Fatalf("status=%q", status)
	}
	var revision int
	_ = json.Unmarshal(tasks[0]["revision"], &revision)
	if revision != 8 || tasks[0]["extension"] == nil {
		t.Fatalf("task fields not preserved: %s", data)
	}

	second, err := service.AnalyzeProject("project-a")
	if err != nil {
		t.Fatal(err)
	}
	if second.Summary.PlannedChanges != 0 {
		t.Fatalf("second run planned changes: %+v", second.Changes)
	}
	before := snapshotTree(t, service.Root)
	secondResult, err := service.Execute(second, "admin")
	if err != nil {
		t.Fatal(err)
	}
	if secondResult.Summary.Changed != 0 || !reflect.DeepEqual(before, snapshotTree(t, service.Root)) {
		t.Fatal("idempotent execute wrote resources")
	}
}

func TestUnknownStatusBlocksWithoutGuessingOrWriting(t *testing.T) {
	service, reg, _ := newFixture(t, "mystery")
	before := snapshotTree(t, service.Root)
	plan, err := service.AnalyzeProject("project-a")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.UnknownStatuses != 1 || !hasBlockingIssues(plan.Issues) {
		t.Fatalf("unknown status not reported: %+v", plan)
	}
	for _, change := range plan.Changes {
		if change.Kind == "schema_version" || change.Kind == "task_status" {
			t.Fatalf("unsafe change planned: %+v", change)
		}
	}
	if _, err := service.Execute(plan, "admin"); !errors.Is(err, ErrBlocked) {
		t.Fatalf("expected blocked, got %v", err)
	}
	if reg.Get("project-a").SchemaVersion != 0 || !reflect.DeepEqual(before, snapshotTree(t, service.Root)) {
		t.Fatal("blocked migration wrote resources")
	}
}

func TestTargetConflictAndStalePlanAreRejected(t *testing.T) {
	service, _, projectDir := newFixture(t, "todo")
	if err := os.WriteFile(filepath.Join(projectDir, "feature_list.md"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	docs := taskdocs.NewDocService()
	version := 0
	if _, _, _, err := docs.AppendWithPath(filepath.Join(projectDir, "docs", "feature_list"), "new", "replace_full", "test", "test", &version); err != nil {
		t.Fatal(err)
	}
	plan, err := service.AnalyzeProject("project-a")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Summary.Conflicts != 1 {
		t.Fatalf("target conflict missing: %+v", plan.Issues)
	}
	if _, err := service.Execute(plan, "admin"); !errors.Is(err, ErrBlocked) {
		t.Fatalf("expected blocked conflict, got %v", err)
	}

	service2, _, projectDir2 := newFixture(t, "done")
	stale, err := service2.AnalyzeTask("project-a", "task-a")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir2, "unrelated.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// A project-tree-only unrelated file is outside this task plan's baseline.
	if _, err := service2.Execute(stale, "admin"); err != nil {
		t.Fatalf("unrelated file should not invalidate task plan: %v", err)
	}

	service3, _, projectDir3 := newFixture(t, "done")
	stale, _ = service3.AnalyzeTask("project-a", "task-a")
	data, _ := os.ReadFile(filepath.Join(projectDir3, "tasks.json"))
	data = append(data, ' ')
	if err := os.WriteFile(filepath.Join(projectDir3, "tasks.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := service3.Execute(stale, "admin"); !errors.Is(err, ErrConflict) {
		t.Fatalf("expected baseline conflict, got %v", err)
	}
}

func TestInjectedFailureRollsBackAllChanges(t *testing.T) {
	service, reg, projectDir := newFixture(t, "done")
	if err := os.WriteFile(filepath.Join(projectDir, "feature_list.md"), []byte("legacy"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, _ := service.AnalyzeProject("project-a")
	before := snapshotTree(t, projectDir)
	service.FaultAfter = 1
	if _, err := service.Execute(plan, "admin"); err == nil {
		t.Fatal("expected injected failure")
	}
	if reg.Get("project-a").SchemaVersion != 0 || !reflect.DeepEqual(before, snapshotTree(t, projectDir)) {
		t.Fatal("failed migration was not rolled back")
	}
	if err := Recover(service.Root); err != nil {
		t.Fatal(err)
	}
}

func TestTaskScopeDoesNotAdvanceProjectSchema(t *testing.T) {
	service, reg, _ := newFixture(t, "in_progress")
	plan, err := service.AnalyzeTask("project-a", "task-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Execute(plan, "admin"); err != nil {
		t.Fatal(err)
	}
	if reg.Get("project-a").SchemaVersion != 0 {
		t.Fatal("task migration advanced project schema")
	}
	if _, err := service.AnalyzeTask("project-a", "missing"); !errors.Is(err, tasklifecycle.ErrNotFound) {
		t.Fatalf("expected task not found, got %v", err)
	}
}

func TestExecutionPlanIsReportOnlyAndMeetingTreeIsUntouched(t *testing.T) {
	service, _, projectDir := newFixture(t, "mystery")
	planDir := filepath.Join(projectDir, "tasks", "task-a")
	if err := os.MkdirAll(planDir, 0o755); err != nil {
		t.Fatal(err)
	}
	planContent := []byte("status: completed\n")
	if err := os.WriteFile(filepath.Join(planDir, "execution_plan.md"), planContent, 0o644); err != nil {
		t.Fatal(err)
	}
	meetingRoot := filepath.Join(filepath.Dir(service.Root), "meetings")
	if err := os.MkdirAll(meetingRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(meetingRoot, "feature_list.md"), []byte("meeting"), 0o644); err != nil {
		t.Fatal(err)
	}
	meetingBefore := snapshotTree(t, meetingRoot)
	migrationPlan, err := service.AnalyzeProject("project-a")
	if err != nil {
		t.Fatal(err)
	}
	if migrationPlan.Summary.UnknownStatuses != 1 || len(migrationPlan.LegacyRecords) != 1 || migrationPlan.LegacyRecords[0].State != "present" {
		t.Fatalf("execution plan improperly affected status report: %+v", migrationPlan)
	}
	if !reflect.DeepEqual(meetingBefore, snapshotTree(t, meetingRoot)) {
		t.Fatal("migration preview touched meeting data")
	}
}

func TestSourceConflictAndSymlinkAreBlocking(t *testing.T) {
	service, _, projectDir := newFixture(t, "todo")
	if err := os.WriteFile(filepath.Join(projectDir, "feature_list.md"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(projectDir, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "docs", "feature_list.md"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(projectDir, "architecture_new.md")); err != nil {
		t.Fatal(err)
	}
	plan, err := service.AnalyzeProject("project-a")
	if err != nil {
		t.Fatal(err)
	}
	codes := map[string]bool{}
	for _, issue := range plan.Issues {
		codes[issue.Code] = true
	}
	if !codes["source_conflict"] || !codes["invalid_legacy_source"] {
		t.Fatalf("missing source issues: %+v", plan.Issues)
	}
	if _, err := service.Execute(plan, "admin"); !errors.Is(err, ErrBlocked) {
		t.Fatalf("expected blocked, got %v", err)
	}
}

func TestRecoverInterruptedTransaction(t *testing.T) {
	service, _, projectDir := newFixture(t, "done")
	before := snapshotTree(t, projectDir)
	txDir, err := service.prepareTransaction("project-a", projectDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "tasks.json"), []byte("corrupted"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Recover(service.Root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(txDir); !os.IsNotExist(err) {
		t.Fatalf("transaction was not cleaned: %v", err)
	}
	if !reflect.DeepEqual(before, snapshotTree(t, projectDir)) {
		t.Fatal("recovery did not restore project snapshot")
	}
}

func TestUnknownStatusReportDoesNotExposeRawValue(t *testing.T) {
	secretStatus := "token=super-secret-value"
	service, _, _ := newFixture(t, secretStatus)
	plan, err := service.AnalyzeProject("project-a")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte(secretStatus)) || bytes.Contains(encoded, []byte("super-secret-value")) {
		t.Fatalf("migration report exposed unknown status: %s", encoded)
	}
}

func TestRollbackRestoresAbsentProjectStateFile(t *testing.T) {
	service, _, _ := newFixture(t, "todo")
	statePath := projects.ProjectsStatePath
	if err := os.Remove(statePath); err != nil {
		t.Fatal(err)
	}
	plan, err := service.AnalyzeProject("project-a")
	if err != nil {
		t.Fatal(err)
	}
	service.FaultAfter = 1 // schema marker is the only planned write
	result, err := service.Execute(plan, "admin")
	if err == nil || result.Summary.Recovery != "rolled_back" {
		t.Fatalf("expected rolled-back failure, result=%+v err=%v", result, err)
	}
	if _, statErr := os.Stat(statePath); !os.IsNotExist(statErr) {
		t.Fatalf("rollback left state file behind: %v", statErr)
	}
}
