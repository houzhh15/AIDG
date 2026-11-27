package publish

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/docslot"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/projects"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/taskdocs"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/tasklifecycle"
)

const (
	ManifestVersion      = "aidg.publish/v1"
	SourceMoteManualSync = "mote-manual-sync"
	maxResources         = 128
	maxContentBytes      = 4 << 20
	maxManifestSize      = 16 << 20
)

var allProjectsGate sync.RWMutex

func gate(string) *sync.RWMutex { return &allProjectsGate }

func ReadLock(projectID string) func() {
	allProjectsGate.RLock()
	return allProjectsGate.RUnlock
}

// WriteLock serializes project-wide maintenance with publish operations.
func WriteLock(projectID string) func() {
	allProjectsGate.Lock()
	return allProjectsGate.Unlock
}

// Manifest is deliberately single-project in protocol v1. It may create tasks,
// but project creation remains a separate idempotent sync operation.
type Manifest struct {
	ManifestVersion string     `json:"manifest_version"`
	ProjectID       string     `json:"project_id"`
	IdempotencyKey  string     `json:"idempotency_key"`
	Source          string     `json:"source"`
	Resources       []Resource `json:"resources"`
}

type Resource struct {
	Kind             string                             `json:"kind"`
	TaskID           string                             `json:"task_id,omitempty"`
	ClientRef        string                             `json:"client_ref,omitempty"`
	ExpectedRevision *int                               `json:"expected_revision,omitempty"`
	ExpectedVersion  *int                               `json:"expected_version,omitempty"`
	ExpectedETag     string                             `json:"expected_etag,omitempty"`
	ContentHash      string                             `json:"content_hash"`
	Content          string                             `json:"content,omitempty"`
	Metadata         map[string]json.RawMessage         `json:"metadata,omitempty"`
	ReviewRequest    *tasklifecycle.ReviewRequest       `json:"review_request,omitempty"`
	ReviewResult     *tasklifecycle.ReviewResultRequest `json:"review_result,omitempty"`
	Lifecycle        *tasklifecycle.LifecycleImport     `json:"lifecycle,omitempty"`
}

var auditMu sync.Mutex

type AuditEvent struct {
	At             time.Time `json:"at"`
	ProjectID      string    `json:"project_id"`
	Actor          string    `json:"actor"`
	Source         string    `json:"source"`
	IdempotencyKey string    `json:"idempotency_key"`
	ResourceKinds  []string  `json:"resource_kinds"`
	Status         string    `json:"status"`
	Duplicate      bool      `json:"duplicate"`
}

type CreatedTask struct {
	ClientRef string `json:"client_ref"`
	TaskID    string `json:"task_id"`
	Revision  int    `json:"revision"`
}

type Receipt struct {
	ProjectID      string        `json:"project_id"`
	IdempotencyKey string        `json:"idempotency_key"`
	ResourceCount  int           `json:"resource_count"`
	ResourceKinds  []string      `json:"resource_kinds"`
	CreatedTasks   []CreatedTask `json:"created_tasks,omitempty"`
	Actor          string        `json:"actor"`
	Source         string        `json:"source"`
	CommittedAt    time.Time     `json:"committed_at"`
	ManifestHash   string        `json:"manifest_hash"`
}

type journal struct {
	State      string `json:"state"`
	ProjectID  string `json:"project_id"`
	BackupDir  string `json:"backup_dir"`
	ProjectDir string `json:"project_dir"`
}

type Service struct {
	Root       string
	Registry   *projects.ProjectRegistry
	FaultAfter int // test-only deterministic fault injection; zero disables.
}

func NewService(root string, registry *projects.ProjectRegistry) *Service {
	return &Service{Root: root, Registry: registry}
}

func HashContent(content string) string {
	sum := sha256.Sum256([]byte(content))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func manifestHash(manifest Manifest) (string, error) {
	data, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	return HashContent(string(data)), nil
}

func resourcePayload(resource Resource) ([]byte, error) {
	switch resource.Kind {
	case "project.metadata", "task.metadata", "task.create":
		return json.Marshal(resource.Metadata)
	case "task.review":
		if (resource.ReviewRequest == nil) == (resource.ReviewResult == nil) {
			return nil, errors.New("exactly one of review_request or review_result is required")
		}
		if resource.ReviewRequest != nil {
			request := *resource.ReviewRequest
			request.ExpectedRevision = resource.ExpectedRevision
			return json.Marshal(request)
		}
		result := *resource.ReviewResult
		result.ExpectedRevision = resource.ExpectedRevision
		return json.Marshal(result)
	default:
		return []byte(resource.Content), nil
	}
}

func (s *Service) validate(manifest Manifest) error {
	if strings.TrimSpace(manifest.ProjectID) == "" || strings.TrimSpace(manifest.IdempotencyKey) == "" {
		return errors.New("project_id and idempotency_key are required")
	}
	if manifest.ManifestVersion != ManifestVersion {
		return fmt.Errorf("unsupported manifest_version %q", manifest.ManifestVersion)
	}
	if manifest.Source != SourceMoteManualSync {
		return fmt.Errorf("unsupported manifest source %q", manifest.Source)
	}
	if s.Registry.Get(manifest.ProjectID) == nil {
		return tasklifecycle.ErrNotFound
	}
	if len(manifest.Resources) == 0 || len(manifest.Resources) > maxResources {
		return errors.New("resources must contain 1..128 entries")
	}
	seen := map[string]bool{}
	lifecycle := tasklifecycle.NewService(s.Root, taskdocs.NewDocService())
	createRefs := map[string]bool{}
	for _, resource := range manifest.Resources {
		key := resource.Kind + "\x00" + resource.TaskID + "\x00" + resource.ClientRef
		if seen[key] {
			return fmt.Errorf("duplicate resource %s", resource.Kind)
		}
		seen[key] = true
		payload, err := resourcePayload(resource)
		if err != nil {
			return err
		}
		if len(payload) > maxContentBytes || !utf8.Valid(payload) || resource.ContentHash != HashContent(string(payload)) {
			return fmt.Errorf("invalid content/hash for %s", resource.Kind)
		}
		switch resource.Kind {
		case "project.metadata":
			if len(resource.Metadata) != 1 || resource.Metadata["product_line"] == nil || resource.ExpectedETag == "" {
				return errors.New("project.metadata only permits product_line and requires expected_etag")
			}
		case "task.create":
			if resource.TaskID != "" || strings.TrimSpace(resource.ClientRef) == "" || resource.ExpectedRevision != nil || resource.ExpectedVersion != nil || resource.ExpectedETag != "" {
				return errors.New("task.create requires only client_ref and task metadata")
			}
			if createRefs[resource.ClientRef] {
				return errors.New("duplicate task.create client_ref")
			}
			createRefs[resource.ClientRef] = true
		case "task.metadata":
			if resource.TaskID == "" || resource.ClientRef != "" || resource.ExpectedRevision == nil {
				return errors.New("task.metadata requires task_id and expected_revision")
			}
			if _, err = lifecycle.Get(manifest.ProjectID, resource.TaskID); err != nil {
				return err
			}
		case "project.feature_list", "project.architecture_design":
			if resource.ExpectedVersion == nil || *resource.ExpectedVersion < 0 || (*resource.ExpectedVersion > 0 && resource.ExpectedETag == "") {
				return errors.New("project document requires a non-negative expected_version and expected_etag after version zero")
			}
		case "task.requirements", "task.design", "task.test":
			if resource.ClientRef != "" {
				if resource.TaskID != "" || !createRefs[resource.ClientRef] || resource.ExpectedVersion != nil || resource.ExpectedETag != "" {
					return errors.New("new task document requires an earlier task.create client_ref and no remote precondition")
				}
				break
			}
			if resource.TaskID == "" || resource.ExpectedVersion == nil || *resource.ExpectedVersion < 0 || (*resource.ExpectedVersion > 0 && resource.ExpectedETag == "") {
				return errors.New("task document requires task_id, a non-negative expected_version, and expected_etag after version zero")
			}
			if _, err = lifecycle.Get(manifest.ProjectID, resource.TaskID); err != nil {
				return err
			}
		case "task.review":
			if resource.TaskID == "" || resource.ExpectedRevision == nil {
				return errors.New("task.review requires task_id and expected_revision")
			}
		case "task.lifecycle_import":
			if resource.Lifecycle == nil || resource.ExpectedRevision == nil || resource.ReviewRequest != nil || resource.ReviewResult != nil {
				return errors.New("task.lifecycle_import requires lifecycle and expected_revision")
			}
			if resource.ClientRef != "" {
				if resource.TaskID != "" || !createRefs[resource.ClientRef] || *resource.ExpectedRevision != 1 {
					return errors.New("new task lifecycle import requires an earlier task.create client_ref and expected_revision 1")
				}
			} else if resource.TaskID == "" {
				return errors.New("task.lifecycle_import requires task_id or client_ref")
			}
		default:
			return fmt.Errorf("unsupported resource kind %q", resource.Kind)
		}
	}
	return nil
}

var ErrPreconditionFailed = errors.New("publish precondition failed")

func (s *Service) Publish(manifest Manifest, actor string) (Receipt, bool, error) {
	return s.publish(manifest, actor, nil)
}

func (s *Service) PublishConditional(manifest Manifest, actor string, precondition func() error) (Receipt, bool, error) {
	return s.publish(manifest, actor, precondition)
}

func (s *Service) publish(manifest Manifest, actor string, precondition func() error) (receipt Receipt, duplicate bool, publishErr error) {
	defer func() {
		if auditErr := s.audit(manifest, actor, receipt, duplicate, publishErr); auditErr != nil && publishErr == nil {
			publishErr = fmt.Errorf("write publish audit: %w", auditErr)
		}
	}()
	if err := s.validate(manifest); err != nil {
		return Receipt{}, false, err
	}
	g := gate(manifest.ProjectID)
	g.Lock()
	defer g.Unlock()
	manifestDigest, err := manifestHash(manifest)
	if err != nil {
		return Receipt{}, false, err
	}
	if receipt, ok, lookupErr := s.lookupReceipt(manifest.ProjectID, manifest.IdempotencyKey); lookupErr != nil {
		return Receipt{}, false, lookupErr
	} else if ok {
		if receipt.ManifestHash != manifestDigest {
			return Receipt{}, false, errors.New("idempotency key reused with different manifest")
		}
		return receipt, true, nil
	}
	if precondition != nil {
		if err := precondition(); err != nil {
			return Receipt{}, false, fmt.Errorf("%w: %v", ErrPreconditionFailed, err)
		}
	}
	projectDir := filepath.Join(s.Root, manifest.ProjectID)
	txRoot := filepath.Join(s.Root, ".publish-transactions")
	if err = os.MkdirAll(txRoot, 0o755); err != nil {
		return Receipt{}, false, err
	}
	txDir, err := os.MkdirTemp(txRoot, "tx-")
	if err != nil {
		return Receipt{}, false, err
	}
	backupDir := filepath.Join(txDir, "project")
	if err = copyTree(projectDir, backupDir); err != nil {
		return Receipt{}, false, err
	}
	projectsBackup := filepath.Join(txDir, "server_projects.json")
	if err = copyFile(projects.ProjectsStatePath, projectsBackup); err != nil {
		return Receipt{}, false, err
	}
	j := journal{State: "applying", ProjectID: manifest.ProjectID, BackupDir: backupDir, ProjectDir: projectDir}
	if err = writeJSON(filepath.Join(txDir, "journal.json"), j); err != nil {
		return Receipt{}, false, err
	}
	oldProductLine := s.Registry.Get(manifest.ProjectID).ProductLine
	rollback := func(cause error) (Receipt, bool, error) {
		if restoreErr := restoreTransaction(s.Root, txDir, j); restoreErr != nil {
			return Receipt{}, false, fmt.Errorf("publish failed: %v; rollback failed: %w", cause, restoreErr)
		}
		if restored := s.Registry.Get(manifest.ProjectID); restored != nil {
			copyProject := *restored
			copyProject.ProductLine = oldProductLine
			s.Registry.Set(&copyProject)
		}
		j.State = "rolled_back"
		if err := writeJSON(filepath.Join(txDir, "journal.json"), j); err != nil {
			return Receipt{}, false, fmt.Errorf("publish failed: %v; record rollback failed: %w", cause, err)
		}
		if err := os.RemoveAll(txDir); err != nil {
			return Receipt{}, false, fmt.Errorf("publish failed: %v; cleanup rollback failed: %w", cause, err)
		}
		return Receipt{}, false, cause
	}
	createdByRef := map[string]CreatedTask{}
	createdTasks := make([]CreatedTask, 0)
	for index, resource := range manifest.Resources {
		if resource.ClientRef != "" && resource.Kind != "task.create" {
			created, ok := createdByRef[resource.ClientRef]
			if !ok {
				return rollback(errors.New("task client_ref was not created"))
			}
			resource.TaskID = created.TaskID
		}
		if resource.Kind == "task.create" {
			created, createErr := s.createTask(manifest.ProjectID, resource)
			if createErr != nil {
				return rollback(createErr)
			}
			createdByRef[created.ClientRef] = created
			createdTasks = append(createdTasks, created)
		} else if applyErr := s.apply(manifest.ProjectID, resource, actor, manifest.Source); applyErr != nil {
			return rollback(applyErr)
		}
		if s.FaultAfter > 0 && index+1 == s.FaultAfter {
			return rollback(errors.New("injected publish failure"))
		}
	}
	kinds := make([]string, 0, len(manifest.Resources))
	for _, resource := range manifest.Resources {
		kinds = append(kinds, resource.Kind)
	}
	receipt = Receipt{ProjectID: manifest.ProjectID, IdempotencyKey: manifest.IdempotencyKey, ResourceCount: len(manifest.Resources), ResourceKinds: kinds, CreatedTasks: createdTasks, Actor: actor, Source: manifest.Source, CommittedAt: time.Now().UTC(), ManifestHash: manifestDigest}
	if err = s.storeReceipt(receipt); err != nil {
		return rollback(err)
	}
	j.State = "committed"
	if err = writeJSON(filepath.Join(txDir, "journal.json"), j); err != nil {
		return rollback(err)
	}
	_ = os.RemoveAll(txDir)
	return receipt, false, nil
}

func (s *Service) createTask(projectID string, resource Resource) (CreatedTask, error) {
	lifecycle := tasklifecycle.NewService(s.Root, taskdocs.NewDocService())
	view, err := lifecycle.Create(projectID, resource.Metadata)
	if err != nil {
		return CreatedTask{}, err
	}
	taskID, _ := view["id"].(string)
	revision, _ := view["revision"].(float64)
	if taskID == "" {
		return CreatedTask{}, errors.New("created task response is missing id")
	}
	return CreatedTask{ClientRef: resource.ClientRef, TaskID: taskID, Revision: int(revision)}, nil
}

func (s *Service) apply(projectID string, resource Resource, actor, source string) error {
	lifecycle := tasklifecycle.NewService(s.Root, taskdocs.NewDocService())
	docs := docslot.NewUnifiedDocService(s.Root)
	switch resource.Kind {
	case "project.metadata":
		project := s.Registry.Get(projectID)
		if HashContent(project.ProductLine) != resource.ExpectedETag {
			return tasklifecycle.ErrVersionConflict
		}
		var productLine string
		if err := json.Unmarshal(resource.Metadata["product_line"], &productLine); err != nil {
			return err
		}
		copyProject := *project
		copyProject.ProductLine = productLine
		s.Registry.Set(&copyProject)
		return projects.SaveProjects(s.Registry)
	case "task.metadata":
		_, err := lifecycle.Update(projectID, resource.TaskID, resource.Metadata, resource.ExpectedRevision)
		return err
	case "task.review":
		if resource.ReviewRequest != nil {
			request := *resource.ReviewRequest
			request.ExpectedRevision = resource.ExpectedRevision
			_, _, err := lifecycle.RequestReview(projectID, resource.TaskID, actor, request)
			return err
		}
		result := *resource.ReviewResult
		result.ExpectedRevision = resource.ExpectedRevision
		_, _, err := lifecycle.SetReviewResult(projectID, resource.TaskID, actor, result)
		return err
	case "task.lifecycle_import":
		_, err := lifecycle.ImportLifecycle(projectID, resource.TaskID, actor, resource.ExpectedRevision, *resource.Lifecycle)
		return err
	}
	scope, slot, taskID := docslot.ScopeProject, "", ""
	switch resource.Kind {
	case "project.feature_list":
		slot = "feature_list"
	case "project.architecture_design":
		slot = "architecture_design"
	case "task.requirements":
		scope, slot, taskID = docslot.ScopeTask, "requirements", resource.TaskID
	case "task.design":
		scope, slot, taskID = docslot.ScopeTask, "design", resource.TaskID
	case "task.test":
		scope, slot, taskID = docslot.ScopeTask, "test", resource.TaskID
	}
	current, err := docs.Export(scope, projectID, slot)
	if scope == docslot.ScopeTask {
		current, err = docs.ExportTask(projectID, taskID, slot)
	}
	if err != nil {
		return err
	}
	if resource.ClientRef != "" {
		resource.ExpectedETag = current.ETag
		version := current.Version
		resource.ExpectedVersion = &version
	} else if current.ETag != resource.ExpectedETag {
		return tasklifecycle.ErrVersionConflict
	}
	if scope == docslot.ScopeProject {
		_, err = docs.Append(scope, projectID, slot, resource.Content, actor, resource.ExpectedVersion, "replace_full", source)
	} else {
		_, err = docs.AppendTask(projectID, taskID, slot, resource.Content, actor, resource.ExpectedVersion, "replace_full", source)
	}
	return err
}

func auditResourceKind(kind string) string {
	switch kind {
	case "project.metadata", "project.feature_list", "project.architecture_design", "task.create", "task.metadata", "task.requirements", "task.design", "task.test", "task.review":
		return kind
	default:
		return "<unsupported>"
	}
}

func (s *Service) audit(manifest Manifest, actor string, receipt Receipt, duplicate bool, publishErr error) error {
	kinds := make([]string, 0, len(manifest.Resources))
	for _, resource := range manifest.Resources {
		kinds = append(kinds, auditResourceKind(resource.Kind))
	}
	status := "committed"
	if publishErr != nil {
		status = "failed"
	}
	projectID := "<unknown>"
	if s.Registry.Get(manifest.ProjectID) != nil {
		projectID = manifest.ProjectID
	}
	source := "<unsupported>"
	if manifest.Source == SourceMoteManualSync {
		source = manifest.Source
	}
	idempotencyHash := ""
	if manifest.IdempotencyKey != "" {
		idempotencyHash = HashContent(manifest.IdempotencyKey)
	}
	event := AuditEvent{At: time.Now().UTC(), ProjectID: projectID, Actor: actor, Source: source, IdempotencyKey: idempotencyHash, ResourceKinds: kinds, Status: status, Duplicate: duplicate}
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	auditMu.Lock()
	defer auditMu.Unlock()
	file, err := os.OpenFile(filepath.Join(s.Root, ".publish-audit.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(append(data, '\n')); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func receiptPath(root, projectID string) string {
	return filepath.Join(root, projectID, ".publish", "receipts.json")
}
func (s *Service) lookupReceipt(projectID, key string) (Receipt, bool, error) {
	var receipts []Receipt
	err := readJSON(receiptPath(s.Root, projectID), &receipts)
	if os.IsNotExist(err) {
		return Receipt{}, false, nil
	}
	if err != nil {
		return Receipt{}, false, fmt.Errorf("read idempotency store: %w", err)
	}
	for _, r := range receipts {
		if r.IdempotencyKey == key {
			return r, true, nil
		}
	}
	return Receipt{}, false, nil
}
func (s *Service) storeReceipt(receipt Receipt) error {
	path := receiptPath(s.Root, receipt.ProjectID)
	var receipts []Receipt
	if err := readJSON(path, &receipts); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("read idempotency store: %w", err)
	}
	cutoff := time.Now().Add(-90 * 24 * time.Hour)
	kept := make([]Receipt, 0, len(receipts)+1)
	for _, r := range receipts {
		if r.CommittedAt.After(cutoff) {
			kept = append(kept, r)
		}
	}
	kept = append(kept, receipt)
	if len(kept) > 10000 {
		kept = kept[len(kept)-10000:]
	}
	return writeJSON(path, kept)
}

func validStoredProjectID(id string) bool {
	return id != "" && id != "." && id != ".." && filepath.Base(id) == id && !strings.ContainsAny(id, `/\\`)
}

func restoreTransaction(root, txDir string, j journal) error {
	if !validStoredProjectID(j.ProjectID) {
		return errors.New("invalid project id in publish journal")
	}
	projectDir := filepath.Join(root, j.ProjectID)
	backupDir := filepath.Join(txDir, "project")
	if err := os.RemoveAll(projectDir); err != nil {
		return err
	}
	if err := copyTree(backupDir, projectDir); err != nil {
		return err
	}
	return copyFile(filepath.Join(txDir, "server_projects.json"), projects.ProjectsStatePath)
}

func Recover(root string) error {
	entries, err := os.ReadDir(filepath.Join(root, ".publish-transactions"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		txDir := filepath.Join(root, ".publish-transactions", entry.Name())
		var j journal
		if err = readJSON(filepath.Join(txDir, "journal.json"), &j); err != nil {
			return fmt.Errorf("read publish journal: %w", err)
		}
		switch j.State {
		case "applying":
			if err = restoreTransaction(root, txDir, j); err != nil {
				return err
			}
		case "committed", "rolled_back":
			// Cleanup only.
		default:
			return fmt.Errorf("invalid publish journal state %q", j.State)
		}
		if err = os.RemoveAll(txDir); err != nil {
			return err
		}
	}
	return nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err = os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
func readJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, target)
}
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err = os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.Mode()&os.ModeSymlink != 0 {
			return errors.New("symlink in project tree")
		}
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode().Perm())
		}
		return copyFile(path, target)
	})
}

func DecodeManifest(reader io.Reader) (Manifest, error) {
	var manifest Manifest
	raw, err := io.ReadAll(io.LimitReader(reader, maxManifestSize+1))
	if err != nil {
		return manifest, err
	}
	if len(raw) > maxManifestSize || !utf8.Valid(raw) {
		return manifest, errors.New("manifest exceeds size limit or is not UTF-8")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&manifest); err != nil {
		return manifest, err
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return manifest, errors.New("multiple JSON values")
	}
	return manifest, nil
}
