package meetingv2

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
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(t.TempDir(), 1024)
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestRecoveryStatusEncodesEmptyMissingChunksAsArray(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.CreateSession("recovery-empty-missing", "", nil); err != nil {
		t.Fatal(err)
	}
	status, err := store.GetRecoveryStatus("recovery-empty-missing", 100)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(status)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"missing_chunks":[]`)) {
		t.Fatalf("missing_chunks must encode as an array, got %s", encoded)
	}
}

func TestStoreRejectsUnsafeSessionIDs(t *testing.T) {
	store := newTestStore(t)
	for _, id := range []string{"", ".", "..", "../escape", "meeting/name", "/absolute", " hidden"} {
		if _, err := store.CreateSession(id, "", nil); !errors.Is(err, ErrInvalidSessionID) {
			t.Errorf("CreateSession(%q) error = %v, want ErrInvalidSessionID", id, err)
		}
	}
}

func TestStoreRejectsSymlinkSessionDirectory(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, "meeting-link")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	store, err := NewStore(root, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateSession("meeting-link", "", nil); !errors.Is(err, ErrInvalidSessionID) {
		t.Fatalf("expected symlink session directory rejection, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "session.json")); !os.IsNotExist(err) {
		t.Fatalf("session escaped meetings root: %v", err)
	}
}

func TestStoreRejectsSymlinkChunkDirectory(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.CreateSession("meeting-chunk-link", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetSessionState("meeting-chunk-link", StateRecording); err != nil {
		t.Fatal(err)
	}
	chunks := filepath.Join(store.root, "meeting-chunk-link", "chunks")
	if err := os.Remove(chunks); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	if err := os.Symlink(outside, chunks); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	_, err := store.IngestChunk("meeting-chunk-link", IngestChunkRequest{Index: 0, Format: "webm"}, bytes.NewReader([]byte("audio")))
	if err == nil {
		t.Fatal("expected symlink chunks directory rejection")
	}
	if entries, readErr := os.ReadDir(outside); readErr != nil || len(entries) != 0 {
		t.Fatalf("audio escaped meeting directory: entries=%v err=%v", entries, readErr)
	}
}

func TestSessionStateAndLegacyDirectoryCoexistence(t *testing.T) {
	root := t.TempDir()
	legacyDir := filepath.Join(root, "legacy-meeting")
	if err := os.MkdirAll(legacyDir, 0o750); err != nil {
		t.Fatal(err)
	}
	legacyPath := filepath.Join(legacyDir, "merged_all.txt")
	if err := os.WriteFile(legacyPath, []byte("legacy transcript"), 0o640); err != nil {
		t.Fatal(err)
	}
	store, err := NewStore(root, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateSession("legacy-meeting", "Imported", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetSessionState("legacy-meeting", StateRecording); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(legacyPath); err != nil || string(data) != "legacy transcript" {
		t.Fatalf("legacy artifact changed: data=%q err=%v", data, err)
	}
}

func TestIngestChunkIsIdempotentAndCreatesPendingJob(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.CreateSession("meeting-1", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetSessionState("meeting-1", StateRecording); err != nil {
		t.Fatal(err)
	}
	audio := []byte("valid audio fixture")
	hash := sha256.Sum256(audio)
	declared := hex.EncodeToString(hash[:])
	request := IngestChunkRequest{Index: 0, Format: "webm", DeclaredSHA256: declared}

	first, err := store.IngestChunk("meeting-1", request, bytes.NewReader(audio))
	if err != nil {
		t.Fatal(err)
	}
	if first.Duplicate || first.Chunk.SHA256 != declared {
		t.Fatalf("unexpected first result: %#v", first)
	}
	second, err := store.IngestChunk("meeting-1", request, bytes.NewReader(audio))
	if err != nil {
		t.Fatal(err)
	}
	if !second.Duplicate {
		t.Fatal("expected duplicate acknowledgement")
	}
	_, err = store.IngestChunk("meeting-1", IngestChunkRequest{Index: 0, Format: "webm"}, bytes.NewReader([]byte("different audio")))
	if !errors.Is(err, ErrChunkConflict) {
		t.Fatalf("expected conflict when duplicate has no declared hash and different content, got %v", err)
	}

	jobPath := filepath.Join(store.root, "meeting-1", "jobs", "000000.json")
	job, err := loadJob(jobPath)
	if err != nil {
		t.Fatal(err)
	}
	if job.State != JobPending || job.ChunkIndex != 0 {
		t.Fatalf("unexpected job: %#v", job)
	}
	session, err := store.LoadSession("meeting-1")
	if err != nil {
		t.Fatal(err)
	}
	if session.LastChunk != 0 {
		t.Fatalf("last chunk = %d, want 0", session.LastChunk)
	}
}

func TestDuplicateIngestRejectsMetadataChanges(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.CreateSession("meeting-metadata", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetSessionState("meeting-metadata", StateRecording); err != nil {
		t.Fatal(err)
	}
	audio := []byte("audio")
	base := IngestChunkRequest{Index: 0, Format: "webm", ClientChunkID: "chunk-a", DurationMS: 5000}
	if _, err := store.IngestChunk("meeting-metadata", base, bytes.NewReader(audio)); err != nil {
		t.Fatal(err)
	}
	changed := base
	changed.Final = true
	if _, err := store.IngestChunk("meeting-metadata", changed, bytes.NewReader(audio)); !errors.Is(err, ErrChunkConflict) {
		t.Fatalf("expected metadata conflict, got %v", err)
	}
}

func TestDuplicateIngestReconcilesMissingJobAndLastChunk(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.CreateSession("meeting-reconcile", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetSessionState("meeting-reconcile", StateRecording); err != nil {
		t.Fatal(err)
	}
	audio := []byte("audio")
	request := IngestChunkRequest{Index: 3, Format: "webm"}
	if _, err := store.IngestChunk("meeting-reconcile", request, bytes.NewReader(audio)); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(store.root, "meeting-reconcile")
	if err := os.Remove(filepath.Join(dir, "jobs", "000003.json")); err != nil {
		t.Fatal(err)
	}
	session, err := store.LoadSession("meeting-reconcile")
	if err != nil {
		t.Fatal(err)
	}
	session.LastChunk = -1
	if err := writeJSONAtomic(filepath.Join(dir, "session.json"), session, 0o640); err != nil {
		t.Fatal(err)
	}
	result, err := store.IngestChunk("meeting-reconcile", request, bytes.NewReader(audio))
	if err != nil || !result.Duplicate {
		t.Fatalf("reconcile ingest result=%#v err=%v", result, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "jobs", "000003.json")); err != nil {
		t.Fatalf("job was not recreated: %v", err)
	}
	session, err = store.LoadSession("meeting-reconcile")
	if err != nil || session.LastChunk != 3 {
		t.Fatalf("session was not reconciled: %#v err=%v", session, err)
	}
}

func TestDuplicateIngestRejectsMissingAudio(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.CreateSession("meeting-missing", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetSessionState("meeting-missing", StateRecording); err != nil {
		t.Fatal(err)
	}
	audio := []byte("audio")
	request := IngestChunkRequest{Index: 0, Format: "webm"}
	if _, err := store.IngestChunk("meeting-missing", request, bytes.NewReader(audio)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(store.root, "meeting-missing", "chunks", "000000.webm")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.IngestChunk("meeting-missing", request, bytes.NewReader(audio)); err == nil {
		t.Fatal("expected missing persisted audio to reject duplicate acknowledgement")
	}
}

func TestIngestChunkRejectsConflictAndOversize(t *testing.T) {
	store := newTestStore(t)
	store.maxChunkSize = 4
	if _, err := store.CreateSession("meeting-2", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetSessionState("meeting-2", StateRecording); err != nil {
		t.Fatal(err)
	}
	if _, err := store.IngestChunk("meeting-2", IngestChunkRequest{Index: 0, Format: "webm"}, bytes.NewReader([]byte("1234"))); err != nil {
		t.Fatal(err)
	}
	wrong := sha256.Sum256([]byte("different"))
	_, err := store.IngestChunk("meeting-2", IngestChunkRequest{Index: 0, Format: "webm", DeclaredSHA256: hex.EncodeToString(wrong[:])}, bytes.NewReader([]byte("1234")))
	if !errors.Is(err, ErrChunkConflict) {
		t.Fatalf("expected chunk conflict, got %v", err)
	}
	_, err = store.IngestChunk("meeting-2", IngestChunkRequest{Index: 1, Format: "webm"}, bytes.NewReader([]byte("12345")))
	if !errors.Is(err, ErrInvalidChunk) {
		t.Fatalf("expected invalid chunk, got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(store.root, "meeting-2", "chunks", "000001.webm")); !os.IsNotExist(statErr) {
		t.Fatalf("oversized chunk was persisted: %v", statErr)
	}
}

func TestRecoveryRecreatesMissingJobAndLastChunk(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.CreateSession("meeting-recovery", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetSessionState("meeting-recovery", StateRecording); err != nil {
		t.Fatal(err)
	}
	request := IngestChunkRequest{Index: 4, Format: "webm"}
	if _, err := store.IngestChunk("meeting-recovery", request, bytes.NewReader([]byte("audio"))); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(store.root, "meeting-recovery")
	if err := os.Remove(filepath.Join(dir, "jobs", "000004.json")); err != nil {
		t.Fatal(err)
	}
	session, err := store.LoadSession("meeting-recovery")
	if err != nil {
		t.Fatal(err)
	}
	session.LastChunk = -1
	if err := writeJSONAtomic(filepath.Join(dir, "session.json"), session, 0o640); err != nil {
		t.Fatal(err)
	}
	recovered, err := store.RecoverInterruptedJobs()
	if err != nil {
		t.Fatal(err)
	}
	if recovered != 1 {
		t.Fatalf("recovered = %d, want 1 reconciliation", recovered)
	}
	if _, err := os.Stat(filepath.Join(dir, "jobs", "000004.json")); err != nil {
		t.Fatal(err)
	}
	session, err = store.LoadSession("meeting-recovery")
	if err != nil || session.LastChunk != 4 {
		t.Fatalf("session was not repaired: %#v err=%v", session, err)
	}
}

func TestRecoveryRejectsCorruptChunkMetadata(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.CreateSession("meeting-corrupt-meta", "", nil); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.root, "meeting-corrupt-meta", "chunks", "000000.json")
	metadata := ChunkMetadata{SchemaVersion: SchemaVersion, SessionID: "other", Index: 0, Format: "webm", Size: 1, SHA256: strings.Repeat("a", 64)}
	if err := writeJSONAtomic(path, metadata, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecoverInterruptedJobs(); err == nil {
		t.Fatal("expected corrupt metadata recovery error")
	}
}

func TestRecoverInterruptedJobs(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.CreateSession("meeting-3", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetSessionState("meeting-3", StateRecording); err != nil {
		t.Fatal(err)
	}
	if _, err := store.IngestChunk("meeting-3", IngestChunkRequest{Index: 7, Format: "webm"}, bytes.NewReader([]byte("audio"))); err != nil {
		t.Fatal(err)
	}
	jobPath := filepath.Join(store.root, "meeting-3", "jobs", "000007.json")
	job, err := loadJob(jobPath)
	if err != nil {
		t.Fatal(err)
	}
	job.State = JobProcessing
	if err := writeJSONAtomic(jobPath, job, 0o640); err != nil {
		t.Fatal(err)
	}

	recovered, err := store.RecoverInterruptedJobs()
	if err != nil {
		t.Fatal(err)
	}
	if recovered != 1 {
		t.Fatalf("recovered = %d, want 1", recovered)
	}
	updated, err := loadJob(jobPath)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != JobPending || updated.LastError == "" {
		t.Fatalf("unexpected recovered job: %#v", updated)
	}
}

func TestRecoveryRejectsOrphanJob(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.CreateSession("meeting-orphan-job", "", nil); err != nil {
		t.Fatal(err)
	}
	jobPath := filepath.Join(store.root, "meeting-orphan-job", "jobs", "000001.json")
	job := ASRJob{SchemaVersion: SchemaVersion, ID: "meeting-orphan-job:1", SessionID: "meeting-orphan-job", ChunkIndex: 1, State: JobProcessing}
	if err := writeJSONAtomic(jobPath, job, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecoverInterruptedJobs(); err == nil {
		t.Fatal("expected orphan job recovery error")
	}
}

func TestCorruptSessionIsNotOverwritten(t *testing.T) {
	store := newTestStore(t)
	dir := filepath.Join(store.root, "meeting-4")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "session.json")
	if err := os.WriteFile(path, []byte("{not-json"), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateSession("meeting-4", "", nil); err == nil {
		t.Fatal("expected existing corrupt session to block creation")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	if json.Unmarshal(data, &decoded) == nil {
		t.Fatal("corrupt session was overwritten")
	}
}

func TestGetTranscriptPageIsOrderedAndBounded(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.CreateSession("meeting-page", "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetSessionState("meeting-page", StateRecording); err != nil {
		t.Fatal(err)
	}
	for _, index := range []int{2, 0, 1} {
		if _, err := store.IngestChunk("meeting-page", IngestChunkRequest{Index: index, Format: "webm"}, bytes.NewReader([]byte(fmt.Sprintf("audio-%d", index)))); err != nil {
			t.Fatal(err)
		}
		claim, err := store.ClaimNextJob(time.Now())
		if err != nil || claim == nil {
			t.Fatalf("claim=%#v err=%v", claim, err)
		}
		if err := store.CompleteJob(*claim, TranscriptChunk{Text: fmt.Sprintf("text-%d", claim.Job.ChunkIndex), Provider: "test", Model: "test"}); err != nil {
			t.Fatal(err)
		}
	}
	page, err := store.GetTranscriptPage("meeting-page", 0, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Chunks) != 1 || page.Chunks[0].ChunkIndex != 1 || !page.HasMore || page.NextAfter != 1 {
		t.Fatalf("unexpected page: %#v", page)
	}
	if _, err := store.GetTranscriptPage("meeting-page", -2, 1); err == nil {
		t.Fatal("expected invalid pagination error")
	}
}

func TestManualSessionCompletesWithoutASRJobs(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	session, err := store.CreateOwnedSessionWithMode("manual-session", "Manual", "alice", nil, TranscriptionManual)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.SetSessionState(session.ID, StateRecording); err != nil {
		t.Fatal(err)
	}
	audio := []byte("standalone-audio")
	hash := sha256.Sum256(audio)
	if _, err = store.IngestChunk(session.ID, IngestChunkRequest{Index: 0, ClientChunkID: "chunk-0", StartedAt: time.Now().UTC(), DurationMS: 1000, Format: "webm", DeclaredSHA256: hex.EncodeToString(hash[:]), Final: true}, bytes.NewReader(audio)); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(root, session.ID, "jobs"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("manual session created ASR jobs: %d", len(entries))
	}
	completed, err := store.SetSessionState(session.ID, StateStopping)
	if err != nil {
		t.Fatal(err)
	}
	if completed.State != StateCompleted {
		t.Fatalf("state = %s, want %s", completed.State, StateCompleted)
	}
}

func TestManualTranscriptRevisionAndIdempotency(t *testing.T) {
	store, err := NewStore(t.TempDir(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreateOwnedSessionWithMode("manual-transcript", "Manual", "alice", nil, TranscriptionManual); err != nil {
		t.Fatal(err)
	}
	request := SubmitManualTranscriptRequest{Text: "人工文字记录", Language: "zh", ExpectedRevision: 0, IdempotencyKey: "request-1", SubmittedBy: "alice"}
	first, duplicate, err := store.SubmitManualTranscript("manual-transcript", request)
	if err != nil || duplicate || first.Revision != 1 {
		t.Fatalf("first submit = %+v, duplicate=%v, err=%v", first, duplicate, err)
	}
	replayed, duplicate, err := store.SubmitManualTranscript("manual-transcript", request)
	if err != nil || !duplicate || replayed.Revision != 1 {
		t.Fatalf("replay = %+v, duplicate=%v, err=%v", replayed, duplicate, err)
	}
	conflict := request
	conflict.Text = "different"
	if _, _, err = store.SubmitManualTranscript("manual-transcript", conflict); !errors.Is(err, ErrTranscriptConflict) {
		t.Fatalf("same key different content error = %v", err)
	}
	second, duplicate, err := store.SubmitManualTranscript("manual-transcript", SubmitManualTranscriptRequest{Text: "修订文字", Language: "zh", ExpectedRevision: 1, IdempotencyKey: "request-2", SubmittedBy: "alice"})
	if err != nil || duplicate || second.Revision != 2 {
		t.Fatalf("second submit = %+v, duplicate=%v, err=%v", second, duplicate, err)
	}
	latest, err := store.GetLatestManualTranscript("manual-transcript")
	if err != nil || latest.Revision != 2 || latest.Text != "修订文字" {
		t.Fatalf("latest = %+v, err=%v", latest, err)
	}
}

func TestImportAudioManualSession(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreateOwnedSessionWithMode("import-session", "Import", "alice", nil, TranscriptionManual); err != nil {
		t.Fatal(err)
	}
	audio := []byte("complete-audio-container")
	hash := sha256.Sum256(audio)
	request := IngestChunkRequest{Index: 0, ClientChunkID: "import-1", StartedAt: time.Now().UTC(), DurationMS: 5000, Format: "mp3", DeclaredSHA256: hex.EncodeToString(hash[:]), Final: true}
	result, session, err := store.ImportAudio("import-session", request, bytes.NewReader(audio))
	if err != nil {
		t.Fatal(err)
	}
	if result.Duplicate || session.State != StateCompleted || session.LastChunk != 0 {
		t.Fatalf("result=%+v session=%+v", result, session)
	}
	replayed, session, err := store.ImportAudio("import-session", request, bytes.NewReader(audio))
	if err != nil || !replayed.Duplicate || session.State != StateCompleted {
		t.Fatalf("replay=%+v session=%+v err=%v", replayed, session, err)
	}
}

func TestImportAudioValidationDoesNotChangeCreatedSession(t *testing.T) {
	store, err := NewStore(t.TempDir(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreateOwnedSessionWithMode("invalid-import", "Import", "alice", nil, TranscriptionManual); err != nil {
		t.Fatal(err)
	}
	request := IngestChunkRequest{Index: 0, ClientChunkID: "import-1", DurationMS: 1000, Format: "mp3", DeclaredSHA256: strings.Repeat("0", 64), Final: true}
	if _, _, err = store.ImportAudio("invalid-import", request, bytes.NewReader(nil)); err == nil {
		t.Fatal("expected empty import to fail")
	}
	session, err := store.LoadSession("invalid-import")
	if err != nil {
		t.Fatal(err)
	}
	if session.State != StateCreated {
		t.Fatalf("state=%s, want created", session.State)
	}
}

func TestRecoveryCompletesInterruptedManualImport(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreateOwnedSessionWithMode("recover-import", "Import", "alice", nil, TranscriptionManual); err != nil {
		t.Fatal(err)
	}
	audio := []byte("audio")
	hash := sha256.Sum256(audio)
	request := IngestChunkRequest{Index: 0, ClientChunkID: "import-1", DurationMS: 1000, Format: "mp3", DeclaredSHA256: hex.EncodeToString(hash[:]), Final: true}
	if _, err = store.ingestChunk("recover-import", request, bytes.NewReader(audio), true); err != nil {
		t.Fatal(err)
	}
	if _, err = store.RecoverInterruptedJobs(); err != nil {
		t.Fatal(err)
	}
	session, err := store.LoadSession("recover-import")
	if err != nil {
		t.Fatal(err)
	}
	if session.State != StateCompleted {
		t.Fatalf("state=%s, want completed", session.State)
	}
}

func TestManualTranscriptIgnoresAtomicTempFile(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreateOwnedSessionWithMode("temp-transcript", "Manual", "alice", nil, TranscriptionManual); err != nil {
		t.Fatal(err)
	}
	manualDir := filepath.Join(root, "temp-transcript", "transcripts", "manual")
	if err = os.MkdirAll(manualDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(manualDir, ".tmp-crash"), []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	transcript, _, err := store.SubmitManualTranscript("temp-transcript", SubmitManualTranscriptRequest{Text: "text", ExpectedRevision: 0, IdempotencyKey: "key-1", SubmittedBy: "alice"})
	if err != nil || transcript.Revision != 1 {
		t.Fatalf("transcript=%+v err=%v", transcript, err)
	}
}

func TestImportAudioRejectsUnsafeImportID(t *testing.T) {
	store, err := NewStore(t.TempDir(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreateOwnedSessionWithMode("unsafe-import", "Import", "alice", nil, TranscriptionManual); err != nil {
		t.Fatal(err)
	}
	audio := []byte("audio")
	hash := sha256.Sum256(audio)
	request := IngestChunkRequest{Index: 0, ClientChunkID: strings.Repeat("x", 257), DurationMS: 1000, Format: "mp3", DeclaredSHA256: hex.EncodeToString(hash[:]), Final: true}
	if _, _, err = store.ImportAudio("unsafe-import", request, bytes.NewReader(audio)); !errors.Is(err, ErrInvalidChunk) {
		t.Fatalf("error=%v", err)
	}
}

func TestDuplicateIngestReportsPersistedJobState(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.CreateSession("job-state", "Job", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetSessionState("job-state", StateRecording); err != nil {
		t.Fatal(err)
	}
	audio := []byte("audio")
	request := IngestChunkRequest{Index: 0, Format: "webm", ClientChunkID: "chunk-0", DurationMS: 1000}
	first, err := store.IngestChunk("job-state", request, bytes.NewReader(audio))
	if err != nil || !first.JobCreated || first.JobState == nil || *first.JobState != JobPending {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	claimed, err := store.ClaimNextJob(time.Now())
	if err != nil || claimed == nil {
		t.Fatalf("claimed=%+v err=%v", claimed, err)
	}
	duplicate, err := store.IngestChunk("job-state", request, bytes.NewReader(audio))
	if err != nil {
		t.Fatal(err)
	}
	if !duplicate.Duplicate || duplicate.JobCreated || duplicate.JobState == nil || *duplicate.JobState != JobProcessing {
		t.Fatalf("duplicate=%+v", duplicate)
	}
}
