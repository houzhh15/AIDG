// Package similarity provides vector index management for semantic similarity
package similarity

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// VectorEntry represents a single vector index entry
type VectorEntry struct {
	ProjectID string    `json:"project_id"`
	TaskID    string    `json:"task_id"`
	DocType   string    `json:"doc_type"`   // requirements/design/test
	SectionID string    `json:"section_id"` // section identifier
	Title     string    `json:"title"`      // section title
	Vector    []float64 `json:"vector"`     // 768-dimensional embedding vector
	UpdatedAt string    `json:"updated_at"` // RFC3339 timestamp
}

// VectorIndexManager manages in-memory vector index for a specific project
// It provides thread-safe operations for loading, saving and querying vectors
type VectorIndexManager struct {
	mu        sync.RWMutex            // Protects concurrent access
	projectID string                  // Bound to specific project
	entries   map[string]*VectorEntry // key: taskID:docType:sectionID
	filePath  string                  // Persistence file path
}

// NewVectorIndexManager creates a new vector index manager for a project
// dataDir: root data directory (default: ./data)
// projectID: project identifier
func NewVectorIndexManager(projectID string, dataDir string) *VectorIndexManager {
	if dataDir == "" {
		dataDir = "./data"
	}

	// Generate per-project index file path: data/projects/{project_id}/vector_index.json
	filePath := filepath.Join(dataDir, "projects", projectID, "vector_index.json")

	mgr := &VectorIndexManager{
		projectID: projectID,
		entries:   make(map[string]*VectorEntry),
		filePath:  filePath,
	}

	// Load existing index on initialization
	if err := mgr.Load(); err != nil {
		log.Printf("[WARN] VectorIndexManager: failed to load index for project %s: %v", projectID, err)
	}

	return mgr
}

// Load reads vector index from JSON file into memory
func (m *VectorIndexManager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("[INFO] VectorIndexManager: no existing index file for project %s, starting with empty index", m.projectID)
			return nil // File not exists, start with empty index
		}
		return fmt.Errorf("failed to read index file: %w", err)
	}

	var entries []*VectorEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("failed to unmarshal index: %w", err)
	}

	// Clear existing entries
	m.entries = make(map[string]*VectorEntry)

	// Load entries with validation
	for _, entry := range entries {
		// Validate project ID consistency
		if entry.ProjectID != m.projectID {
			log.Printf("[WARN] VectorIndexManager: skip entry with mismatched projectID: %s (expected: %s)",
				entry.ProjectID, m.projectID)
			continue
		}

		// Generate key: taskID:docType:sectionID
		key := makeKey(entry.TaskID, entry.DocType, entry.SectionID)
		m.entries[key] = entry
	}

	log.Printf("[INFO] VectorIndexManager: loaded %d entries for project %s", len(m.entries), m.projectID)
	return nil
}

// Save persists vector index to JSON file (ensures directory exists)
func (m *VectorIndexManager) Save() error {
	m.mu.RLock()
	entries := make([]*VectorEntry, 0, len(m.entries))
	for _, entry := range m.entries {
		entries = append(entries, entry)
	}
	m.mu.RUnlock()

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(m.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write to file
	if err := os.WriteFile(m.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write index file: %w", err)
	}

	log.Printf("[INFO] VectorIndexManager: saved %d entries for project %s", len(entries), m.projectID)
	return nil
}

// Update adds or updates vector entries in memory (batch operation)
func (m *VectorIndexManager) Update(entries []*VectorEntry) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, entry := range entries {
		// Validate project ID consistency
		if entry.ProjectID != m.projectID {
			log.Printf("[WARN] VectorIndexManager: reject entry with wrong projectID: %s (expected: %s)",
				entry.ProjectID, m.projectID)
			continue
		}

		// Update or insert entry
		key := makeKey(entry.TaskID, entry.DocType, entry.SectionID)
		m.entries[key] = entry
	}

	log.Printf("[INFO] VectorIndexManager: updated %d entries for project %s", len(entries), m.projectID)
}

// Delete removes vector entries by task ID
func (m *VectorIndexManager) Delete(taskID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for key := range m.entries {
		if m.entries[key].TaskID == taskID {
			delete(m.entries, key)
			count++
		}
	}

	log.Printf("[INFO] VectorIndexManager: deleted %d entries for task %s", count, taskID)
	return count
}

// Count returns the number of entries in the index
func (m *VectorIndexManager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.entries)
}

// GetProjectID returns the project ID this manager is bound to
func (m *VectorIndexManager) GetProjectID() string {
	return m.projectID
}

// GetFilePath returns the persistence file path
func (m *VectorIndexManager) GetFilePath() string {
	return m.filePath
}

// GetVector retrieves the cached vector for a specific section
// Returns nil if the vector doesn't exist in the cache
func (m *VectorIndexManager) GetVector(taskID, docType, sectionID string) []float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := makeKey(taskID, docType, sectionID)
	if entry, exists := m.entries[key]; exists {
		return entry.Vector
	}
	return nil
}

// RecommendationResult represents a similarity query result
type RecommendationResult struct {
	TaskID     string  `json:"task_id"`
	DocType    string  `json:"doc_type"`
	SectionID  string  `json:"section_id"`
	Title      string  `json:"title"`
	Similarity float64 `json:"similarity"`
	Snippet    string  `json:"snippet,omitempty"` // Content snippet (first 50 chars)

	// 源章节信息(当前任务中匹配的章节)
	SourceSectionID string `json:"source_section_id,omitempty"`
	SourceTitle     string `json:"source_title,omitempty"`
}

// Query performs similarity search using cosine similarity
// queryVector: query embedding vector (768-dim)
// topK: maximum number of results to return
// threshold: minimum similarity score (0.0 to 1.0)
func (m *VectorIndexManager) Query(queryVector []float64, topK int, threshold float64) []RecommendationResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []RecommendationResult

	// Calculate similarity for all entries
	for _, entry := range m.entries {
		similarity := cosineSimilarity(queryVector, entry.Vector)
		if similarity >= threshold {
			results = append(results, RecommendationResult{
				TaskID:     entry.TaskID,
				DocType:    entry.DocType,
				SectionID:  entry.SectionID,
				Title:      entry.Title,
				Similarity: similarity,
			})
		}
	}

	// Sort by similarity in descending order
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	// Return top-K results
	if len(results) > topK {
		results = results[:topK]
	}

	return results
}

// cosineSimilarity calculates cosine similarity between two vectors
// Formula: cos(θ) = (A·B) / (||A|| * ||B||)
func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		log.Printf("[WARN] cosineSimilarity: vector length mismatch (a:%d, b:%d)", len(a), len(b))
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	// Avoid division by zero
	if normA == 0 || normB == 0 {
		return 0
	}

	// Calculate cosine similarity
	return dotProduct / (sqrt(normA) * sqrt(normB))
}

// sqrt calculates square root
func sqrt(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// Use Newton's method for square root approximation
	z := x
	for i := 0; i < 10; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// makeKey generates a unique key for vector entry
func makeKey(taskID, docType, sectionID string) string {
	return fmt.Sprintf("%s:%s:%s", taskID, docType, sectionID)
}

// SaveAsync performs asynchronous save operation
func (m *VectorIndexManager) SaveAsync() {
	go func() {
		if err := m.Save(); err != nil {
			log.Printf("[ERROR] VectorIndexManager: async save failed: %v", err)
		}
	}()
}

// SchedulePeriodicSave starts periodic save with specified interval
// Call stop channel to stop the goroutine
func (m *VectorIndexManager) SchedulePeriodicSave(interval time.Duration) chan struct{} {
	stop := make(chan struct{})

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if err := m.Save(); err != nil {
					log.Printf("[ERROR] VectorIndexManager: periodic save failed: %v", err)
				}
			case <-stop:
				log.Printf("[INFO] VectorIndexManager: stopping periodic save for project %s", m.projectID)
				return
			}
		}
	}()

	log.Printf("[INFO] VectorIndexManager: started periodic save (interval: %v) for project %s", interval, m.projectID)
	return stop
}
