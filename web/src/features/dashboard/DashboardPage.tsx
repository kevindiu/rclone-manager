import { Activity, Clock, HardDrive, Play, RefreshCw, Square, Terminal, Zap } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../../api/client';
import { StatusBadge } from '../../components/ui/StatusBadge';
import type { Tab } from '../../hooks/useRoute';
import type {
  ActiveRunSnapshot,
  JobRun,
  JobWithStatus,
  RemoteAbout,
  RemoteInfo,
  SystemInfo,
} from '../../types/api';
import {
  formatBytes,
  formatDateTime,
  formatDurationMs,
  formatETA,
  formatRelative,
  formatSpeed,
} from '../../utils/format';

export function DashboardPage({
  activeRuns,
  jobs,
  remotes,
  recentRuns,
  systemInfo,
  onNavigate,
  onViewLogs,
  onRefreshAll,
}: {
  activeRuns: ActiveRunSnapshot[];
  jobs: JobWithStatus[];
  remotes: RemoteInfo[];
  recentRuns: JobRun[];
  systemInfo: SystemInfo | null;
  onNavigate: (tab: Tab, opts?: { remote?: string }) => void;
  onViewLogs: (run: JobRun) => void;
  onRefreshAll: () => void;
}) {
  const [aboutMap, setAboutMap] = useState<Record<string, RemoteAbout>>({});
  const [loadingAbout, setLoadingAbout] = useState<Record<string, boolean>>({});
  const fetchedRef = useRef<Set<string>>(new Set());

  const fetchAbout = useCallback(async (name: string) => {
    setLoadingAbout((p) => ({ ...p, [name]: true }));
    try {
      const res = await api.aboutRemote(name);
      setAboutMap((p) => ({ ...p, [name]: res }));
    } catch {
      // ignore transient about errors
    } finally {
      setLoadingAbout((p) => ({ ...p, [name]: false }));
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    const activeNames = new Set(remotes.map((r) => r.name));
    for (const seen of fetchedRef.current) {
      if (!activeNames.has(seen)) {
        fetchedRef.current.delete(seen);
      }
    }
    for (const r of remotes) {
      if (!fetchedRef.current.has(r.name)) {
        fetchedRef.current.add(r.name);
        if (!cancelled) {
          void fetchAbout(r.name);
        }
      }
    }
    return () => {
      cancelled = true;
    };
  }, [remotes, fetchAbout]);

  const enabledScheduled = jobs
    .filter((j) => j.enabled && j.next_run)
    .sort((a, b) => new Date(a.next_run ?? 0).getTime() - new Date(b.next_run ?? 0).getTime())
    .slice(0, 5);

  return (
    <div className="space-y-6">
      {/* KPI Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 sm:gap-4">
        <div className="p-4 rounded-2xl bg-slate-900/90 border border-slate-800">
          <div className="text-xs text-slate-400">Active Transfers</div>
          <div className="mt-1.5 flex items-baseline gap-2">
            <span className="text-2xl font-bold text-slate-100 tabular-nums">
              {activeRuns.length}
            </span>
            {activeRuns.length > 0 && (
              <span className="text-xs font-medium text-sky-400 flex items-center gap-1">
                <Activity className="w-3.5 h-3.5 animate-pulse" />
                Live
              </span>
            )}
          </div>
        </div>

        <div className="p-4 rounded-2xl bg-slate-900/90 border border-slate-800">
          <div className="text-xs text-slate-400">Backup Jobs</div>
          <div className="mt-1.5 flex items-baseline gap-2">
            <span className="text-2xl font-bold text-slate-100 tabular-nums">{jobs.length}</span>
            <span className="text-xs text-emerald-400">
              {jobs.filter((j) => j.enabled).length} enabled
            </span>
          </div>
        </div>

        <div className="p-4 rounded-2xl bg-slate-900/90 border border-slate-800">
          <div className="text-xs text-slate-400">Configured Remotes</div>
          <div className="mt-1.5 flex items-baseline gap-2">
            <span className="text-2xl font-bold text-slate-100 tabular-nums">{remotes.length}</span>
            <button
              type="button"
              onClick={() => onNavigate('remotes')}
              className="text-xs text-sky-400 hover:underline"
            >
              Manage
            </button>
          </div>
        </div>

        <div className="p-4 rounded-2xl bg-slate-900/90 border border-slate-800">
          <div className="text-xs text-slate-400">Bundled rclone</div>
          <div className="mt-1.5 flex items-baseline gap-2">
            <span className="text-lg sm:text-xl font-bold font-mono text-emerald-400 truncate">
              {systemInfo?.rclone_version || '—'}
            </span>
          </div>
        </div>
      </div>

      {/* Live Transfers */}
      <section className="space-y-3">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-semibold uppercase tracking-wider text-slate-400 flex items-center gap-2">
            <Zap className="w-4 h-4 text-sky-400" />
            Live Progress
          </h2>
          <button
            type="button"
            onClick={onRefreshAll}
            className="inline-flex items-center gap-1.5 text-xs text-slate-400 hover:text-slate-200"
          >
            <RefreshCw className="w-3.5 h-3.5" />
            Refresh
          </button>
        </div>

        {activeRuns.length === 0 ? (
          <div className="p-6 rounded-2xl bg-slate-900/50 border border-slate-800/80 text-center text-sm text-slate-400">
            No active jobs running right now. Trigger a job from{' '}
            <button
              type="button"
              onClick={() => onNavigate('jobs')}
              className="text-sky-400 hover:underline font-medium"
            >
              Jobs
            </button>{' '}
            to watch live transfer speed, ETA, and per-file progress here.
          </div>
        ) : (
          <div className="space-y-3">
            {activeRuns.map((run) => {
              const p = run.progress;
              const pct = Math.min(100, Math.max(0, p?.percentage ?? 0));
              return (
                <div
                  key={run.run_id}
                  className="p-4 sm:p-5 rounded-2xl bg-slate-900 border border-sky-500/40 shadow-lg space-y-3"
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div className="flex items-center gap-2.5 min-w-0">
                      <span className="font-semibold text-slate-100 truncate">{run.job_name}</span>
                      <StatusBadge status={run.status} dryRun={run.dry_run} />
                    </div>
                    <button
                      type="button"
                      onClick={() => void api.stopJob(run.job_id)}
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold bg-rose-500/15 hover:bg-rose-500/25 text-rose-300 border border-rose-500/30"
                    >
                      <Square className="w-3.5 h-3.5 fill-current" />
                      Stop
                    </button>
                  </div>

                  <div>
                    <div className="flex flex-wrap items-center justify-between text-xs text-slate-300 mb-1.5 tabular-nums gap-2">
                      <span>
                        {formatBytes(p?.bytes)} / {formatBytes(p?.total_bytes)} ({pct.toFixed(1)}%)
                      </span>
                      <span className="flex items-center gap-3">
                        <span className="text-emerald-400 font-medium">
                          {formatSpeed(p?.speed)}
                        </span>
                        <span className="text-slate-400">ETA: {formatETA(p?.eta)}</span>
                        <span className="text-slate-400">
                          Files: {p?.transfers ?? 0}/{p?.total_transfers ?? 0}
                        </span>
                      </span>
                    </div>
                    <div className="w-full h-2.5 rounded-full bg-slate-800 overflow-hidden">
                      <div
                        className="h-full bg-gradient-to-r from-sky-500 to-emerald-400 transition-all duration-300"
                        style={{ width: `${pct}%` }}
                      />
                    </div>
                  </div>

                  {p?.transferring && p.transferring.length > 0 && (
                    <div className="pt-2 border-t border-slate-800/80 space-y-1.5">
                      <div className="text-[11px] font-medium text-slate-400">
                        Currently Transferring ({p.transferring.length})
                      </div>
                      {p.transferring.slice(0, 6).map((f) => (
                        <div key={f.name} className="text-xs flex items-center gap-3">
                          <span className="font-mono text-slate-200 truncate flex-1">{f.name}</span>
                          <span className="text-slate-400 tabular-nums shrink-0">
                            {formatSpeed(f.speed)}
                          </span>
                          <div className="w-20 h-1.5 rounded-full bg-slate-800 overflow-hidden shrink-0">
                            <div
                              className="h-full bg-sky-400"
                              style={{ width: `${Math.min(100, f.percentage)}%` }}
                            />
                          </div>
                          <span className="w-9 text-right text-slate-300 tabular-nums shrink-0">
                            {f.percentage}%
                          </span>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        )}
      </section>

      {/* Remote Storage Quotas */}
      {remotes.length > 0 && (
        <section className="space-y-3">
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-semibold uppercase tracking-wider text-slate-400 flex items-center gap-2">
              <HardDrive className="w-4 h-4 text-emerald-400" />
              Remote Storage Quota
            </h2>
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
            {remotes.map((rem) => {
              const about = aboutMap[rem.name];
              const usedPct =
                about?.supported && about.total && about.used !== undefined
                  ? Math.min(100, (about.used / about.total) * 100)
                  : null;
              return (
                <div
                  key={rem.name}
                  className="p-4 rounded-2xl bg-slate-900/80 border border-slate-800 space-y-2.5"
                >
                  <div className="flex items-center justify-between gap-2">
                    <div className="min-w-0">
                      <div className="font-semibold text-sm text-slate-100 truncate">
                        {rem.name}
                      </div>
                      <div className="text-[11px] font-mono text-slate-400">{rem.type}</div>
                    </div>
                    <button
                      type="button"
                      onClick={() => void fetchAbout(rem.name)}
                      className="p-1.5 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-slate-800"
                      title="Refresh quota"
                    >
                      <RefreshCw
                        className={`w-3.5 h-3.5 ${loadingAbout[rem.name] ? 'animate-spin' : ''}`}
                      />
                    </button>
                  </div>

                  {!about ? (
                    <div className="text-xs text-slate-500">Checking quota...</div>
                  ) : !about.supported ? (
                    <div className="text-xs text-slate-500">
                      Quota reporting not supported by this backend
                    </div>
                  ) : (
                    <div className="space-y-1.5">
                      <div className="flex justify-between text-xs text-slate-300 tabular-nums">
                        <span>Used: {formatBytes(about.used)}</span>
                        <span>Free: {formatBytes(about.free)}</span>
                      </div>
                      {usedPct !== null && (
                        <div className="w-full h-2 rounded-full bg-slate-800 overflow-hidden">
                          <div
                            className={`h-full rounded-full ${
                              usedPct > 85 ? 'bg-amber-400' : 'bg-emerald-400'
                            }`}
                            style={{ width: `${usedPct}%` }}
                          />
                        </div>
                      )}
                      {about.total !== undefined && (
                        <div className="text-[11px] text-slate-500 tabular-nums">
                          Total capacity: {formatBytes(about.total)}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </section>
      )}

      {/* Upcoming Schedule & Recent Runs */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <section className="p-4 rounded-2xl bg-slate-900/80 border border-slate-800 space-y-3">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-slate-200 flex items-center gap-2">
              <Clock className="w-4 h-4 text-sky-400" />
              Upcoming Scheduled Jobs
            </h3>
            <button
              type="button"
              onClick={() => onNavigate('jobs')}
              className="text-xs text-sky-400 hover:underline"
            >
              All Jobs
            </button>
          </div>
          {enabledScheduled.length === 0 ? (
            <p className="text-xs text-slate-500 py-4">No scheduled jobs enabled.</p>
          ) : (
            <div className="divide-y divide-slate-800/70">
              {enabledScheduled.map((j) => (
                <div key={j.id} className="py-2.5 flex items-center justify-between gap-3">
                  <div className="min-w-0">
                    <div className="text-sm font-medium text-slate-100 truncate">{j.name}</div>
                    <div className="text-xs text-slate-400 font-mono truncate">
                      {j.schedule} · {formatRelative(j.next_run)}
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={() => void api.runJob(j.id, false)}
                    disabled={j.is_running}
                    className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-emerald-400 disabled:opacity-40"
                    title="Run now"
                  >
                    <Play className="w-3.5 h-3.5 fill-current" />
                  </button>
                </div>
              ))}
            </div>
          )}
        </section>

        <section className="p-4 rounded-2xl bg-slate-900/80 border border-slate-800 space-y-3">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-slate-200 flex items-center gap-2">
              <Terminal className="w-4 h-4 text-emerald-400" />
              Recent Runs
            </h3>
            <button
              type="button"
              onClick={() => onNavigate('history')}
              className="text-xs text-sky-400 hover:underline"
            >
              View History
            </button>
          </div>
          {recentRuns.length === 0 ? (
            <p className="text-xs text-slate-500 py-4">No job runs recorded yet.</p>
          ) : (
            <div className="divide-y divide-slate-800/70">
              {recentRuns.slice(0, 5).map((r) => (
                <div key={r.id} className="py-2.5 flex items-center justify-between gap-3">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <span className="text-sm font-medium text-slate-100 truncate">
                        {r.job_name}
                      </span>
                      <StatusBadge status={r.status} dryRun={r.dry_run} />
                    </div>
                    <div className="text-xs text-slate-400 mt-0.5 tabular-nums">
                      {formatDateTime(r.started_at)} · {formatDurationMs(r.duration_ms)} ·{' '}
                      {formatBytes(r.bytes_transferred)}
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={() => onViewLogs(r)}
                    className="px-2.5 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 shrink-0"
                  >
                    Logs
                  </button>
                </div>
              ))}
            </div>
          )}
        </section>
      </div>
    </div>
  );
}
