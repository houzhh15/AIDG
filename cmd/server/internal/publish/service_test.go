package publish

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/docslot"
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
	manifest := Manifest{ManifestVersion: ManifestVersion, ProjectID: projectID, IdempotencyKey: "key-1", Source: SourceMoteManualSync, Resources: []Resource{projectResource, taskResource}}

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

func TestPublishConditionalChecksPreconditionInsideProjectGate(t *testing.T) {
	service, _, projectID, _ := newPublishTest(t)
	metadata := map[string]json.RawMessage{"product_line": json.RawMessage(`"new"`)}
	resource := metadataResource("project.metadata", "", nil, metadata)
	resource.ExpectedETag = HashContent("old")
	manifest := Manifest{ManifestVersion: ManifestVersion, ProjectID: projectID, IdempotencyKey: "conditional", Source: SourceMoteManualSync, Resources: []Resource{resource}}
	if _, _, err := service.PublishConditional(manifest, "user", func() error { return errors.New("snapshot changed") }); !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("expected precondition failure, got %v", err)
	}
	if got := service.Registry.Get(projectID).ProductLine; got != "old" {
		t.Fatalf("precondition failure mutated project: %q", got)
	}
}

func TestManifestRejectsBadHashAndMissingCAS(t *testing.T) {
	service, _, projectID, taskID := newPublishTest(t)
	resource := Resource{Kind: "task.metadata", TaskID: taskID, Metadata: map[string]json.RawMessage{"name": json.RawMessage(`"x"`)}, ContentHash: "sha256:bad"}
	manifest := Manifest{ManifestVersion: ManifestVersion, ProjectID: projectID, IdempotencyKey: "key", Source: SourceMoteManualSync, Resources: []Resource{resource}}
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
	if _, err := DecodeManifest(strings.NewReader(`{"manifest_version":"aidg.publish/v1","unknown":true}`)); err == nil {
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

func TestPublishConditionalReplaysReceiptBeforeSnapshotPrecondition(t *testing.T) {
	service, _, projectID, _ := newPublishTest(t)
	metadata := map[string]json.RawMessage{"product_line": json.RawMessage(`"new"`)}
	resource := metadataResource("project.metadata", "", nil, metadata)
	resource.ExpectedETag = HashContent("old")
	manifest := Manifest{ManifestVersion: ManifestVersion, ProjectID: projectID, IdempotencyKey: "conditional-replay", Source: SourceMoteManualSync, Resources: []Resource{resource}}
	first, duplicate, err := service.PublishConditional(manifest, "user", func() error { return nil })
	if err != nil || duplicate {
		t.Fatalf("first publish failed: duplicate=%v err=%v", duplicate, err)
	}
	replayed, duplicate, err := service.PublishConditional(manifest, "user", func() error { return errors.New("new snapshot") })
	if err != nil || !duplicate || replayed.IdempotencyKey != first.IdempotencyKey || replayed.ManifestHash != first.ManifestHash {
		t.Fatalf("receipt replay failed: duplicate=%v receipt=%+v err=%v", duplicate, replayed, err)
	}
}

func TestPublishCreatesTaskIdempotentlyAndReturnsMapping(t *testing.T) {
	service, lifecycle, projectID, _ := newPublishTest(t)
	metadata := map[string]json.RawMessage{
		"name":               json.RawMessage(`"Created by sync"`),
		"description":        json.RawMessage(`"content"`),
		"sync_client_ref":    json.RawMessage(`"local-task-1"`),
		"sync_connection_id": json.RawMessage(`"connection-1"`),
	}
	resource := metadataResource("task.create", "", nil, metadata)
	resource.ClientRef = "local-task-1"
	manifest := Manifest{ManifestVersion: ManifestVersion, ProjectID: projectID, IdempotencyKey: "create-task", Source: SourceMoteManualSync, Resources: []Resource{resource}}

	receipt, duplicate, err := service.Publish(manifest, "user")
	if err != nil || duplicate || len(receipt.CreatedTasks) != 1 {
		t.Fatalf("publish=%+v duplicate=%v err=%v", receipt, duplicate, err)
	}
	created := receipt.CreatedTasks[0]
	if created.ClientRef != "local-task-1" || created.TaskID == "" || created.Revision != 1 {
		t.Fatalf("unexpected created task mapping: %+v", created)
	}
	createdView, err := lifecycle.Get(projectID, created.TaskID)
	if err != nil {
		t.Fatalf("created task missing: %v", err)
	}
	if createdView["sync_client_ref"] != "local-task-1" || createdView["sync_connection_id"] != "connection-1" || createdView["status"] != "todo" {
		t.Fatalf("created task recovery metadata/status missing: %#v", createdView)
	}
	second, duplicate, err := service.Publish(manifest, "user")
	if err != nil || !duplicate || len(second.CreatedTasks) != 1 || second.CreatedTasks[0].TaskID != created.TaskID {
		t.Fatalf("retry=%+v duplicate=%v err=%v", second, duplicate, err)
	}
}

func TestPublishAllowsMissingDocumentPreconditionAtVersionZero(t *testing.T) {
	service, _, projectID, _ := newPublishTest(t)
	versionZero := 0
	content := "# Feature list\n\nCreated by first sync.\n"
	resource := Resource{Kind: "project.feature_list", ExpectedVersion: &versionZero, ExpectedETag: "", ContentHash: HashContent(content), Content: content}
	manifest := Manifest{ManifestVersion: ManifestVersion, ProjectID: projectID, IdempotencyKey: "create-missing-document", Source: SourceMoteManualSync, Resources: []Resource{resource}}

	if _, duplicate, err := service.Publish(manifest, "user"); err != nil || duplicate {
		t.Fatalf("first document publish failed: duplicate=%v err=%v", duplicate, err)
	}
	stored, err := docslot.NewUnifiedDocService(service.Root).Export(docslot.ScopeProject, projectID, "feature_list")
	if err != nil || stored.Content != content || stored.Version != 1 || stored.ETag == "" {
		t.Fatalf("document not created: result=%+v err=%v", stored, err)
	}

	versionOne := 1
	invalid := resource
	invalid.ExpectedVersion = &versionOne
	invalid.Content = content + "changed"
	invalid.ContentHash = HashContent(invalid.Content)
	invalidManifest := Manifest{ManifestVersion: ManifestVersion, ProjectID: projectID, IdempotencyKey: "missing-etag-after-v0", Source: SourceMoteManualSync, Resources: []Resource{invalid}}
	if _, _, err = service.Publish(invalidManifest, "user"); err == nil || !strings.Contains(err.Error(), "expected_etag after version zero") {
		t.Fatalf("versioned document without etag was accepted: %v", err)
	}

	negativeVersion := -1
	invalid.ExpectedVersion = &negativeVersion
	invalidManifest.IdempotencyKey = "negative-document-version"
	invalidManifest.Resources = []Resource{invalid}
	if _, _, err = service.Publish(invalidManifest, "user"); err == nil || !strings.Contains(err.Error(), "non-negative expected_version") {
		t.Fatalf("negative document version was accepted: %v", err)
	}
}

func TestPublishRequiresCanonicalProtocolIdentity(t *testing.T) {
	service, _, projectID, _ := newPublishTest(t)
	versionZero := 0
	content := "# Contract\n"
	resource := Resource{Kind: "project.feature_list", ExpectedVersion: &versionZero, ContentHash: HashContent(content), Content: content}
	for name, version := range map[string]string{"legacy": "1", "arbitrary": "other/v9", "empty": "", "long unicode": strings.Repeat("错", 1024)} {
		t.Run("version/"+name, func(t *testing.T) {
			manifest := Manifest{ManifestVersion: version, ProjectID: projectID, IdempotencyKey: "invalid-version-" + name, Source: SourceMoteManualSync, Resources: []Resource{resource}}
			if _, _, err := service.Publish(manifest, "user"); err == nil || !strings.Contains(err.Error(), "unsupported manifest_version") {
				t.Fatalf("invalid manifest version was accepted: %v", err)
			}
		})
	}
	for name, source := range map[string]string{"legacy": "mote", "arbitrary": "other-client", "empty": "", "long unicode": strings.Repeat("错", 1024)} {
		t.Run("source/"+name, func(t *testing.T) {
			manifest := Manifest{ManifestVersion: ManifestVersion, ProjectID: projectID, IdempotencyKey: "invalid-source-" + name, Source: source, Resources: []Resource{resource}}
			if _, _, err := service.Publish(manifest, "user"); err == nil || !strings.Contains(err.Error(), "unsupported manifest source") {
				t.Fatalf("invalid manifest source was accepted: %v", err)
			}
		})
	}
}

func TestPublishAuditDoesNotPersistUntrustedManifestFields(t *testing.T) {
	service, _, projectID, _ := newPublishTest(t)
	secret := "SECRET-TOKEN-AND-DOCUMENT-CONTENT"
	resource := Resource{Kind: secret, ContentHash: HashContent("")}
	manifest := Manifest{ManifestVersion: ManifestVersion, ProjectID: projectID, IdempotencyKey: secret, Source: secret, Resources: []Resource{resource}}
	if _, _, err := service.Publish(manifest, "user"); err == nil {
		t.Fatal("invalid manifest unexpectedly accepted")
	}
	data, err := os.ReadFile(filepath.Join(service.Root, ".publish-audit.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), secret) {
		t.Fatalf("audit leaked untrusted manifest content: %s", data)
	}
	var event AuditEvent
	if err = json.Unmarshal(bytes.TrimSpace(data), &event); err != nil {
		t.Fatal(err)
	}
	if event.IdempotencyKey != HashContent(secret) || event.Source != "<unsupported>" || len(event.ResourceKinds) != 1 || event.ResourceKinds[0] != "<unsupported>" {
		t.Fatalf("audit omitted safe diagnostics: %+v", event)
	}
}

func TestPublishLifecycleImportIsTransactionalAndReplayable(t *testing.T) {
	service, lifecycle, projectID, taskID := newPublishTest(t)
	revision := 1
	review := "# 审核文档\n\n## 执行结果\n\n### 变更摘要\n完成\n\n### 验证\n通过测试\n\n## 审核结果\n\n### 结果\n通过\n"
	value := tasklifecycle.LifecycleImport{
		Status: tasklifecycle.StatusCompleted,
		Review: tasklifecycle.LifecycleReview{Content: review, Verdict: tasklifecycle.ReviewPassed, ChangeSummary: "完成", Verification: "通过测试"},
		Source: tasklifecycle.LifecycleSource{System: "mote", ProjectID: "local-project", TaskID: "local-task", Actor: "mote"},
	}
	value.EvidenceHash, _ = tasklifecycle.LifecycleEvidenceHash(value)
	resource := Resource{Kind: "task.lifecycle_import", TaskID: taskID, ExpectedRevision: &revision, Content: review, ContentHash: HashContent(review), Lifecycle: &value}
	manifest := Manifest{ManifestVersion: ManifestVersion, ProjectID: projectID, IdempotencyKey: "lifecycle-key", Source: SourceMoteManualSync, Resources: []Resource{resource}}

	service.FaultAfter = 1
	if _, _, err := service.Publish(manifest, "remote-admin"); err == nil {
		t.Fatal("fault injection unexpectedly committed lifecycle import")
	}
	current, _ := lifecycle.Get(projectID, taskID)
	if current["status"] != tasklifecycle.StatusTodo || int(current["revision"].(float64)) != 1 {
		t.Fatalf("lifecycle rollback failed: %#v", current)
	}
	service.FaultAfter = 0
	receipt, duplicate, err := service.Publish(manifest, "remote-admin")
	if err != nil || duplicate || receipt.ResourceKinds[0] != "task.lifecycle_import" {
		t.Fatalf("receipt=%+v duplicate=%v err=%v", receipt, duplicate, err)
	}
	current, _ = lifecycle.Get(projectID, taskID)
	if current["status"] != tasklifecycle.StatusCompleted || current["reviewed_by"] != "remote-admin" {
		t.Fatalf("lifecycle import not committed: %#v", current)
	}
	if _, duplicate, err = service.Publish(manifest, "remote-admin"); err != nil || !duplicate {
		t.Fatalf("idempotent lifecycle replay duplicate=%v err=%v", duplicate, err)
	}
}
