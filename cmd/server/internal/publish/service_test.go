package publish

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/projects"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/taskdocs"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/tasklifecycle"
)

func newPublishTest(t *testing.T) (*Service, *tasklifecycle.Service, string, string) {
	t.Helper()
	root := t.TempDir()
	oldRoot, oldState := projects.ProjectsRoot, projects.ProjectsStatePath
	projects.ProjectsRoot, projects.ProjectsStatePath = root, filepath.Join(root, "server_projects.json")
	t.Cleanup(func() { projects.ProjectsRoot, projects.ProjectsStatePath = oldRoot, oldState })
	projectID := "project"
	if err := os.MkdirAll(filepath.Join(root, projectID, "tasks"), 0o755); err != nil {
		t.Fatal(err)
	}
	registry := projects.NewProjectRegistry()
	registry.Set(&projects.Project{ID: projectID, Name: "Project", ProductLine: "old"})
	if err := projects.SaveProjects(registry); err != nil {
		t.Fatal(err)
	}
	lifecycle := tasklifecycle.NewService(root, taskdocs.NewDocService())
	task, err := lifecycle.Create(projectID, map[string]json.RawMessage{"name": json.RawMessage(`"Task"`)})
	if err != nil {
		t.Fatal(err)
	}
	return NewService(root, registry), lifecycle, projectID, task["id"].(string)
}

func metadataResource(kind, taskID string, revision *int, metadata map[string]json.RawMessage) Resource {
	payload, _ := json.Marshal(metadata)
	return Resource{Kind: kind, TaskID: taskID, ExpectedRevision: revision, Metadata: metadata, ContentHash: HashContent(string(payload))}
}

func TestPublishIdempotencyConflictAndFaultRollback(t *testing.T) {
	service, lifecycle, projectID, taskID := newPublishTest(t)
	revision := 1
	projectMetadata := map[string]json.RawMessage{"product_line": json.RawMessage(`"new"`)}
	projectResource := metadataResource("project.metadata", "", nil, projectMetadata)
	projectResource.ExpectedETag = HashContent("old")
	taskResource := metadataResource("task.metadata", taskID, &revision, map[string]json.RawMessage{"description": json.RawMessage(`"published"`)})
	manifest := Manifest{ManifestVersion: "1", ProjectID: projectID, IdempotencyKey: "key-1", Source: "mote", Resources: []Resource{projectResource, taskResource}}

	service.FaultAfter = 1
	if _, _, err := service.Publish(manifest, "user"); err == nil {
		t.Fatal("fault injection unexpectedly committed")
	}
	if got := service.Registry.Get(projectID).ProductLine; got != "old" {
		t.Fatalf("project rollback failed: %q", got)
	}
	task, err := lifecycle.Get(projectID, taskID)
	if err != nil {
		t.Fatal(err)
	}
	if task["description"] != nil {
		t.Fatalf("task changed after rollback: %#v", task)
	}

	service.FaultAfter = 0
	receipt, duplicate, err := service.Publish(manifest, "user")
	if err != nil || duplicate || receipt.ResourceCount != 2 {
		t.Fatalf("publish=%+v duplicate=%v err=%v", receipt, duplicate, err)
	}
	second, duplicate, err := service.Publish(manifest, "user")
	if err != nil || !duplicate || second.ManifestHash != receipt.ManifestHash {
		t.Fatalf("idempotent retry=%+v duplicate=%v err=%v", second, duplicate, err)
	}

	changed := manifest
	changed.Resources = append([]Resource(nil), manifest.Resources...)
	changed.Resources[1].ContentHash = HashContent("different")
	if _, _, err = service.Publish(changed, "user"); err == nil {
		t.Fatal("idempotency key reuse with different manifest accepted")
	}
}

func TestManifestRejectsBadHashAndMissingCAS(t *testing.T) {
	service, _, projectID, taskID := newPublishTest(t)
	resource := Resource{Kind: "task.metadata", TaskID: taskID, Metadata: map[string]json.RawMessage{"name": json.RawMessage(`"x"`)}, ContentHash: "sha256:bad"}
	manifest := Manifest{ManifestVersion: "1", ProjectID: projectID, IdempotencyKey: "key", Source: "mote", Resources: []Resource{resource}}
	if _, _, err := service.Publish(manifest, "user"); err == nil {
		t.Fatal("bad hash/missing revision accepted")
	}
	if err := Recover(service.Root); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}

func TestRecoverRollsBackInterruptedTransaction(t *testing.T) {
	service, _, projectID, _ := newPublishTest(t)
	projectDir := filepath.Join(service.Root, projectID)
	original := filepath.Join(projectDir, "marker.txt")
	if err := os.WriteFile(original, []byte("before"), 0o644); err != nil {
		t.Fatal(err)
	}
	txDir := filepath.Join(service.Root, ".publish-transactions", "tx-crash")
	backup := filepath.Join(txDir, "project")
	if err := copyTree(projectDir, backup); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(projects.ProjectsStatePath, filepath.Join(txDir, "server_projects.json")); err != nil {
		t.Fatal(err)
	}
	if err := writeJSON(filepath.Join(txDir, "journal.json"), journal{State: "applying", ProjectID: projectID, BackupDir: backup, ProjectDir: projectDir}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(original, []byte("half-committed"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Recover(service.Root); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(original)
	if err != nil || string(got) != "before" {
		t.Fatalf("recovery=%q err=%v", got, err)
	}
	if _, err = os.Stat(txDir); !os.IsNotExist(err) {
		t.Fatalf("transaction evidence not cleaned: %v", err)
	}
}

func TestDecodeManifestRejectsUnknownAndOversizedInput(t *testing.T) {
	if _, err := DecodeManifest(strings.NewReader(`{"manifest_version":"1","unknown":true}`)); err == nil {
		t.Fatal("unknown field accepted")
	}
	if _, err := DecodeManifest(strings.NewReader(strings.Repeat(" ", maxManifestSize+1))); err == nil {
		t.Fatal("oversized manifest accepted")
	}
}

func TestRecoverRejectsUnknownJournalState(t *testing.T) {
	service, _, projectID, _ := newPublishTest(t)
	txDir := filepath.Join(service.Root, ".publish-transactions", "tx-unknown")
	if err := writeJSON(filepath.Join(txDir, "journal.json"), journal{State: "mystery", ProjectID: projectID}); err != nil {
		t.Fatal(err)
	}
	if err := Recover(service.Root); err == nil {
		t.Fatal("unknown journal state was accepted")
	}
	if _, err := os.Stat(txDir); err != nil {
		t.Fatalf("unknown journal evidence was removed: %v", err)
	}
}

func TestReviewResourceRequiresExactlyOneNormalizedAction(t *testing.T) {
	revision := 3
	request := &tasklifecycle.ReviewRequest{TargetType: "代码", TargetPath: "src", Criteria: "safe"}
	result := &tasklifecycle.ReviewResultRequest{Result: "通过", ChangeSummary: "done", Verification: "tests"}
	both := Resource{Kind: "task.review", ExpectedRevision: &revision, ReviewRequest: request, ReviewResult: result}
	if _, err := resourcePayload(both); err == nil {
		t.Fatal("review accepted two actions")
	}
	only := Resource{Kind: "task.review", ExpectedRevision: &revision, ReviewRequest: request}
	payload, err := resourcePayload(only)
	if err != nil {
		t.Fatal(err)
	}
	var normalized tasklifecycle.ReviewRequest
	if err = json.Unmarshal(payload, &normalized); err != nil {
		t.Fatal(err)
	}
	if normalized.ExpectedRevision == nil || *normalized.ExpectedRevision != revision {
		t.Fatalf("outer revision not normalized into hashed action: %#v", normalized)
	}
}
