package domain

import "time"

// GlobalSettings holds application-wide rclone and scheduler defaults.
type GlobalSettings struct {
	Transfers         int    `json:"transfers" yaml:"transfers"`
	Checkers          int    `json:"checkers" yaml:"checkers"`
	BwLimit           string `json:"bwlimit" yaml:"bwlimit"`
	Retries           int    `json:"retries" yaml:"retries"`
	LowLevelRetries   int    `json:"low_level_retries" yaml:"low_level_retries"`
	Timeout           string `json:"timeout" yaml:"timeout"`
	ConTimeout        string `json:"contimeout" yaml:"contimeout"`
	LogRetentionDays  int    `json:"log_retention_days" yaml:"log_retention_days"`
	Timezone          string `json:"timezone" yaml:"timezone"`
	MaxConcurrentJobs    int    `json:"max_concurrent_jobs" yaml:"max_concurrent_jobs"`
	UploadBatchTransfers int    `json:"upload_batch_transfers,omitempty" yaml:"upload_batch_transfers,omitempty"`
	UploadBatchCheckers  int    `json:"upload_batch_checkers,omitempty" yaml:"upload_batch_checkers,omitempty"`
	UploadBatchMaxFiles  int    `json:"upload_batch_max_files,omitempty" yaml:"upload_batch_max_files,omitempty"`
	UploadBatchMaxMB     int    `json:"upload_batch_max_mb,omitempty" yaml:"upload_batch_max_mb,omitempty"`
	UploadSmallFileMaxMB int    `json:"upload_small_file_max_mb,omitempty" yaml:"upload_small_file_max_mb,omitempty"`
	WebhookURL           string `json:"webhook_url,omitempty" yaml:"webhook_url,omitempty"`
	WebhookOn         string `json:"webhook_on,omitempty" yaml:"webhook_on,omitempty"`     // "failure" | "always" | "off"
	WebhookType       string `json:"webhook_type,omitempty" yaml:"webhook_type,omitempty"` // "generic" | "ntfy" | "telegram"
	TelegramBotToken  string `json:"telegram_bot_token,omitempty" yaml:"telegram_bot_token,omitempty"`
	TelegramChatID    string `json:"telegram_chat_id,omitempty" yaml:"telegram_chat_id,omitempty"`
}

// Job represents a configurable backup/sync job.
type Job struct {
	ID          string `json:"id" yaml:"id"`
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Enabled     bool   `json:"enabled" yaml:"enabled"`
	Operation   string `json:"operation" yaml:"operation"` // "copy" | "sync" | "move" | "check" | "bisync"
	Source      string `json:"source" yaml:"source"`
	Destination string `json:"destination" yaml:"destination"`
	Schedule    string `json:"schedule,omitempty" yaml:"schedule,omitempty"`

	ExcludePatterns         []string `json:"exclude_patterns,omitempty" yaml:"exclude_patterns,omitempty"`
	IncludePatterns         []string `json:"include_patterns,omitempty" yaml:"include_patterns,omitempty"`
	FilterRules             []string `json:"filter_rules,omitempty" yaml:"filter_rules,omitempty"`
	CustomFlags             string   `json:"custom_flags,omitempty" yaml:"custom_flags,omitempty"`
	FastList                bool     `json:"fast_list,omitempty" yaml:"fast_list,omitempty"`
	Checksum                bool     `json:"checksum,omitempty" yaml:"checksum,omitempty"`
	UpdateOnly              bool     `json:"update_only,omitempty" yaml:"update_only,omitempty"`
	ServerSideAcrossConfigs bool     `json:"server_side_across_configs,omitempty" yaml:"server_side_across_configs,omitempty"`
	DeleteTiming            string   `json:"delete_timing,omitempty" yaml:"delete_timing,omitempty"` // "" | "during" | "after" | "before"
	DeleteEmptySrcDirs      bool     `json:"delete_empty_src_dirs,omitempty" yaml:"delete_empty_src_dirs,omitempty"`
	SymlinkMode             string   `json:"symlink_mode,omitempty" yaml:"symlink_mode,omitempty"` // "" | "links" | "copy_links"
	SizeOnly                bool     `json:"size_only,omitempty" yaml:"size_only,omitempty"`
	IgnoreExisting          bool     `json:"ignore_existing,omitempty" yaml:"ignore_existing,omitempty"`
	MinAge                  string   `json:"min_age,omitempty" yaml:"min_age,omitempty"`
	MaxAge                  string   `json:"max_age,omitempty" yaml:"max_age,omitempty"`
	MinSize                 string   `json:"min_size,omitempty" yaml:"min_size,omitempty"`
	MaxSize                 string   `json:"max_size,omitempty" yaml:"max_size,omitempty"`
	BisyncConflictResolve   string   `json:"bisync_conflict_resolve,omitempty" yaml:"bisync_conflict_resolve,omitempty"`
	BisyncResync            bool     `json:"-" yaml:"-"`
	NotifyOverride          string   `json:"notify_override,omitempty" yaml:"notify_override,omitempty"` // "" | "failure" | "always" | "off"

	MaxTransfer string `json:"max_transfer,omitempty" yaml:"max_transfer,omitempty"`
	CutoffMode  string `json:"cutoff_mode,omitempty" yaml:"cutoff_mode,omitempty"` // "soft" | "hard" | "cautious"

	// Incremental versioning (--backup-dir)
	VersioningEnabled    bool   `json:"versioning_enabled" yaml:"versioning_enabled"`
	BackupDirBase        string `json:"backup_dir_base,omitempty" yaml:"backup_dir_base,omitempty"`
	VersionFormat        string `json:"version_format,omitempty" yaml:"version_format,omitempty"`
	VersionRetentionDays int    `json:"version_retention_days" yaml:"version_retention_days"` // 0 = keep forever

	// Pre-flight safety guards
	AbortIfSourceEmpty bool   `json:"abort_if_source_empty" yaml:"abort_if_source_empty"`
	RequiredMarkerFile string `json:"required_marker_file,omitempty" yaml:"required_marker_file,omitempty"`
	MaxDelete          *int   `json:"max_delete,omitempty" yaml:"max_delete,omitempty"`

	// Per-job overrides (nil/empty = inherit global settings)
	Transfers       *int   `json:"transfers,omitempty" yaml:"transfers,omitempty"`
	Checkers        *int   `json:"checkers,omitempty" yaml:"checkers,omitempty"`
	BwLimit         string `json:"bwlimit,omitempty" yaml:"bwlimit,omitempty"`
	Retries         *int   `json:"retries,omitempty" yaml:"retries,omitempty"`
	LowLevelRetries *int   `json:"low_level_retries,omitempty" yaml:"low_level_retries,omitempty"`
	Timeout         string `json:"timeout,omitempty" yaml:"timeout,omitempty"`
	ConTimeout      string `json:"contimeout,omitempty" yaml:"contimeout,omitempty"`

	CreatedAt time.Time `json:"created_at" yaml:"created_at"`
	UpdatedAt time.Time `json:"updated_at" yaml:"updated_at"`
}

// JobRun records a single execution of a Job.
type JobRun struct {
	ID               string     `json:"id"`
	JobID            string     `json:"job_id"`
	JobName          string     `json:"job_name"`
	Trigger          string     `json:"trigger"` // "manual" | "schedule" | "dry_run"
	DryRun           bool       `json:"dry_run"`
	Status           string     `json:"status"` // "queued" | "running" | "success" | "failed" | "cancelled" | "skipped"
	StartedAt        time.Time  `json:"started_at"`
	FinishedAt       *time.Time `json:"finished_at,omitempty"`
	DurationMs       int64      `json:"duration_ms"`
	BytesTransferred int64      `json:"bytes_transferred"`
	TotalBytes       int64      `json:"total_bytes"`
	FilesTransferred int64      `json:"files_transferred"`
	FilesChecked     int64      `json:"files_checked"`
	FilesDeleted     int64      `json:"files_deleted"`
	ErrorsCount      int64      `json:"errors_count"`
	ErrorMessage     string     `json:"error_message,omitempty"`
	CommandPreview   string     `json:"command_preview,omitempty"`
	LogFile          string     `json:"log_file,omitempty"`
	BackupDirUsed    string     `json:"backup_dir_used,omitempty"`
	PrunedVersions   []string   `json:"pruned_versions,omitempty"`
}

// TransferringFile describes an active file currently being transferred by rclone.
type TransferringFile struct {
	Name       string  `json:"name"`
	Size       int64   `json:"size"`
	Bytes      int64   `json:"bytes"`
	Percentage int     `json:"percentage"`
	Speed      float64 `json:"speed"`
	SpeedAvg   float64 `json:"speed_avg"`
	ETA        *int64  `json:"eta,omitempty"`
}

// TransferProgress captures live progress stats from rclone --use-json-log.
type TransferProgress struct {
	Bytes          int64              `json:"bytes"`
	TotalBytes     int64              `json:"total_bytes"`
	Percentage     float64            `json:"percentage"`
	Speed          float64            `json:"speed"` // bytes/sec
	ETA            *int64             `json:"eta,omitempty"`
	Transfers      int64              `json:"transfers"`
	TotalTransfers int64              `json:"total_transfers"`
	Checks         int64              `json:"checks"`
	TotalChecks    int64              `json:"total_checks"`
	Deletes        int64              `json:"deletes"`
	Errors         int64              `json:"errors"`
	ElapsedTime    float64            `json:"elapsed_time"`
	Transferring   []TransferringFile `json:"transferring,omitempty"`
}

// ActiveRunSnapshot represents an active or queued job run sent over SSE.
type ActiveRunSnapshot struct {
	RunID     string           `json:"run_id"`
	JobID     string           `json:"job_id"`
	JobName   string           `json:"job_name"`
	Trigger   string           `json:"trigger"`
	DryRun    bool             `json:"dry_run"`
	Status    string           `json:"status"`
	StartedAt time.Time        `json:"started_at"`
	Progress  TransferProgress `json:"progress"`
}

// ExportBundle is the portable backup/migration format for settings and jobs.
type ExportBundle struct {
	Version    int            `json:"version" yaml:"version"`
	ExportedAt time.Time      `json:"exported_at" yaml:"exported_at"`
	Settings   GlobalSettings `json:"settings" yaml:"settings"`
	Jobs       []Job          `json:"jobs" yaml:"jobs"`
}

// User represents a local administrator account.
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// Session represents an authenticated server-side login session.
type Session struct {
	TokenHash string    `json:"-"`
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// DailyRunStat aggregates finished job runs per calendar day (UTC) for dashboard charts.
type DailyRunStat struct {
	Date             string `json:"date"`
	SuccessCount     int    `json:"success_count"`
	FailedCount      int    `json:"failed_count"`
	OtherCount       int    `json:"other_count"`
	BytesTransferred int64  `json:"bytes_transferred"`
}
