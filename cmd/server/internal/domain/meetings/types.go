package meetings

import (
	"fmt"
	orchestrator "github.com/houzhh15-hub/AIDG/cmd/server/internal/orchestrator"
	"sync"
	"time"
)

// Task represents a meeting orchestration task
type Task struct {
	ID        string                     `json:"id"`
	Cfg       orchestrator.Config        `json:"config"`
	Orch      *orchestrator.Orchestrator `json:"-"`
	State     orchestrator.State         `json:"state"`
	CreatedAt time.Time                  `json:"created_at"`
}

// Registry maintains a thread-safe collection of tasks
type Registry struct {
	mu sync.Mutex
	m  map[string]*Task
}

// NewRegistry creates a new task registry
func NewRegistry() *Registry {
	return &Registry{m: map[string]*Task{}}
}

// Get retrieves a task by ID
func (r *Registry) Get(id string) *Task {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.m[id]
}

// Set stores or updates a task
func (r *Registry) Set(t *Task) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[t.ID] = t
}

// List returns all tasks as a slice
func (r *Registry) List() []*Task {
	r.mu.Lock()
	defer r.mu.Unlock()

	list := make([]*Task, 0, len(r.m))
	for _, t := range r.m {
		list = append(list, t)
	}
	return list
}

// Delete removes a task by ID and returns it
func (r *Registry) Delete(id string) *Task {
	r.mu.Lock()
	defer r.mu.Unlock()

	t := r.m[id]
	if t != nil {
		delete(r.m, id)
	}
	return t
}

// Rename changes a task's ID
func (r *Registry) Rename(oldID, newID string) (*Task, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	t := r.m[oldID]
	if t == nil {
		return nil, fmt.Errorf("task not found")
	}

	if _, exists := r.m[newID]; exists {
		return nil, fmt.Errorf("new_id exists")
	}

	delete(r.m, oldID)
	t.ID = newID
	r.m[newID] = t

	return t, nil
}

// ContentHistory records the edit history of content
type ContentHistory struct {
	Timestamp time.Time `json:"timestamp"`
	Content   string    `json:"content"`
	Version   int       `json:"version"`
}

// Document type constants
const (
	DocTypeFeatureList    = "feature-list"
	DocTypeArchitecture   = "architecture"
	DocTypeTechDesign     = "tech-design"
	DocTypeMeetingSummary = "meeting-summary"
	DocTypeMeetingContext = "meeting-context"
	DocTypeTopic          = "topic"
	DocTypePolish         = "polish"
)

// Document type to filename mapping
var DocTypeToFilename = map[string]string{
	DocTypeFeatureList:    "feature_list.md",
	DocTypeArchitecture:   "architecture_new.md",
	DocTypeTechDesign:     "tech_design_*.md", // glob pattern
	DocTypeMeetingSummary: "meeting_summary.md",
	DocTypeMeetingContext: "meeting_context.md",
	DocTypeTopic:          "topic.md",
	DocTypePolish:         "polish_all.md",
}

// persistedTask is the structure saved to disk
type persistedTask struct {
	ID        string              `json:"id"`
	Cfg       orchestrator.Config `json:"config"`
	State     orchestrator.State  `json:"state"`
	CreatedAt time.Time           `json:"created_at"`
}
