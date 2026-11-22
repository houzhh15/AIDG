package meetingv2

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newMigrationStore(t *testing.T) (*Store, string) {
	t.Helper()
	root := t.TempDir()
	store, err := NewStore(root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	return store, root
}

func writeLegacyTaskFile(t *testing.T, root string, ids ...string) {
	t.Helper()
	tasks := make([]map[string]any, 0, len(ids))
	for _, id := range ids {
		tasks = append(tasks, map[string]any{"id": id, "config": map[string]any{"OutputDir": filepath.Join(root, id)}, "state": "created", "created_at": "2024-01-01T00:00:00Z"})
	}
	data, _ := json.Marshal(map[string]any{"tasks": tasks})
	if err := os.WriteFile(filepath.Join(root, "server_tasks.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func writeLegacyMeeting(t *testing.T, root, id string) string {
	t.Helper()
	dir := filepath.Join(root, id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"merged_all.txt":     "legacy transcript\n",
		"polish_all.md":      "# Legacy polished\n",
		"meeting-summary.md": "# Legacy summary\n",
		"unknown.keep":       "must remain untouched\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestMigrateLegacyDocumentsIsIdempotentAndPreservesSources(t *testing.T) {
	store, root := newMigrationStore(t)
	writeLegacyTaskFile(t, root, "legacy-1")
	dir := writeLegacyMeeting(t, root, "legacy-1")

	first, err := store.MigrateLegacy()
	if err != nil {
		t.Fatal(err)
	}
	if first.Migrated != 1 || first.Failed != 0 {
		t.Fatalf("first=%+v", first)
	}
	session, err := store.LoadSession("legacy-1")
	if err != nil {
		t.Fatal(err)
	}
	if session.Owner != "" || session.Metadata["task_id"] != "legacy-1" || session.TranscriptionMode != TranscriptionManual {
		t.Fatalf("session=%+v", session)
	}
	for kind, expected := range map[DocumentKind]string{
		DocumentRawTranscript: "legacy transcript\n", DocumentPolishedTranscript: "# Legacy polished\n", DocumentMeetingSummary: "# Legacy summary\n",
	} {
		document, err := store.GetActiveDocument("legacy-1", kind)
		if err != nil || document.Content != expected || document.Source != DocumentSourceGenerated {
			t.Fatalf("kind=%s document=%+v err=%v", kind, document, err)
		}
	}
	if data, err := os.ReadFile(filepath.Join(dir, "unknown.keep")); err != nil || string(data) != "must remain untouched\n" {
		t.Fatalf("unknown source modified: %q %v", data, err)
	}
	second, err := store.MigrateLegacy()
	if err != nil {
		t.Fatal(err)
	}
	if second.Skipped != 1 || second.Migrated != 0 {
		t.Fatalf("second=%+v", second)
	}
	state := readMigrationStateForTest(t, root)
	if state.Items["legacy-1"].Attempts != 1 || state.Items["legacy-1"].Status != MigrationCompleted {
		t.Fatalf("state=%+v", state.Items["legacy-1"])
	}
}

func TestMigrateLegacySourceChangeCreatesRevisionButDoesNotOverrideUserDocument(t *testing.T) {
	store, root := newMigrationStore(t)
	writeLegacyTaskFile(t, root, "legacy-2")
	dir := writeLegacyMeeting(t, root, "legacy-2")
	if _, err := store.MigrateLegacy(); err != nil {
		t.Fatal(err)
	}
	active, err := store.GetActiveDocument("legacy-2", DocumentMeetingSummary)
	if err != nil {
		t.Fatal(err)
	}
	user, _, err := store.PutDocument("legacy-2", PutDocumentRequest{Kind: DocumentMeetingSummary, Source: DocumentSourceUser, Content: "# User summary\n", ExpectedRevision: active.Revision, IdempotencyKey: "user-summary", CreatedBy: "legacy-2"})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "meeting-summary.md"), []byte("# Changed legacy summary\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := store.MigrateLegacy()
	if err != nil || result.Migrated != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	current, err := store.GetActiveDocument("legacy-2", DocumentMeetingSummary)
	if err != nil || current.Revision != user.Revision || current.Content != "# User summary\n" || current.Source != DocumentSourceUser {
		t.Fatalf("user active overwritten: %+v err=%v", current, err)
	}
}

func TestMigrateLegacyFailureRecordedAndRecoversAfterSourceFix(t *testing.T) {
	store, root := newMigrationStore(t)
	writeLegacyTaskFile(t, root, "legacy-3")
	dir := writeLegacyMeeting(t, root, "legacy-3")
	if err := os.WriteFile(filepath.Join(dir, "polish_all.md"), []byte{0xff, 0xfe}, 0o600); err != nil {
		t.Fatal(err)
	}
	first, err := store.MigrateLegacy()
	if err != nil || first.Failed != 1 {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	failed := readMigrationStateForTest(t, root).Items["legacy-3"]
	if failed.Status != MigrationFailed || failed.LastError == "" || failed.Attempts != 1 {
		t.Fatalf("failed=%+v", failed)
	}
	if err := os.WriteFile(filepath.Join(dir, "polish_all.md"), []byte("# Fixed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, err := store.MigrateLegacy()
	if err != nil || second.Migrated != 1 {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	completed := readMigrationStateForTest(t, root).Items["legacy-3"]
	if completed.Status != MigrationCompleted || completed.Attempts != 2 || completed.LastError != "" {
		t.Fatalf("completed=%+v", completed)
	}
}

func TestMigrateLegacyImportsWhitelistedCompleteAudio(t *testing.T) {
	store, root := newMigrationStore(t)
	writeLegacyTaskFile(t, root, "legacy-audio")
	dir := writeLegacyMeeting(t, root, "legacy-audio")
	if err := os.WriteFile(filepath.Join(dir, "recording.webm"), append([]byte{0x1a, 0x45, 0xdf, 0xa3}, []byte("synthetic-webm-container")...), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := store.MigrateLegacy()
	if err != nil || result.Migrated != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	metadata, err := loadChunkMetadata(filepath.Join(root, "legacy-audio", "chunks", "000000.json"))
	if err != nil || metadata.Index != 0 || !metadata.Final || metadata.Format != "webm" {
		t.Fatalf("metadata=%+v err=%v", metadata, err)
	}
}

func TestMigrateLegacyRejectsSymlinkArtifactAndContinuesOtherTasks(t *testing.T) {
	store, root := newMigrationStore(t)
	writeLegacyTaskFile(t, root, "bad-link", "good-task")
	badDir := writeLegacyMeeting(t, root, "bad-link")
	writeLegacyMeeting(t, root, "good-task")
	if err := os.Remove(filepath.Join(badDir, "merged_all.txt")); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "outside.txt")
	if err := os.WriteFile(target, []byte("outside"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(badDir, "merged_all.txt")); err != nil {
		t.Fatal(err)
	}
	result, err := store.MigrateLegacy()
	if err != nil || result.Failed != 1 || result.Migrated != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if data, _ := os.ReadFile(target); string(data) != "outside" {
		t.Fatal("symlink target modified")
	}
}

func TestMigrateLegacyInvalidTaskConfigFailsOnlyThatItem(t *testing.T) {
	store, root := newMigrationStore(t)
	writeLegacyMeeting(t, root, "bad-config")
	writeLegacyMeeting(t, root, "good-config")
	tasks := map[string]any{"tasks": []any{
		map[string]any{"id": "bad-config", "config": "not-an-object"},
		map[string]any{"id": "good-config", "config": map[string]any{"OutputDir": filepath.Join(root, "good-config")}},
	}}
	data, _ := json.Marshal(tasks)
	if err := os.WriteFile(filepath.Join(root, "server_tasks.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := store.MigrateLegacy()
	if err != nil || result.Failed != 1 || result.Migrated != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestMigrateLegacyRejectsMalformedTasksAndState(t *testing.T) {
	t.Run("tasks", func(t *testing.T) {
		store, root := newMigrationStore(t)
		if err := os.WriteFile(filepath.Join(root, "server_tasks.json"), []byte("{"), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.MigrateLegacy(); err == nil || !strings.Contains(err.Error(), "read legacy tasks") {
			t.Fatalf("error=%v", err)
		}
	})
	t.Run("state", func(t *testing.T) {
		store, root := newMigrationStore(t)
		if err := os.WriteFile(filepath.Join(root, "migration-state.json"), []byte(`{"schema_version":99}`), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := store.MigrateLegacy(); err == nil || !strings.Contains(err.Error(), "invalid migration state") {
			t.Fatalf("error=%v", err)
		}
	})
}

func TestResolveLegacyTaskDirMapsHistoricalPathsAndRejectsMismatch(t *testing.T) {
	root := t.TempDir()
	for name, configured := range map[string]string{
		"legacy-relative": "tasks/legacy-relative",
		"legacy-data":     "data/meetings/legacy-data",
		"legacy-docker":   "/app/data/meetings/legacy-docker",
	} {
		t.Run(name, func(t *testing.T) {
			config, _ := json.Marshal(legacyTaskConfig{OutputDir: configured})
			resolved, err := resolveLegacyTaskDir(root, legacyTaskRecord{ID: name, Config: config})
			if err != nil || resolved != filepath.Join(root, name) {
				t.Fatalf("resolved=%q err=%v", resolved, err)
			}
		})
	}
	config, _ := json.Marshal(legacyTaskConfig{OutputDir: "tasks/different"})
	if _, err := resolveLegacyTaskDir(root, legacyTaskRecord{ID: "expected", Config: config}); err == nil {
		t.Fatal("mismatched output dir accepted")
	}
	config, _ = json.Marshal(legacyTaskConfig{OutputDir: "../expected"})
	if _, err := resolveLegacyTaskDir(root, legacyTaskRecord{ID: "expected", Config: config}); err == nil {
		t.Fatal("traversal output dir accepted")
	}
}

func TestMigrateLegacyReconcilesCompletedStateAfterTargetLoss(t *testing.T) {
	store, root := newMigrationStore(t)
	writeLegacyTaskFile(t, root, "legacy-reconcile")
	writeLegacyMeeting(t, root, "legacy-reconcile")
	if result, err := store.MigrateLegacy(); err != nil || result.Migrated != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if err := os.RemoveAll(filepath.Join(root, "legacy-reconcile", "documents")); err != nil {
		t.Fatal(err)
	}
	result, err := store.MigrateLegacy()
	if err != nil || result.Skipped != 1 || result.Failed != 0 {
		t.Fatalf("reconcile=%+v err=%v state=%+v", result, err, readMigrationStateForTest(t, root).Items["legacy-reconcile"])
	}
	if _, err := store.GetActiveDocument("legacy-reconcile", DocumentMeetingSummary); err != nil {
		t.Fatalf("completed state was trusted without reconciliation: %v", err)
	}
	state := readMigrationStateForTest(t, root)
	if state.Items["legacy-reconcile"].Attempts != 1 {
		t.Fatalf("reconciliation changed attempt count: %+v", state.Items["legacy-reconcile"])
	}
}

func TestMigrationIdentifiersSupportUnicodeAndRejectTraversal(t *testing.T) {
	target, stateKey, err := migrationIdentifiers("旧会议-一")
	if err != nil || !validMigrationID(target) || target != stateKey || !strings.HasPrefix(target, "legacy-") {
		t.Fatalf("target=%q state=%q err=%v", target, stateKey, err)
	}
	if _, _, err := migrationIdentifiers("../escape"); err == nil {
		t.Fatal("path traversal id accepted")
	}
}

func TestMigrateLegacyRejectsInvalidAudioSignature(t *testing.T) {
	store, root := newMigrationStore(t)
	writeLegacyTaskFile(t, root, "legacy-bad-audio")
	dir := writeLegacyMeeting(t, root, "legacy-bad-audio")
	if err := os.WriteFile(filepath.Join(dir, "recording.webm"), []byte("not-a-webm"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := store.MigrateLegacy()
	if err != nil || result.Failed != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestLoadMigrationStateRejectsInvalidItemStatusAndFingerprint(t *testing.T) {
	for name, item := range map[string]map[string]any{
		"status":      {"source_id": "legacy", "source_path": "legacy", "target_session_id": "legacy", "status": "unknown", "attempts": 1},
		"fingerprint": {"source_id": "legacy", "source_path": "legacy", "target_session_id": "legacy", "status": "completed", "source_sha256": "bad", "attempts": 1, "completed_at": time.Now()},
	} {
		t.Run(name, func(t *testing.T) {
			store, root := newMigrationStore(t)
			data, _ := json.Marshal(map[string]any{"schema_version": 1, "items": map[string]any{"legacy": item}})
			if err := os.WriteFile(filepath.Join(root, "migration-state.json"), data, 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := store.MigrateLegacy(); err == nil {
				t.Fatal("invalid migration state accepted")
			}
		})
	}
}

func TestMigrateLegacyUnicodeTaskIDUsesStableSessionMapping(t *testing.T) {
	store, root := newMigrationStore(t)
	sourceID := "旧会议-一"
	writeLegacyTaskFile(t, root, sourceID)
	writeLegacyMeeting(t, root, sourceID)
	result, err := store.MigrateLegacy()
	if err != nil || result.Migrated != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	targetID, _, _ := migrationIdentifiers(sourceID)
	session, err := store.LoadSession(targetID)
	if err != nil || session.Metadata["task_id"] != sourceID || session.Metadata["legacy_source_id"] != sourceID {
		t.Fatalf("session=%+v err=%v", session, err)
	}
	if second, err := store.MigrateLegacy(); err != nil || second.Skipped != 1 {
		t.Fatalf("second=%+v err=%v", second, err)
	}
}

func TestMigrateLegacyRejectsSymlinkMeetingDirectory(t *testing.T) {
	store, root := newMigrationStore(t)
	writeLegacyTaskFile(t, root, "linked-meeting")
	outside := t.TempDir()
	writeLegacyMeeting(t, outside, "linked-meeting")
	if err := os.Symlink(filepath.Join(outside, "linked-meeting"), filepath.Join(root, "linked-meeting")); err != nil {
		t.Fatal(err)
	}
	result, err := store.MigrateLegacy()
	if err != nil || result.Failed != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, err := os.Stat(filepath.Join(outside, "linked-meeting", "session.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("symlink target mutated: %v", err)
	}
}

func TestAcquireServiceLockRejectsSecondProcessAndReleases(t *testing.T) {
	root := t.TempDir()
	first, err := AcquireServiceLock(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AcquireServiceLock(root); !errors.Is(err, ErrMeetingStoreInUse) {
		t.Fatalf("second lock error=%v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := AcquireServiceLock(root)
	if err != nil {
		t.Fatalf("lock not released: %v", err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateLegacyExistingUnrelatedSessionConflictsWithoutMutation(t *testing.T) {
	store, root := newMigrationStore(t)
	if _, err := store.CreateOwnedSessionWithMode("conflict", "owner", "", map[string]string{"task_id": "other"}, TranscriptionManual); err != nil {
		t.Fatal(err)
	}
	writeLegacyTaskFile(t, root, "conflict")
	writeLegacyMeeting(t, root, "conflict")
	result, err := store.MigrateLegacy()
	if err != nil || result.Failed != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if _, err := store.GetActiveDocument("conflict", DocumentMeetingSummary); !errors.Is(err, ErrDocumentNotFound) {
		t.Fatalf("conflicting session mutated: %v", err)
	}
}

func readMigrationStateForTest(t *testing.T, root string) MigrationState {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "migration-state.json"))
	if err != nil {
		t.Fatal(err)
	}
	var state MigrationState
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatal(err)
	}
	if state.SchemaVersion != migrationSchemaVersion || state.UpdatedAt.Before(time.Unix(0, 0)) {
		t.Fatalf("state=%+v", state)
	}
	return state
}
