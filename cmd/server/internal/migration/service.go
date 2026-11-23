package migration

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/houzhh15/AIDG/cmd/server/internal/domain/docslot"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/projects"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/taskdocs"
	"github.com/houzhh15/AIDG/cmd/server/internal/domain/tasklifecycle"
	"github.com/houzhh15/AIDG/cmd/server/internal/publish"
)

const (
	PlanVersion    = "1"
	maxTasksBytes  = 16 << 20
	maxLegacyBytes = 4 << 20
)

var (
	ErrConflict       = errors.New("migration plan conflict")
	ErrBlocked        = errors.New("migration blocked")
	ErrInvalidScope   = errors.New("invalid migration scope")
	ErrInvalidPlan    = errors.New("invalid migration plan")
	canonicalStatuses = map[string]bool{"todo": true, "in-progress": true, "review": true, "completed": true}
	legacyStatusesV1  = map[string]string{"in_progress": "in-progress", "done": "completed"}
)

type Summary struct {
	Scanned         int    `json:"scanned"`
	PlannedChanges  int    `json:"planned_changes"`
	Changed         int    `json:"changed"`
	Skipped         int    `json:"skipped"`
	Conflicts       int    `json:"conflicts"`
	UnknownStatuses int    `json:"unknown_statuses"`
	Failed          int    `json:"failed"`
	Recovery        string `json:"recovery,omitempty"`
}

type Issue struct {
	Code     string `json:"code"`
	Resource string `json:"resource"`
	Detail   string `json:"detail,omitempty"`
	Blocking bool   `json:"blocking"`
}

type Change struct {
	Kind       string `json:"kind"`
	Resource   string `json:"resource"`
	TaskID     string `json:"task_id,omitempty"`
	Slot       string `json:"slot,omitempty"`
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
	Source     string `json:"source,omitempty"`
	SourceHash string `json:"source_hash,omitempty"`
}

type Baseline struct {
	RegistryHash string            `json:"registry_hash"`
	TasksHash    string            `json:"tasks_hash"`
	Sources      map[string]string `json:"sources,omitempty"`
	Targets      map[string]string `json:"targets,omitempty"`
}

type LegacyRecord struct {
	Resource string `json:"resource"`
	State    string `json:"state"`
	Hash     string `json:"hash,omitempty"`
}

type Plan struct {
	PlanVersion          string         `json:"plan_version"`
	Scope                string         `json:"scope"`
	ProjectID            string         `json:"project_id"`
	TaskID               string         `json:"task_id,omitempty"`
	CurrentSchemaVersion int            `json:"current_schema_version"`
	TargetSchemaVersion  int            `json:"target_schema_version"`
	Baseline             Baseline       `json:"baseline"`
	Changes              []Change       `json:"changes"`
	Issues               []Issue        `json:"issues"`
	LegacyRecords        []LegacyRecord `json:"legacy_records"`
	Summary              Summary        `json:"summary"`
	Digest               string         `json:"digest"`
}

type Result struct {
	PlanDigest string  `json:"plan_digest"`
	ProjectID  string  `json:"project_id"`
	TaskID     string  `json:"task_id,omitempty"`
	Actor      string  `json:"actor"`
	Summary    Summary `json:"summary"`
}

type Service struct {
	Root       string
	Registry   *projects.ProjectRegistry
	FaultAfter int // test-only: fail after this many applied changes.
}

func NewService(root string, registry *projects.ProjectRegistry) *Service {
	return &Service{Root: root, Registry: registry}
}

type documentMapping struct {
	slot       string
	candidates []string
}

var projectDocumentMappings = []documentMapping{
	{slot: "architecture_design", candidates: []string{"architecture_new.md", "docs/architecture_design.md"}},
	{slot: "feature_list", candidates: []string{"feature_list.md", "docs/feature_list.md"}},
	{slot: "tech_design", candidates: []string{"tech_design_v1.md", "docs/tech_design.md"}},
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func validateID(value string) error {
	if value == "" || len(value) > 200 || value == "." || value == ".." || filepath.Base(value) != value || strings.ContainsAny(value, "/\\\x00") {
		return ErrInvalidScope
	}
	return nil
}

func (s *Service) projectDir(projectID string) (string, error) {
	if err := validateID(projectID); err != nil {
		return "", err
	}
	if s.Registry == nil || s.Registry.Get(projectID) == nil {
		return "", tasklifecycle.ErrNotFound
	}
	root, err := filepath.Abs(s.Root)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(root, projectID)
	rel, err := filepath.Rel(root, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", ErrInvalidScope
	}
	info, err := os.Lstat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", tasklifecycle.ErrNotFound
		}
		return "", err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", ErrInvalidScope
	}
	return dir, nil
}

func readBoundedRegular(path string, limit int64) ([]byte, bool, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return nil, true, fmt.Errorf("not a regular file")
	}
	if info.Size() > limit {
		return nil, true, fmt.Errorf("file exceeds %d bytes", limit)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, true, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil {
		return nil, true, err
	}
	if int64(len(data)) > limit {
		return nil, true, fmt.Errorf("file exceeds %d bytes", limit)
	}
	return data, true, nil
}

func rawString(task map[string]json.RawMessage, key string) (string, bool) {
	raw, ok := task[key]
	if !ok {
		return "", false
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return "", false
	}
	return value, true
}

func digestPlan(plan Plan) (string, error) {
	plan.Digest = ""
	plan.Summary.Changed = 0
	plan.Summary.Failed = 0
	plan.Summary.Recovery = ""
	data, err := json.Marshal(plan)
	if err != nil {
		return "", err
	}
	return hashBytes(data), nil
}

func hasBlockingIssues(issues []Issue) bool {
	for _, issue := range issues {
		if issue.Blocking {
			return true
		}
	}
	return false
}

func (s *Service) AnalyzeProject(projectID string) (Plan, error) {
	unlock := publish.ReadLock(projectID)
	defer unlock()
	return s.analyze(projectID, "")
}

func (s *Service) AnalyzeTask(projectID, taskID string) (Plan, error) {
	if err := validateID(taskID); err != nil {
		return Plan{}, err
	}
	unlock := publish.ReadLock(projectID)
	defer unlock()
	return s.analyze(projectID, taskID)
}

func (s *Service) analyze(projectID, selectedTaskID string) (Plan, error) {
	dir, err := s.projectDir(projectID)
	if err != nil {
		return Plan{}, err
	}
	project := s.Registry.Get(projectID)
	plan := Plan{
		PlanVersion:          PlanVersion,
		Scope:                "project",
		ProjectID:            projectID,
		CurrentSchemaVersion: project.SchemaVersion,
		TargetSchemaVersion:  projects.CurrentSchemaVersion,
		Baseline: Baseline{
			RegistryHash: hashProject(project),
			Sources:      map[string]string{},
			Targets:      map[string]string{},
		},
		Changes:       []Change{},
		Issues:        []Issue{},
		LegacyRecords: []LegacyRecord{},
	}
	if selectedTaskID != "" {
		plan.Scope = "task"
		plan.TaskID = selectedTaskID
	}

	tasksPath := filepath.Join(dir, "tasks.json")
	tasksData, exists, readErr := readBoundedRegular(tasksPath, maxTasksBytes)
	if readErr != nil {
		plan.Issues = append(plan.Issues, Issue{Code: "invalid_task_store", Resource: "tasks", Detail: readErr.Error(), Blocking: true})
	} else if !exists {
		if selectedTaskID != "" {
			return Plan{}, tasklifecycle.ErrNotFound
		}
		plan.Baseline.TasksHash = hashBytes(nil)
	} else {
		plan.Baseline.TasksHash = hashBytes(tasksData)
		var tasks []map[string]json.RawMessage
		if err := json.Unmarshal(tasksData, &tasks); err != nil {
			plan.Issues = append(plan.Issues, Issue{Code: "invalid_task_store", Resource: "tasks", Detail: "invalid JSON", Blocking: true})
		} else {
			found := selectedTaskID == ""
			for _, task := range tasks {
				taskID, validID := rawString(task, "id")
				if selectedTaskID != "" && taskID != selectedTaskID {
					continue
				}
				if selectedTaskID != "" {
					found = true
				}
				plan.Summary.Scanned++
				if !validID || validateID(taskID) != nil {
					plan.Issues = append(plan.Issues, Issue{Code: "invalid_task_id", Resource: "tasks", Blocking: true})
					continue
				}
				record, recordIssue := inspectExecutionPlan(dir, taskID)
				plan.LegacyRecords = append(plan.LegacyRecords, record)
				if recordIssue != nil {
					plan.Issues = append(plan.Issues, *recordIssue)
				}
				status, validStatus := rawString(task, "status")
				resource := "task:" + taskID
				switch {
				case !validStatus || strings.TrimSpace(status) == "":
					plan.Issues = append(plan.Issues, Issue{Code: "missing_or_invalid_status", Resource: resource, Blocking: true})
				case canonicalStatuses[status]:
					plan.Summary.Skipped++
				case legacyStatusesV1[status] != "":
					plan.Changes = append(plan.Changes, Change{Kind: "task_status", Resource: resource, TaskID: taskID, From: status, To: legacyStatusesV1[status]})
				default:
					plan.Issues = append(plan.Issues, Issue{Code: "unknown_status", Resource: resource, Detail: fmt.Sprintf("unrecognized value (%d bytes)", len(status)), Blocking: true})
					plan.Summary.UnknownStatuses++
				}
			}
			if !found {
				return Plan{}, tasklifecycle.ErrNotFound
			}
		}
	}

	if selectedTaskID == "" {
		s.analyzeProjectDocuments(dir, &plan)
		if project.SchemaVersion > projects.CurrentSchemaVersion {
			plan.Issues = append(plan.Issues, Issue{Code: "unsupported_schema_version", Resource: "project", Blocking: true})
		} else if project.SchemaVersion < projects.CurrentSchemaVersion && !hasBlockingIssues(plan.Issues) {
			plan.Changes = append(plan.Changes, Change{Kind: "schema_version", Resource: "project", From: fmt.Sprint(project.SchemaVersion), To: fmt.Sprint(projects.CurrentSchemaVersion)})
		} else if project.SchemaVersion == projects.CurrentSchemaVersion {
			plan.Summary.Skipped++
		}
	}

	sort.Slice(plan.Changes, func(i, j int) bool { return plan.Changes[i].Resource < plan.Changes[j].Resource })
	sort.Slice(plan.LegacyRecords, func(i, j int) bool { return plan.LegacyRecords[i].Resource < plan.LegacyRecords[j].Resource })
	sort.Slice(plan.Issues, func(i, j int) bool {
		if plan.Issues[i].Resource == plan.Issues[j].Resource {
			return plan.Issues[i].Code < plan.Issues[j].Code
		}
		return plan.Issues[i].Resource < plan.Issues[j].Resource
	})
	for _, issue := range plan.Issues {
		if strings.Contains(issue.Code, "conflict") {
			plan.Summary.Conflicts++
		}
	}
	plan.Summary.PlannedChanges = len(plan.Changes)
	plan.Digest, err = digestPlan(plan)
	return plan, err
}

func hashProject(project *projects.Project) string {
	if project == nil {
		return hashBytes(nil)
	}
	data, _ := json.Marshal(struct {
		ID            string `json:"id"`
		SchemaVersion int    `json:"schema_version"`
		UpdatedAt     string `json:"updated_at"`
	}{project.ID, project.SchemaVersion, project.UpdatedAt.UTC().Format(time.RFC3339Nano)})
	return hashBytes(data)
}

func (s *Service) analyzeProjectDocuments(projectDir string, plan *Plan) {
	for _, mapping := range projectDocumentMappings {
		plan.Summary.Scanned++
		resource := "project_document:" + mapping.slot
		type sourceValue struct {
			rel  string
			data []byte
			hash string
		}
		var sources []sourceValue
		for _, rel := range mapping.candidates {
			data, exists, err := readBoundedRegular(filepath.Join(projectDir, filepath.FromSlash(rel)), maxLegacyBytes)
			if err != nil {
				plan.Issues = append(plan.Issues, Issue{Code: "invalid_legacy_source", Resource: resource, Detail: rel, Blocking: true})
				continue
			}
			if exists {
				if !utf8.Valid(data) {
					plan.Issues = append(plan.Issues, Issue{Code: "invalid_legacy_source", Resource: resource, Detail: rel, Blocking: true})
					continue
				}
				hash := hashBytes(data)
				plan.Baseline.Sources[rel] = hash
				sources = append(sources, sourceValue{rel: rel, data: data, hash: hash})
			}
		}
		if len(sources) == 0 {
			plan.Summary.Skipped++
			continue
		}
		sort.Slice(sources, func(i, j int) bool { return sources[i].rel < sources[j].rel })
		for _, source := range sources[1:] {
			if source.hash != sources[0].hash {
				plan.Issues = append(plan.Issues, Issue{Code: "source_conflict", Resource: resource, Blocking: true})
				sources = nil
				break
			}
		}
		if len(sources) == 0 {
			continue
		}
		targetDir := filepath.Join(projectDir, "docs", mapping.slot)
		targetInfo, statErr := os.Lstat(targetDir)
		targetExists := statErr == nil
		if statErr != nil && !os.IsNotExist(statErr) {
			plan.Issues = append(plan.Issues, Issue{Code: "invalid_target", Resource: resource, Blocking: true})
			continue
		}
		if targetExists && (!targetInfo.IsDir() || targetInfo.Mode()&os.ModeSymlink != 0) {
			plan.Issues = append(plan.Issues, Issue{Code: "invalid_target", Resource: resource, Blocking: true})
			continue
		}
		content, meta, err := taskdocs.ExportReadOnlyWithPath(targetDir)
		if err != nil {
			plan.Issues = append(plan.Issues, Issue{Code: "invalid_target", Resource: resource, Blocking: true})
			continue
		}
		targetHash := hashBytes([]byte(content))
		if targetExists {
			plan.Baseline.Targets[mapping.slot] = fmt.Sprintf("%s@%d", targetHash, meta.Version)
			if targetHash == sources[0].hash {
				plan.Summary.Skipped++
				continue
			}
			plan.Issues = append(plan.Issues, Issue{Code: "target_conflict", Resource: resource, Blocking: true})
			continue
		}
		plan.Baseline.Targets[mapping.slot] = "absent"
		plan.Changes = append(plan.Changes, Change{Kind: "project_document", Resource: resource, Slot: mapping.slot, Source: sources[0].rel, SourceHash: sources[0].hash})
	}
}

func inspectExecutionPlan(projectDir, taskID string) (LegacyRecord, *Issue) {
	resource := "execution_plan:" + taskID
	path := filepath.Join(projectDir, "tasks", taskID, "execution_plan.md")
	data, exists, err := readBoundedRegular(path, maxLegacyBytes)
	if err != nil {
		issue := Issue{Code: "invalid_execution_plan", Resource: resource, Blocking: false}
		return LegacyRecord{Resource: resource, State: "invalid"}, &issue
	}
	if !exists {
		return LegacyRecord{Resource: resource, State: "missing"}, nil
	}
	if !utf8.Valid(data) {
		issue := Issue{Code: "invalid_execution_plan", Resource: resource, Blocking: false}
		return LegacyRecord{Resource: resource, State: "invalid"}, &issue
	}
	return LegacyRecord{Resource: resource, State: "present", Hash: hashBytes(data)}, nil
}

type transactionJournal struct {
	State            string `json:"state"`
	ProjectID        string `json:"project_id"`
	StateFileExisted bool   `json:"state_file_existed"`
}

func (s *Service) Execute(plan Plan, actor string) (Result, error) {
	if strings.TrimSpace(actor) == "" || plan.PlanVersion != PlanVersion || plan.ProjectID == "" {
		return Result{}, ErrInvalidPlan
	}
	expectedDigest, err := digestPlan(plan)
	if err != nil || expectedDigest != plan.Digest {
		return Result{}, ErrInvalidPlan
	}
	unlock := publish.WriteLock(plan.ProjectID)
	defer unlock()

	current, err := s.analyze(plan.ProjectID, plan.TaskID)
	if err != nil {
		return Result{}, err
	}
	if current.Scope != plan.Scope || current.Digest != plan.Digest {
		return Result{}, ErrConflict
	}
	if hasBlockingIssues(current.Issues) {
		return Result{}, ErrBlocked
	}
	result := Result{PlanDigest: plan.Digest, ProjectID: plan.ProjectID, TaskID: plan.TaskID, Actor: actor, Summary: current.Summary}
	if len(current.Changes) == 0 {
		return result, nil
	}

	projectDir, err := s.projectDir(plan.ProjectID)
	if err != nil {
		return Result{}, err
	}
	txDir, err := s.prepareTransaction(plan.ProjectID, projectDir)
	if err != nil {
		return Result{}, err
	}
	oldProject := *s.Registry.Get(plan.ProjectID)
	rollback := func(cause error) (Result, error) {
		result.Summary.Failed++
		restoreErr := restoreTransaction(s.Root, txDir)
		s.Registry.Set(&oldProject)
		if restoreErr != nil {
			result.Summary.Recovery = "pending_recovery"
			return result, fmt.Errorf("migration failed: %v; recovery required: %w", cause, restoreErr)
		}
		result.Summary.Recovery = "rolled_back"
		_ = os.RemoveAll(txDir)
		return result, cause
	}

	applied := 0
	for _, change := range current.Changes {
		switch change.Kind {
		case "project_document":
			data, exists, readErr := readBoundedRegular(filepath.Join(projectDir, filepath.FromSlash(change.Source)), maxLegacyBytes)
			if readErr != nil || !exists || hashBytes(data) != change.SourceHash {
				return rollback(ErrConflict)
			}
			docs := docslot.NewUnifiedDocService(s.Root)
			version := 0
			if _, writeErr := docs.Append(docslot.ScopeProject, plan.ProjectID, change.Slot, string(data), actor, &version, "replace_full", "schema_migration_v1"); writeErr != nil {
				return rollback(writeErr)
			}
			applied++
		case "task_status":
			// All task status changes are applied in one raw-preserving write below.
		case "schema_version":
			// The schema marker is deliberately the final visible change.
		default:
			return rollback(ErrInvalidPlan)
		}
		if s.FaultAfter > 0 && applied == s.FaultAfter {
			return rollback(errors.New("injected migration failure"))
		}
	}

	statusChanges := make(map[string]Change)
	for _, change := range current.Changes {
		if change.Kind == "task_status" {
			statusChanges[change.TaskID] = change
		}
	}
	if len(statusChanges) > 0 {
		if err := applyTaskStatusChanges(filepath.Join(projectDir, "tasks.json"), current.Baseline.TasksHash, statusChanges); err != nil {
			return rollback(err)
		}
		applied += len(statusChanges)
		if s.FaultAfter > 0 && applied >= s.FaultAfter {
			return rollback(errors.New("injected migration failure"))
		}
	}

	for _, change := range current.Changes {
		if change.Kind != "schema_version" {
			continue
		}
		project := s.Registry.Get(plan.ProjectID)
		if project == nil || hashProject(project) != current.Baseline.RegistryHash {
			return rollback(ErrConflict)
		}
		updated := *project
		updated.SchemaVersion = projects.CurrentSchemaVersion
		updated.UpdatedAt = time.Now().UTC()
		s.Registry.Set(&updated)
		if err := projects.SaveProjects(s.Registry); err != nil {
			return rollback(err)
		}
		applied++
		if s.FaultAfter > 0 && applied >= s.FaultAfter {
			return rollback(errors.New("injected migration failure"))
		}
	}

	if err := writeJournal(txDir, transactionJournal{State: "committed", ProjectID: plan.ProjectID}); err != nil {
		return rollback(err)
	}
	if err := os.RemoveAll(txDir); err != nil {
		result.Summary.Recovery = "committed; transaction cleanup pending"
	}
	result.Summary.Changed = applied
	return result, nil
}

func applyTaskStatusChanges(path, expectedHash string, changes map[string]Change) error {
	data, exists, err := readBoundedRegular(path, maxTasksBytes)
	if err != nil || !exists {
		if err != nil {
			return err
		}
		return ErrConflict
	}
	if hashBytes(data) != expectedHash {
		return ErrConflict
	}
	var tasks []map[string]json.RawMessage
	if err := json.Unmarshal(data, &tasks); err != nil {
		return err
	}
	remaining := make(map[string]Change, len(changes))
	for key, value := range changes {
		remaining[key] = value
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	for _, task := range tasks {
		id, ok := rawString(task, "id")
		change, selected := remaining[id]
		if !ok || !selected {
			continue
		}
		status, valid := rawString(task, "status")
		if !valid || status != change.From {
			return ErrConflict
		}
		statusRaw, _ := json.Marshal(change.To)
		task["status"] = statusRaw
		var revision int
		_ = json.Unmarshal(task["revision"], &revision)
		revisionRaw, _ := json.Marshal(revision + 1)
		task["revision"] = revisionRaw
		updatedRaw, _ := json.Marshal(now)
		task["updated_at"] = updatedRaw
		delete(remaining, id)
	}
	if len(remaining) != 0 {
		return ErrConflict
	}
	encoded, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(path, append(encoded, '\n'))
}

func atomicWrite(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".migration-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
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
	if err := os.Rename(name, path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func projectStatePath(root string) (string, error) {
	statePath := projects.ProjectsStatePath
	if statePath == "" {
		statePath = filepath.Join(root, "server_projects.json")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	stateAbs, err := filepath.Abs(statePath)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(rootAbs, stateAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", ErrInvalidScope
	}
	return stateAbs, nil
}

func (s *Service) prepareTransaction(projectID, projectDir string) (string, error) {
	txRoot := filepath.Join(s.Root, ".schema-migration-transactions")
	if err := os.MkdirAll(txRoot, 0o700); err != nil {
		return "", err
	}
	txDir, err := os.MkdirTemp(txRoot, "tx-")
	if err != nil {
		return "", err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.RemoveAll(txDir)
		}
	}()
	if err := copyTree(projectDir, filepath.Join(txDir, "project")); err != nil {
		return "", err
	}
	statePath, err := projectStatePath(s.Root)
	if err != nil {
		return "", err
	}
	stateData, stateExisted, err := readBoundedRegular(statePath, maxTasksBytes)
	if err != nil {
		return "", err
	}
	if stateExisted {
		if err := os.WriteFile(filepath.Join(txDir, "server_projects.json"), stateData, 0o600); err != nil {
			return "", err
		}
	}
	if err := writeJournal(txDir, transactionJournal{State: "applying", ProjectID: projectID, StateFileExisted: stateExisted}); err != nil {
		return "", err
	}
	cleanup = false
	return txDir, nil
}

func writeJournal(txDir string, journal transactionJournal) error {
	data, err := json.Marshal(journal)
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(txDir, "journal.json"), append(data, '\n'))
}

func copyTree(source, target string) error {
	type directoryTime struct {
		path string
		time time.Time
	}
	var directories []directoryTime
	err := filepath.Walk(source, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, rel)
		if info.Mode()&os.ModeSymlink != 0 || (!info.Mode().IsRegular() && !info.IsDir()) {
			return fmt.Errorf("unsupported project entry %s", rel)
		}
		if info.IsDir() {
			if err := os.MkdirAll(destination, info.Mode().Perm()); err != nil {
				return err
			}
			directories = append(directories, directoryTime{path: destination, time: info.ModTime()})
			return nil
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
			_ = input.Close()
			return err
		}
		output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, info.Mode().Perm())
		if err != nil {
			_ = input.Close()
			return err
		}
		_, copyErr := io.Copy(output, input)
		inputCloseErr := input.Close()
		outputCloseErr := output.Close()
		if copyErr != nil {
			return copyErr
		}
		if inputCloseErr != nil {
			return inputCloseErr
		}
		if outputCloseErr != nil {
			return outputCloseErr
		}
		return os.Chtimes(destination, info.ModTime(), info.ModTime())
	})
	if err != nil {
		return err
	}
	for index := len(directories) - 1; index >= 0; index-- {
		if err := os.Chtimes(directories[index].path, directories[index].time, directories[index].time); err != nil {
			return err
		}
	}
	return nil
}

func restoreTransaction(root, txDir string) error {
	data, exists, err := readBoundedRegular(filepath.Join(txDir, "journal.json"), 64<<10)
	if err != nil || !exists {
		if err != nil {
			return err
		}
		return errors.New("migration journal missing")
	}
	var journal transactionJournal
	if json.Unmarshal(data, &journal) != nil || validateID(journal.ProjectID) != nil {
		return errors.New("invalid migration journal")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	projectDir := filepath.Join(rootAbs, journal.ProjectID)
	rel, err := filepath.Rel(rootAbs, projectDir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return ErrInvalidScope
	}
	backupDir := filepath.Join(txDir, "project")
	if info, err := os.Lstat(backupDir); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("migration project backup missing")
	}
	restoreDir, err := os.MkdirTemp(rootAbs, ".migration-restore-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(restoreDir)
	staged := filepath.Join(restoreDir, journal.ProjectID)
	if err := copyTree(backupDir, staged); err != nil {
		return err
	}
	if err := os.RemoveAll(projectDir); err != nil {
		return err
	}
	if err := os.Rename(staged, projectDir); err != nil {
		return err
	}
	statePath, err := projectStatePath(root)
	if err != nil {
		return err
	}
	stateBackup := filepath.Join(txDir, "server_projects.json")
	state, backupExists, err := readBoundedRegular(stateBackup, maxTasksBytes)
	if err != nil {
		return err
	}
	if journal.StateFileExisted {
		if !backupExists {
			return errors.New("migration state backup missing")
		}
		if err := atomicWrite(statePath, state); err != nil {
			return err
		}
	} else if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// Recover restores every interrupted migration before project registries load.
func Recover(root string) error {
	txRoot := filepath.Join(root, ".schema-migration-transactions")
	entries, err := os.ReadDir(txRoot)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "tx-") {
			continue
		}
		txDir := filepath.Join(txRoot, entry.Name())
		data, exists, readErr := readBoundedRegular(filepath.Join(txDir, "journal.json"), 64<<10)
		if readErr != nil {
			return fmt.Errorf("read migration journal %s: %w", entry.Name(), readErr)
		}
		if !exists {
			// No project mutation begins before the applying journal is durable.
			// A journal-less transaction is therefore an abandoned preparation.
			if err := os.RemoveAll(txDir); err != nil {
				return err
			}
			continue
		}
		var journal transactionJournal
		if err := json.Unmarshal(data, &journal); err != nil {
			return fmt.Errorf("decode migration journal %s: %w", entry.Name(), err)
		}
		if journal.State == "applying" {
			if err := restoreTransaction(root, txDir); err != nil {
				return fmt.Errorf("recover migration %s: %w", entry.Name(), err)
			}
		} else if journal.State != "committed" {
			return fmt.Errorf("unknown migration journal state %q", journal.State)
		}
		if err := os.RemoveAll(txDir); err != nil {
			return err
		}
	}
	return nil
}
