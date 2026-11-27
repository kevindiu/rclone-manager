package rclone

import "errors"

var (
	// ErrRemoteNotFound is returned when a named remote does not exist in rclone.conf.
	ErrRemoteNotFound = errors.New("remote not found")
	// ErrRemoteExists is returned when creating a remote whose name already exists.
	ErrRemoteExists = errors.New("remote already exists")
)

// RemoteInfo represents a configured remote with secret fields masked.
type RemoteInfo struct {
	Name        string            `json:"name"`
	Type        string            `json:"type"`
	Description string            `json:"description,omitempty"`
	Config      map[string]string `json:"config"`
	MaskedKeys  []string          `json:"masked_keys,omitempty"`
}

// ProviderOption describes a single configuration option for an rclone backend.
type ProviderOption struct {
	Name       string            `json:"Name"`
	Help       string            `json:"Help"`
	Provider   string            `json:"Provider,omitempty"`
	DefaultStr string            `json:"DefaultStr,omitempty"`
	Required   bool              `json:"Required"`
	IsPassword bool              `json:"IsPassword"`
	Advanced   bool              `json:"Advanced"`
	Sensitive  bool              `json:"Sensitive"`
	Type       string            `json:"Type"`
	Examples   []ProviderExample `json:"Examples,omitempty"`
}

// ProviderExample represents a suggested value for a provider option.
type ProviderExample struct {
	Value    string `json:"Value"`
	Help     string `json:"Help"`
	Provider string `json:"Provider,omitempty"`
}

// ProviderInfo describes a supported rclone backend provider.
type ProviderInfo struct {
	Name        string           `json:"Name"`
	Description string           `json:"Description"`
	Prefix      string           `json:"Prefix"`
	IsOAuth     bool             `json:"is_oauth"`
	Options     []ProviderOption `json:"Options"`
}

// RemoteTestResult reports connectivity/auth test outcome for a remote.
type RemoteTestResult struct {
	Remote    string `json:"remote"`
	OK        bool   `json:"ok"`
	LatencyMs int64  `json:"latency_ms"`
	Message   string `json:"message,omitempty"`
	Error     string `json:"error,omitempty"`
}

// RemoteAbout reports quota/storage usage returned by `rclone about`.
type RemoteAbout struct {
	Remote    string `json:"remote"`
	Supported bool   `json:"supported"`
	Total     *int64 `json:"total,omitempty"`
	Used      *int64 `json:"used,omitempty"`
	Free      *int64 `json:"free,omitempty"`
	Trashed   *int64 `json:"trashed,omitempty"`
	Other     *int64 `json:"other,omitempty"`
	Error     string `json:"error,omitempty"`
}

// BrowseEntry represents a file or directory returned by `rclone lsjson`.
type BrowseEntry struct {
	Path     string `json:"Path"`
	Name     string `json:"Name"`
	Size     int64  `json:"Size"`
	MimeType string `json:"MimeType,omitempty"`
	ModTime  string `json:"ModTime,omitempty"`
	IsDir    bool   `json:"IsDir"`
}

// RestoreRequest restores a file or folder from a remote (or _versions folder) to a target path.
type RestoreRequest struct {
	SourceRemote string `json:"source_remote"`
	SourcePath   string `json:"source_path"`
	TargetPath   string `json:"target_path"`
	IsDir        bool   `json:"is_dir"`
	ConflictMode string `json:"conflict_mode,omitempty"` // "overwrite" | "skip" | "newer_only"
	DryRun       bool   `json:"dry_run,omitempty"`
}

// FileVersion represents a dated historical version of a file or directory found in backup-dir storage.
type FileVersion struct {
	Date       string `json:"date"`
	SourcePath string `json:"source_path"`
	Size       int64  `json:"size"`
	ModTime    string `json:"mod_time,omitempty"`
	IsDir      bool   `json:"is_dir"`
	BackupRoot string `json:"backup_root"`
}

// VersionRootCandidate describes a candidate backup-dir root on a remote and the relative path inside it.
type VersionRootCandidate struct {
	BackupRoot string
	RelPath    string
}

// DirSize reports file count and total size for a remote directory via `rclone size --json`.
type DirSize struct {
	Count int64 `json:"count"`
	Bytes int64 `json:"bytes"`
}
