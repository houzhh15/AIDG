package remotes

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/houzhh15/AIDG/cmd/server/internal/config"
)

var remotesFilePath = "./data/remotes.json"

// Service manages persistent remote AIDG peers
type Service struct {
	mu      sync.Mutex
	remotes map[string]*Remote
}

// NewService creates a new remotes service and loads persisted data
func NewService() *Service {
	initPath()
	s := &Service{remotes: make(map[string]*Remote)}
	_ = s.load()
	return s
}

func initPath() {
	base := "./data"
	if config.GlobalConfig != nil && config.GlobalConfig.Data.ProjectsDir != "" {
		base = filepath.Dir(config.GlobalConfig.Data.ProjectsDir) // data/
	}
	remotesFilePath = filepath.Join(base, "remotes.json")
}

// List returns all remotes (safe version without secrets)
func (s *Service) List() []RemoteSafe {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]RemoteSafe, 0, len(s.remotes))
	for _, r := range s.remotes {
		out = append(out, r.Safe())
	}
	return out
}

// Get returns a single remote by ID
func (s *Service) Get(id string) (*Remote, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.remotes[id]
	return r, ok
}

// Create adds a new remote
func (s *Service) Create(req CreateRemoteRequest) (*Remote, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("remote-%d", time.Now().UnixNano())
	now := time.Now()

	secret := req.Secret
	if secret == "" {
		secret = os.Getenv("SYNC_SHARED_SECRET")
		if secret == "" {
			secret = "neteye@123"
		}
	}

	r := &Remote{
		ID:        id,
		Name:      req.Name,
		URL:       req.URL,
		Secret:    secret,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.remotes[id] = r
	if err := s.saveLocked(); err != nil {
		delete(s.remotes, id)
		return nil, err
	}
	return r, nil
}

// Update modifies an existing remote
func (s *Service) Update(id string, req UpdateRemoteRequest) (*Remote, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	r, ok := s.remotes[id]
	if !ok {
		return nil, fmt.Errorf("remote not found: %s", id)
	}
	if req.Name != "" {
		r.Name = req.Name
	}
	if req.URL != "" {
		r.URL = req.URL
	}
	if req.Secret != "" {
		r.Secret = req.Secret
	}
	r.UpdatedAt = time.Now()

	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return r, nil
}

// Delete removes a remote by ID
func (s *Service) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.remotes[id]; !ok {
		return fmt.Errorf("remote not found: %s", id)
	}
	delete(s.remotes, id)
	return s.saveLocked()
}

// TestConnection tests connectivity to a remote AIDG system
func (s *Service) TestConnection(remoteURL string) TestResult {
	start := time.Now()
	client := &http.Client{Timeout: 10 * time.Second}

	resp, err := client.Get(remoteURL + "/api/v1/health")
	latency := time.Since(start)

	if err != nil {
		return TestResult{
			Reachable: false,
			Status:    "unreachable",
			Latency:   latency.String(),
			Error:     err.Error(),
		}
	}
	defer resp.Body.Close()

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)

	service, _ := body["service"].(string)
	version, _ := body["version"].(string)

	return TestResult{
		Reachable: resp.StatusCode == 200,
		Status:    fmt.Sprintf("%d", resp.StatusCode),
		Latency:   latency.String(),
		Service:   service,
		Version:   version,
	}
}

// --- persistence ---

type persistedRemotes struct {
	Remotes []Remote `json:"remotes"`
}

func (s *Service) load() error {
	b, err := os.ReadFile(remotesFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var wrapper persistedRemotes
	if err := json.Unmarshal(b, &wrapper); err != nil {
		return err
	}
	for i := range wrapper.Remotes {
		r := wrapper.Remotes[i]
		s.remotes[r.ID] = &r
	}
	return nil
}

func (s *Service) saveLocked() error {
	list := make([]Remote, 0, len(s.remotes))
	for _, r := range s.remotes {
		list = append(list, *r)
	}
	b, err := json.MarshalIndent(persistedRemotes{Remotes: list}, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(remotesFilePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := remotesFilePath + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, remotesFilePath)
}
