import {
  ChevronRight,
  Download,
  File,
  Folder,
  History,
  Loader2,
  RefreshCw,
  RotateCcw,
  Search,
  Square,
} from 'lucide-react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { api } from '../../api/client';
import { StatusBadge } from '../../components/ui/StatusBadge';
import type {
  ActiveRunSnapshot,
  BrowseEntry,
  JobRun,
  JobWithStatus,
  RemoteInfo,
} from '../../types/api';
import { formatBytes, formatDateTime, formatETA, formatSpeed } from '../../utils/format';
import { RestoreModal } from './RestoreModal';
import { VersionHistoryModal } from './VersionHistoryModal';

export function BrowsePage({
  remotes,
  jobs,
  activeRuns,
  initialRemote,
  onToast,
}: {
  remotes: RemoteInfo[];
  jobs: JobWithStatus[];
  activeRuns: ActiveRunSnapshot[];
  initialRemote?: string;
  onToast: (msg: string, isError?: boolean) => void;
}) {
  const [remote, setRemote] = useState(initialRemote || remotes[0]?.name || '');
  const [subpath, setSubpath] = useState('');
  const [entries, setEntries] = useState<BrowseEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [restoreTarget, setRestoreTarget] = useState<{
    entry: BrowseEntry;
    fullPath: string;
  } | null>(null);
  const [versionTarget, setVersionTarget] = useState<{
    entry: BrowseEntry;
    fullPath: string;
  } | null>(null);

  useEffect(() => {
    if (initialRemote) {
      setRemote(initialRemote);
      setSubpath('');
    } else if (!remote && remotes[0]?.name) {
      setRemote(remotes[0].name);
    }
  }, [initialRemote, remotes, remote]);

  const loadDir = useCallback(
    async (targetRemote = remote, targetPath = subpath) => {
      if (!targetRemote) return;
      setLoading(true);
      setError('');
      try {
        const res = await api.browse(targetRemote, targetPath);
        setEntries(res.entries ?? []);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to browse remote');
        setEntries([]);
      } finally {
        setLoading(false);
      }
    },
    [remote, subpath]
  );

  useEffect(() => {
    if (remote) void loadDir(remote, subpath);
  }, [remote, subpath, loadDir]);

  const activeRestores = useMemo(
    () => activeRuns.filter((r) => r.trigger === 'restore' || r.trigger === 'restore_dry_run'),
    [activeRuns]
  );

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return entries;
    return entries.filter((e) => e.Name.toLowerCase().includes(q));
  }, [entries, query]);

  const crumbs = subpath ? subpath.split('/').filter(Boolean) : [];
  const fullEntryPath = (name: string) => (subpath ? `${subpath}/${name}` : name);
  const insideVersionsDir =
    subpath === '_versions' || subpath.startsWith('_versions/') || subpath.includes('/_versions');

  const handleRestoreStarted = (run: JobRun) => {
    onToast(
      run.dry_run
        ? `Started dry-run restore (${run.job_name})`
        : `Restore started (${run.job_name})`
    );
  };

  return (
    <div className="space-y-4">
      {activeRestores.length > 0 && (
        <div className="space-y-2.5">
          {activeRestores.map((run) => {
            const p = run.progress;
            const pct = Math.min(100, Math.max(0, p?.percentage ?? 0));
            return (
              <div
                key={run.run_id}
                className="p-4 rounded-2xl bg-slate-900 border border-emerald-500/40 shadow-lg space-y-2.5"
              >
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="flex items-center gap-2 min-w-0">
                    <span className="font-semibold text-sm text-slate-100 truncate">
                      {run.job_name}
                    </span>
                    <StatusBadge status={run.status} dryRun={run.dry_run} />
                  </div>
                  <button
                    type="button"
                    onClick={() => void api.stopJob(run.job_id)}
                    className="inline-flex items-center gap-1.5 px-3 py-1 rounded-lg text-xs font-semibold bg-rose-500/15 hover:bg-rose-500/25 text-rose-300 border border-rose-500/30"
                  >
                    <Square className="w-3 h-3 fill-current" />
                    Stop
                  </button>
                </div>
                <div className="flex flex-wrap items-center justify-between text-xs text-slate-300 tabular-nums gap-2">
                  <span>
                    {formatBytes(p?.bytes)} / {formatBytes(p?.total_bytes)} ({pct.toFixed(1)}%)
                  </span>
                  <span className="flex items-center gap-3">
                    <span className="text-emerald-400 font-medium">{formatSpeed(p?.speed)}</span>
                    <span className="text-slate-400">ETA: {formatETA(p?.eta)}</span>
                  </span>
                </div>
                <div className="w-full h-2 rounded-full bg-slate-800 overflow-hidden">
                  <div
                    className="h-full bg-gradient-to-r from-emerald-500 to-sky-400 transition-all duration-300"
                    style={{ width: `${pct}%` }}
                  />
                </div>
              </div>
            );
          })}
        </div>
      )}

      <div className="flex flex-wrap items-center gap-2.5">
        <select
          value={remote}
          onChange={(e) => {
            setRemote(e.target.value);
            setSubpath('');
          }}
          className="px-3 py-2 rounded-xl bg-slate-900 border border-slate-800 text-sm text-slate-100 focus:outline-none focus:border-sky-500"
        >
          {remotes.length === 0 && <option value="">No remotes configured</option>}
          {remotes.map((r) => (
            <option key={r.name} value={r.name}>
              {r.name} ({r.type})
            </option>
          ))}
        </select>

        <div className="relative flex-1 min-w-[180px]">
          <Search className="w-4 h-4 text-slate-500 absolute left-3 top-2.5" />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Filter current folder..."
            className="w-full pl-9 pr-3 py-2 rounded-xl bg-slate-900 border border-slate-800 text-sm text-slate-100 focus:outline-none focus:border-sky-500"
          />
        </div>

        <button
          type="button"
          onClick={() => void loadDir()}
          disabled={loading || !remote}
          className="inline-flex items-center gap-1.5 px-3 py-2 rounded-xl text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 disabled:opacity-50"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </button>
      </div>

      {remote && (
        <div className="flex items-center gap-1 px-3.5 py-2.5 rounded-xl bg-slate-900 border border-slate-800 text-xs overflow-x-auto">
          <button
            type="button"
            onClick={() => setSubpath('')}
            className="font-mono font-semibold text-sky-400 hover:underline shrink-0"
          >
            {remote}:
          </button>
          {crumbs.map((c, idx) => {
            const p = crumbs.slice(0, idx + 1).join('/');
            return (
              <span key={p} className="inline-flex items-center gap-1 shrink-0">
                <ChevronRight className="w-3.5 h-3.5 text-slate-600" />
                <button
                  type="button"
                  onClick={() => setSubpath(p)}
                  className="font-mono text-slate-200 hover:text-sky-400"
                >
                  {c}
                </button>
              </span>
            );
          })}
        </div>
      )}

      {error && (
        <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-xs text-rose-300">
          {error}
        </div>
      )}

      <div className="rounded-2xl bg-slate-900/90 border border-slate-800 divide-y divide-slate-800/70 overflow-hidden">
        {loading ? (
          <div className="flex items-center justify-center py-12 text-sm text-slate-400 gap-2">
            <Loader2 className="w-4 h-4 animate-spin" />
            Listing remote contents...
          </div>
        ) : !remote ? (
          <div className="py-12 text-center text-sm text-slate-500">
            Configure a remote first to browse files.
          </div>
        ) : filtered.length === 0 ? (
          <div className="py-12 text-center text-sm text-slate-500">This directory is empty.</div>
        ) : (
          filtered.map((entry) => {
            const fPath = fullEntryPath(entry.Name);
            const isVersionDir = entry.IsDir && entry.Name === '_versions';
            return (
              <div
                key={entry.Path || entry.Name}
                className="px-4 py-3 flex items-center justify-between gap-3 hover:bg-slate-800/40 transition-colors"
              >
                <div className="flex items-center gap-3 min-w-0 flex-1">
                  {entry.IsDir ? (
                    <Folder
                      className={`w-4 h-4 shrink-0 ${
                        isVersionDir ? 'text-emerald-400' : 'text-sky-400'
                      }`}
                    />
                  ) : (
                    <File className="w-4 h-4 text-slate-400 shrink-0" />
                  )}
                  <div className="min-w-0">
                    {entry.IsDir ? (
                      <button
                        type="button"
                        onClick={() => setSubpath(fPath)}
                        className="text-sm font-medium text-slate-100 hover:text-sky-400 truncate flex items-center gap-2 text-left"
                      >
                        <span className="truncate">{entry.Name}</span>
                        {isVersionDir && (
                          <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] bg-emerald-500/15 text-emerald-300 border border-emerald-500/30">
                            <History className="w-2.5 h-2.5" />
                            Recovery Versions
                          </span>
                        )}
                      </button>
                    ) : (
                      <div className="text-sm text-slate-100 truncate">{entry.Name}</div>
                    )}
                    <div className="text-[11px] text-slate-500 tabular-nums">
                      {entry.IsDir ? 'Folder' : formatBytes(entry.Size)}
                      {entry.ModTime && ` · ${formatDateTime(entry.ModTime)}`}
                    </div>
                  </div>
                </div>

                <div className="flex items-center gap-1.5 shrink-0">
                  {!isVersionDir && !insideVersionsDir && (
                    <button
                      type="button"
                      onClick={() => setVersionTarget({ entry, fullPath: fPath })}
                      className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-amber-300"
                      title="View archived versions"
                    >
                      <History className="w-3.5 h-3.5" />
                      <span className="hidden sm:inline">Versions</span>
                    </button>
                  )}
                  {!entry.IsDir && (
                    <a
                      href={api.downloadUrl(remote, fPath)}
                      className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200"
                      title="Download file"
                    >
                      <Download className="w-3.5 h-3.5 text-sky-400" />
                      <span className="hidden sm:inline">Download</span>
                    </a>
                  )}
                  <button
                    type="button"
                    onClick={() => setRestoreTarget({ entry, fullPath: fPath })}
                    className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-emerald-300"
                    title="Restore to path"
                  >
                    <RotateCcw className="w-3.5 h-3.5" />
                    <span className="hidden sm:inline">Restore</span>
                  </button>
                </div>
              </div>
            );
          })
        )}
      </div>

      {restoreTarget && (
        <RestoreModal
          remote={remote}
          entry={restoreTarget.entry}
          fullPath={restoreTarget.fullPath}
          remotes={remotes}
          jobs={jobs}
          onStarted={handleRestoreStarted}
          onClose={() => setRestoreTarget(null)}
        />
      )}

      {versionTarget && (
        <VersionHistoryModal
          remote={remote}
          entry={versionTarget.entry}
          fullPath={versionTarget.fullPath}
          onRestoreVersion={(vEntry, vPath) => setRestoreTarget({ entry: vEntry, fullPath: vPath })}
          onClose={() => setVersionTarget(null)}
        />
      )}
    </div>
  );
}
