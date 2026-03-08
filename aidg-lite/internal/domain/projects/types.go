package projects

import (
	"sync"
	"time"
)

// Project represents a project entity
type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ProductLine string    `json:"product_line"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ProjectRegistry maintains a thread-safe collection of projects
type ProjectRegistry struct {
	mu sync.Mutex
	m  map[string]*Project
}

// NewProjectRegistry creates a new project registry
func NewProjectRegistry() *ProjectRegistry {
	return &ProjectRegistry{m: map[string]*Project{}}
}

// Get retrieves a project by ID
func (r *ProjectRegistry) Get(id string) *Project {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.m[id]
}

// Set stores or updates a project
func (r *ProjectRegistry) Set(p *Project) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[p.ID] = p
}

// List returns all projects as a slice
func (r *ProjectRegistry) List() []*Project {
	r.mu.Lock()
	defer r.mu.Unlock()

	list := make([]*Project, 0, len(r.m))
	for _, p := range r.m {
		list = append(list, p)
	}
	return list
}

// Delete removes a project by ID
func (r *ProjectRegistry) Delete(id string) *Project {
	r.mu.Lock()
	defer r.mu.Unlock()

	p := r.m[id]
	if p != nil {
		delete(r.m, id)
	}
	return p
}

// Update updates a project's fields
func (r *ProjectRegistry) Update(id string, name, productLine string) *Project {
	r.mu.Lock()
	defer r.mu.Unlock()

	p := r.m[id]
	if p != nil {
		if name != "" {
			p.Name = name
		}
		if productLine != "" {
			p.ProductLine = productLine
		}
		p.UpdatedAt = time.Now()
	}
	return p
}

// persistedProject is the structure saved to disk
type persistedProject struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ProductLine string    `json:"product_line"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
