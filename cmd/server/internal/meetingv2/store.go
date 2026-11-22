package meetingv2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidSessionID           = errors.New("invalid meeting session id")
	ErrInvalidChunk               = errors.New("invalid meeting audio chunk")
	ErrChunkConflict              = errors.New("meeting audio chunk conflict")
	ErrSessionNotFound            = errors.New("meeting session not found")
	ErrInvalidTranscript          = errors.New("invalid manual transcript")
	ErrTranscriptConflict         = errors.New("manual transcript conflict")
	ErrTranscriptRevisionConflict = errors.New("manual transcript revision conflict")
	ErrTranscriptNotFound         = errors.New("manual transcript not found")
)

var safeSessionID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)
var safeLanguage = regexp.MustCompile(`^[A-Za-z]{2,8}(?:[-_][A-Za-z0-9]{1,8})*$`)
var safeIdempotencyKey = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
var safeChunkID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,255}$`)

const maxStoredChunkIndex = 1000000
const maxManualTranscriptBytes = 1 << 20
const maxManualTranscriptRevisions = 10000

var supportedAudioFormats = map[string]bool{
	"webm": true,
	"wav":  true,
	"mp3":  true,
	"ogg":  true,
	"flac": true,
	"m4a":  true,
}

// Store persists v2 meeting state inside the existing meetings root. Each
// meeting keeps its legacy files while v2 data is isolated in subdirectories.
type Store struct {
	root         string
	maxChunkSize int64
	now          func() time.Time
	mu           sync.Mutex
}

func NewStore(root string, maxChunkSize int64) (*Store, error) {
	if strings.TrimSpace(root) == "" {
		return nil, fmt.Errorf("meetings root is required")
	}
	if maxChunkSize <= 0 {
		return nil, fmt.Errorf("max chunk size must be positive")
	}
	cleanRoot, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return nil, fmt.Errorf("resolve meetings root: %w", err)
	}
	if err := os.MkdirAll(cleanRoot, 0o750); err != nil {
		return nil, fmt.Errorf("create meetings root: %w", err)
	}
	info, err := os.Lstat(cleanRoot)
	if err != nil {
		return nil, fmt.Errorf("inspect meetings root: %w", err)
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("meetings root must be a real directory")
	}
	return &Store{root: cleanRoot, maxChunkSize: maxChunkSize, now: time.Now}, nil
}

func (s *Store) MaxChunkSize() int64 {
	return s.maxChunkSize
}

func (s *Store) CreateSession(id, title string, metadata map[string]string) (Session, error) {
	return s.CreateOwnedSession(id, title, "", metadata)
}

func (s *Store) CreateOwnedSession(id, title, owner string, metadata map[string]string) (Session, error) {
	return s.CreateOwnedSessionWithMode(id, title, owner, metadata, TranscriptionAutomatic)
}

func (s *Store) CreateOwnedSessionWithMode(id, title, owner string, metadata map[string]string, mode TranscriptionMode) (Session, error) {
	if mode != TranscriptionAutomatic && mode != TranscriptionManual {
		return Session{}, fmt.Errorf("invalid transcription mode")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	dir, err := s.sessionDir(id)
	if err != nil {
		return Session{}, err
	}
	if _, err := os.Stat(filepath.Join(dir, "session.json")); err == nil {
		return Session{}, fmt.Errorf("session %q already exists", id)
	} else if !os.IsNotExist(err) {
		return Session{}, fmt.Errorf("inspect session: %w", err)
	}
	if err := s.ensureSessionDirs(dir); err != nil {
		return Session{}, err
	}
	now := s.now().UTC()
	session := Session{
		SchemaVersion:     SchemaVersion,
		ID:                id,
		Title:             strings.TrimSpace(title),
		Owner:             strings.TrimSpace(owner),
		State:             StateCreated,
		TranscriptionMode: mode,
		CreatedAt:         now,
		UpdatedAt:         now,
		LastChunk:         -1,
		Metadata:          cloneMetadata(metadata),
	}
	if err := writeJSONAtomic(filepath.Join(dir, "session.json"), session, 0o640); err != nil {
		return Session{}, err
	}
	if session.Owner != "" {
		metadataPath := filepath.Join(dir, "metadata.json")
		ownerProjection := map[string]any{}
		if data, readErr := readBoundedRegularFile(metadataPath, 1<<20); readErr == nil {
			if jsonErr := json.Unmarshal(data, &ownerProjection); jsonErr != nil {
				return Session{}, fmt.Errorf("decode legacy meeting metadata: %w", jsonErr)
			}
		} else if !os.IsNotExist(readErr) {
			return Session{}, readErr
		}
		ownerProjection["owner"] = session.Owner
		if _, exists := ownerProjection["created_at"]; !exists {
			ownerProjection["created_at"] = session.CreatedAt
		}
		if err := writeJSONAtomic(metadataPath, ownerProjection, 0o640); err != nil {
			return Session{}, err
		}
	}
	return session, nil
}

func (s *Store) LoadSession(id string) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadSessionLocked(id)
}

func (s *Store) SetSessionState(id string, state SessionState) (Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, err := s.loadSessionLocked(id)
	if err != nil {
		return Session{}, err
	}
	if !validTransition(session.State, state) {
		return Session{}, fmt.Errorf("invalid session state transition %s -> %s", session.State, state)
	}
	now := s.now().UTC()
	session.State = state
	session.UpdatedAt = now
	if state == StateRecording && session.StartedAt == nil {
		session.StartedAt = &now
	}
	if state == StateStopping && session.StoppedAt == nil {
		session.StoppedAt = &now
	}
	dir, _ := s.sessionDir(id)
	if err := writeJSONAtomic(filepath.Join(dir, "session.json"), session, 0o640); err != nil {
		return Session{}, err
	}
	if state == StateStopping {
		if err := s.reconcileSessionStateLocked(id, dir); err != nil {
			return Session{}, err
		}
		return s.loadSessionLocked(id)
	}
	return session, nil
}

func (s *Store) IngestChunk(sessionID string, request IngestChunkRequest, audio io.Reader) (IngestChunkResult, error) {
	return s.ingestChunk(sessionID, request, audio, false)
}

func (s *Store) ingestChunk(sessionID string, request IngestChunkRequest, audio io.Reader, allowCreated bool) (IngestChunkResult, error) {
	if audio == nil || request.Index < 0 {
		return IngestChunkResult{}, ErrInvalidChunk
	}
	format := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(request.Format), "."))
	if !supportedAudioFormats[format] {
		return IngestChunkResult{}, fmt.Errorf("%w: unsupported format %q", ErrInvalidChunk, format)
	}
	if request.DurationMS < 0 {
		return IngestChunkResult{}, fmt.Errorf("%w: negative duration", ErrInvalidChunk)
	}
	declaredHash := strings.ToLower(strings.TrimSpace(request.DeclaredSHA256))
	if declaredHash != "" {
		if len(declaredHash) != sha256.Size*2 {
			return IngestChunkResult{}, fmt.Errorf("%w: invalid SHA-256", ErrInvalidChunk)
		}
		if _, err := hex.DecodeString(declaredHash); err != nil {
			return IngestChunkResult{}, fmt.Errorf("%w: invalid SHA-256", ErrInvalidChunk)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	session, err := s.loadSessionLocked(sessionID)
	if err != nil {
		return IngestChunkResult{}, err
	}
	acceptsNewChunk := session.State == StateRecording || session.State == StateStopping || session.State == StateDegraded || (allowCreated && session.State == StateCreated)
	dir, _ := s.sessionDir(sessionID)
	if err := s.ensureSessionDirs(dir); err != nil {
		return IngestChunkResult{}, err
	}
	metaPath := filepath.Join(dir, "chunks", fmt.Sprintf("%06d.json", request.Index))
	if existing, err := loadChunkMetadata(metaPath); err == nil {
		if existing.Format != format ||
			existing.ClientChunkID != request.ClientChunkID ||
			existing.DurationMS != request.DurationMS ||
			existing.Final != request.Final ||
			!existing.StartedAt.Equal(request.StartedAt) {
			return IngestChunkResult{}, ErrChunkConflict
		}
		if declaredHash != "" && existing.SHA256 != declaredHash {
			return IngestChunkResult{}, ErrChunkConflict
		}
		actualHash, hashErr := hashAudioReader(audio, s.maxChunkSize)
		if hashErr != nil {
			return IngestChunkResult{}, hashErr
		}
		if actualHash != existing.SHA256 {
			return IngestChunkResult{}, ErrChunkConflict
		}
		jobCreated, err := s.reconcileChunkLocked(dir, &session, existing)
		if err != nil {
			return IngestChunkResult{}, err
		}
		return s.ingestResultLocked(dir, existing, true, jobCreated, session)
	} else if !os.IsNotExist(err) {
		return IngestChunkResult{}, fmt.Errorf("read chunk metadata: %w", err)
	}
	if !acceptsNewChunk {
		return IngestChunkResult{}, fmt.Errorf("%w: session state is %s", ErrInvalidChunk, session.State)
	}

	temp, err := os.CreateTemp(filepath.Join(dir, "chunks"), ".upload-*")
	if err != nil {
		return IngestChunkResult{}, fmt.Errorf("create chunk temp file: %w", err)
	}
	tempPath := temp.Name()
	removeTemp := true
	defer func() {
		_ = temp.Close()
		if removeTemp {
			_ = os.Remove(tempPath)
		}
	}()

	hash := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(temp, hash), io.LimitReader(audio, s.maxChunkSize+1))
	if copyErr != nil {
		return IngestChunkResult{}, fmt.Errorf("store audio chunk: %w", copyErr)
	}
	if written == 0 || written > s.maxChunkSize {
		return IngestChunkResult{}, fmt.Errorf("%w: chunk size %d exceeds limit %d", ErrInvalidChunk, written, s.maxChunkSize)
	}
	actualHash := hex.EncodeToString(hash.Sum(nil))
	if declaredHash != "" && actualHash != declaredHash {
		return IngestChunkResult{}, fmt.Errorf("%w: SHA-256 mismatch", ErrChunkConflict)
	}
	if err := temp.Sync(); err != nil {
		return IngestChunkResult{}, fmt.Errorf("sync audio chunk: %w", err)
	}
	if err := temp.Close(); err != nil {
		return IngestChunkResult{}, fmt.Errorf("close audio chunk: %w", err)
	}

	audioPath := filepath.Join(dir, "chunks", fmt.Sprintf("%06d.%s", request.Index, format))
	if err := os.Rename(tempPath, audioPath); err != nil {
		return IngestChunkResult{}, fmt.Errorf("commit audio chunk: %w", err)
	}
	removeTemp = false

	now := s.now().UTC()
	metadata := ChunkMetadata{
		SchemaVersion: SchemaVersion,
		SessionID:     sessionID,
		Index:         request.Index,
		ClientChunkID: request.ClientChunkID,
		StartedAt:     request.StartedAt,
		DurationMS:    request.DurationMS,
		Format:        format,
		SHA256:        actualHash,
		Size:          written,
		Final:         request.Final,
		AcceptedAt:    now,
	}
	if err := writeJSONAtomic(metaPath, metadata, 0o640); err != nil {
		_ = os.Remove(audioPath)
		return IngestChunkResult{}, err
	}

	if effectiveTranscriptionMode(session) == TranscriptionAutomatic {
		job := ASRJob{
			SchemaVersion: SchemaVersion,
			ID:            fmt.Sprintf("%s:%d", sessionID, request.Index),
			SessionID:     sessionID,
			ChunkIndex:    request.Index,
			State:         JobPending,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		if err := writeJSONAtomic(filepath.Join(dir, "jobs", fmt.Sprintf("%06d.json", request.Index)), job, 0o640); err != nil {
			_ = os.Remove(metaPath)
			_ = os.Remove(audioPath)
			return IngestChunkResult{}, err
		}
	}

	if request.Index > session.LastChunk {
		session.LastChunk = request.Index
	}
	session.UpdatedAt = now
	if err := writeJSONAtomic(filepath.Join(dir, "session.json"), session, 0o640); err != nil {
		return IngestChunkResult{}, err
	}
	return s.ingestResultLocked(dir, metadata, false, effectiveTranscriptionMode(session) == TranscriptionAutomatic, session)
}

func (s *Store) ingestResultLocked(dir string, metadata ChunkMetadata, duplicate, jobCreated bool, session Session) (IngestChunkResult, error) {
	result := IngestChunkResult{Chunk: metadata, Duplicate: duplicate, JobCreated: jobCreated}
	if effectiveTranscriptionMode(session) != TranscriptionAutomatic {
		return result, nil
	}
	jobPath := filepath.Join(dir, "jobs", fmt.Sprintf("%06d.json", metadata.Index))
	job, err := loadJob(jobPath)
	if err != nil {
		return IngestChunkResult{}, err
	}
	if err := validateJob(job, metadata.SessionID, jobPath); err != nil {
		return IngestChunkResult{}, err
	}
	state := job.State
	result.JobState = &state
	return result, nil
}

func (s *Store) reconcileChunkLocked(dir string, session *Session, metadata ChunkMetadata) (bool, error) {
	if err := s.verifyPersistedChunk(dir, metadata); err != nil {
		return false, err
	}

	jobCreated := false
	if effectiveTranscriptionMode(*session) == TranscriptionAutomatic {
		jobPath := filepath.Join(dir, "jobs", fmt.Sprintf("%06d.json", metadata.Index))
		if job, err := loadJob(jobPath); os.IsNotExist(err) {
			now := s.now().UTC()
			job = ASRJob{
				SchemaVersion: SchemaVersion,
				ID:            fmt.Sprintf("%s:%d", metadata.SessionID, metadata.Index),
				SessionID:     metadata.SessionID,
				ChunkIndex:    metadata.Index,
				State:         JobPending,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			if err := writeJSONAtomic(jobPath, job, 0o640); err != nil {
				return false, err
			}
			jobCreated = true
		} else if err != nil {
			return false, fmt.Errorf("read ASR job: %w", err)
		} else if err := validateJob(job, metadata.SessionID, jobPath); err != nil {
			return false, err
		}
	}

	if metadata.Index > session.LastChunk {
		session.LastChunk = metadata.Index
		session.UpdatedAt = s.now().UTC()
		if err := writeJSONAtomic(filepath.Join(dir, "session.json"), *session, 0o640); err != nil {
			return false, err
		}
	}
	return jobCreated, nil
}

func (s *Store) ImportAudio(sessionID string, request IngestChunkRequest, audio io.Reader) (IngestChunkResult, Session, error) {
	if request.Index != 0 || !request.Final || !safeChunkID.MatchString(request.ClientChunkID) {
		return IngestChunkResult{}, Session{}, ErrInvalidChunk
	}
	session, err := s.LoadSession(sessionID)
	if err != nil {
		return IngestChunkResult{}, Session{}, err
	}
	if session.LastChunk > 0 || (session.State != StateCreated && session.State != StateRecording && session.State != StateStopping && session.State != StateProcessing && session.State != StateCompleted) {
		return IngestChunkResult{}, Session{}, ErrChunkConflict
	}
	if request.StartedAt.IsZero() {
		request.StartedAt = session.CreatedAt
	}
	result, err := s.ingestChunk(sessionID, request, audio, true)
	if err != nil {
		return IngestChunkResult{}, Session{}, err
	}
	session, err = s.LoadSession(sessionID)
	if err != nil {
		return IngestChunkResult{}, Session{}, err
	}
	if session.State == StateCreated {
		if session, err = s.SetSessionState(sessionID, StateRecording); err != nil {
			return IngestChunkResult{}, Session{}, err
		}
	}
	if session.State == StateRecording || session.State == StateDegraded {
		session, err = s.SetSessionState(sessionID, StateStopping)
		if err != nil {
			return IngestChunkResult{}, Session{}, err
		}
	}
	return result, session, nil
}

func (s *Store) verifyPersistedChunk(dir string, metadata ChunkMetadata) error {
	audioPath := filepath.Join(dir, "chunks", fmt.Sprintf("%06d.%s", metadata.Index, metadata.Format))
	actualHash, size, err := hashAudioFile(audioPath, s.maxChunkSize)
	if err != nil {
		return fmt.Errorf("verify persisted audio chunk: %w", err)
	}
	if actualHash != metadata.SHA256 || size != metadata.Size {
		return ErrChunkConflict
	}
	return nil
}

func (s *Store) RecoverInterruptedJobs() (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.root)
	if err != nil {
		return 0, fmt.Errorf("list meeting sessions: %w", err)
	}
	recovered := 0
	for _, entry := range entries {
		if !entry.IsDir() || !safeSessionID.MatchString(entry.Name()) {
			continue
		}
		session, err := s.loadSessionLocked(entry.Name())
		if errors.Is(err, ErrSessionNotFound) {
			continue // legacy meeting directory without a v2 manifest
		}
		if err != nil {
			return recovered, err
		}
		dir, err := s.sessionDir(entry.Name())
		if err != nil {
			return recovered, err
		}
		metadataPaths, err := filepath.Glob(filepath.Join(dir, "chunks", "*.json"))
		if err != nil {
			return recovered, err
		}
		sort.Strings(metadataPaths)
		for _, metadataPath := range metadataPaths {
			metadata, err := loadChunkMetadata(metadataPath)
			if err != nil {
				return recovered, err
			}
			if err := validateChunkMetadata(metadata, entry.Name(), metadataPath, s.maxChunkSize); err != nil {
				return recovered, err
			}
			jobPath := filepath.Join(dir, "jobs", fmt.Sprintf("%06d.json", metadata.Index))
			_, jobErr := loadJob(jobPath)
			missingJob := effectiveTranscriptionMode(session) == TranscriptionAutomatic && os.IsNotExist(jobErr)
			staleChunk := metadata.Index > session.LastChunk
			if _, err := s.reconcileChunkLocked(dir, &session, metadata); err != nil {
				return recovered, err
			}
			if missingJob || staleChunk {
				recovered++
			}
		}

		jobs, err := filepath.Glob(filepath.Join(dir, "jobs", "*.json"))
		if err != nil {
			return recovered, err
		}
		sort.Strings(jobs)
		for _, path := range jobs {
			job, err := loadJob(path)
			if err != nil {
				return recovered, err
			}
			if err := validateJob(job, entry.Name(), path); err != nil {
				return recovered, err
			}
			metadataPath := filepath.Join(dir, "chunks", fmt.Sprintf("%06d.json", job.ChunkIndex))
			metadata, err := loadChunkMetadata(metadataPath)
			if err != nil {
				return recovered, fmt.Errorf("ASR job %s has no valid chunk metadata: %w", path, err)
			}
			if err := validateChunkMetadata(metadata, entry.Name(), metadataPath, s.maxChunkSize); err != nil {
				return recovered, err
			}
			if err := s.verifyPersistedChunk(dir, metadata); err != nil {
				return recovered, err
			}
			if job.State != JobProcessing {
				continue
			}
			job.State = JobPending
			job.NextAttemptAt = nil
			job.LastError = "processing interrupted by service restart"
			job.UpdatedAt = s.now().UTC()
			if err := writeJSONAtomic(path, job, 0o640); err != nil {
				return recovered, err
			}
			recovered++
		}
		if err := s.reconcileDocumentArtifactsLocked(session.ID); err != nil {
			return recovered, err
		}
		previousState := session.State
		if session.State == StateCreated || session.State == StateRecording {
			finalChunk, finalErr := s.singleFinalChunkLocked(dir, session.ID)
			if finalErr != nil {
				return recovered, finalErr
			}
			if finalChunk {
				session.State = StateStopping
				session.UpdatedAt = s.now().UTC()
				if err := writeJSONAtomic(filepath.Join(dir, "session.json"), session, 0o640); err != nil {
					return recovered, err
				}
				recovered++
			}
		}
		if err := s.reconcileSessionStateLocked(session.ID, dir); err != nil {
			return recovered, err
		}
		updated, err := s.loadSessionLocked(session.ID)
		if err != nil {
			return recovered, err
		}
		if updated.State != previousState {
			recovered++
		}
	}
	return recovered, nil
}

func (s *Store) singleFinalChunkLocked(dir, sessionID string) (bool, error) {
	entries, err := os.ReadDir(filepath.Join(dir, "chunks"))
	if err != nil {
		return false, err
	}
	metadataCount := 0
	var final bool
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		metadata, err := loadChunkMetadata(filepath.Join(dir, "chunks", entry.Name()))
		if err != nil {
			return false, err
		}
		if err := validateChunkMetadata(metadata, sessionID, filepath.Join(dir, "chunks", entry.Name()), s.maxChunkSize); err != nil {
			return false, err
		}
		metadataCount++
		final = metadata.Index == 0 && metadata.Final
	}
	return metadataCount == 1 && final, nil
}

func (s *Store) loadSessionLocked(id string) (Session, error) {
	dir, err := s.sessionDir(id)
	if err != nil {
		return Session{}, err
	}
	data, err := readBoundedRegularFile(filepath.Join(dir, "session.json"), 1<<20)
	if err != nil {
		if os.IsNotExist(err) {
			return Session{}, ErrSessionNotFound
		}
		return Session{}, fmt.Errorf("read session: %w", err)
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return Session{}, fmt.Errorf("decode session: %w", err)
	}
	validState := map[SessionState]bool{
		StateCreated: true, StateRecording: true, StateStopping: true, StateProcessing: true,
		StateDegraded: true, StateCompleted: true, StateCompletedWithErrors: true,
		StateFailed: true, StateCancelled: true,
	}
	if session.SchemaVersion != SchemaVersion || session.ID != id || !validState[session.State] ||
		session.CreatedAt.IsZero() || session.UpdatedAt.IsZero() || session.LastChunk < -1 || session.LastChunk >= maxStoredChunkIndex {
		return Session{}, fmt.Errorf("invalid session manifest")
	}
	if session.TranscriptionMode != "" && session.TranscriptionMode != TranscriptionAutomatic && session.TranscriptionMode != TranscriptionManual {
		return Session{}, fmt.Errorf("invalid session transcription mode")
	}
	return session, nil
}

func (s *Store) sessionDir(id string) (string, error) {
	if !safeSessionID.MatchString(id) || id == "." || id == ".." {
		return "", ErrInvalidSessionID
	}
	dir := filepath.Join(s.root, id)
	rel, err := filepath.Rel(s.root, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrInvalidSessionID
	}
	if info, err := os.Lstat(dir); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return "", fmt.Errorf("%w: session path is not a real directory", ErrInvalidSessionID)
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect session path: %w", err)
	}
	return dir, nil
}

func (s *Store) ensureSessionDirs(dir string) error {
	if err := ensureRealDirectory(dir); err != nil {
		return fmt.Errorf("create session directory: %w", err)
	}
	for _, subdir := range []string{"chunks", "jobs", "transcripts", "documents", "events"} {
		if err := ensureRealDirectory(filepath.Join(dir, subdir)); err != nil {
			return fmt.Errorf("create session directory: %w", err)
		}
	}
	return nil
}

func ensureRealDirectory(path string) error {
	if err := os.Mkdir(path, 0o750); err != nil && !os.IsExist(err) {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("%s is not a real directory", path)
	}
	return nil
}

func hashAudioReader(audio io.Reader, maxSize int64) (string, error) {
	hash := sha256.New()
	written, err := io.Copy(hash, io.LimitReader(audio, maxSize+1))
	if err != nil {
		return "", fmt.Errorf("read duplicate audio chunk: %w", err)
	}
	if written == 0 || written > maxSize {
		return "", fmt.Errorf("%w: chunk size %d exceeds limit %d", ErrInvalidChunk, written, maxSize)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func hashAudioFile(path string, maxSize int64) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()
	hash := sha256.New()
	written, err := io.Copy(hash, io.LimitReader(file, maxSize+1))
	if err != nil {
		return "", 0, err
	}
	if written == 0 || written > maxSize {
		return "", written, ErrInvalidChunk
	}
	return hex.EncodeToString(hash.Sum(nil)), written, nil
}

func loadChunkMetadata(path string) (ChunkMetadata, error) {
	data, err := readBoundedRegularFile(path, 1<<20)
	if err != nil {
		return ChunkMetadata{}, err
	}
	var metadata ChunkMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return ChunkMetadata{}, err
	}
	return metadata, nil
}

func validateChunkMetadata(metadata ChunkMetadata, sessionID, path string, maxSize int64) error {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	expectedIndex, err := strconv.Atoi(base)
	if err != nil || metadata.SchemaVersion != SchemaVersion || metadata.SessionID != sessionID || metadata.Index != expectedIndex {
		return fmt.Errorf("invalid chunk metadata %s", path)
	}
	if !supportedAudioFormats[metadata.Format] || metadata.Size <= 0 || metadata.Size > maxSize || len(metadata.SHA256) != sha256.Size*2 {
		return fmt.Errorf("invalid chunk metadata %s", path)
	}
	if _, err := hex.DecodeString(metadata.SHA256); err != nil {
		return fmt.Errorf("invalid chunk metadata %s", path)
	}
	return nil
}

func loadJob(path string) (ASRJob, error) {
	data, err := readBoundedRegularFile(path, 1<<20)
	if err != nil {
		return ASRJob{}, err
	}
	var job ASRJob
	if err := json.Unmarshal(data, &job); err != nil {
		return ASRJob{}, err
	}
	return job, nil
}

func validateJob(job ASRJob, sessionID, path string) error {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	expectedIndex, err := strconv.Atoi(base)
	if err != nil || job.SchemaVersion != SchemaVersion || job.SessionID != sessionID || job.ChunkIndex != expectedIndex || job.ID != fmt.Sprintf("%s:%d", sessionID, expectedIndex) {
		return fmt.Errorf("invalid ASR job %s", path)
	}
	validState := map[JobState]bool{JobPending: true, JobProcessing: true, JobRetryWait: true, JobCompleted: true, JobDead: true}
	if !validState[job.State] || job.Attempts < 0 {
		return fmt.Errorf("invalid ASR job %s", path)
	}
	return nil
}

func writeJSONAtomic(path string, value any, mode os.FileMode) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("encode %s: %w", filepath.Base(path), err)
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func writeFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		return fmt.Errorf("refusing to replace non-regular file")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func validTransition(from, to SessionState) bool {
	if from == to {
		return true
	}
	allowed := map[SessionState]map[SessionState]bool{
		StateCreated:    {StateRecording: true, StateCancelled: true},
		StateRecording:  {StateStopping: true, StateDegraded: true, StateFailed: true, StateCancelled: true},
		StateStopping:   {StateProcessing: true, StateDegraded: true, StateFailed: true},
		StateProcessing: {StateCompleted: true, StateCompletedWithErrors: true, StateDegraded: true, StateFailed: true},
		StateDegraded:   {StateRecording: true, StateStopping: true, StateProcessing: true, StateCompletedWithErrors: true, StateFailed: true, StateCancelled: true},
	}
	return allowed[from][to]
}

func effectiveTranscriptionMode(session Session) TranscriptionMode {
	if session.TranscriptionMode == "" {
		return TranscriptionAutomatic
	}
	return session.TranscriptionMode
}

func (s *Store) SubmitManualTranscript(sessionID string, request SubmitManualTranscriptRequest) (ManualTranscript, bool, error) {
	if !validManualTranscriptRequest(request) {
		return ManualTranscript{}, false, ErrInvalidTranscript
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	session, err := s.loadSessionLocked(sessionID)
	if err != nil {
		return ManualTranscript{}, false, err
	}
	if session.State == StateCancelled {
		return ManualTranscript{}, false, fmt.Errorf("%w: cancelled session", ErrInvalidTranscript)
	}
	dir, _ := s.sessionDir(sessionID)
	manualDir := filepath.Join(dir, "transcripts", "manual")
	if err := ensureRealDirectory(manualDir); err != nil {
		return ManualTranscript{}, false, err
	}
	entries, err := os.ReadDir(manualDir)
	if err != nil {
		return ManualTranscript{}, false, err
	}
	latest := ManualTranscript{}
	revisionCount := 0
	contentHash := sha256.Sum256([]byte(request.Text))
	encodedHash := hex.EncodeToString(contentHash[:])
	for _, entry := range entries {
		if isAtomicTempEntry(entry) {
			continue
		}
		if entry.IsDir() || len(entry.Name()) != 11 || !strings.HasSuffix(entry.Name(), ".json") {
			return ManualTranscript{}, false, fmt.Errorf("invalid manual transcript entry")
		}
		revision, parseErr := strconv.Atoi(strings.TrimSuffix(entry.Name(), ".json"))
		if parseErr != nil || revision < 1 || revision > maxManualTranscriptRevisions {
			return ManualTranscript{}, false, fmt.Errorf("invalid manual transcript entry")
		}
		revisionCount++
		if revisionCount > maxManualTranscriptRevisions {
			return ManualTranscript{}, false, fmt.Errorf("too many manual transcript revisions")
		}
		item, loadErr := loadManualTranscript(filepath.Join(manualDir, entry.Name()), sessionID, revision)
		if loadErr != nil {
			return ManualTranscript{}, false, loadErr
		}
		if item.IdempotencyKey == request.IdempotencyKey {
			if item.ContentSHA256 == encodedHash && item.Text == request.Text && item.Language == request.Language {
				return item, true, nil
			}
			return ManualTranscript{}, false, ErrTranscriptConflict
		}
		if item.Revision > latest.Revision {
			latest = item
		}
	}
	if request.ExpectedRevision != latest.Revision {
		return ManualTranscript{}, false, ErrTranscriptRevisionConflict
	}
	if latest.Revision >= maxManualTranscriptRevisions {
		return ManualTranscript{}, false, fmt.Errorf("manual transcript revision limit reached")
	}
	transcript := ManualTranscript{
		SchemaVersion: SchemaVersion, SessionID: sessionID, Revision: latest.Revision + 1,
		Source: "manual", Language: request.Language, Text: request.Text, ContentSHA256: encodedHash,
		IdempotencyKey: request.IdempotencyKey, SubmittedBy: request.SubmittedBy, SubmittedAt: s.now().UTC(),
	}
	path := filepath.Join(manualDir, fmt.Sprintf("%06d.json", transcript.Revision))
	if err := writeJSONAtomic(path, transcript, 0o640); err != nil {
		return ManualTranscript{}, false, err
	}
	return transcript, false, nil
}

func (s *Store) GetLatestManualTranscript(sessionID string) (ManualTranscript, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.loadSessionLocked(sessionID); err != nil {
		return ManualTranscript{}, err
	}
	dir, _ := s.sessionDir(sessionID)
	manualDir := filepath.Join(dir, "transcripts", "manual")
	entries, err := os.ReadDir(manualDir)
	if os.IsNotExist(err) {
		return ManualTranscript{}, ErrTranscriptNotFound
	}
	if err != nil {
		return ManualTranscript{}, err
	}
	latest := ManualTranscript{}
	for _, entry := range entries {
		if isAtomicTempEntry(entry) {
			continue
		}
		if entry.IsDir() || len(entry.Name()) != 11 || !strings.HasSuffix(entry.Name(), ".json") {
			return ManualTranscript{}, fmt.Errorf("invalid manual transcript entry")
		}
		revision, parseErr := strconv.Atoi(strings.TrimSuffix(entry.Name(), ".json"))
		if parseErr != nil || revision < 1 || revision > maxManualTranscriptRevisions {
			return ManualTranscript{}, fmt.Errorf("invalid manual transcript entry")
		}
		item, loadErr := loadManualTranscript(filepath.Join(manualDir, entry.Name()), sessionID, revision)
		if loadErr != nil {
			return ManualTranscript{}, loadErr
		}
		if item.Revision > latest.Revision {
			latest = item
		}
	}
	if latest.Revision == 0 {
		return ManualTranscript{}, ErrTranscriptNotFound
	}
	return latest, nil
}

func isAtomicTempEntry(entry os.DirEntry) bool {
	if !strings.HasPrefix(entry.Name(), ".tmp-") {
		return false
	}
	info, err := entry.Info()
	return err == nil && info.Mode().IsRegular()
}

func validManualTranscriptRequest(request SubmitManualTranscriptRequest) bool {
	return request.ExpectedRevision >= 0 && request.ExpectedRevision <= maxManualTranscriptRevisions &&
		request.Text != "" && strings.TrimSpace(request.Text) != "" && len([]byte(request.Text)) <= maxManualTranscriptBytes &&
		utf8.ValidString(request.Text) && !strings.ContainsRune(request.Text, '\x00') &&
		(request.Language == "" || safeLanguage.MatchString(request.Language)) && safeIdempotencyKey.MatchString(request.IdempotencyKey) &&
		request.SubmittedBy != "" && utf8.ValidString(request.SubmittedBy) && len(request.SubmittedBy) <= 256
}

func loadManualTranscript(path, sessionID string, revision int) (ManualTranscript, error) {
	data, err := readBoundedRegularFile(path, maxManualTranscriptBytes+(64<<10))
	if err != nil {
		return ManualTranscript{}, err
	}
	var transcript ManualTranscript
	if err := json.Unmarshal(data, &transcript); err != nil {
		return ManualTranscript{}, err
	}
	hash := sha256.Sum256([]byte(transcript.Text))
	if transcript.SchemaVersion != SchemaVersion || transcript.SessionID != sessionID || transcript.Revision != revision ||
		transcript.Source != "manual" || transcript.ContentSHA256 != hex.EncodeToString(hash[:]) || transcript.SubmittedAt.IsZero() ||
		!validManualTranscriptRequest(SubmitManualTranscriptRequest{Text: transcript.Text, Language: transcript.Language, ExpectedRevision: revision - 1, IdempotencyKey: transcript.IdempotencyKey, SubmittedBy: transcript.SubmittedBy}) {
		return ManualTranscript{}, fmt.Errorf("invalid manual transcript revision")
	}
	return transcript, nil
}

func cloneMetadata(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}

func (s *Store) ClaimNextJob(now time.Time) (*ClaimedJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() || !safeSessionID.MatchString(entry.Name()) {
			continue
		}
		dir, err := s.sessionDir(entry.Name())
		if err != nil {
			return nil, err
		}
		session, err := s.loadSessionLocked(entry.Name())
		if err != nil {
			return nil, err
		}
		if session.State == StateCancelled || session.State == StateCompleted || session.State == StateCompletedWithErrors || session.State == StateFailed {
			continue
		}
		paths, err := filepath.Glob(filepath.Join(dir, "jobs", "*.json"))
		if err != nil {
			return nil, err
		}
		sort.Strings(paths)
		for _, jobPath := range paths {
			job, err := loadJob(jobPath)
			if err != nil {
				return nil, err
			}
			if err := validateJob(job, entry.Name(), jobPath); err != nil {
				return nil, err
			}
			if job.State != JobPending && !(job.State == JobRetryWait && (job.NextAttemptAt == nil || !job.NextAttemptAt.After(now))) {
				continue
			}
			transcriptPath := filepath.Join(dir, "transcripts", "chunks", fmt.Sprintf("%06d.json", job.ChunkIndex))
			if transcript, transcriptErr := loadTranscript(transcriptPath); transcriptErr == nil {
				metadataPath := filepath.Join(dir, "chunks", fmt.Sprintf("%06d.json", job.ChunkIndex))
				metadata, metadataErr := loadChunkMetadata(metadataPath)
				if metadataErr != nil || transcript.SessionID != job.SessionID || transcript.ChunkIndex != job.ChunkIndex || transcript.ChunkSHA256 != metadata.SHA256 {
					return nil, fmt.Errorf("invalid persisted transcript for ASR job")
				}
				if err := validateChunkMetadata(metadata, job.SessionID, metadataPath, s.maxChunkSize); err != nil {
					return nil, err
				}
				if err := s.verifyPersistedChunk(dir, metadata); err != nil {
					return nil, err
				}
				job.State = JobCompleted
				job.LastError = ""
				job.NextAttemptAt = nil
				job.UpdatedAt = now.UTC()
				if err := writeJSONAtomic(jobPath, job, 0o640); err != nil {
					return nil, err
				}
				if err := s.reconcileSessionStateLocked(job.SessionID, dir); err != nil {
					return nil, err
				}
				continue
			} else if !os.IsNotExist(transcriptErr) {
				return nil, transcriptErr
			}
			metadataPath := filepath.Join(dir, "chunks", fmt.Sprintf("%06d.json", job.ChunkIndex))
			metadata, err := loadChunkMetadata(metadataPath)
			if err != nil {
				return nil, err
			}
			if err := validateChunkMetadata(metadata, entry.Name(), metadataPath, s.maxChunkSize); err != nil {
				return nil, err
			}
			if err := s.verifyPersistedChunk(dir, metadata); err != nil {
				return nil, err
			}
			job.State = JobProcessing
			job.Attempts++
			job.NextAttemptAt = nil
			job.UpdatedAt = now.UTC()
			if err := writeJSONAtomic(jobPath, job, 0o640); err != nil {
				return nil, err
			}
			return &ClaimedJob{Job: job, Chunk: metadata, AudioPath: filepath.Join(dir, "chunks", fmt.Sprintf("%06d.%s", metadata.Index, metadata.Format))}, nil
		}
	}
	return nil, nil
}

func (s *Store) RetryJob(job ASRJob, next time.Time, message string) error {
	return s.updateJob(job, JobRetryWait, &next, message)
}

func (s *Store) DeadLetterJob(job ASRJob, message string) error {
	return s.updateJob(job, JobDead, nil, message)
}

func (s *Store) CompleteJob(claim ClaimedJob, transcript TranscriptChunk) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir, err := s.sessionDir(claim.Job.SessionID)
	if err != nil {
		return err
	}
	jobPath := filepath.Join(dir, "jobs", fmt.Sprintf("%06d.json", claim.Job.ChunkIndex))
	current, err := loadJob(jobPath)
	if err != nil {
		return err
	}
	if current.State != JobProcessing || current.Attempts != claim.Job.Attempts {
		return fmt.Errorf("ASR job is no longer claimed")
	}
	transcript.SchemaVersion = SchemaVersion
	transcript.SessionID = claim.Job.SessionID
	transcript.ChunkIndex = claim.Job.ChunkIndex
	transcript.ChunkSHA256 = claim.Chunk.SHA256
	if transcript.TranscribedAt.IsZero() {
		transcript.TranscribedAt = s.now().UTC()
	}
	chunkDir := filepath.Join(dir, "transcripts", "chunks")
	if err := ensureRealDirectory(chunkDir); err != nil {
		return err
	}
	transcriptPath := filepath.Join(chunkDir, fmt.Sprintf("%06d.json", transcript.ChunkIndex))
	if existing, err := loadTranscript(transcriptPath); err == nil {
		if existing.SessionID != transcript.SessionID || existing.ChunkIndex != transcript.ChunkIndex || existing.ChunkSHA256 != transcript.ChunkSHA256 {
			return fmt.Errorf("transcript conflict")
		}
		transcript = existing
	} else if !os.IsNotExist(err) {
		return err
	} else if err := writeJSONAtomic(transcriptPath, transcript, 0o640); err != nil {
		return err
	}
	current.State = JobCompleted
	current.LastError = ""
	current.NextAttemptAt = nil
	current.UpdatedAt = s.now().UTC()
	if err := writeJSONAtomic(jobPath, current, 0o640); err != nil {
		return err
	}
	return s.reconcileSessionStateLocked(claim.Job.SessionID, dir)
}

func (s *Store) GetRecoveryStatus(sessionID string, missingLimit int) (RecoveryStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if missingLimit <= 0 || missingLimit > 1000 {
		missingLimit = 1000
	}
	session, err := s.loadSessionLocked(sessionID)
	if err != nil {
		return RecoveryStatus{}, err
	}
	dir, _ := s.sessionDir(sessionID)
	status := RecoveryStatus{HighestChunk: -1, MissingChunks: make([]int, 0), Recoverable: true}
	seen := make(map[int]bool)
	metadataPaths, _ := filepath.Glob(filepath.Join(dir, "chunks", "*.json"))
	for _, metadataPath := range metadataPaths {
		metadata, err := loadChunkMetadata(metadataPath)
		if err != nil || validateChunkMetadata(metadata, sessionID, metadataPath, s.maxChunkSize) != nil || s.verifyPersistedChunk(dir, metadata) != nil {
			status.Recoverable = false
			continue
		}
		seen[metadata.Index] = true
		status.ReceivedCount++
		if metadata.Index > status.HighestChunk {
			status.HighestChunk = metadata.Index
		}
		if metadata.Final {
			status.FinalChunkReceived = true
		}
	}
	highest := status.HighestChunk
	if session.LastChunk > highest {
		highest = session.LastChunk
	}
	for index := 0; index <= highest; index++ {
		if seen[index] {
			continue
		}
		if len(status.MissingChunks) >= missingLimit {
			status.MissingTruncated = true
			break
		}
		status.MissingChunks = append(status.MissingChunks, index)
	}
	jobPaths, _ := filepath.Glob(filepath.Join(dir, "jobs", "*.json"))
	for _, jobPath := range jobPaths {
		job, err := loadJob(jobPath)
		if err != nil || validateJob(job, sessionID, jobPath) != nil {
			status.Recoverable = false
			continue
		}
		switch job.State {
		case JobPending:
			status.PendingJobs++
		case JobProcessing:
			status.ProcessingJobs++
		case JobRetryWait:
			status.RetryWaitJobs++
		case JobCompleted:
			status.CompletedJobs++
		case JobDead:
			status.DeadJobs++
		}
	}
	transcriptPaths, _ := filepath.Glob(filepath.Join(dir, "transcripts", "chunks", "*.json"))
	status.TranscriptChunks = len(transcriptPaths)
	return status, nil
}

func (s *Store) updateJob(job ASRJob, state JobState, next *time.Time, message string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir, err := s.sessionDir(job.SessionID)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, "jobs", fmt.Sprintf("%06d.json", job.ChunkIndex))
	current, err := loadJob(path)
	if err != nil {
		return err
	}
	if current.State != JobProcessing || current.Attempts != job.Attempts {
		return fmt.Errorf("ASR job is no longer claimed")
	}
	current.State = state
	current.NextAttemptAt = next
	if len(message) > 256 {
		message = message[:256]
	}
	current.LastError = message
	current.UpdatedAt = s.now().UTC()
	if err := writeJSONAtomic(path, current, 0o640); err != nil {
		return err
	}
	return s.reconcileSessionStateLocked(job.SessionID, dir)
}

func (s *Store) reconcileSessionStateLocked(sessionID, dir string) error {
	session, err := s.loadSessionLocked(sessionID)
	if err != nil {
		return err
	}
	if session.State != StateStopping && session.State != StateProcessing {
		return nil
	}
	jobs, _ := filepath.Glob(filepath.Join(dir, "jobs", "*.json"))
	active, dead := false, false
	for _, jobPath := range jobs {
		job, err := loadJob(jobPath)
		if err != nil {
			return err
		}
		switch job.State {
		case JobPending, JobProcessing, JobRetryWait:
			active = true
		case JobDead:
			dead = true
		}
	}
	if active {
		session.State = StateProcessing
	} else if dead {
		session.State = StateCompletedWithErrors
	} else {
		session.State = StateCompleted
	}
	if !active {
		if err := rebuildTranscriptProjections(dir); err != nil {
			return err
		}
		if err := s.projectRawTranscriptLocked(session.ID, dir); err != nil {
			return err
		}
	}
	session.UpdatedAt = s.now().UTC()
	return writeJSONAtomic(filepath.Join(dir, "session.json"), session, 0o640)
}

func loadTranscript(path string) (TranscriptChunk, error) {
	data, err := readBoundedRegularFile(path, 2<<20)
	if err != nil {
		return TranscriptChunk{}, err
	}
	var transcript TranscriptChunk
	if err := json.Unmarshal(data, &transcript); err != nil {
		return TranscriptChunk{}, err
	}
	return transcript, nil
}

func rebuildTranscriptProjections(dir string) error {
	paths, err := listTranscriptPaths(filepath.Join(dir, "transcripts", "chunks"))
	if err != nil {
		return err
	}
	sort.Strings(paths)
	transcriptsDir := filepath.Join(dir, "transcripts")
	jsonl, jsonlPath, err := createProjectionTemp(transcriptsDir)
	if err != nil {
		return err
	}
	defer os.Remove(jsonlPath)
	text, textPath, err := createProjectionTemp(transcriptsDir)
	if err != nil {
		jsonl.Close()
		return err
	}
	defer os.Remove(textPath)
	var total int64
	for _, transcriptPath := range paths {
		transcript, err := loadTranscript(transcriptPath)
		if err != nil {
			jsonl.Close()
			text.Close()
			return err
		}
		if err := validateTranscriptForProjection(dir, transcriptPath, transcript); err != nil {
			jsonl.Close()
			text.Close()
			return err
		}
		encoded, err := json.Marshal(transcript)
		if err != nil {
			jsonl.Close()
			text.Close()
			return err
		}
		total += int64(len(encoded) + len(transcript.Text) + 32)
		if total > 1<<30 {
			jsonl.Close()
			text.Close()
			return fmt.Errorf("transcript projections exceed 1 GiB")
		}
		if _, err := jsonl.Write(append(encoded, '\n')); err != nil {
			jsonl.Close()
			text.Close()
			return err
		}
		if _, err := fmt.Fprintf(text, "[%06d] %s\n", transcript.ChunkIndex, transcript.Text); err != nil {
			jsonl.Close()
			text.Close()
			return err
		}
	}
	if err := commitProjectionTemp(jsonl, jsonlPath, filepath.Join(transcriptsDir, "transcript.jsonl")); err != nil {
		text.Close()
		return err
	}
	return commitProjectionTemp(text, textPath, filepath.Join(transcriptsDir, "transcript.txt"))
}

func createProjectionTemp(dir string) (*os.File, string, error) {
	file, err := os.CreateTemp(dir, ".projection-*")
	if err != nil {
		return nil, "", err
	}
	if err := file.Chmod(0o640); err != nil {
		path := file.Name()
		file.Close()
		os.Remove(path)
		return nil, "", err
	}
	return file, file.Name(), nil
}

func commitProjectionTemp(file *os.File, tempPath, targetPath string) error {
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, targetPath); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(targetPath))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func listTranscriptPaths(dir string) ([]string, error) {
	directory, err := os.Open(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer directory.Close()
	paths := make([]string, 0, 128)
	seen := make(map[int]bool)
	for {
		entries, err := directory.ReadDir(256)
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
				continue
			}
			base := strings.TrimSuffix(entry.Name(), ".json")
			index, parseErr := strconv.Atoi(base)
			if parseErr != nil || entry.Name() != fmt.Sprintf("%06d.json", index) || index < 0 || index >= maxStoredChunkIndex || seen[index] {
				return nil, fmt.Errorf("invalid transcript chunk filename %s", entry.Name())
			}
			seen[index] = true
			if len(paths) >= maxStoredChunkIndex {
				return nil, fmt.Errorf("too many transcript chunks")
			}
			paths = append(paths, filepath.Join(dir, entry.Name()))
		}
		if err == io.EOF {
			return paths, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

func validateTranscriptForProjection(dir, transcriptPath string, transcript TranscriptChunk) error {
	base := strings.TrimSuffix(filepath.Base(transcriptPath), filepath.Ext(transcriptPath))
	index, err := strconv.Atoi(base)
	if err != nil || transcript.SchemaVersion != SchemaVersion || transcript.ChunkIndex != index || transcript.SessionID == "" || transcript.ChunkSHA256 == "" {
		return fmt.Errorf("invalid transcript chunk %s", transcriptPath)
	}
	sessionDir := filepath.Dir(filepath.Dir(filepath.Dir(transcriptPath)))
	if filepath.Clean(sessionDir) != filepath.Clean(dir) || filepath.Base(sessionDir) != transcript.SessionID {
		return fmt.Errorf("transcript session mismatch %s", transcriptPath)
	}
	metadataPath := filepath.Join(dir, "chunks", fmt.Sprintf("%06d.json", index))
	metadata, err := loadChunkMetadata(metadataPath)
	if err != nil {
		return err
	}
	if err := validateChunkMetadata(metadata, transcript.SessionID, metadataPath, 1<<30); err != nil {
		return err
	}
	if metadata.SHA256 != transcript.ChunkSHA256 {
		return fmt.Errorf("transcript audio hash mismatch")
	}
	actualHash, size, err := hashAudioFile(filepath.Join(dir, "chunks", fmt.Sprintf("%06d.%s", index, metadata.Format)), 1<<30)
	if err != nil || actualHash != metadata.SHA256 || size != metadata.Size {
		return fmt.Errorf("transcript audio integrity failure")
	}
	return nil
}

func readBoundedRegularFile(path string, limit int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() > limit {
		return nil, fmt.Errorf("invalid or oversized data file %s", path)
	}
	pathInfo, err := os.Lstat(path)
	if err != nil || pathInfo.Mode()&os.ModeSymlink != 0 || !os.SameFile(pathInfo, info) {
		return nil, fmt.Errorf("data file changed during open %s", path)
	}
	limited := &io.LimitedReader{R: file, N: limit + 1}
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("oversized data file %s", path)
	}
	return data, nil
}

func (s *Store) OpenClaimedAudio(claim ClaimedJob) (*os.File, error) {
	file, err := os.Open(claim.AudioPath)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Size() != claim.Chunk.Size || info.Size() > s.maxChunkSize {
		file.Close()
		return nil, ErrChunkConflict
	}
	hash := sha256.New()
	written, err := io.Copy(hash, io.LimitReader(file, s.maxChunkSize+1))
	if err != nil || written != claim.Chunk.Size || hex.EncodeToString(hash.Sum(nil)) != claim.Chunk.SHA256 {
		file.Close()
		return nil, ErrChunkConflict
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func (s *Store) IsSessionCancelled(sessionID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, err := s.loadSessionLocked(sessionID)
	if err != nil {
		return false, err
	}
	return session.State == StateCancelled, nil
}

func (s *Store) GetTranscriptPage(sessionID string, after, limit int) (TranscriptPage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if after < -1 || limit < 1 || limit > 500 {
		return TranscriptPage{}, fmt.Errorf("invalid transcript pagination")
	}
	if _, err := s.loadSessionLocked(sessionID); err != nil {
		return TranscriptPage{}, err
	}
	dir, _ := s.sessionDir(sessionID)
	paths, err := listTranscriptPaths(filepath.Join(dir, "transcripts", "chunks"))
	if err != nil {
		return TranscriptPage{}, err
	}
	sort.Strings(paths)
	page := TranscriptPage{Chunks: make([]TranscriptChunk, 0, limit)}
	const maxPageBytes = 8 << 20
	pageBytes := 0
	for _, transcriptPath := range paths {
		base := strings.TrimSuffix(filepath.Base(transcriptPath), ".json")
		index, _ := strconv.Atoi(base)
		if index <= after {
			continue
		}
		if len(page.Chunks) == limit {
			page.HasMore = true
			break
		}
		if len(page.Chunks) > 0 && index <= page.Chunks[len(page.Chunks)-1].ChunkIndex {
			return TranscriptPage{}, fmt.Errorf("transcript cursor order violation")
		}
		transcript, err := loadTranscript(transcriptPath)
		if err != nil {
			return TranscriptPage{}, err
		}
		if err := validateTranscriptForProjection(dir, transcriptPath, transcript); err != nil {
			return TranscriptPage{}, err
		}
		encodedTranscript, err := json.Marshal(transcript)
		if err != nil {
			return TranscriptPage{}, err
		}
		chunkBytes := len(encodedTranscript) + 1
		if pageBytes+chunkBytes > maxPageBytes {
			if len(page.Chunks) == 0 {
				return TranscriptPage{}, fmt.Errorf("transcript chunk exceeds page response limit")
			}
			page.HasMore = true
			break
		}
		pageBytes += chunkBytes
		page.Chunks = append(page.Chunks, transcript)
		page.NextAfter = transcript.ChunkIndex
	}
	for {
		encodedPage, err := json.Marshal(page)
		if err != nil {
			return TranscriptPage{}, err
		}
		if len(encodedPage) <= maxPageBytes {
			return page, nil
		}
		if len(page.Chunks) <= 1 {
			return TranscriptPage{}, fmt.Errorf("transcript chunk exceeds page response limit")
		}
		page.Chunks = page.Chunks[:len(page.Chunks)-1]
		page.NextAfter = page.Chunks[len(page.Chunks)-1].ChunkIndex
		page.HasMore = true
	}
}
