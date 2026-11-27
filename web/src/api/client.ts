import type {
  ActiveRunSnapshot,
  AuthStatus,
  AuthUser,
  BrowseEntry,
  BulkDeleteEntry,
  BulkDeleteResponse,
  DailyRunStat,
  DedupeResult,
  DirSize,
  DraftDryRunResult,
  FileVersion,
  GlobalSettings,
  Job,
  JobRun,
  JobWithStatus,
  ParsedLogLine,
  ProviderInfo,
  RemoteAbout,
  RemoteInfo,
  RemoteTestResult,
  RestoreConflictMode,
  SessionInfo,
  SystemInfo,
} from '../types/api';

let unauthorizedHandler: (() => void) | null = null;

export function setUnauthorizedHandler(fn: (() => void) | null) {
  unauthorizedHandler = fn;
}

async function readErrorMessage(res: Response): Promise<string> {
  let errMsg = `HTTP ${res.status}`;
  try {
    const data = await res.json();
    if (data?.error) errMsg = data.error;
  } catch {
    // ignore parse failure
  }
  return errMsg;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    credentials: 'same-origin',
    headers: {
      ...(init?.body && typeof init.body === 'string' && !init.headers
        ? { 'Content-Type': 'application/json' }
        : {}),
      ...(init?.headers ?? {}),
    },
    ...init,
  });

  if (res.status === 401 && !path.startsWith('/api/auth/')) {
    unauthorizedHandler?.();
  }

  if (!res.ok) {
    throw new Error(await readErrorMessage(res));
  }
  return res.json() as Promise<T>;
}

export const api = {
  // Authentication
  authStatus: () => request<AuthStatus>('/api/auth/status'),
  authSetup: (username: string, password: string) =>
    request<{ user: AuthUser }>('/api/auth/setup', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  authLogin: (username: string, password: string) =>
    request<{ user: AuthUser }>('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify({ username, password }),
    }),
  authLogout: () =>
    request<{ status: string }>('/api/auth/logout', {
      method: 'POST',
    }),
  authChangePassword: (current_password: string, new_password: string) =>
    request<{ status: string }>('/api/auth/change-password', {
      method: 'POST',
      body: JSON.stringify({ current_password, new_password }),
    }),
  authSessions: () => request<SessionInfo[]>('/api/auth/sessions'),
  authRevokeOtherSessions: () =>
    request<{ status: string }>('/api/auth/sessions/revoke-others', {
      method: 'POST',
    }),

  // System
  getSystemInfo: () => request<SystemInfo>('/api/system/info'),

  // Settings & Config Export/Import
  getSettings: () => request<GlobalSettings>('/api/settings'),
  updateSettings: (settings: GlobalSettings) =>
    request<GlobalSettings>('/api/settings', {
      method: 'PUT',
      body: JSON.stringify(settings),
    }),
  testWebhook: (opts: {
    webhook_url?: string;
    webhook_type?: string;
    telegram_bot_token?: string;
    telegram_chat_id?: string;
  }) =>
    request<{ status: string }>('/api/settings/test-webhook', {
      method: 'POST',
      body: JSON.stringify(opts),
    }),
  importConfig: (content: string, mode: 'merge' | 'replace', contentType = 'application/json') =>
    request<{ status: string; mode: string; jobs_imported: number }>(
      `/api/config/import?mode=${encodeURIComponent(mode)}`,
      {
        method: 'POST',
        headers: { 'Content-Type': contentType },
        body: content,
      }
    ),
  exportFullBackup: async (passphrase: string): Promise<{ blob: Blob; filename: string }> => {
    const res = await fetch('/api/config/export-full', {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ passphrase }),
    });
    if (res.status === 401) {
      unauthorizedHandler?.();
    }
    if (!res.ok) {
      throw new Error(await readErrorMessage(res));
    }
    const cd = res.headers.get('Content-Disposition') ?? '';
    const quotedMatch = /filename="([^"]+)"/.exec(cd);
    const bareMatch = /filename=([^;\s]+)/.exec(cd);
    const filename = quotedMatch?.[1] || bareMatch?.[1] || 'rclone-manager-backup.rcmbak';
    return { blob: await res.blob(), filename };
  },
  importFullBackup: (content: string, passphrase: string, mode: 'merge' | 'replace') =>
    request<{
      status: string;
      mode: string;
      jobs_imported: number;
      remotes_imported: number;
      remote_names: string[];
    }>('/api/config/import-full', {
      method: 'POST',
      body: JSON.stringify({ content, passphrase, mode }),
    }),

  // Remotes
  listProviders: () => request<ProviderInfo[]>('/api/remotes/providers'),
  listRemotes: () => request<RemoteInfo[]>('/api/remotes'),
  createRemote: (name: string, type: string, params: Record<string, string>) =>
    request<{ name: string; status: string }>('/api/remotes', {
      method: 'POST',
      body: JSON.stringify({ name, type, params }),
    }),
  updateRemote: (name: string, params: Record<string, string>) =>
    request<{ name: string; status: string }>(`/api/remotes/${encodeURIComponent(name)}`, {
      method: 'PUT',
      body: JSON.stringify({ params }),
    }),
  deleteRemote: (name: string) =>
    request<{ name: string; status: string }>(`/api/remotes/${encodeURIComponent(name)}`, {
      method: 'DELETE',
    }),
  importConf: (content: string, mode: 'merge' | 'replace') =>
    request<{ status: string; mode: string; imported: string[] }>('/api/remotes/import-conf', {
      method: 'POST',
      body: JSON.stringify({ content, mode }),
    }),
  testRemote: (name: string) =>
    request<RemoteTestResult>(`/api/remotes/${encodeURIComponent(name)}/test`, {
      method: 'POST',
    }),
  testDraftRemote: (name: string, type: string, params: Record<string, string>) =>
    request<RemoteTestResult>('/api/remotes/test-draft', {
      method: 'POST',
      body: JSON.stringify({ name, type, params }),
    }),
  aboutRemote: (name: string) =>
    request<RemoteAbout>(`/api/remotes/${encodeURIComponent(name)}/about`),
  cleanupRemote: (name: string) =>
    request<{ name: string; status: string }>(`/api/remotes/${encodeURIComponent(name)}/cleanup`, {
      method: 'POST',
    }),
  dedupeRemote: (name: string, opts: { path?: string; mode?: string; dry_run?: boolean } = {}) =>
    request<DedupeResult>(`/api/remotes/${encodeURIComponent(name)}/dedupe`, {
      method: 'POST',
      body: JSON.stringify(opts),
    }),

  // Jobs
  listJobs: () => request<JobWithStatus[]>('/api/jobs'),
  createJob: (job: Partial<Job>) =>
    request<Job>('/api/jobs', {
      method: 'POST',
      body: JSON.stringify(job),
    }),
  updateJob: (id: string, job: Partial<Job>) =>
    request<Job>(`/api/jobs/${encodeURIComponent(id)}`, {
      method: 'PUT',
      body: JSON.stringify(job),
    }),
  previewJobCommand: (job: Partial<Job>, dry_run = false, resync = false) =>
    request<{ command: string }>('/api/jobs/preview-command', {
      method: 'POST',
      body: JSON.stringify({ job, dry_run, resync }),
    }),
  dryRunDraftJob: (job: Partial<Job>) =>
    request<DraftDryRunResult>('/api/jobs/dry-run', {
      method: 'POST',
      body: JSON.stringify(job),
    }),
  deleteJob: (id: string) =>
    request<{ status: string }>(`/api/jobs/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    }),
  cloneJob: (id: string) =>
    request<Job>(`/api/jobs/${encodeURIComponent(id)}/clone`, {
      method: 'POST',
    }),
  toggleJob: (id: string) =>
    request<Job>(`/api/jobs/${encodeURIComponent(id)}/toggle`, {
      method: 'POST',
    }),
  runJob: (id: string, dryRun = false, resync = false) =>
    request<JobRun>(`/api/jobs/${encodeURIComponent(id)}/run`, {
      method: 'POST',
      body: JSON.stringify({ dry_run: dryRun, resync }),
    }),
  stopJob: (id: string) =>
    request<{ status: string }>(`/api/jobs/${encodeURIComponent(id)}/stop`, {
      method: 'POST',
    }),
  previewCron: (schedule: string, timezone = '', count = 5) =>
    request<{ timezone: string; next: string[] }>('/api/cron/preview', {
      method: 'POST',
      body: JSON.stringify({ schedule, timezone, count }),
    }),

  // Runs & Logs
  listRuns: (
    params: {
      job_id?: string;
      status?: string;
      trigger?: string;
      limit?: number;
      offset?: number;
    } = {}
  ) => {
    const q = new URLSearchParams();
    if (params.job_id) q.set('job_id', params.job_id);
    if (params.status) q.set('status', params.status);
    if (params.trigger) q.set('trigger', params.trigger);
    if (params.limit) q.set('limit', String(params.limit));
    if (params.offset) q.set('offset', String(params.offset));
    return request<{ runs: JobRun[]; total: number }>(`/api/runs?${q.toString()}`);
  },
  getRun: (id: string) => request<JobRun>(`/api/runs/${encodeURIComponent(id)}`),
  getRunLogs: (id: string) =>
    request<{
      run_id: string;
      lines: ParsedLogLine[];
      raw: string;
      truncated?: boolean;
      total_bytes?: number;
    }>(`/api/runs/${encodeURIComponent(id)}/logs`),
  deleteRun: (id: string) =>
    request<{ status: string }>(`/api/runs/${encodeURIComponent(id)}`, {
      method: 'DELETE',
    }),
  getRunStats: (days = 14) => request<DailyRunStat[]>(`/api/runs/stats?days=${days}`),
  clearRuns: (params: { job_id?: string; status?: string; trigger?: string } = {}) => {
    const q = new URLSearchParams();
    if (params.job_id) q.set('job_id', params.job_id);
    if (params.status) q.set('status', params.status);
    if (params.trigger) q.set('trigger', params.trigger);
    const qs = q.toString();
    return request<{ deleted: number }>(`/api/runs${qs ? `?${qs}` : ''}`, {
      method: 'DELETE',
    });
  },

  // Browse & Restore
  browse: (remote: string, path = '') =>
    request<{ remote: string; path: string; entries: BrowseEntry[] }>(
      `/api/browse?remote=${encodeURIComponent(remote)}&path=${encodeURIComponent(path)}`
    ),
  browseLocal: (path = '/') =>
    request<{ path: string; entries: BrowseEntry[] }>(
      `/api/browse/local?path=${encodeURIComponent(path)}`
    ),
  browseSize: (remote: string, path = '') =>
    request<DirSize>(
      `/api/browse/size?remote=${encodeURIComponent(remote)}&path=${encodeURIComponent(path)}`
    ),
  browseMkdir: (remote: string, path: string) =>
    request<{ status: string }>('/api/browse/mkdir', {
      method: 'POST',
      body: JSON.stringify({ remote, path }),
    }),
  browseUpload: (remote: string, dirPath: string, file: File) => {
    const fd = new FormData();
    fd.append('file', file);
    return request<{ status: string; path: string; name: string }>(
      `/api/browse/upload?remote=${encodeURIComponent(remote)}&path=${encodeURIComponent(dirPath)}`,
      {
        method: 'POST',
        body: fd,
      }
    );
  },
  browseRename: (remote: string, path: string, new_name: string) =>
    request<{ status: string; path: string }>('/api/browse/rename', {
      method: 'POST',
      body: JSON.stringify({ remote, path, new_name }),
    }),
  browseDelete: (remote: string, path: string, is_dir: boolean) =>
    request<{ status: string }>(
      `/api/browse/entry?remote=${encodeURIComponent(remote)}&path=${encodeURIComponent(path)}&is_dir=${is_dir ? '1' : '0'}`,
      { method: 'DELETE' }
    ),
  browseBulkDelete: (remote: string, entries: BulkDeleteEntry[]) =>
    request<BulkDeleteResponse>('/api/browse/bulk-delete', {
      method: 'POST',
      body: JSON.stringify({ remote, entries }),
    }),
  downloadUrl: (remote: string, path: string) =>
    `/api/browse/download?remote=${encodeURIComponent(remote)}&path=${encodeURIComponent(path)}`,
  previewUrl: (remote: string, path: string) =>
    `/api/browse/download?remote=${encodeURIComponent(remote)}&path=${encodeURIComponent(path)}&inline=1`,
  listVersions: (remote: string, path: string) =>
    request<{ remote: string; path: string; versions: FileVersion[] }>(
      `/api/browse/versions?remote=${encodeURIComponent(remote)}&path=${encodeURIComponent(path)}`
    ),
  restore: (
    source_remote: string,
    source_path: string,
    target_path: string,
    is_dir: boolean,
    opts?: { conflict_mode?: RestoreConflictMode; dry_run?: boolean }
  ) =>
    request<JobRun>('/api/browse/restore', {
      method: 'POST',
      body: JSON.stringify({
        source_remote,
        source_path,
        target_path,
        is_dir,
        conflict_mode: opts?.conflict_mode ?? 'overwrite',
        dry_run: opts?.dry_run ?? false,
      }),
    }),
};

export interface SSEHandlers {
  onSnapshot?: (runs: ActiveRunSnapshot[]) => void;
  onRunStarted?: (run: ActiveRunSnapshot) => void;
  onProgress?: (run: ActiveRunSnapshot) => void;
  onRunCompleted?: (run: JobRun) => void;
  onJobUpdated?: () => void;
  onStatusChange?: (connected: boolean) => void;
  onError?: () => void;
}

export function connectSSE(handlers: SSEHandlers): () => void {
  let es: EventSource | null = null;
  let closed = false;
  let retryTimer: number | undefined;
  let retryDelay = 3000;
  const maxRetryDelay = 30000;

  const open = () => {
    if (closed) return;
    es = new EventSource('/api/events');

    es.onopen = () => {
      retryDelay = 3000;
      handlers.onStatusChange?.(true);
    };

    es.onerror = () => {
      handlers.onStatusChange?.(false);
      handlers.onError?.();
      es?.close();
      if (!closed) {
        retryTimer = window.setTimeout(open, retryDelay);
        retryDelay = Math.min(maxRetryDelay, retryDelay * 2);
      }
    };

    es.addEventListener('snapshot', (e) => {
      try {
        const data = JSON.parse((e as MessageEvent).data) as { active_runs: ActiveRunSnapshot[] };
        handlers.onSnapshot?.(data.active_runs ?? []);
      } catch {
        // ignore
      }
    });

    es.addEventListener('run_started', (e) => {
      try {
        handlers.onRunStarted?.(JSON.parse((e as MessageEvent).data));
      } catch {
        // ignore
      }
    });

    es.addEventListener('progress', (e) => {
      try {
        handlers.onProgress?.(JSON.parse((e as MessageEvent).data));
      } catch {
        // ignore
      }
    });

    es.addEventListener('run_completed', (e) => {
      try {
        handlers.onRunCompleted?.(JSON.parse((e as MessageEvent).data));
      } catch {
        // ignore
      }
    });

    es.addEventListener('job_updated', () => {
      handlers.onJobUpdated?.();
    });
  };

  open();

  return () => {
    closed = true;
    if (retryTimer) window.clearTimeout(retryTimer);
    es?.close();
  };
}
