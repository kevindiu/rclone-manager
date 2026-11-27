import { Activity, Clock, HardDrive, Play, RefreshCw, Terminal, Zap } from 'lucide-react';
import { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../../api/client';
import { ActiveRunCard } from '../../components/ui/ActiveRunCard';
import { StatusBadge } from '../../components/ui/StatusBadge';
import type { Tab } from '../../hooks/useRoute';
import type {
  ActiveRunSnapshot,
  DailyRunStat,
  JobRun,
  JobWithStatus,
  RemoteAbout,
  RemoteInfo,
  SystemInfo,
} from '../../types/api';
import { formatBytes, formatDateTime, formatDurationMs, formatRelative } from '../../utils/format';
import { StatsChart } from './StatsChart';

export function DashboardPage({
  activeRuns,
  jobs,
  remotes,
  recentRuns,
  systemInfo,
  onNavigate,
  onViewLogs,
  onViewActiveLogs,
  onRefreshAll,
  onToast,
}: {
  activeRuns: ActiveRunSnapshot[];
  jobs: JobWithStatus[];
  remotes: RemoteInfo[];
  recentRuns: JobRun[];
  systemInfo: SystemInfo | null;
  onNavigate: (tab: Tab, opts?: { remote?: string }) => void;
  onViewLogs: (run: JobRun) => void;
  onViewActiveLogs: (run: ActiveRunSnapshot) => void;
  onRefreshAll: () => void;
  onToast: (msg: string, isError?: boolean) => void;
}) {
  const [aboutMap, setAboutMap] = useState<Record<string, RemoteAbout>>({});
  const [loadingAbout, setLoadingAbout] = useState<Record<string, boolean>>({});
  const [runStats, setRunStats] = useState<DailyRunStat[]>([]);
  const [loadingStats, setLoadingStats] = useState(true);
  const fetchedRef = useRef<Set<string>>(new Set());

  const handleStop = async (jobId: string) => {
    try {
      await api.stopJob(jobId);
      onToast('Stopping job...');
      onRefreshAll();
    } catch (err) {
      onToast(err instanceof Error ? err.message : 'Failed to stop job', true);
    }
  };

  const handleRunNow = async (job: JobWithStatus) => {
    try {
      await api.runJob(job.id, false);
      onToast(`Started "${job.name}"`);
      onRefreshAll();
    } catch (err) {
      onToast(err instanceof Error ? err.message : 'Failed to start job', true);
    }
  };

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

  useEffect(() => {
    void recentRuns;
    let cancelled = false;
    api
      .getRunStats(14)
      .then((res) => {
        if (!cancelled) setRunStats(res ?? []);
      })
      .catch(() => {})
      .finally(() => {
        if (!cancelled) setLoadingStats(false);
      });
    return () => {
      cancelled = true;
    };
  }, [recentRuns]);

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
          <div className="text-[11px] font-mono text-slate-400 mt-1 truncate">
            App {systemInfo?.app_version || 'dev'}
          </div>
        </div>
      </div>

      {/* Quick Onboarding when freshly installed */}
      {remotes.length === 0 && jobs.length === 0 && (
        <section className="p-5 rounded-2xl bg-sky-500/10 border border-sky-500/30 space-y-3">
          <div className="text-sm font-semibold text-sky-200">Getting Started</div>
          <p className="text-xs text-slate-300">
            Set up your first cloud remote and scheduled backup job in two quick steps:
          </p>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <button
              type="button"
              onClick={() => onNavigate('remotes')}
              className="p-3.5 rounded-xl bg-slate-900/90 border border-slate-800 hover:border-sky-500/50 text-left transition-colors"
            >
              <div className="text-xs font-semibold text-sky-300">1. Add a Storage Remote</div>
              <p className="text-[11px] text-slate-400 mt-1">
                Connect S3, R2, Google Drive, OneDrive, SFTP, or import an existing rclone.conf.
              </p>
            </button>
            <button
              type="button"
              onClick={() => onNavigate('jobs')}
              className="p-3.5 rounded-xl bg-slate-900/90 border border-slate-800 hover:border-sky-500/50 text-left transition-colors"
            >
              <div className="text-xs font-semibold text-emerald-300">2. Create a Backup Job</div>
              <p className="text-[11px] text-slate-400 mt-1">
                Pick local or remote folders, choose Copy or Sync, and set a cron schedule.
              </p>
            </button>
          </div>
        </section>
      )}

      {/* Live Transfers */}
      {activeRuns.length > 0 && (
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

          <div className="space-y-3">
            {activeRuns.map((run) => (
              <ActiveRunCard
                key={run.run_id}
                run={run}
                onViewLogs={onViewActiveLogs}
                onStop={(jobId) => void handleStop(jobId)}
              />
            ))}
          </div>
        </section>
      )}

      {/* Remote Storage Quotas (only show remotes that support quota reporting) */}
      {(() => {
        const quotaRemotes = remotes.filter((rem) => aboutMap[rem.name]?.supported === true);
        if (quotaRemotes.length === 0) return null;
        return (
          <section className="space-y-3">
            <div className="flex items-center justify-between">
              <h2 className="text-sm font-semibold uppercase tracking-wider text-slate-400 flex items-center gap-2">
                <HardDrive className="w-4 h-4 text-emerald-400" />
                Remote Storage Quota
              </h2>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
              {quotaRemotes.map((rem) => {
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
                  </div>
                );
              })}
            </div>
          </section>
        );
      })()}

      {/* Upcoming Schedule & Recent Runs */}
      <StatsChart stats={runStats} loading={loadingStats} />

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
                    onClick={() => void handleRunNow(j)}
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
