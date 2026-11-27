import type {
  ActiveRunSnapshot,
  AuthStatus,
  AuthUser,
  BrowseEntry,
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
  SystemInfo,
} from '../types/api';

let unauthorizedHandler: (() => void) | null = null;

export function setUnauthorizedHandler(fn: (() => void) | null) {
  unauthorizedHandler = fn;
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
    let errMsg = `HTTP ${res.status}`;
    try {
      const data = await res.json();
      if (data?.error) errMsg = data.error;
    } catch {
      // ignore parse failure
    }
    throw new Error(errMsg);
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

  // System
  getSystemInfo: () => request<SystemInfo>('/api/system/info'),

  // Settings & Config Export/Import
  getSettings: () => request<GlobalSettings>('/api/settings'),
  updateSettings: (settings: GlobalSettings) =>
    request<GlobalSettings>('/api/settings', {
      method: 'PUT',
      body: JSON.stringify(settings),
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
  aboutRemote: (name: string) =>
    request<RemoteAbout>(`/api/remotes/${encodeURIComponent(name)}/about`),

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
  runJob: (id: string, dryRun = false) =>
    request<JobRun>(`/api/jobs/${encodeURIComponent(id)}/run`, {
      method: 'POST',
      body: JSON.stringify({ dry_run: dryRun }),
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
    params: { job_id?: string; status?: string; limit?: number; offset?: number } = {}
  ) => {
    const q = new URLSearchParams();
    if (params.job_id) q.set('job_id', params.job_id);
    if (params.status) q.set('status', params.status);
    if (params.limit) q.set('limit', String(params.limit));
    if (params.offset) q.set('offset', String(params.offset));
    return request<{ runs: JobRun[]; total: number }>(`/api/runs?${q.toString()}`);
  },
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

  // Browse & Restore
  browse: (remote: string, path = '') =>
    request<{ remote: string; path: string; entries: BrowseEntry[] }>(
      `/api/browse?remote=${encodeURIComponent(remote)}&path=${encodeURIComponent(path)}`
    ),
  downloadUrl: (remote: string, path: string) =>
    `/api/browse/download?remote=${encodeURIComponent(remote)}&path=${encodeURIComponent(path)}`,
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

  const open = () => {
    if (closed) return;
    es = new EventSource('/api/events');

    es.onopen = () => {
      handlers.onStatusChange?.(true);
    };

    es.onerror = () => {
      handlers.onStatusChange?.(false);
      handlers.onError?.();
      es?.close();
      if (!closed) {
        retryTimer = window.setTimeout(open, 3000);
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
