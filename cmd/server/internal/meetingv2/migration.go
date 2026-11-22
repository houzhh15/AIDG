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
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

const (
	migrationSchemaVersion = 1
	maxLegacyTaskFileSize  = 8 << 20
	maxLegacyTasks         = 10000
	maxLegacyEntries       = 10000
	maxMigrationErrorBytes = 2048
)

var ErrMeetingStoreInUse = errors.New("meeting data root is already in use")

// ServiceLock prevents two v2 server processes from writing the same file-backed
// meetings root. Legacy releases do not participate in this protocol, so an
// upgrade must stop the old process before starting the new one.
type ServiceLock struct {
	file *os.File
}

func AcquireServiceLock(root string) (*ServiceLock, error) {
	info, err := os.Lstat(root)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return nil, fmt.Errorf("invalid meetings root")
	}
	path := filepath.Join(root, ".meetingv2-service.lock")
	fd, err := unix.Open(path, unix.O_CREAT|unix.O_RDWR|unix.O_NOFOLLOW, 0o640)
	if err != nil {
		return nil, fmt.Errorf("open meeting service lock: %w", err)
	}
	file := os.NewFile(uintptr(fd), path)
	if file == nil {
		_ = unix.Close(fd)
		return nil, fmt.Errorf("open meeting service lock")
	}
	if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, ErrMeetingStoreInUse
		}
		return nil, fmt.Errorf("lock meeting data root: %w", err)
	}
	return &ServiceLock{file: file}, nil
}

func (lock *ServiceLock) Close() error {
	if lock == nil || lock.file == nil {
		return nil
	}
	file := lock.file
	lock.file = nil
	unlockErr := unix.Flock(int(file.Fd()), unix.LOCK_UN)
	closeErr := file.Close()
	if unlockErr != nil {
		return unlockErr
	}
	return closeErr
}

type MigrationItemStatus string

const (
	MigrationPending   MigrationItemStatus = "pending"
	MigrationCompleted MigrationItemStatus = "completed"
	MigrationFailed    MigrationItemStatus = "failed"
)

type MigrationItem struct {
	SourceID      string              `json:"source_id"`
	SourcePath    string              `json:"source_path"`
	SourceSHA256  string              `json:"source_sha256"`
	TargetSession string              `json:"target_session_id"`
	Status        MigrationItemStatus `json:"status"`
	Attempts      int                 `json:"attempts"`
	LastError     string              `json:"last_error,omitempty"`
	StartedAt     time.Time           `json:"started_at,omitempty"`
	CompletedAt   *time.Time          `json:"completed_at,omitempty"`
	UpdatedAt     time.Time           `json:"updated_at"`
}

type MigrationState struct {
	SchemaVersion int                       `json:"schema_version"`
	UpdatedAt     time.Time                 `json:"updated_at"`
	Items         map[string]*MigrationItem `json:"items"`
}

type MigrationResult struct {
	Discovered int `json:"discovered"`
	Migrated   int `json:"migrated"`
	Skipped    int `json:"skipped"`
	Failed     int `json:"failed"`
}

type legacyTaskRecord struct {
	ID        string          `json:"id"`
	Config    json.RawMessage `json:"config"`
	State     string          `json:"state,omitempty"`
	CreatedAt time.Time       `json:"created_at,omitempty"`
}

type legacyTaskConfig struct {
	OutputDir       string `json:"OutputDir"`
	OutputDirLegacy string `json:"output_dir"`
}

func (c legacyTaskConfig) outputDir() string {
	if strings.TrimSpace(c.OutputDir) != "" {
		return strings.TrimSpace(c.OutputDir)
	}
	return strings.TrimSpace(c.OutputDirLegacy)
}

type migrationCandidate struct {
	SourceID     string
	TargetID     string
	StateKey     string
	Dir          string
	CreatedAt    time.Time
	TaskJSON     []byte
	DiscoveryErr error
}

type legacyArtifact struct {
	Kind DocumentKind
	Name string
}

var legacyDocumentArtifacts = []legacyArtifact{
	{Kind: DocumentRawTranscript, Name: "merged_all.txt"},
	{Kind: DocumentPolishedTranscript, Name: "polish_all.md"},
	{Kind: DocumentMeetingSummary, Name: "meeting-summary.md"},
	{Kind: DocumentMeetingSummary, Name: "meeting_summary.md"},
}

var legacyAudioNames = []string{
	"complete_audio.webm", "complete_audio.ogg", "complete_audio.wav", "complete_audio.mp3", "complete_audio.m4a",
	"recording.webm", "recording.ogg", "recording.wav", "recording.mp3", "recording.m4a",
	"audio.webm", "audio.ogg", "audio.wav", "audio.mp3", "audio.m4a",
}

// MigrateLegacy imports legacy tasks and meeting artifacts without moving or deleting source files.
// Per-item failures are persisted and do not abort other candidates.
func (s *Store) MigrateLegacy() (MigrationResult, error) {
	lock, err := acquireMigrationLock(filepath.Join(s.root, ".meetingv2-migration.lock"))
	if err != nil {
		return MigrationResult{}, fmt.Errorf("acquire legacy migration lock: %w", err)
	}
	defer releaseMigrationLock(lock)

	state, err := s.loadMigrationState()
	if err != nil {
		return MigrationResult{}, err
	}
	candidates, err := s.discoverLegacyCandidates()
	if err != nil {
		return MigrationResult{}, err
	}
	result := MigrationResult{Discovered: len(candidates)}
	for _, candidate := range candidates {
		fingerprint, fingerprintErr := s.legacyCandidateFingerprint(candidate)
		item := state.Items[candidate.StateKey]
		wasCompleted := item != nil && item.Status == MigrationCompleted && item.SourceSHA256 == fingerprint && fingerprintErr == nil
		now := s.now().UTC()
		if item == nil {
			item = &MigrationItem{SourceID: candidate.SourceID, TargetSession: candidate.TargetID}
			state.Items[candidate.StateKey] = item
		}
		item.SourcePath = candidate.SourceID
		item.SourceSHA256 = fingerprint
		item.Status = MigrationPending
		if !wasCompleted {
			item.Attempts++
		}
		item.StartedAt = now
		item.CompletedAt = nil
		item.LastError = ""
		item.UpdatedAt = now
		if err := s.saveMigrationState(state); err != nil {
			return result, err
		}
		migrationErr := candidate.DiscoveryErr
		if migrationErr == nil {
			migrationErr = fingerprintErr
		}
		if migrationErr == nil {
			migrationErr = s.migrateLegacyCandidate(candidate, fingerprint)
		}
		now = s.now().UTC()
		item.UpdatedAt = now
		if migrationErr != nil {
			item.Status = MigrationFailed
			item.LastError = boundedMigrationError(migrationErr)
			result.Failed++
		} else {
			item.Status = MigrationCompleted
			item.CompletedAt = &now
			if wasCompleted {
				result.Skipped++
			} else {
				result.Migrated++
			}
		}
		if err := s.saveMigrationState(state); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (s *Store) discoverLegacyCandidates() ([]migrationCandidate, error) {
	candidates := map[string]migrationCandidate{}
	tasksPath := filepath.Join(s.root, "server_tasks.json")
	tasks, err := readLegacyTasks(tasksPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read legacy tasks: %w", err)
	}
	for _, task := range tasks {
		targetID, stateKey, idErr := migrationIdentifiers(task.ID)
		encoded, marshalErr := json.Marshal(task)
		if marshalErr != nil {
			return nil, marshalErr
		}
		taskDir, pathErr := resolveLegacyTaskDir(s.root, task)
		if pathErr == nil {
			pathErr = idErr
		}
		if taskDir == "" {
			taskDir = filepath.Join(s.root, targetID)
		}
		candidates[stateKey] = migrationCandidate{SourceID: task.ID, TargetID: targetID, StateKey: stateKey, Dir: taskDir, CreatedAt: task.CreatedAt, TaskJSON: encoded, DiscoveryErr: pathErr}
	}
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return nil, fmt.Errorf("list legacy meetings: %w", err)
	}
	if len(entries) > maxLegacyEntries {
		return nil, fmt.Errorf("too many legacy meeting entries")
	}
	for _, entry := range entries {
		if !entry.IsDir() || !validMigrationID(entry.Name()) {
			continue
		}
		dir := filepath.Join(s.root, entry.Name())
		info, statErr := os.Lstat(dir)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			continue
		}
		if _, exists := candidates[entry.Name()]; exists || !hasLegacyArtifact(dir) {
			continue
		}
		candidates[entry.Name()] = migrationCandidate{SourceID: entry.Name(), TargetID: entry.Name(), StateKey: entry.Name(), Dir: dir}
	}
	result := make([]migrationCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		result = append(result, candidate)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].StateKey < result[j].StateKey })
	return result, nil
}

func readLegacyTasks(path string) ([]legacyTaskRecord, error) {
	data, err := readBoundedRegularFile(path, maxLegacyTaskFileSize)
	if err != nil {
		return nil, err
	}
	var wrapper struct {
		Tasks []legacyTaskRecord `json:"tasks"`
	}
	if err := json.Unmarshal(data, &wrapper); err != nil {
		return nil, err
	}
	if len(wrapper.Tasks) > maxLegacyTasks {
		return nil, fmt.Errorf("too many legacy tasks")
	}
	seen := map[string]bool{}
	for _, task := range wrapper.Tasks {
		if seen[task.ID] {
			return nil, fmt.Errorf("duplicate legacy task id")
		}
		seen[task.ID] = true
	}
	return wrapper.Tasks, nil
}

func validMigrationID(id string) bool {
	return safeSessionID.MatchString(id) && id != "." && id != ".." && !strings.HasPrefix(id, ".")
}

func migrationIdentifiers(sourceID string) (targetID, stateKey string, err error) {
	sum := sha256.Sum256([]byte(sourceID))
	hashedID := "legacy-" + hex.EncodeToString(sum[:16])
	if sourceID == "" || strings.HasPrefix(sourceID, ".") || strings.ContainsAny(sourceID, "/\\\x00") || !utf8.ValidString(sourceID) || len(sourceID) > 1024 || filepath.Base(filepath.Clean(sourceID)) != sourceID {
		return hashedID, hashedID, fmt.Errorf("invalid legacy task id")
	}
	if validMigrationID(sourceID) {
		return sourceID, sourceID, nil
	}
	return hashedID, hashedID, nil
}

func hasLegacyArtifact(dir string) bool {
	for _, artifact := range legacyDocumentArtifacts {
		if regularLegacyFile(filepath.Join(dir, artifact.Name), maxDocumentBytes) {
			return true
		}
	}
	for _, name := range legacyAudioNames {
		if regularLegacyFile(filepath.Join(dir, name), 1<<62) {
			return true
		}
	}
	return false
}

func resolveLegacyTaskDir(root string, task legacyTaskRecord) (string, error) {
	fallback := filepath.Join(root, task.ID)
	if !pathWithinRoot(root, fallback) {
		return "", fmt.Errorf("legacy task directory escapes meetings root")
	}
	if len(task.Config) == 0 || string(task.Config) == "null" {
		return fallback, nil
	}
	var config legacyTaskConfig
	if err := json.Unmarshal(task.Config, &config); err != nil {
		return "", fmt.Errorf("invalid legacy task config: %w", err)
	}
	configured := config.outputDir()
	if configured == "" {
		return fallback, nil
	}
	clean := filepath.Clean(configured)
	if filepath.Base(clean) != task.ID {
		return "", fmt.Errorf("legacy output directory does not match task id")
	}
	if filepath.IsAbs(clean) {
		// Historical host/container roots are never used directly. Only the
		// already-validated task-id basename is mapped into the current root.
		return fallback, nil
	}
	for _, component := range strings.Split(filepath.ToSlash(clean), "/") {
		if component == ".." {
			return "", fmt.Errorf("legacy output directory escapes meetings root")
		}
	}
	// Legacy releases used task-id, tasks/task-id and data/meetings/task-id.
	// Other prefixes are harmless because only the basename is retained.
	return fallback, nil
}

func pathWithinRoot(root, path string) bool {
	relative, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func regularLegacyFile(path string, maxSize int64) bool {
	info, err := os.Lstat(path)
	return err == nil && info.Mode()&os.ModeSymlink == 0 && info.Mode().IsRegular() && info.Size() >= 0 && info.Size() <= maxSize
}

func (s *Store) legacyCandidateFingerprint(candidate migrationCandidate) (string, error) {
	if err := validateLegacyDirectory(s.root, candidate.Dir); err != nil {
		return "", err
	}
	hash := sha256.New()
	_, _ = hash.Write([]byte("task\x00"))
	_, _ = hash.Write(candidate.TaskJSON)
	seenKinds := map[DocumentKind]bool{}
	for _, artifact := range legacyDocumentArtifacts {
		if seenKinds[artifact.Kind] {
			continue
		}
		path := filepath.Join(candidate.Dir, artifact.Name)
		data, err := readBoundedRegularFile(path, maxDocumentBytes)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("fingerprint %s: %w", artifact.Name, err)
		}
		seenKinds[artifact.Kind] = true
		_, _ = hash.Write([]byte("\x00" + artifact.Name + "\x00"))
		_, _ = hash.Write(data)
	}
	if audioPath, _, err := s.findLegacyAudio(candidate.Dir); err != nil {
		return "", err
	} else if audioPath != "" {
		audioHash, size, err := hashAudioFile(audioPath, s.maxChunkSize)
		if err != nil {
			return "", err
		}
		_, _ = hash.Write([]byte(fmt.Sprintf("\x00audio\x00%s\x00%d", audioHash, size)))
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (s *Store) migrateLegacyCandidate(candidate migrationCandidate, fingerprint string) error {
	session, err := s.LoadSession(candidate.TargetID)
	if errors.Is(err, ErrSessionNotFound) {
		metadata := map[string]string{"task_id": candidate.SourceID, "migration_source": "legacy", "migration_fingerprint": fingerprint, "legacy_source_id": candidate.SourceID}
		if !candidate.CreatedAt.IsZero() {
			metadata["legacy_created_at"] = candidate.CreatedAt.UTC().Format(time.RFC3339Nano)
		}
		session, err = s.CreateOwnedSessionWithMode(candidate.TargetID, candidate.TargetID, "", metadata, TranscriptionManual)
	}
	if err != nil {
		return err
	}
	if session.ID != candidate.TargetID || session.Metadata["task_id"] != candidate.SourceID || session.Metadata["migration_source"] != "legacy" {
		return fmt.Errorf("target session conflicts with existing non-migrated session")
	}
	if err := s.ensureSessionDirs(candidate.Dir); err != nil {
		return err
	}
	seenKinds := map[DocumentKind]bool{}
	for _, artifact := range legacyDocumentArtifacts {
		if seenKinds[artifact.Kind] {
			continue
		}
		path := filepath.Join(candidate.Dir, artifact.Name)
		content, readErr := readBoundedRegularFile(path, maxDocumentBytes)
		if errors.Is(readErr, os.ErrNotExist) {
			continue
		}
		if readErr != nil {
			return fmt.Errorf("read %s: %w", artifact.Name, readErr)
		}
		if len(content) == 0 || !utf8.Valid(content) || strings.IndexByte(string(content), 0) >= 0 {
			return fmt.Errorf("invalid legacy document %s", artifact.Name)
		}
		seenKinds[artifact.Kind] = true
		contentHash := sha256.Sum256(content)
		_, _, putErr := s.PutDocumentLatest(candidate.TargetID, PutDocumentRequest{
			Kind: artifact.Kind, Source: DocumentSourceGenerated, Content: string(content),
			IdempotencyKey: "legacy:" + string(artifact.Kind) + ":" + hex.EncodeToString(contentHash[:16]), CreatedBy: "legacy-migration",
		})
		if putErr != nil {
			return fmt.Errorf("import %s: %w", artifact.Name, putErr)
		}
	}
	return s.migrateLegacyAudio(candidate, session)
}

func (s *Store) migrateLegacyAudio(candidate migrationCandidate, session Session) error {
	path, format, err := s.findLegacyAudio(candidate.Dir)
	if err != nil || path == "" {
		return err
	}
	content, err := readBoundedRegularFile(path, s.maxChunkSize)
	if err != nil || len(content) == 0 {
		if err == nil {
			err = fmt.Errorf("empty legacy audio")
		}
		return err
	}
	if !validLegacyAudioSignature(format, content) {
		return fmt.Errorf("legacy audio signature does not match %s", format)
	}
	sum := sha256.Sum256(content)
	contentHash := hex.EncodeToString(sum[:])
	_, _, err = s.ImportAudio(candidate.TargetID, IngestChunkRequest{Index: 0, ClientChunkID: "legacy:" + contentHash[:32], StartedAt: session.CreatedAt,
		Format: format, DeclaredSHA256: contentHash, Final: true}, bytes.NewReader(content))
	if errors.Is(err, ErrChunkConflict) {
		return fmt.Errorf("legacy audio conflicts with existing session audio: %w", err)
	}
	return err
}

func (s *Store) findLegacyAudio(dir string) (string, string, error) {
	for _, name := range legacyAudioNames {
		path := filepath.Join(dir, name)
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", "", err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > s.maxChunkSize {
			return "", "", fmt.Errorf("invalid legacy audio %s", name)
		}
		return path, strings.TrimPrefix(filepath.Ext(name), "."), nil
	}
	return "", "", nil
}
func validLegacyAudioSignature(format string, content []byte) bool {
	switch format {
	case "webm":
		return len(content) >= 4 && bytes.Equal(content[:4], []byte{0x1a, 0x45, 0xdf, 0xa3})
	case "ogg":
		return len(content) >= 4 && bytes.Equal(content[:4], []byte("OggS"))
	case "wav":
		return len(content) >= 12 && bytes.Equal(content[:4], []byte("RIFF")) && bytes.Equal(content[8:12], []byte("WAVE"))
	case "mp3":
		return len(content) >= 3 && (bytes.Equal(content[:3], []byte("ID3")) || (content[0] == 0xff && content[1]&0xe0 == 0xe0))
	case "m4a":
		return len(content) >= 12 && bytes.Equal(content[4:8], []byte("ftyp"))
	default:
		return false
	}
}

func validateLegacyDirectory(root, dir string) error {
	if !pathWithinRoot(root, dir) {
		return fmt.Errorf("legacy directory escapes meetings root")
	}
	rootInfo, err := os.Lstat(root)
	if err != nil || rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return fmt.Errorf("invalid meetings root")
	}
	relative, err := filepath.Rel(root, dir)
	if err != nil {
		return err
	}
	current := root
	for _, component := range strings.Split(relative, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("legacy directory contains symlink or non-directory component")
		}
	}
	return nil
}

func (s *Store) loadMigrationState() (*MigrationState, error) {
	path := filepath.Join(s.root, "migration-state.json")
	data, err := readBoundedRegularFile(path, maxLegacyTaskFileSize)
	if errors.Is(err, os.ErrNotExist) {
		return &MigrationState{SchemaVersion: migrationSchemaVersion, Items: map[string]*MigrationItem{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read migration state: %w", err)
	}
	var state MigrationState
	if err := json.Unmarshal(data, &state); err != nil || state.SchemaVersion != migrationSchemaVersion || state.Items == nil || len(state.Items) > maxLegacyTasks {
		return nil, fmt.Errorf("invalid migration state")
	}
	for id, item := range state.Items {
		if !validMigrationID(id) || item == nil || item.TargetSession != id || item.Attempts < 0 || item.SourceID == "" || item.SourcePath != item.SourceID {
			return nil, fmt.Errorf("invalid migration state item")
		}
		if item.Status != MigrationPending && item.Status != MigrationCompleted && item.Status != MigrationFailed {
			return nil, fmt.Errorf("invalid migration state status")
		}
		if item.SourceSHA256 != "" {
			decoded, decodeErr := hex.DecodeString(item.SourceSHA256)
			if decodeErr != nil || len(decoded) != sha256.Size {
				return nil, fmt.Errorf("invalid migration state fingerprint")
			}
		}
		if item.Status == MigrationCompleted && (item.SourceSHA256 == "" || item.CompletedAt == nil || item.LastError != "") {
			return nil, fmt.Errorf("invalid completed migration state")
		}
		if item.Status == MigrationFailed && item.LastError == "" {
			return nil, fmt.Errorf("invalid failed migration state")
		}
		if item.Status == MigrationPending && item.CompletedAt != nil {
			return nil, fmt.Errorf("invalid pending migration state")
		}
	}
	return &state, nil
}

func (s *Store) saveMigrationState(state *MigrationState) error {
	state.SchemaVersion = migrationSchemaVersion
	state.UpdatedAt = s.now().UTC()
	return writeJSONAtomic(filepath.Join(s.root, "migration-state.json"), state, 0o640)
}

func boundedMigrationError(err error) string {
	message := strings.ReplaceAll(err.Error(), "\x00", "")
	if len(message) > maxMigrationErrorBytes {
		message = message[:maxMigrationErrorBytes]
	}
	return message
}

func acquireMigrationLock(path string) (*os.File, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o640)
	if err != nil {
		return nil, err
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func releaseMigrationLock(file *os.File) {
	if file == nil {
		return
	}
	_ = unix.Flock(int(file.Fd()), unix.LOCK_UN)
	_ = file.Close()
}
