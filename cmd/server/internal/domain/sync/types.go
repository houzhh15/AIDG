package sync

// SyncMode defines the synchronization strategy
type SyncMode string

const (
	ModeClientOverwrite  SyncMode = "client_overwrite"
	ModeServerOverwrite  SyncMode = "server_overwrite"
	ModeMergeNoOverwrite SyncMode = "merge_no_overwrite"
	ModePullOverwrite    SyncMode = "pull_overwrite"
)

// SyncFile represents a file to be synchronized
type SyncFile struct {
	Path    string `json:"path"`
	Hash    string `json:"hash"`
	Content string `json:"content"`
	Size    int64  `json:"size"`
}

// SyncRequest contains client synchronization request data
type SyncRequest struct {
	Mode       SyncMode       `json:"mode"`
	ClientHost string         `json:"client_host"`
	Timestamp  string         `json:"timestamp"`
	Files      []SyncFile     `json:"files"`
	Options    map[string]any `json:"options"`
}

// SyncApplied records the action taken on a synchronized file
type SyncApplied struct {
	Path   string `json:"path"`
	Action string `json:"action"`
}

// SyncResponse contains server synchronization response data
type SyncResponse struct {
	Mode        SyncMode      `json:"mode"`
	Applied     []SyncApplied `json:"applied"`
	Conflicts   []SyncApplied `json:"conflicts"`
	ServerFiles []SyncFile    `json:"server_files,omitempty"`
	Summary     any           `json:"summary"`
}
