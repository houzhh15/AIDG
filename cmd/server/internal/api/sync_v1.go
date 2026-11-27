package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/docslot"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/projects"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/tasklifecycle"
	"github.com/houzhh15/AIDG/cmd/server/internal/publish"
	"github.com/houzhh15/AIDG/cmd/server/internal/services"
	"github.com/houzhh15/AIDG/cmd/server/internal/users"
)

const (
	syncProtocolVersion = 1
	maxSyncContentBytes = 4 << 20
)

var projectSyncSlots = []string{"feature_list", "architecture_design"}
var taskSyncSlots = []string{"requirements", "design", "test", "review"}
var syncProjectCreateMu sync.Mutex

type syncCreateProjectRequest struct {
	ProtocolVersion int    `json:"protocol_version"`
	ServerID        string `json:"server_id"`
	IdempotencyKey  string `json:"idempotency_key"`
	Name            string `json:"name"`
	ProductLine     string `json:"product_line"`
}

type syncCreateProjectReceipt struct {
	ProtocolVersion int       `json:"protocol_version"`
	ServerID        string    `json:"server_id"`
	IdempotencyKey  string    `json:"idempotency_key"`
	RequestHash     string    `json:"request_hash"`
	ProjectID       string    `json:"project_id"`
	State           string    `json:"state"`
	CreatedAt       time.Time `json:"created_at"`
}

type SyncV1Handler struct {
	serverID  string
	root      string
	projects  *projects.ProjectRegistry
	documents docslot.UnifiedDocService
	userRoles services.UserRoleService
	publisher *publish.Service
}

type syncCapabilities struct {
	Read            bool `json:"read"`
	CreateProject   bool `json:"create_project"`
	UpdateProject   bool `json:"update_project"`
	CreateTask      bool `json:"create_task"`
	UpdateTask      bool `json:"update_task"`
	ReadDocuments   bool `json:"read_documents"`
	WriteDocuments  bool `json:"write_documents"`
	ImportLifecycle bool `json:"import_lifecycle"`
}

type syncDocument struct {
	Slot      string    `json:"slot"`
	Exists    bool      `json:"exists"`
	Version   int       `json:"version"`
	ETag      string    `json:"etag"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

type syncProjectSummary struct {
	ID            string           `json:"id"`
	Name          string           `json:"name"`
	ProductLine   string           `json:"product_line"`
	SchemaVersion int              `json:"schema_version"`
	CreatedAt     time.Time        `json:"created_at"`
	UpdatedAt     time.Time        `json:"updated_at"`
	TaskCount     int              `json:"task_count"`
	ContentHash   string           `json:"content_hash"`
	Capabilities  syncCapabilities `json:"capabilities"`
}

type syncTaskManifest struct {
	Task      map[string]any `json:"task"`
	Documents []syncDocument `json:"documents"`
}

type syncProjectManifest struct {
	ProtocolVersion int                `json:"protocol_version"`
	ServerID        string             `json:"server_id"`
	Project         syncProjectSummary `json:"project"`
	Documents       []syncDocument     `json:"documents"`
	Tasks           []syncTaskManifest `json:"tasks"`
	SnapshotHash    string             `json:"snapshot_hash"`
}

func NewSyncV1Handler(serverID string, registry *projects.ProjectRegistry, documents docslot.UnifiedDocService, userRoles services.UserRoleService) *SyncV1Handler {
	root := projectsRoot()
	return &SyncV1Handler{
		serverID: serverID, root: root, projects: registry, documents: documents, userRoles: userRoles,
		publisher: publish.NewService(root, registry),
	}
}

func (h *SyncV1Handler) Capabilities(c *gin.Context) {
	username, ok := syncUsername(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"server_id":             h.serverID,
		"sync_protocol_version": syncProtocolVersion,
		"user":                  gin.H{"username": username},
		"capabilities": gin.H{
			"create_project":  users.HasScope(syncContextScopes(c), users.ScopeProjectAdmin),
			"create_task":     users.HasScope(syncContextScopes(c), users.ScopeTaskWrite) || users.HasScope(syncContextScopes(c), users.ScopeProjectAdmin),
			"write_documents": users.HasScope(syncContextScopes(c), users.ScopeProjectDocWrite) || users.HasScope(syncContextScopes(c), users.ScopeProjectAdmin),
		},
		"features": gin.H{
			"projects":           true,
			"tasks":              true,
			"documents":          true,
			"conditional_write":  true,
			"read_only_sync_api": false,
			"delete_propagation": false,
		},
	})
}

func (h *SyncV1Handler) CreateProject(c *gin.Context) {
	if _, ok := syncUsername(c); !ok {
		return
	}
	if !users.HasScope(syncContextScopes(c), users.ScopeProjectAdmin) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	root := h.root
	if root == "" {
		root = projectsRoot()
	}
	var request syncCreateProjectRequest
	decoder := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, maxSyncContentBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sync project create request"})
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid trailing sync project create payload"})
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	request.ProductLine = strings.TrimSpace(request.ProductLine)
	request.IdempotencyKey = strings.TrimSpace(request.IdempotencyKey)
	if request.ProtocolVersion != syncProtocolVersion || request.ServerID != h.serverID || request.IdempotencyKey == "" || len(request.IdempotencyKey) > 128 || request.Name == "" || len(request.Name) > 200 || len(request.ProductLine) > 256 || strings.ContainsRune(request.Name+request.ProductLine+request.IdempotencyKey, '\x00') {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid sync project create fields"})
		return
	}
	requestHash := syncHash(request)
	keyHash := sha256.Sum256([]byte(request.IdempotencyKey))
	receiptPath := filepath.Join(root, ".sync-v1", "project-create", hex.EncodeToString(keyHash[:])+".json")

	syncProjectCreateMu.Lock()
	defer syncProjectCreateMu.Unlock()
	var receipt syncCreateProjectReceipt
	if data, err := os.ReadFile(receiptPath); err == nil {
		if json.Unmarshal(data, &receipt) != nil || receipt.RequestHash != requestHash {
			c.JSON(http.StatusConflict, gin.H{"error": "idempotency_key_reused"})
			return
		}
		if h.projects.Get(receipt.ProjectID) != nil {
			receipt.State = "committed"
			if err := writeSyncJSONAtomic(receiptPath, receipt); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "restore project create receipt failed"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"receipt": receipt, "duplicate": true})
			return
		}
	} else if !os.IsNotExist(err) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read project create receipt failed"})
		return
	}

	now := time.Now().UTC()
	projectID := receipt.ProjectID
	if projectID == "" {
		projectID = fmt.Sprintf("p%d", now.UnixNano())
		for h.projects.Get(projectID) != nil {
			now = time.Now().UTC()
			projectID = fmt.Sprintf("p%d", now.UnixNano())
		}
	}
	receipt = syncCreateProjectReceipt{ProtocolVersion: syncProtocolVersion, ServerID: h.serverID, IdempotencyKey: request.IdempotencyKey, RequestHash: requestHash, ProjectID: projectID, State: "prepared", CreatedAt: now}
	if err := writeSyncJSONAtomic(receiptPath, receipt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "prepare project create receipt failed"})
		return
	}
	projectDir := filepath.Join(root, projectID)
	if err := os.MkdirAll(filepath.Join(projectDir, "tasks"), 0o755); err != nil {
		_ = os.Remove(receiptPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "create project directory failed"})
		return
	}
	rollback := func() { _ = os.RemoveAll(projectDir) }
	project := &projects.Project{ID: projectID, Name: request.Name, ProductLine: request.ProductLine, SchemaVersion: projects.CurrentSchemaVersion, CreatedAt: now, UpdatedAt: now}
	h.projects.Set(project)
	if err := projects.SaveProjects(h.projects); err != nil {
		h.projects.Delete(projectID)
		rollback()
		_ = os.Remove(receiptPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "save project failed"})
		return
	}
	receipt.State = "committed"
	if err := writeSyncJSONAtomic(receiptPath, receipt); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "store project create receipt failed", "project_id": projectID})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"receipt": receipt, "duplicate": false})
}

func writeSyncJSONAtomic(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".sync-*.tmp")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(data)
	}
	if err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(temporaryName, path)
}

func (h *SyncV1Handler) Projects(c *gin.Context) {
	username, ok := syncUsername(c)
	if !ok {
		return
	}
	globalScopes := syncContextScopes(c)
	list := h.projects.List()
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	out := make([]syncProjectSummary, 0, len(list))
	for _, project := range list {
		scopes := h.projectScopes(username, project.ID, globalScopes)
		if !syncCanSeeProject(username, project.ID, scopes) || !IsProjectVisibleToUser(username, project.ID) {
			continue
		}
		summary, err := h.projectSummary(project, scopes)
		if err != nil {
			internalErrorResponse(c, err)
			return
		}
		out = append(out, summary)
	}
	c.JSON(http.StatusOK, gin.H{
		"protocol_version": syncProtocolVersion,
		"server_id":        h.serverID,
		"projects":         out,
	})
}

func (h *SyncV1Handler) ProjectManifest(c *gin.Context) {
	project, scopes, ok := h.visibleProject(c)
	if !ok {
		return
	}
	manifest, err := h.projectManifest(project, scopes)
	if err != nil {
		internalErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, manifest)
}

func (h *SyncV1Handler) ProjectDocument(c *gin.Context) {
	project, scopes, ok := h.visibleProject(c)
	if !ok {
		return
	}
	if !syncHasAnyScope(scopes, users.ScopeProjectDocRead, users.ScopeProjectAdmin, users.ScopeUserManage) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	slot := c.Param("slot")
	if !containsString(projectSyncSlots, slot) {
		badRequestResponse(c, "unsupported project document slot")
		return
	}
	result, err := h.documents.Export(docslot.ScopeProject, project.ID, slot)
	if err != nil {
		internalErrorResponse(c, err)
		return
	}
	writeSyncDocument(c, slot, result)
}

func (h *SyncV1Handler) TaskDocument(c *gin.Context) {
	project, scopes, ok := h.visibleProject(c)
	if !ok {
		return
	}
	if !syncHasAnyScope(scopes, users.ScopeTaskRead, users.ScopeProjectAdmin, users.ScopeUserManage) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	if _, err := taskLifecycleService().Get(project.ID, c.Param("task_id")); err != nil {
		notFoundResponse(c, "task")
		return
	}
	slot := c.Param("slot")
	if !containsString(taskSyncSlots, slot) {
		badRequestResponse(c, "unsupported task document slot")
		return
	}
	result, err := h.documents.ExportTask(project.ID, c.Param("task_id"), slot)
	if err != nil {
		internalErrorResponse(c, err)
		return
	}
	writeSyncDocument(c, slot, result)
}

type syncApplyEnvelope struct {
	ProtocolVersion      int             `json:"protocol_version"`
	ServerID             string          `json:"server_id"`
	IdempotencyKey       string          `json:"idempotency_key"`
	ExpectedSnapshotHash string          `json:"expected_snapshot_hash"`
	Manifest             json.RawMessage `json:"manifest"`
}

// ApplyProject conditionally applies one already-existing project bundle. The
// publish service supplies the project-wide transaction; the sync receipt makes
// a successful response replayable when the caller loses the HTTP response.
func (h *SyncV1Handler) ApplyProject(c *gin.Context) {
	project, scopes, ok := h.visibleProject(c)
	if !ok {
		return
	}
	admin := syncHasAnyScope(scopes, users.ScopeProjectAdmin, users.ScopeUserManage)
	if !admin && (!syncHasAnyScope(scopes, users.ScopeProjectDocWrite) || !syncHasAnyScope(scopes, users.ScopeTaskWrite)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "forbidden"})
		return
	}
	const maxSyncApplyBytes = 16 << 20
	if c.Request.ContentLength > maxSyncApplyBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "sync_payload_too_large"})
		return
	}
	body := http.MaxBytesReader(c.Writer, c.Request.Body, maxSyncApplyBytes)
	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()
	var envelope syncApplyEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		badRequestResponse(c, "invalid sync apply envelope")
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		badRequestResponse(c, "invalid trailing sync apply payload")
		return
	}
	if envelope.ProtocolVersion != syncProtocolVersion || envelope.ServerID != h.serverID {
		c.JSON(http.StatusConflict, gin.H{"error": "sync_identity_mismatch"})
		return
	}
	key := strings.TrimSpace(envelope.IdempotencyKey)
	if key == "" || len(key) > 128 || strings.TrimSpace(envelope.ExpectedSnapshotHash) == "" || len(envelope.Manifest) == 0 {
		badRequestResponse(c, "idempotency_key, expected_snapshot_hash and manifest are required")
		return
	}
	manifest, err := publish.DecodeManifest(strings.NewReader(string(envelope.Manifest)))
	if err != nil || manifest.ProjectID != project.ID {
		badRequestResponse(c, "invalid publish manifest")
		return
	}
	for _, resource := range manifest.Resources {
		if resource.Kind == "task.lifecycle_import" && !admin && !syncHasAnyScope(scopes, users.ScopeTaskLifecycleImport) {
			c.JSON(http.StatusForbidden, gin.H{"error": "lifecycle_import_forbidden"})
			return
		}
	}
	manifest.IdempotencyKey = key
	publishReceipt, duplicate, err := h.publisher.PublishConditional(manifest, c.GetString("user"), func() error {
		current, currentErr := h.projectManifest(h.projects.Get(project.ID), scopes)
		if currentErr != nil {
			return currentErr
		}
		if current.SnapshotHash != envelope.ExpectedSnapshotHash {
			return errors.New("snapshot changed")
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, publish.ErrPreconditionFailed) || errors.Is(err, tasklifecycle.ErrVersionConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "conditional_write_conflict"})
		} else if errors.Is(err, tasklifecycle.ErrNotFound) {
			notFoundResponse(c, "publish resource not found")
		} else {
			slog.Warn("sync publish rejected", "project_id", project.ID, "error_code", "publish_rejected")
			badRequestResponse(c, "publish rejected")
		}
		return
	}
	updatedProject := h.projects.Get(project.ID)
	manifestAfter, err := h.projectManifest(updatedProject, scopes)
	if err != nil {
		internalErrorResponse(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"protocol_version": syncProtocolVersion, "server_id": h.serverID, "project_id": project.ID,
		"idempotency_key": key, "snapshot_hash": manifestAfter.SnapshotHash,
		"receipt": publishReceipt, "duplicate": duplicate,
	})
}

func writeSyncDocument(c *gin.Context, slot string, result *docslot.ExportResult) {
	if result == nil || len(result.Content) > maxSyncContentBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "document_too_large"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"slot":       slot,
		"content":    result.Content,
		"exists":     result.Exists,
		"version":    result.Version,
		"etag":       result.ETag,
		"updated_at": result.UpdatedAt,
	})
}

func (h *SyncV1Handler) projectSummary(project *projects.Project, scopes []string) (syncProjectSummary, error) {
	capabilities := capabilitiesForScopes(scopes)
	taskCount := 0
	hashInput := struct {
		ID            string             `json:"id"`
		Name          string             `json:"name"`
		ProductLine   string             `json:"product_line"`
		SchemaVersion int                `json:"schema_version"`
		UpdatedAt     time.Time          `json:"updated_at"`
		Tasks         []syncTaskManifest `json:"tasks,omitempty"`
		Documents     []syncDocument     `json:"documents,omitempty"`
	}{ID: project.ID, Name: project.Name, ProductLine: project.ProductLine, SchemaVersion: project.SchemaVersion, UpdatedAt: project.UpdatedAt}
	if capabilities.Read {
		tasks, err := taskLifecycleService().List(project.ID)
		if err != nil {
			return syncProjectSummary{}, err
		}
		sortTasks(tasks)
		taskCount = len(tasks)
		for _, task := range tasks {
			taskID, _ := task["id"].(string)
			documents, docsErr := h.taskDocuments(project.ID, taskID)
			if docsErr != nil {
				return syncProjectSummary{}, docsErr
			}
			hashInput.Tasks = append(hashInput.Tasks, syncTaskManifest{Task: task, Documents: documents})
		}
	}
	if capabilities.ReadDocuments {
		docs, err := h.projectDocuments(project.ID)
		if err != nil {
			return syncProjectSummary{}, err
		}
		hashInput.Documents = docs
	}
	return syncProjectSummary{
		ID: project.ID, Name: project.Name, ProductLine: project.ProductLine,
		SchemaVersion: project.SchemaVersion, CreatedAt: project.CreatedAt, UpdatedAt: project.UpdatedAt,
		TaskCount: taskCount, ContentHash: syncHash(hashInput), Capabilities: capabilities,
	}, nil
}

func (h *SyncV1Handler) projectManifest(project *projects.Project, scopes []string) (*syncProjectManifest, error) {
	summary, err := h.projectSummary(project, scopes)
	if err != nil {
		return nil, err
	}
	manifest := &syncProjectManifest{ProtocolVersion: syncProtocolVersion, ServerID: h.serverID, Project: summary, Documents: []syncDocument{}, Tasks: []syncTaskManifest{}}
	if summary.Capabilities.ReadDocuments {
		manifest.Documents, err = h.projectDocuments(project.ID)
		if err != nil {
			return nil, err
		}
	}
	if summary.Capabilities.Read {
		tasks, listErr := taskLifecycleService().List(project.ID)
		if listErr != nil {
			return nil, listErr
		}
		sortTasks(tasks)
		for _, task := range tasks {
			taskID, _ := task["id"].(string)
			documents, docsErr := h.taskDocuments(project.ID, taskID)
			if docsErr != nil {
				return nil, docsErr
			}
			manifest.Tasks = append(manifest.Tasks, syncTaskManifest{Task: task, Documents: documents})
		}
	}
	manifest.SnapshotHash = syncHash(struct {
		Project   syncProjectSummary `json:"project"`
		Documents []syncDocument     `json:"documents"`
		Tasks     []syncTaskManifest `json:"tasks"`
	}{manifest.Project, manifest.Documents, manifest.Tasks})
	return manifest, nil
}

func (h *SyncV1Handler) projectDocuments(projectID string) ([]syncDocument, error) {
	out := make([]syncDocument, 0, len(projectSyncSlots))
	for _, slot := range projectSyncSlots {
		result, err := h.documents.Export(docslot.ScopeProject, projectID, slot)
		if err != nil {
			return nil, err
		}
		out = append(out, syncDocument{Slot: slot, Exists: result.Exists, Version: result.Version, ETag: result.ETag, UpdatedAt: result.UpdatedAt})
	}
	return out, nil
}

func (h *SyncV1Handler) taskDocuments(projectID, taskID string) ([]syncDocument, error) {
	out := make([]syncDocument, 0, len(taskSyncSlots))
	for _, slot := range taskSyncSlots {
		result, err := h.documents.ExportTask(projectID, taskID, slot)
		if err != nil {
			return nil, err
		}
		out = append(out, syncDocument{Slot: slot, Exists: result.Exists, Version: result.Version, ETag: result.ETag, UpdatedAt: result.UpdatedAt})
	}
	return out, nil
}

func (h *SyncV1Handler) projectScopes(username, projectID string, global []string) []string {
	scopes := append([]string(nil), global...)
	if h.userRoles == nil {
		return scopes
	}
	projectScopes, err := h.userRoles.ComputeEffectiveScopes(username, projectID)
	if err != nil {
		return scopes
	}
	for _, scope := range projectScopes {
		if !containsString(scopes, scope) {
			scopes = append(scopes, scope)
		}
	}
	return scopes
}

func (h *SyncV1Handler) visibleProject(c *gin.Context) (*projects.Project, []string, bool) {
	username, ok := syncUsername(c)
	if !ok {
		return nil, nil, false
	}
	project := h.projects.Get(c.Param("id"))
	if project == nil {
		notFoundResponse(c, "project")
		return nil, nil, false
	}
	scopes := h.projectScopes(username, project.ID, syncContextScopes(c))
	if !syncCanSeeProject(username, project.ID, scopes) || !IsProjectVisibleToUser(username, project.ID) {
		notFoundResponse(c, "project")
		return nil, nil, false
	}
	return project, scopes, true
}

func syncUsername(c *gin.Context) (string, bool) {
	value, ok := c.Get("user")
	username, typeOK := value.(string)
	if !ok || !typeOK || strings.TrimSpace(username) == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return "", false
	}
	return username, true
}

func syncContextScopes(c *gin.Context) []string {
	value, _ := c.Get("scopes")
	scopes, _ := value.([]string)
	return scopes
}

func syncCanSeeProject(username, projectID string, scopes []string) bool {
	return syncHasAnyScope(scopes, users.ScopeUserManage, users.ScopeProjectAdmin, users.ScopeProjectDocRead, users.ScopeTaskRead) || hasProjectPermission(username, projectID)
}

func capabilitiesForScopes(scopes []string) syncCapabilities {
	admin := syncHasAnyScope(scopes, users.ScopeUserManage, users.ScopeProjectAdmin)
	return syncCapabilities{
		Read:            syncHasAnyScope(scopes, users.ScopeTaskRead) || admin,
		CreateProject:   admin,
		UpdateProject:   syncHasAnyScope(scopes, users.ScopeProjectDocWrite) || admin,
		CreateTask:      syncHasAnyScope(scopes, users.ScopeTaskWrite) || admin,
		UpdateTask:      syncHasAnyScope(scopes, users.ScopeTaskWrite) || admin,
		ReadDocuments:   syncHasAnyScope(scopes, users.ScopeProjectDocRead, users.ScopeTaskRead) || admin,
		WriteDocuments:  syncHasAnyScope(scopes, users.ScopeProjectDocWrite, users.ScopeTaskWrite) || admin,
		ImportLifecycle: syncHasAnyScope(scopes, users.ScopeTaskLifecycleImport) || admin,
	}
}

func syncHasAnyScope(scopes []string, wanted ...string) bool {
	for _, scope := range scopes {
		if containsString(wanted, scope) {
			return true
		}
	}
	return false
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func sortTasks(tasks []map[string]any) {
	sort.Slice(tasks, func(i, j int) bool {
		left, _ := tasks[i]["id"].(string)
		right, _ := tasks[j]["id"].(string)
		return left < right
	})
}

func syncHash(value any) string {
	encoded, _ := json.Marshal(value)
	sum := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(sum[:])
}
