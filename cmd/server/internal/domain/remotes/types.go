package remotes

import "time"

// Remote represents a configured remote AIDG system
type Remote struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Secret    string    `json:"secret,omitempty"` // HMAC shared secret; omit in list responses
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RemoteSafe is a Remote without the secret field (for list/get responses)
type RemoteSafe struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Safe converts a Remote to a RemoteSafe (without secret)
func (r *Remote) Safe() RemoteSafe {
	return RemoteSafe{
		ID:        r.ID,
		Name:      r.Name,
		URL:       r.URL,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

// CreateRemoteRequest is the request body for creating a remote
type CreateRemoteRequest struct {
	Name   string `json:"name" binding:"required"`
	URL    string `json:"url" binding:"required"`
	Secret string `json:"secret"` // optional, defaults to SYNC_SHARED_SECRET
}

// UpdateRemoteRequest is the request body for updating a remote
type UpdateRemoteRequest struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Secret string `json:"secret"`
}

// TestResult holds connectivity test results
type TestResult struct {
	Reachable bool   `json:"reachable"`
	Status    string `json:"status"`
	Latency   string `json:"latency"`
	Error     string `json:"error,omitempty"`
	Service   string `json:"service,omitempty"`
	Version   string `json:"version,omitempty"`
}
