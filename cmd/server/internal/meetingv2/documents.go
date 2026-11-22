package meetingv2

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type DocumentKind string
type DocumentSource string

const (
	DocumentRawTranscript      DocumentKind = "raw_transcript"
	DocumentPolishedTranscript DocumentKind = "polished_transcript"
	DocumentMeetingSummary     DocumentKind = "meeting_summary"

	DocumentSourceGenerated DocumentSource = "generated"
	DocumentSourceUser      DocumentSource = "user"
	DocumentSourceExternal  DocumentSource = "external"

	maxDocumentBytes     = 2 << 20
	maxDocumentRevisions = 10000
)

var (
	ErrInvalidDocument          = errors.New("invalid meeting document")
	ErrDocumentNotFound         = errors.New("meeting document not found")
	ErrDocumentConflict         = errors.New("meeting document conflict")
	ErrDocumentRevisionConflict = errors.New("meeting document revision conflict")
	ErrAmbiguousTaskSession     = errors.New("multiple meeting sessions match task")
)

type DocumentRevision struct {
	SchemaVersion  int            `json:"schema_version"`
	SessionID      string         `json:"session_id"`
	Kind           DocumentKind   `json:"kind"`
	Revision       int            `json:"revision"`
	Source         DocumentSource `json:"source"`
	Content        string         `json:"content"`
	ContentSHA256  string         `json:"content_sha256"`
	IdempotencyKey string         `json:"idempotency_key"`
	CreatedBy      string         `json:"created_by"`
	CreatedAt      time.Time      `json:"created_at"`
	Active         bool           `json:"active"`
}

type PutDocumentRequest struct {
	Kind             DocumentKind
	Source           DocumentSource
	Content          string
	ExpectedRevision int
	IdempotencyKey   string
	CreatedBy        string
}

type documentPointer struct {
	SchemaVersion int          `json:"schema_version"`
	SessionID     string       `json:"session_id"`
	Kind          DocumentKind `json:"kind"`
	Revision      int          `json:"revision"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

func (s *Store) PutDocument(sessionID string, request PutDocumentRequest) (DocumentRevision, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.putDocumentLocked(sessionID, request)
}

func (s *Store) PutDocumentLatest(sessionID string, request PutDocumentRequest) (DocumentRevision, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.loadSessionLocked(sessionID); err != nil {
		return DocumentRevision{}, false, err
	}
	active, err := s.getActiveDocumentLocked(sessionID, request.Kind)
	if err == nil {
		request.ExpectedRevision = active.Revision
	} else if errors.Is(err, ErrDocumentNotFound) {
		request.ExpectedRevision = 0
	} else {
		return DocumentRevision{}, false, err
	}
	return s.putDocumentLocked(sessionID, request)
}

func (s *Store) GetActiveDocument(sessionID string, kind DocumentKind) (DocumentRevision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.loadSessionLocked(sessionID); err != nil {
		return DocumentRevision{}, err
	}
	return s.getActiveDocumentLocked(sessionID, kind)
}

func (s *Store) reconcileDocumentArtifactsLocked(sessionID string) error {
	for _, kind := range []DocumentKind{DocumentRawTranscript, DocumentPolishedTranscript, DocumentMeetingSummary} {
		active, err := s.getActiveDocumentLocked(sessionID, kind)
		if errors.Is(err, ErrDocumentNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		dir, _ := s.sessionDir(sessionID)
		if err := validateLegacyDocumentArtifactTarget(dir, kind); err != nil {
			return err
		}
		if err := writeLegacyDocumentArtifact(dir, kind, active.Content); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) FindSessionByTaskID(taskID string) (Session, error) {
	if !safeSessionID.MatchString(taskID) || taskID == "." || taskID == ".." {
		return Session{}, ErrInvalidSessionID
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return Session{}, err
	}
	var match *Session
	for _, entry := range entries {
		if !entry.IsDir() || !safeSessionID.MatchString(entry.Name()) {
			continue
		}
		session, loadErr := s.loadSessionLocked(entry.Name())
		if loadErr != nil {
			if entry.Name() == taskID {
				return Session{}, loadErr
			}
			continue
		}
		if session.ID != taskID && session.Metadata["task_id"] != taskID {
			continue
		}
		if match != nil && match.ID != session.ID {
			return Session{}, ErrAmbiguousTaskSession
		}
		copy := session
		match = &copy
	}
	if match == nil {
		return Session{}, ErrSessionNotFound
	}
	return *match, nil
}

func (s *Store) putDocumentLocked(sessionID string, request PutDocumentRequest) (DocumentRevision, bool, error) {
	if !validDocumentRequest(request) {
		return DocumentRevision{}, false, ErrInvalidDocument
	}
	if _, err := s.loadSessionLocked(sessionID); err != nil {
		return DocumentRevision{}, false, err
	}
	dir, _ := s.sessionDir(sessionID)
	kindDir := filepath.Join(dir, "documents", string(request.Kind))
	revisionsDir := filepath.Join(kindDir, "revisions")
	if err := ensureRealDirectory(kindDir); err != nil {
		return DocumentRevision{}, false, err
	}
	if err := ensureRealDirectory(revisionsDir); err != nil {
		return DocumentRevision{}, false, err
	}
	entries, err := os.ReadDir(revisionsDir)
	if err != nil {
		return DocumentRevision{}, false, err
	}
	hash := sha256.Sum256([]byte(request.Content))
	encodedHash := hex.EncodeToString(hash[:])
	latestRevision := 0
	count := 0
	for _, entry := range entries {
		if isAtomicTempEntry(entry) {
			continue
		}
		revision, ok := documentRevisionFilename(entry)
		if !ok {
			return DocumentRevision{}, false, fmt.Errorf("invalid document revision entry")
		}
		count++
		if count > maxDocumentRevisions {
			return DocumentRevision{}, false, fmt.Errorf("too many document revisions")
		}
		item, loadErr := loadDocumentRevision(filepath.Join(revisionsDir, entry.Name()), sessionID, request.Kind, revision)
		if loadErr != nil {
			return DocumentRevision{}, false, loadErr
		}
		if item.IdempotencyKey == request.IdempotencyKey {
			if item.ContentSHA256 == encodedHash && item.Content == request.Content && item.Source == request.Source {
				active, activeErr := s.getActiveDocumentLocked(sessionID, request.Kind)
				item.Active = activeErr == nil && active.Revision == item.Revision
				return item, true, nil
			}
			return DocumentRevision{}, false, ErrDocumentConflict
		}
		if revision > latestRevision {
			latestRevision = revision
		}
	}
	activeRevision := 0
	if active, activeErr := s.getActiveDocumentLocked(sessionID, request.Kind); activeErr == nil {
		activeRevision = active.Revision
	} else if !errors.Is(activeErr, ErrDocumentNotFound) {
		return DocumentRevision{}, false, activeErr
	}
	if request.ExpectedRevision != activeRevision {
		return DocumentRevision{}, false, ErrDocumentRevisionConflict
	}
	if latestRevision >= maxDocumentRevisions {
		return DocumentRevision{}, false, fmt.Errorf("document revision limit reached")
	}
	item := DocumentRevision{SchemaVersion: SchemaVersion, SessionID: sessionID, Kind: request.Kind,
		Revision: latestRevision + 1, Source: request.Source, Content: request.Content, ContentSHA256: encodedHash,
		IdempotencyKey: request.IdempotencyKey, CreatedBy: request.CreatedBy, CreatedAt: s.now().UTC()}
	path := filepath.Join(revisionsDir, fmt.Sprintf("%06d.json", item.Revision))
	if err := writeJSONAtomic(path, item, 0o640); err != nil {
		return DocumentRevision{}, false, err
	}
	activate := request.Source != DocumentSourceGenerated
	if !activate {
		active, activeErr := s.getActiveDocumentLocked(sessionID, request.Kind)
		activate = errors.Is(activeErr, ErrDocumentNotFound) || (activeErr == nil && active.Source == DocumentSourceGenerated)
		if activeErr != nil && !errors.Is(activeErr, ErrDocumentNotFound) {
			return DocumentRevision{}, false, activeErr
		}
	}
	if activate {
		if err := validateLegacyDocumentArtifactTarget(dir, request.Kind); err != nil {
			return DocumentRevision{}, false, err
		}
		previous, previousErr := s.getActiveDocumentLocked(sessionID, request.Kind)
		if previousErr != nil && !errors.Is(previousErr, ErrDocumentNotFound) {
			return DocumentRevision{}, false, previousErr
		}
		if err := writeLegacyDocumentArtifact(dir, request.Kind, request.Content); err != nil {
			return DocumentRevision{}, false, err
		}
		pointer := documentPointer{SchemaVersion: SchemaVersion, SessionID: sessionID, Kind: request.Kind, Revision: item.Revision, UpdatedAt: s.now().UTC()}
		if err := writeJSONAtomic(filepath.Join(kindDir, "active.json"), pointer, 0o640); err != nil {
			var rollbackErr error
			if previousErr == nil {
				rollbackErr = writeLegacyDocumentArtifact(dir, request.Kind, previous.Content)
			} else {
				rollbackErr = removeLegacyDocumentArtifact(dir, request.Kind)
			}
			if rollbackErr != nil {
				return DocumentRevision{}, false, fmt.Errorf("commit active pointer: %w; rollback legacy artifact: %v", err, rollbackErr)
			}
			return DocumentRevision{}, false, err
		}
		item.Active = true
	}
	return item, false, nil
}

func (s *Store) getActiveDocumentLocked(sessionID string, kind DocumentKind) (DocumentRevision, error) {
	if !validDocumentKind(kind) {
		return DocumentRevision{}, ErrInvalidDocument
	}
	dir, _ := s.sessionDir(sessionID)
	kindDir := filepath.Join(dir, "documents", string(kind))
	data, err := readBoundedRegularFile(filepath.Join(kindDir, "active.json"), 64<<10)
	if os.IsNotExist(err) {
		return DocumentRevision{}, ErrDocumentNotFound
	}
	if err != nil {
		return DocumentRevision{}, err
	}
	var pointer documentPointer
	if err := json.Unmarshal(data, &pointer); err != nil || pointer.SchemaVersion != SchemaVersion || pointer.SessionID != sessionID || pointer.Kind != kind || pointer.Revision < 1 || pointer.Revision > maxDocumentRevisions || pointer.UpdatedAt.IsZero() {
		return DocumentRevision{}, fmt.Errorf("invalid active document pointer")
	}
	item, err := loadDocumentRevision(filepath.Join(kindDir, "revisions", fmt.Sprintf("%06d.json", pointer.Revision)), sessionID, kind, pointer.Revision)
	if err != nil {
		return DocumentRevision{}, err
	}
	item.Active = true
	return item, nil
}

func validDocumentRequest(request PutDocumentRequest) bool {
	return validDocumentKind(request.Kind) && validDocumentSource(request.Source) && request.Content != "" && strings.TrimSpace(request.Content) != "" &&
		len([]byte(request.Content)) <= maxDocumentBytes && utf8.ValidString(request.Content) && !strings.ContainsRune(request.Content, '\x00') &&
		request.ExpectedRevision >= 0 && request.ExpectedRevision <= maxDocumentRevisions && safeIdempotencyKey.MatchString(request.IdempotencyKey) &&
		request.CreatedBy != "" && len(request.CreatedBy) <= 256 && utf8.ValidString(request.CreatedBy)
}

func validDocumentKind(kind DocumentKind) bool {
	return kind == DocumentRawTranscript || kind == DocumentPolishedTranscript || kind == DocumentMeetingSummary
}

func validDocumentSource(source DocumentSource) bool {
	return source == DocumentSourceGenerated || source == DocumentSourceUser || source == DocumentSourceExternal
}

func documentRevisionFilename(entry os.DirEntry) (int, bool) {
	if entry.IsDir() || len(entry.Name()) != 11 || !strings.HasSuffix(entry.Name(), ".json") {
		return 0, false
	}
	revision, err := strconv.Atoi(strings.TrimSuffix(entry.Name(), ".json"))
	return revision, err == nil && revision >= 1 && revision <= maxDocumentRevisions
}

func loadDocumentRevision(path, sessionID string, kind DocumentKind, revision int) (DocumentRevision, error) {
	data, err := readBoundedRegularFile(path, maxDocumentBytes+(64<<10))
	if err != nil {
		return DocumentRevision{}, err
	}
	var item DocumentRevision
	if err := json.Unmarshal(data, &item); err != nil {
		return DocumentRevision{}, err
	}
	hash := sha256.Sum256([]byte(item.Content))
	if item.SchemaVersion != SchemaVersion || item.SessionID != sessionID || item.Kind != kind || item.Revision != revision || item.CreatedAt.IsZero() ||
		item.ContentSHA256 != hex.EncodeToString(hash[:]) || !validDocumentRequest(PutDocumentRequest{Kind: item.Kind, Source: item.Source, Content: item.Content, ExpectedRevision: revision - 1, IdempotencyKey: item.IdempotencyKey, CreatedBy: item.CreatedBy}) {
		return DocumentRevision{}, fmt.Errorf("invalid document revision")
	}
	return item, nil
}

func writeLegacyDocumentArtifact(sessionDir string, kind DocumentKind, content string) error {
	name := map[DocumentKind]string{DocumentRawTranscript: "merged_all.txt", DocumentPolishedTranscript: "polish_all.md", DocumentMeetingSummary: "meeting-summary.md"}[kind]
	if name == "" {
		return ErrInvalidDocument
	}
	return writeFileAtomic(filepath.Join(sessionDir, name), []byte(content), 0o640)
}

func removeLegacyDocumentArtifact(sessionDir string, kind DocumentKind) error {
	name := map[DocumentKind]string{DocumentRawTranscript: "merged_all.txt", DocumentPolishedTranscript: "polish_all.md", DocumentMeetingSummary: "meeting-summary.md"}[kind]
	if name == "" {
		return ErrInvalidDocument
	}
	path := filepath.Join(sessionDir, name)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	dir, err := os.Open(sessionDir)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

func validateLegacyDocumentArtifactTarget(sessionDir string, kind DocumentKind) error {
	name := map[DocumentKind]string{DocumentRawTranscript: "merged_all.txt", DocumentPolishedTranscript: "polish_all.md", DocumentMeetingSummary: "meeting-summary.md"}[kind]
	if name == "" {
		return ErrInvalidDocument
	}
	path := filepath.Join(sessionDir, name)
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("invalid legacy document artifact target")
	}
	return nil
}

func (s *Store) projectRawTranscriptLocked(sessionID, dir string) error {
	paths, err := listTranscriptPaths(filepath.Join(dir, "transcripts", "chunks"))
	if err != nil {
		return err
	}
	sort.Strings(paths)
	var content strings.Builder
	hasher := sha256.New()
	for _, path := range paths {
		item, err := loadTranscript(path)
		if err != nil {
			return err
		}
		if err := validateTranscriptForProjection(dir, path, item); err != nil || item.SessionID != sessionID {
			return fmt.Errorf("invalid transcript projection input")
		}
		text := strings.TrimSpace(item.Text)
		additional := len(text)
		if content.Len() > 0 && text != "" {
			additional++
		}
		if content.Len()+additional > maxDocumentBytes {
			return fmt.Errorf("raw transcript document exceeds limit")
		}
		if content.Len() > 0 && text != "" {
			content.WriteByte('\n')
		}
		content.WriteString(text)
		hasher.Write([]byte(item.ChunkSHA256))
		hasher.Write([]byte{0})
	}
	rawContent := strings.TrimSpace(content.String())
	if rawContent == "" {
		return nil
	}
	keyHash := hex.EncodeToString(hasher.Sum(nil))
	activeRevision := 0
	if active, activeErr := s.getActiveDocumentLocked(sessionID, DocumentRawTranscript); activeErr == nil {
		activeRevision = active.Revision
	} else if !errors.Is(activeErr, ErrDocumentNotFound) {
		return activeErr
	}
	_, _, err = s.putDocumentLocked(sessionID, PutDocumentRequest{Kind: DocumentRawTranscript, Source: DocumentSourceGenerated,
		Content: rawContent, ExpectedRevision: activeRevision, IdempotencyKey: "asr-" + keyHash, CreatedBy: "meetingv2-asr"})
	return err
}
