package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/projects"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/tasklifecycle"
	"github.com/houzhh15/AIDG/cmd/server/internal/publish"
	"github.com/houzhh15/AIDG/cmd/server/internal/users"
)

func TestSyncDocumentEndpointsRespectHiddenProjectVisibility(t *testing.T) {
	gin.SetMode(gin.TestMode)
	usersDir := t.TempDir()
	t.Setenv("USERS_DIR", usersDir)
	userDir := filepath.Join(usersDir, "alice")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content, _ := json.Marshal(UserHiddenProjects{HiddenProjectIDs: []string{"hidden-project"}})
	if err := os.WriteFile(filepath.Join(userDir, "hidden_projects.json"), content, 0o600); err != nil {
		t.Fatal(err)
	}
	registry := projects.NewProjectRegistry()
	registry.Set(&projects.Project{ID: "hidden-project", Name: "Hidden"})
	handler := &SyncV1Handler{projects: registry}

	for _, invoke := range []func(*gin.Context){handler.ProjectDocument, handler.TaskDocument} {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		ctx.Params = gin.Params{{Key: "id", Value: "hidden-project"}, {Key: "task_id", Value: "task-1"}, {Key: "slot", Value: "requirements"}}
		ctx.Set("user", "alice")
		ctx.Set("scopes", []string{users.ScopeProjectDocRead, users.ScopeTaskRead})
		invoke(ctx)
		if recorder.Code != http.StatusNotFound {
			t.Fatalf("hidden project endpoint returned %d, body=%s", recorder.Code, recorder.Body.String())
		}
	}
}

func TestSyncApplyRejectsServerIdentityMismatchBeforeWrite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registry := projects.NewProjectRegistry()
	registry.Set(&projects.Project{ID: "project-1", Name: "Project"})
	handler := &SyncV1Handler{serverID: "server-1", projects: registry}
	body := []byte(`{"protocol_version":1,"server_id":"other-server","idempotency_key":"operation-1","expected_snapshot_hash":"sha256:old","manifest":{}}`)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	ctx.Params = gin.Params{{Key: "id", Value: "project-1"}}
	ctx.Set("user", "alice")
	ctx.Set("scopes", []string{users.ScopeProjectAdmin})

	handler.ApplyProject(ctx)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected identity conflict, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestSyncApplyRequiresBothDocumentAndTaskWriteScopes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registry := projects.NewProjectRegistry()
	registry.Set(&projects.Project{ID: "project-1", Name: "Project"})
	handler := &SyncV1Handler{serverID: "server-1", projects: registry}
	body := []byte(`{"protocol_version":1,"server_id":"server-1","idempotency_key":"operation-1","expected_snapshot_hash":"sha256:old","manifest":{}}`)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	ctx.Params = gin.Params{{Key: "id", Value: "project-1"}}
	ctx.Set("user", "alice")
	ctx.Set("scopes", []string{users.ScopeProjectDocWrite, users.ScopeTaskRead})

	handler.ApplyProject(ctx)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestSyncApplyRequiresLifecycleImportScope(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registry := projects.NewProjectRegistry()
	registry.Set(&projects.Project{ID: "project-1", Name: "Project"})
	handler := &SyncV1Handler{serverID: "server-1", projects: registry}
	revision := 1
	lifecycle := tasklifecycle.LifecycleImport{Status: tasklifecycle.StatusTodo, Source: tasklifecycle.LifecycleSource{System: "mote", ProjectID: "local-project", TaskID: "local-task", Actor: "mote"}}
	lifecycle.EvidenceHash, _ = tasklifecycle.LifecycleEvidenceHash(lifecycle)
	manifest := publish.Manifest{ManifestVersion: publish.ManifestVersion, ProjectID: "project-1", IdempotencyKey: "operation-1", Source: publish.SourceMoteManualSync, Resources: []publish.Resource{{Kind: "task.lifecycle_import", TaskID: "task-1", ExpectedRevision: &revision, Content: "", ContentHash: publish.HashContent(""), Lifecycle: &lifecycle}}}
	manifestJSON, _ := json.Marshal(manifest)
	body, _ := json.Marshal(map[string]any{"protocol_version": 1, "server_id": "server-1", "idempotency_key": "operation-1", "expected_snapshot_hash": "sha256:old", "manifest": json.RawMessage(manifestJSON)})
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	ctx.Params = gin.Params{{Key: "id", Value: "project-1"}}
	ctx.Set("user", "alice")
	ctx.Set("scopes", []string{users.ScopeProjectDocRead, users.ScopeProjectDocWrite, users.ScopeTaskRead, users.ScopeTaskWrite})

	handler.ApplyProject(ctx)
	if recorder.Code != http.StatusForbidden || !strings.Contains(recorder.Body.String(), "lifecycle_import_forbidden") {
		t.Fatalf("expected lifecycle scope rejection, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if !capabilitiesForScopes([]string{users.ScopeTaskLifecycleImport}).ImportLifecycle {
		t.Fatal("lifecycle import scope was not advertised")
	}
}

func TestSyncApplyRejectsPayloadOverLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	registry := projects.NewProjectRegistry()
	registry.Set(&projects.Project{ID: "project-1", Name: "Project"})
	handler := &SyncV1Handler{serverID: "server-1", projects: registry}
	body := `{"protocol_version":1,"server_id":"server-1","idempotency_key":"operation","expected_snapshot_hash":"snapshot","manifest":"` + strings.Repeat("x", 17<<20) + `"}`
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	ctx.Request.ContentLength = int64(len(body))
	ctx.Params = gin.Params{{Key: "id", Value: "project-1"}}
	ctx.Set("user", "alice")
	ctx.Set("scopes", []string{users.ScopeProjectAdmin})
	handler.ApplyProject(ctx)
	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestSyncCreateProjectIsAuthorizedAndIdempotent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	root := t.TempDir()
	t.Setenv("AIDG_DATA_PROJECTS_DIR", root)
	oldRoot, oldState := projects.ProjectsRoot, projects.ProjectsStatePath
	projects.ProjectsRoot, projects.ProjectsStatePath = root, filepath.Join(root, "server_projects.json")
	t.Cleanup(func() { projects.ProjectsRoot, projects.ProjectsStatePath = oldRoot, oldState })
	registry := projects.NewProjectRegistry()
	handler := &SyncV1Handler{serverID: "server-1", root: root, projects: registry}
	body := `{"protocol_version":1,"server_id":"server-1","idempotency_key":"create-1","name":"Synced","product_line":"line"}`
	invoke := func(scopes []string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		ctx.Set("user", "alice")
		ctx.Set("scopes", scopes)
		handler.CreateProject(ctx)
		return recorder
	}
	if recorder := invoke([]string{users.ScopeProjectDocRead}); recorder.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d: %s", recorder.Code, recorder.Body.String())
	}
	first := invoke([]string{users.ScopeProjectAdmin})
	if first.Code != http.StatusCreated {
		t.Fatalf("create=%d: %s", first.Code, first.Body.String())
	}
	var created struct {
		Receipt syncCreateProjectReceipt `json:"receipt"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &created); err != nil || created.Receipt.ProjectID == "" {
		t.Fatalf("invalid create receipt: %v %s", err, first.Body.String())
	}
	created.Receipt.State = "prepared"
	keyHash := sha256.Sum256([]byte("create-1"))
	if err := writeSyncJSONAtomic(filepath.Join(root, ".sync-v1", "project-create", hex.EncodeToString(keyHash[:])+".json"), created.Receipt); err != nil {
		t.Fatal(err)
	}
	second := invoke([]string{users.ScopeProjectAdmin})
	if second.Code != http.StatusOK || !strings.Contains(second.Body.String(), `"duplicate":true`) || len(registry.List()) != 1 {
		t.Fatalf("retry=%d projects=%d: %s", second.Code, len(registry.List()), second.Body.String())
	}
}
