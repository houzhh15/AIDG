package taskdocs

import (
	"sync"
	"time"
)

// DocChunk represents a chunk of incremental document content
type DocChunk struct {
	Sequence  int       `json:"sequence"`
	Timestamp time.Time `json:"timestamp"`
	Op        string    `json:"op"`
	Content   string    `json:"content"`
	User      string    `json:"user"`
	Source    string    `json:"source"`
	Hash      string    `json:"hash"`
	Active    bool      `json:"active"`
}

// DocMeta contains metadata about the incremental document
type DocMeta struct {
	Version      int       `json:"version"`
	LastSequence int       `json:"last_sequence"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	DocType      string    `json:"doc_type"`
	HashWindow   []string  `json:"hash_window"`
	ChunkCount   int       `json:"chunk_count"`
	DeletedCount int       `json:"deleted_count"`
	ETag         string    `json:"etag"`
}

// DocService provides thread-safe incremental document operations
type DocService struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex // key: projectID|taskID|docType
}

// NewDocService creates a new document service
func NewDocService() *DocService {
	return &DocService{locks: map[string]*sync.Mutex{}}
}

// GetLock retrieves or creates a lock for a specific document
func (s *DocService) GetLock(projectID, taskID, docType string) *sync.Mutex {
	k := projectID + "|" + taskID + "|" + docType
	s.mu.Lock()
	l := s.locks[k]
	if l == nil {
		l = &sync.Mutex{}
		s.locks[k] = l
	}
	s.mu.Unlock()
	return l
}

const DocHashWindowSize = 10
