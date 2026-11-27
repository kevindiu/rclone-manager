export interface GlobalSettings {
  transfers: number;
  checkers: number;
  bwlimit: string;
  retries: number;
  low_level_retries: number;
  timeout: string;
  contimeout: string;
  log_retention_days: number;
  timezone: string;
  max_concurrent_jobs: number;
  webhook_url?: string;
  webhook_on?: 'off' | 'failure' | 'always' | '';
  webhook_type?: 'generic' | 'ntfy' | 'telegram' | '';
  telegram_bot_token?: string;
  telegram_chat_id?: string;
}

export interface Job {
  id: string;
  name: string;
  description?: string;
  enabled: boolean;
  operation: 'copy' | 'sync' | 'move' | 'check' | 'bisync';
  source: string;
  destination: string;
  schedule?: string;
  exclude_patterns?: string[];
  include_patterns?: string[];
  filter_rules?: string[];
  custom_flags?: string;
  fast_list?: boolean;
  checksum?: boolean;
  update_only?: boolean;
  server_side_across_configs?: boolean;
  delete_timing?: '' | 'during' | 'after' | 'before';
  delete_empty_src_dirs?: boolean;
  symlink_mode?: '' | 'links' | 'copy_links';
  size_only?: boolean;
  ignore_existing?: boolean;
  min_age?: string;
  max_age?: string;
  min_size?: string;
  max_size?: string;
  bisync_conflict_resolve?:
    | ''
    | 'none'
    | 'path1'
    | 'path2'
    | 'newer'
    | 'older'
    | 'larger'
    | 'smaller';
  notify_override?: '' | 'failure' | 'always' | 'off';
  max_transfer?: string;
  cutoff_mode?: 'soft' | 'hard' | 'cautious' | '';
  versioning_enabled: boolean;
  backup_dir_base?: string;
  version_format?: string;
  version_retention_days: number;
  abort_if_source_empty: boolean;
  required_marker_file?: string;
  max_delete?: number | null;
  transfers?: number | null;
  checkers?: number | null;
  bwlimit?: string;
  retries?: number | null;
  low_level_retries?: number | null;
  timeout?: string;
  contimeout?: string;
  created_at?: string;
  updated_at?: string;
}

export interface JobWithStatus extends Job {
  next_run?: string;
  is_running: boolean;
  active_run_id?: string;
  last_run?: JobRun;
}

export interface JobRun {
  id: string;
  job_id: string;
  job_name: string;
  trigger: 'manual' | 'schedule' | 'dry_run' | 'restore' | 'restore_dry_run' | string;
  dry_run: boolean;
  status: 'queued' | 'running' | 'success' | 'failed' | 'cancelled' | 'skipped' | string;
  started_at: string;
  finished_at?: string;
  duration_ms: number;
  bytes_transferred: number;
  total_bytes: number;
  files_transferred: number;
  files_checked: number;
  files_deleted: number;
  errors_count: number;
  error_message?: string;
  command_preview?: string;
  log_file?: string;
  backup_dir_used?: string;
  pruned_versions?: string[];
}

export interface TransferringFile {
  name: string;
  size: number;
  bytes: number;
  percentage: number;
  speed: number;
  speed_avg: number;
  eta?: number;
}

export interface TransferProgress {
  bytes: number;
  total_bytes: number;
  percentage: number;
  speed: number;
  eta?: number;
  transfers: number;
  total_transfers: number;
  checks: number;
  total_checks: number;
  deletes: number;
  errors: number;
  elapsed_time: number;
  transferring?: TransferringFile[];
}

export interface ActiveRunSnapshot {
  run_id: string;
  job_id: string;
  job_name: string;
  trigger: string;
  dry_run: boolean;
  status: string;
  started_at: string;
  progress: TransferProgress;
}

export interface RemoteInfo {
  name: string;
  type: string;
  description?: string;
  config: Record<string, string>;
  masked_keys?: string[];
}

export interface ProviderExample {
  Value: string;
  Help: string;
  Provider?: string;
}

export interface ProviderOption {
  Name: string;
  Help: string;
  Provider?: string;
  DefaultStr?: string;
  Required: boolean;
  IsPassword: boolean;
  Advanced: boolean;
  Sensitive: boolean;
  Type: string;
  Examples?: ProviderExample[];
}

export interface ProviderInfo {
  Name: string;
  Description: string;
  Prefix: string;
  is_oauth: boolean;
  Options: ProviderOption[];
}

export interface RemoteTestResult {
  remote: string;
  ok: boolean;
  latency_ms: number;
  message?: string;
  error?: string;
}

export interface RemoteAbout {
  remote: string;
  supported: boolean;
  total?: number;
  used?: number;
  free?: number;
  trashed?: number;
  other?: number;
  error?: string;
}

export interface BrowseEntry {
  Path: string;
  Name: string;
  Size: number;
  MimeType?: string;
  ModTime?: string;
  IsDir: boolean;
}

export interface DirSize {
  remote: string;
  path: string;
  count: number;
  bytes: number;
}

export type RestoreConflictMode = 'overwrite' | 'skip' | 'newer_only';

export interface FileVersion {
  date: string;
  source_path: string;
  size: number;
  mod_time?: string;
  is_dir: boolean;
  backup_root: string;
}

export interface SystemInfo {
  app_version: string;
  rclone_version: string;
  rclone_os_arch?: string;
  go_version: string;
  data_dir: string;
  rclone_config: string;
  timezone: string;
  uptime_seconds: number;
}

export interface ParsedLogLine {
  time?: string;
  level?: string;
  msg: string;
  object?: string;
  size?: number;
  is_stats?: boolean;
}

export interface AuthUser {
  id: string;
  username: string;
}

export interface AuthStatus {
  authenticated: boolean;
  setup_required: boolean;
  user?: AuthUser;
}

export interface SessionInfo {
  id: string;
  created_at: string;
  expires_at: string;
  current: boolean;
}

export interface DailyRunStat {
  date: string;
  success_count: number;
  failed_count: number;
  other_count: number;
  bytes_transferred: number;
}

export interface DedupeResult {
  remote: string;
  mode: string;
  dry_run: boolean;
  output: string;
}

export interface BulkDeleteEntry {
  path: string;
  is_dir: boolean;
}

export interface BulkDeleteResponse {
  deleted: number;
  failed: { path: string; error: string }[];
}

export interface DraftDryRunResult {
  ok: boolean;
  duration_ms: number;
  command: string;
  summary: string;
  lines?: string[];
  error?: string;
}
