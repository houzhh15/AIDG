package copy

// CopyMode defines how to handle conflicts during resource copy
type CopyMode string

const (
	ModeOverwrite    CopyMode = "overwrite"     // overwrite existing files
	ModeSkipExisting CopyMode = "skip_existing" // skip if file already exists
)

// CopyPushRequest is the frontend request to push resources to a remote
type CopyPushRequest struct {
	RemoteID  string         `json:"remote_id"`
	RemoteURL string         `json:"remote_url,omitempty"`
	Resources []CopyResource `json:"resources" binding:"required,min=1"`
	Mode      CopyMode       `json:"mode"`
	Options   CopyOptions    `json:"options"`
}

// CopyResource identifies a single resource to copy
type CopyResource struct {
	Type      string `json:"type" binding:"required"`
	ID        string `json:"id" binding:"required"`
	ProjectID string `json:"project_id,omitempty"`
}

// CopyOptions contains optional flags for the copy operation
type CopyOptions struct {
	IncludeAudio    bool `json:"include_audio"`
	IncludeSubTasks bool `json:"include_sub_tasks"`
}

// CopyEnvelope is the signed payload sent between AIDG systems
type CopyEnvelope struct {
	Timestamp int64             `json:"ts"`
	Signature string            `json:"sig"`
	Mode      CopyMode          `json:"mode"`
	Resources []ResourcePayload `json:"resources"`
}

// ResourcePayload contains all data needed to recreate a resource on the target
type ResourcePayload struct {
	Type          string         `json:"type"`
	ID            string         `json:"id"`
	ProjectID     string         `json:"project_id,omitempty"`
	RegistryEntry interface{}    `json:"registry_entry"`
	Files         []ResourceFile `json:"files"`
}

// ResourceFile represents a single file within a resource
type ResourceFile struct {
	RelPath string `json:"rel_path"`
	Hash    string `json:"hash"`
	Content string `json:"content"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"is_dir"`
}

// CopyReceiveResponse is what the receiving AIDG returns
type CopyReceiveResponse struct {
	Success   bool              `json:"success"`
	Resources []CopyResult      `json:"resources"`
	Summary   CopyResultSummary `json:"summary"`
}

// CopyResult tracks the outcome for a single resource
type CopyResult struct {
	Type   string `json:"type"`
	ID     string `json:"id"`
	Status string `json:"status"`
	Files  int    `json:"files"`
	Error  string `json:"error,omitempty"`
}

// CopyResultSummary aggregates results
type CopyResultSummary struct {
	Total   int `json:"total"`
	Created int `json:"created"`
	Updated int `json:"updated"`
	Skipped int `json:"skipped"`
	Errors  int `json:"errors"`
}

// RemoteResourceListResponse is what a remote returns for browsable resources
type RemoteResourceListResponse struct {
	Meetings []RemoteResourceInfo `json:"meetings"`
	Projects []RemoteResourceInfo `json:"projects"`
}

// RemoteResourceInfo is a lightweight descriptor of a resource
type RemoteResourceInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}
