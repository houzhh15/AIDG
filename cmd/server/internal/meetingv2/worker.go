package meetingv2

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type WorkerOptions struct {
	Concurrency       int
	MaxAttempts       int
	RetryInitialDelay time.Duration
	RetryMaxDelay     time.Duration
	ProviderName      string
	Model             string
	Language          string
}

type Worker struct {
	store      *Store
	provider   ASRProvider
	options    WorkerOptions
	wake       chan struct{}
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	mu         sync.Mutex
	activeMu   sync.Mutex
	activeSeq  uint64
	activeJobs map[string]map[uint64]context.CancelFunc
	errMu      sync.Mutex
	lastErr    error
}

func NewWorker(store *Store, provider ASRProvider, options WorkerOptions) (*Worker, error) {
	if store == nil || provider == nil {
		return nil, fmt.Errorf("meeting store and ASR provider are required")
	}
	if options.Concurrency < 1 || options.Concurrency > 16 || options.MaxAttempts < 1 {
		return nil, fmt.Errorf("invalid worker options")
	}
	if options.RetryInitialDelay <= 0 || options.RetryMaxDelay < options.RetryInitialDelay {
		return nil, fmt.Errorf("invalid worker retry delays")
	}
	return &Worker{store: store, provider: provider, options: options, wake: make(chan struct{}, 1), activeJobs: make(map[string]map[uint64]context.CancelFunc)}, nil
}

func (w *Worker) Start(parent context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.cancel != nil {
		return fmt.Errorf("worker already started")
	}
	if _, err := w.store.RecoverInterruptedJobs(); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(parent)
	w.cancel = cancel
	for index := 0; index < w.options.Concurrency; index++ {
		w.wg.Add(1)
		go w.run(ctx)
	}
	w.Wake()
	return nil
}

func (w *Worker) Wake() {
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

func (w *Worker) Shutdown(ctx context.Context) error {
	w.mu.Lock()
	cancel := w.cancel
	w.cancel = nil
	w.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *Worker) run(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		for {
			if ctx.Err() != nil {
				return
			}
			claim, err := w.store.ClaimNextJob(time.Now())
			if err != nil {
				w.recordError(err)
				break
			}
			if claim == nil {
				break
			}
			cancelled, cancelErr := w.store.IsSessionCancelled(claim.Job.SessionID)
			if cancelErr != nil {
				w.recordError(cancelErr)
				break
			}
			if cancelled {
				if err := w.store.RetryJob(claim.Job, time.Now(), "session_cancelled"); err != nil {
					w.recordError(err)
				}
				continue
			}
			w.process(ctx, *claim)
			if ctx.Err() != nil {
				return
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-w.wake:
		case <-ticker.C:
		}
	}
}

func (w *Worker) process(ctx context.Context, claim ClaimedJob) {
	jobCtx, unregister := w.registerActiveJob(ctx, claim.Job.SessionID)
	defer unregister()
	cancelled, err := w.store.IsSessionCancelled(claim.Job.SessionID)
	if err != nil {
		w.recordError(err)
		return
	}
	if cancelled {
		return
	}
	audio, err := w.store.OpenClaimedAudio(claim)
	if err != nil {
		if persistErr := w.store.DeadLetterJob(claim.Job, "audio_open_failed"); persistErr != nil {
			w.recordError(persistErr)
		}
		return
	}
	result, transcribeErr := w.provider.Transcribe(jobCtx, TranscribeRequest{
		Audio:    audio,
		Filename: fmt.Sprintf("%06d.%s", claim.Chunk.Index, claim.Chunk.Format),
		Model:    w.options.Model,
		Language: w.options.Language,
	})
	_ = audio.Close()
	if jobCtx.Err() != nil {
		if err := w.store.RetryJob(claim.Job, time.Now(), "worker_interrupted"); err != nil {
			w.recordError(err)
		}
		return
	}
	if transcribeErr == nil {
		if err := w.store.CompleteJob(claim, TranscriptChunk{
			Provider:   w.options.ProviderName,
			Model:      w.options.Model,
			Language:   result.Language,
			DurationMS: result.DurationMS,
			Text:       result.Text,
		}); err != nil {
			if persistErr := w.store.RetryJob(claim.Job, time.Now().Add(w.options.RetryInitialDelay), "transcript_commit_failed"); persistErr != nil {
				w.recordError(fmt.Errorf("commit transcript: %w; persist retry: %v", err, persistErr))
			}
		}
		return
	}
	if ctx.Err() != nil && (errors.Is(transcribeErr, context.Canceled) || errors.Is(transcribeErr, context.DeadlineExceeded)) {
		if err := w.store.RetryJob(claim.Job, time.Now(), "worker_interrupted"); err != nil {
			w.recordError(err)
		}
		return
	}
	if errors.Is(transcribeErr, ErrProviderRetryable) && claim.Job.Attempts < w.options.MaxAttempts {
		delay := w.retryDelay(claim.Job.Attempts)
		if err := w.store.RetryJob(claim.Job, time.Now().Add(delay), "provider_retryable"); err != nil {
			w.recordError(err)
		}
		return
	}
	message := "provider_permanent"
	if errors.Is(transcribeErr, ErrProviderRetryable) {
		message = "provider_attempts_exhausted"
	}
	if err := w.store.DeadLetterJob(claim.Job, message); err != nil {
		w.recordError(err)
	}
}

func (w *Worker) LastError() error {
	w.errMu.Lock()
	defer w.errMu.Unlock()
	return w.lastErr
}

func (w *Worker) recordError(err error) {
	w.errMu.Lock()
	w.lastErr = err
	w.errMu.Unlock()
}

func (w *Worker) retryDelay(attempt int) time.Duration {
	delay := w.options.RetryInitialDelay
	for count := 1; count < attempt && delay < w.options.RetryMaxDelay/2; count++ {
		delay *= 2
	}
	if delay > w.options.RetryMaxDelay {
		return w.options.RetryMaxDelay
	}
	return delay
}

func (w *Worker) registerActiveJob(parent context.Context, sessionID string) (context.Context, func()) {
	ctx, cancel := context.WithCancel(parent)
	w.activeMu.Lock()
	w.activeSeq++
	id := w.activeSeq
	if w.activeJobs[sessionID] == nil {
		w.activeJobs[sessionID] = make(map[uint64]context.CancelFunc)
	}
	w.activeJobs[sessionID][id] = cancel
	w.activeMu.Unlock()
	return ctx, func() {
		cancel()
		w.activeMu.Lock()
		delete(w.activeJobs[sessionID], id)
		if len(w.activeJobs[sessionID]) == 0 {
			delete(w.activeJobs, sessionID)
		}
		w.activeMu.Unlock()
	}
}

func (w *Worker) CancelSession(sessionID string) {
	w.activeMu.Lock()
	cancellations := make([]context.CancelFunc, 0, len(w.activeJobs[sessionID]))
	for _, cancel := range w.activeJobs[sessionID] {
		cancellations = append(cancellations, cancel)
	}
	w.activeMu.Unlock()
	for _, cancel := range cancellations {
		cancel()
	}
}
