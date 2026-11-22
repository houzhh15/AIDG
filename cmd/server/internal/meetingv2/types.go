package meetingv2

import "time"

const SchemaVersion = 2

type SessionState string

type TranscriptionMode string

const (
	TranscriptionAutomatic TranscriptionMode = "automatic"
	TranscriptionManual    TranscriptionMode = "manual"

	StateCreated             SessionState = "created"
	StateRecording           SessionState = "recording"
	StateStopping            SessionState = "stopping"
	StateProcessing          SessionState = "processing"
	StateDegraded            SessionState = "degraded"
	StateCompleted           SessionState = "completed"
	StateCompletedWithErrors SessionState = "completed_with_errors"
	StateFailed              SessionState = "failed"
	StateCancelled           SessionState = "cancelled"
)

type Session struct {
	SchemaVersion     int               `json:"schema_version"`
	ID                string            `json:"id"`
	Title             string            `json:"title,omitempty"`
	Owner             string            `json:"owner,omitempty"`
	State             SessionState      `json:"state"`
	TranscriptionMode TranscriptionMode `json:"transcription_mode,omitempty"`
	CreatedAt         time.Time         `json:"created_at"`
	UpdatedAt         time.Time         `json:"updated_at"`
	StartedAt         *time.Time        `json:"started_at,omitempty"`
	StoppedAt         *time.Time        `json:"stopped_at,omitempty"`
	LastChunk         int               `json:"last_chunk"`
	Metadata          map[string]string `json:"metadata,omitempty"`
}

type ChunkMetadata struct {
	SchemaVersion int       `json:"schema_version"`
	SessionID     string    `json:"session_id"`
	Index         int       `json:"index"`
	ClientChunkID string    `json:"client_chunk_id,omitempty"`
	StartedAt     time.Time `json:"started_at,omitempty"`
	DurationMS    int64     `json:"duration_ms,omitempty"`
	Format        string    `json:"format"`
	SHA256        string    `json:"sha256"`
	Size          int64     `json:"size"`
	Final         bool      `json:"final"`
	AcceptedAt    time.Time `json:"accepted_at"`
}

type JobState string

const (
	JobPending    JobState = "pending"
	JobProcessing JobState = "processing"
	JobRetryWait  JobState = "retry_wait"
	JobCompleted  JobState = "completed"
	JobDead       JobState = "dead"
)

type ASRJob struct {
	SchemaVersion int        `json:"schema_version"`
	ID            string     `json:"id"`
	SessionID     string     `json:"session_id"`
	ChunkIndex    int        `json:"chunk_index"`
	State         JobState   `json:"state"`
	Attempts      int        `json:"attempts"`
	NextAttemptAt *time.Time `json:"next_attempt_at,omitempty"`
	Provider      string     `json:"provider,omitempty"`
	Model         string     `json:"model,omitempty"`
	LastError     string     `json:"last_error,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

type IngestChunkRequest struct {
	Index          int
	ClientChunkID  string
	StartedAt      time.Time
	DurationMS     int64
	Format         string
	DeclaredSHA256 string
	Final          bool
}

type IngestChunkResult struct {
	Chunk      ChunkMetadata
	Duplicate  bool
	JobCreated bool
	JobState   *JobState
}

type ClaimedJob struct {
	Job       ASRJob
	Chunk     ChunkMetadata
	AudioPath string
}

type TranscriptChunk struct {
	SchemaVersion int       `json:"schema_version"`
	SessionID     string    `json:"session_id"`
	ChunkIndex    int       `json:"chunk_index"`
	ChunkSHA256   string    `json:"chunk_sha256"`
	Provider      string    `json:"provider"`
	Model         string    `json:"model"`
	Language      string    `json:"language,omitempty"`
	DurationMS    int64     `json:"duration_ms,omitempty"`
	Text          string    `json:"text"`
	TranscribedAt time.Time `json:"transcribed_at"`
}

type TranscriptPage struct {
	Chunks    []TranscriptChunk `json:"chunks"`
	NextAfter int               `json:"next_after,omitempty"`
	HasMore   bool              `json:"has_more"`
}

type ManualTranscript struct {
	SchemaVersion  int       `json:"schema_version"`
	SessionID      string    `json:"session_id"`
	Revision       int       `json:"revision"`
	Source         string    `json:"source"`
	Language       string    `json:"language,omitempty"`
	Text           string    `json:"text"`
	ContentSHA256  string    `json:"content_sha256"`
	IdempotencyKey string    `json:"idempotency_key"`
	SubmittedBy    string    `json:"submitted_by"`
	SubmittedAt    time.Time `json:"submitted_at"`
}

type SubmitManualTranscriptRequest struct {
	Text             string
	Language         string
	ExpectedRevision int
	IdempotencyKey   string
	SubmittedBy      string
}

type RecoveryStatus struct {
	HighestChunk       int   `json:"highest_chunk"`
	ReceivedCount      int   `json:"received_count"`
	MissingChunks      []int `json:"missing_chunks"`
	MissingTruncated   bool  `json:"missing_chunks_truncated"`
	PendingJobs        int   `json:"pending_jobs"`
	ProcessingJobs     int   `json:"processing_jobs"`
	RetryWaitJobs      int   `json:"retry_wait_jobs"`
	CompletedJobs      int   `json:"completed_jobs"`
	DeadJobs           int   `json:"dead_jobs"`
	TranscriptChunks   int   `json:"transcript_chunks"`
	FinalChunkReceived bool  `json:"final_chunk_received"`
	Recoverable        bool  `json:"recoverable"`
}
