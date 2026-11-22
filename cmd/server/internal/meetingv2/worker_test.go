package meetingv2

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeProvider struct {
	mu       sync.Mutex
	results  []TranscribeResult
	errors   []error
	calls    atomic.Int32
	block    chan struct{}
	observed atomic.Int32
}

func (p *fakeProvider) Transcribe(ctx context.Context, _ TranscribeRequest) (TranscribeResult, error) {
	call := int(p.calls.Add(1)) - 1
	current := p.observed.Add(1)
	defer p.observed.Add(-1)
	_ = current
	if p.block != nil {
		select {
		case <-ctx.Done():
			return TranscribeResult{}, ctx.Err()
		case <-p.block:
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if call < len(p.errors) && p.errors[call] != nil {
		return TranscribeResult{}, p.errors[call]
	}
	if call < len(p.results) {
		return p.results[call], nil
	}
	return TranscribeResult{Text: "transcript"}, nil
}

func (p *fakeProvider) Check(context.Context) error { return nil }

func setupWorkerJob(t *testing.T, store *Store, id string, index int) {
	t.Helper()
	if _, err := store.CreateSession(id, "", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SetSessionState(id, StateRecording); err != nil {
		t.Fatal(err)
	}
	if _, err := store.IngestChunk(id, IngestChunkRequest{Index: index, Format: "webm"}, strings.NewReader("audio")); err != nil {
		t.Fatal(err)
	}
}

func TestWorkerCompletesJobAndTranscript(t *testing.T) {
	store := newTestStore(t)
	setupWorkerJob(t, store, "worker-success", 0)
	provider := &fakeProvider{results: []TranscribeResult{{Text: "hello", Language: "en", DurationMS: 3000}}}
	worker, err := NewWorker(store, provider, WorkerOptions{Concurrency: 1, MaxAttempts: 3, RetryInitialDelay: time.Millisecond, RetryMaxDelay: time.Second, ProviderName: "test", Model: "whisper"})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := worker.Start(ctx); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		status, _ := store.GetRecoveryStatus("worker-success", 100)
		return status.CompletedJobs == 1
	})
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	if err := worker.Shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
	// Projections are finalized when the recording is stopped; per-chunk JSON is
	// already durable before then.
	if _, err := store.SetSessionState("worker-success", StateStopping); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(store.root, "worker-success", "transcripts", "transcript.txt"))
	if err != nil || string(data) != "[000000] hello\n" {
		t.Fatalf("unexpected transcript %q err=%v", data, err)
	}
}

func TestWorkerRetriesThenCompletes(t *testing.T) {
	store := newTestStore(t)
	setupWorkerJob(t, store, "worker-retry", 0)
	provider := &fakeProvider{errors: []error{ErrProviderRetryable}, results: []TranscribeResult{{}, {Text: "ok"}}}
	worker, err := NewWorker(store, provider, WorkerOptions{Concurrency: 1, MaxAttempts: 3, RetryInitialDelay: time.Millisecond, RetryMaxDelay: time.Millisecond, ProviderName: "test", Model: "whisper"})
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		status, _ := store.GetRecoveryStatus("worker-retry", 100)
		return status.CompletedJobs == 1
	})
	_ = worker.Shutdown(context.Background())
	if provider.calls.Load() != 2 {
		t.Fatalf("calls = %d, want 2", provider.calls.Load())
	}
}

func TestWorkerDeadLettersPermanentFailure(t *testing.T) {
	store := newTestStore(t)
	setupWorkerJob(t, store, "worker-dead", 0)
	provider := &fakeProvider{errors: []error{ErrProviderPermanent}}
	worker, _ := NewWorker(store, provider, WorkerOptions{Concurrency: 1, MaxAttempts: 3, RetryInitialDelay: time.Millisecond, RetryMaxDelay: time.Second, ProviderName: "test", Model: "whisper"})
	if err := worker.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		status, _ := store.GetRecoveryStatus("worker-dead", 100)
		return status.DeadJobs == 1
	})
	_ = worker.Shutdown(context.Background())
}

func TestWorkerShutdownCancelsProvider(t *testing.T) {
	store := newTestStore(t)
	setupWorkerJob(t, store, "worker-cancel", 0)
	provider := &fakeProvider{block: make(chan struct{})}
	worker, _ := NewWorker(store, provider, WorkerOptions{Concurrency: 1, MaxAttempts: 3, RetryInitialDelay: time.Millisecond, RetryMaxDelay: time.Second, ProviderName: "test", Model: "whisper"})
	if err := worker.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool { return provider.calls.Load() == 1 })
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := worker.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	status, err := store.GetRecoveryStatus("worker-cancel", 100)
	if err != nil || status.RetryWaitJobs != 1 {
		t.Fatalf("interrupted job was not retained: %#v err=%v", status, err)
	}
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not reached")
}

func TestOpenClaimedAudioRejectsTampering(t *testing.T) {
	store := newTestStore(t)
	setupWorkerJob(t, store, "worker-tamper", 0)
	claim, err := store.ClaimNextJob(time.Now())
	if err != nil || claim == nil {
		t.Fatalf("claim=%#v err=%v", claim, err)
	}
	if err := os.WriteFile(claim.AudioPath, []byte("tampered"), 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := store.OpenClaimedAudio(*claim); !errors.Is(err, ErrChunkConflict) {
		t.Fatalf("expected tampered audio rejection, got %v", err)
	}
}

func TestCancelledSessionDoesNotClaimPendingJob(t *testing.T) {
	store := newTestStore(t)
	setupWorkerJob(t, store, "worker-cancelled-session", 0)
	if _, err := store.SetSessionState("worker-cancelled-session", StateCancelled); err != nil {
		t.Fatal(err)
	}
	claim, err := store.ClaimNextJob(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if claim != nil {
		t.Fatalf("cancelled session claimed job: %#v", claim)
	}
}
