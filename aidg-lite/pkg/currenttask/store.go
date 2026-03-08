// Package currenttask provides an Authorization-header-keyed LRU store for
// recording "current task" state in AIDG Lite mode.
//
// In full-AIDG, every authenticated user has one current task persisted under
// their home directory.  Lite mode has no user registry, so it uses the raw
// Authorization header value (SHA-256 hashed for a bounded, privacy-safe key)
// as the per-client identity.
//
// The store keeps at most Cap entries (default 1000).  When the limit is
// reached, the least-recently-used entry is evicted.  The store is safe for
// concurrent use and persists its state to a JSON file so that restarts do
// not lose data.
package currenttask

import (
	"container/list"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// DefaultCapacity is the maximum number of concurrent "current task" slots.
const DefaultCapacity = 1000

// Entry is the value stored for each Authorization identity.
type Entry struct {
	ProjectID string    `json:"project_id"`
	TaskID    string    `json:"task_id"`
	SetAt     time.Time `json:"set_at"`
}

// lruNode is the payload stored inside each list.Element.
type lruNode struct {
	key   string // SHA-256 hex of the Authorization header value
	entry Entry
}

// Store is a thread-safe, capacity-bounded LRU map from
// hash(Authorization) → Entry, with optional disk persistence.
type Store struct {
	mu       sync.Mutex
	cap      int
	ll       *list.List               // front = most recently used
	index    map[string]*list.Element // key → element
	filePath string                   // "" means no persistence
}

// New returns a new Store.  filePath may be empty to disable persistence.
// If filePath references an existing file it is loaded on construction.
func New(capacity int, filePath string) *Store {
	if capacity <= 0 {
		capacity = DefaultCapacity
	}
	s := &Store{
		cap:      capacity,
		ll:       list.New(),
		index:    make(map[string]*list.Element, capacity),
		filePath: filePath,
	}
	if filePath != "" {
		s.load()
	}
	return s
}

// KeyFromAuth derives a deterministic, fixed-length key from an Authorization
// header value.  An empty value maps to the literal key "anon".
func KeyFromAuth(authHeader string) string {
	if authHeader == "" {
		return "anon"
	}
	sum := sha256.Sum256([]byte(authHeader))
	return fmt.Sprintf("%x", sum)
}

// Get returns the Entry associated with authHeader and true, or the zero
// value and false if no entry exists.  A hit promotes the entry to MRU.
func (s *Store) Get(authHeader string) (Entry, bool) {
	key := KeyFromAuth(authHeader)
	s.mu.Lock()
	defer s.mu.Unlock()
	if el, ok := s.index[key]; ok {
		s.ll.MoveToFront(el)
		return el.Value.(*lruNode).entry, true
	}
	return Entry{}, false
}

// Set creates or updates the Entry for authHeader and promotes it to MRU.
// If capacity is exceeded the LRU entry is evicted before insertion.
// The store is persisted to disk after every successful write.
func (s *Store) Set(authHeader string, e Entry) {
	key := KeyFromAuth(authHeader)
	s.mu.Lock()
	defer s.mu.Unlock()

	if el, ok := s.index[key]; ok {
		// Update existing entry and mark as recently used.
		el.Value.(*lruNode).entry = e
		s.ll.MoveToFront(el)
	} else {
		// Evict LRU if at capacity.
		if s.ll.Len() >= s.cap {
			s.evictLocked()
		}
		node := &lruNode{key: key, entry: e}
		el := s.ll.PushFront(node)
		s.index[key] = el
	}

	s.persistLocked()
}

// Len returns the current number of stored entries (useful for tests/metrics).
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ll.Len()
}

// ─── internal helpers ─────────────────────────────────────────────────────────

// evictLocked removes the LRU (back-of-list) entry.  Must be called with s.mu held.
func (s *Store) evictLocked() {
	back := s.ll.Back()
	if back == nil {
		return
	}
	node := back.Value.(*lruNode)
	s.ll.Remove(back)
	delete(s.index, node.key)
}

// diskEntry is the serialised form of a single record.
type diskEntry struct {
	Key   string `json:"k"`
	Entry Entry  `json:"e"`
}

// persistLocked writes the current state to disk.  Must be called with s.mu held.
// Writes are best-effort: errors are silently swallowed to avoid disrupting the
// request path.
func (s *Store) persistLocked() {
	if s.filePath == "" {
		return
	}
	records := make([]diskEntry, 0, s.ll.Len())
	for el := s.ll.Front(); el != nil; el = el.Next() {
		n := el.Value.(*lruNode)
		records = append(records, diskEntry{Key: n.key, Entry: n.entry})
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return
	}
	// Atomic-ish write: write to a temp file then rename.
	tmp := s.filePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, s.filePath)
}

// load reads the persisted state from disk.  Must be called before the store is
// shared across goroutines (only called from New, so no lock needed).
func (s *Store) load() {
	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return // file absent or unreadable – start empty
	}
	var records []diskEntry
	if err := json.Unmarshal(data, &records); err != nil {
		return // corrupt file – start empty
	}
	// Records are saved MRU-first.  To restore the same order, insert them
	// in reverse (oldest first) using PushFront, so MRU ends up at the front.
	for i := len(records) - 1; i >= 0; i-- {
		r := records[i]
		if s.ll.Len() >= s.cap {
			break // respect capacity
		}
		node := &lruNode{key: r.Key, entry: r.Entry}
		el := s.ll.PushFront(node)
		s.index[r.Key] = el
	}
}
