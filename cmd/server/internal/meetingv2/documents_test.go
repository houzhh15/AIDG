package meetingv2

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDocumentUserRevisionProtectsActiveArtifact(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreateOwnedSessionWithMode("docs", "Docs", "alice", map[string]string{"task_id": "task-1"}, TranscriptionManual); err != nil {
		t.Fatal(err)
	}
	generated, duplicate, err := store.PutDocument("docs", PutDocumentRequest{Kind: DocumentMeetingSummary, Source: DocumentSourceGenerated, Content: "generated-1", ExpectedRevision: 0, IdempotencyKey: "gen-1", CreatedBy: "generator"})
	if err != nil || duplicate || !generated.Active {
		t.Fatalf("generated=%+v duplicate=%v err=%v", generated, duplicate, err)
	}
	user, duplicate, err := store.PutDocument("docs", PutDocumentRequest{Kind: DocumentMeetingSummary, Source: DocumentSourceUser, Content: "user-summary", ExpectedRevision: 1, IdempotencyKey: "user-1", CreatedBy: "alice"})
	if err != nil || duplicate || !user.Active {
		t.Fatalf("user=%+v duplicate=%v err=%v", user, duplicate, err)
	}
	auto, duplicate, err := store.PutDocument("docs", PutDocumentRequest{Kind: DocumentMeetingSummary, Source: DocumentSourceGenerated, Content: "generated-2", ExpectedRevision: 2, IdempotencyKey: "gen-2", CreatedBy: "generator"})
	if err != nil || duplicate || auto.Active {
		t.Fatalf("auto=%+v duplicate=%v err=%v", auto, duplicate, err)
	}
	user2, duplicate, err := store.PutDocument("docs", PutDocumentRequest{Kind: DocumentMeetingSummary, Source: DocumentSourceUser, Content: "user-summary-2", ExpectedRevision: 2, IdempotencyKey: "user-2", CreatedBy: "alice"})
	if err != nil || duplicate || !user2.Active || user2.Revision != 4 {
		t.Fatalf("user2=%+v duplicate=%v err=%v", user2, duplicate, err)
	}
	active, err := store.GetActiveDocument("docs", DocumentMeetingSummary)
	if err != nil || active.Content != "user-summary-2" || active.Revision != 4 {
		t.Fatalf("active=%+v err=%v", active, err)
	}
	artifact, err := os.ReadFile(filepath.Join(root, "docs", "meeting-summary.md"))
	if err != nil || string(artifact) != "user-summary-2" {
		t.Fatalf("artifact=%q err=%v", artifact, err)
	}
}

func TestDocumentIdempotencyAndRevisionConflict(t *testing.T) {
	store, err := NewStore(t.TempDir(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreateOwnedSessionWithMode("docs-idempotent", "Docs", "alice", nil, TranscriptionManual); err != nil {
		t.Fatal(err)
	}
	request := PutDocumentRequest{Kind: DocumentPolishedTranscript, Source: DocumentSourceExternal, Content: "polished", ExpectedRevision: 0, IdempotencyKey: "external-1", CreatedBy: "alice"}
	first, duplicate, err := store.PutDocument("docs-idempotent", request)
	if err != nil || duplicate || first.Revision != 1 {
		t.Fatalf("first=%+v duplicate=%v err=%v", first, duplicate, err)
	}
	replay, duplicate, err := store.PutDocument("docs-idempotent", request)
	if err != nil || !duplicate || replay.Revision != 1 || !replay.Active {
		t.Fatalf("replay=%+v duplicate=%v err=%v", replay, duplicate, err)
	}
	changed := request
	changed.Content = "different"
	if _, _, err = store.PutDocument("docs-idempotent", changed); !errors.Is(err, ErrDocumentConflict) {
		t.Fatalf("same key conflict=%v", err)
	}
	if _, _, err = store.PutDocument("docs-idempotent", PutDocumentRequest{Kind: DocumentPolishedTranscript, Source: DocumentSourceUser, Content: "new", ExpectedRevision: 0, IdempotencyKey: "user-2", CreatedBy: "alice"}); !errors.Is(err, ErrDocumentRevisionConflict) {
		t.Fatalf("revision conflict=%v", err)
	}
}

func TestFindSessionByTaskIDRejectsAmbiguity(t *testing.T) {
	store, err := NewStore(t.TempDir(), 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"session-a", "session-b"} {
		if _, err = store.CreateOwnedSessionWithMode(id, id, "alice", map[string]string{"task_id": "task-1"}, TranscriptionManual); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = store.FindSessionByTaskID("task-1"); !errors.Is(err, ErrAmbiguousTaskSession) {
		t.Fatalf("ambiguity error=%v", err)
	}
}

func TestLegacyArtifactRejectsSymlink(t *testing.T) {
	root := t.TempDir()
	store, err := NewStore(root, 1<<20)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.CreateOwnedSessionWithMode("docs-link", "Docs", "alice", nil, TranscriptionManual); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(root, "outside")
	if err = os.WriteFile(target, []byte("safe"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(target, filepath.Join(root, "docs-link", "meeting-summary.md")); err != nil {
		t.Fatal(err)
	}
	if _, _, err = store.PutDocument("docs-link", PutDocumentRequest{Kind: DocumentMeetingSummary, Source: DocumentSourceUser, Content: "attack", ExpectedRevision: 0, IdempotencyKey: "key-1", CreatedBy: "alice"}); err == nil {
		t.Fatal("expected symlink projection to fail")
	}
	if _, err = store.GetActiveDocument("docs-link", DocumentMeetingSummary); !errors.Is(err, ErrDocumentNotFound) {
		t.Fatalf("failed projection changed active document: %v", err)
	}
	content, err := os.ReadFile(target)
	if err != nil || string(content) != "safe" {
		t.Fatalf("outside content=%q err=%v", content, err)
	}
}
