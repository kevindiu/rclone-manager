import {
  Copy,
  Edit3,
  FlaskConical,
  History,
  Play,
  Plus,
  Search,
  ShieldCheck,
  Square,
  Trash2,
} from 'lucide-react';
import { useMemo, useState } from 'react';
import { api } from '../../api/client';
import { StatusBadge } from '../../components/ui/StatusBadge';
import type { GlobalSettings, Job, JobWithStatus, RemoteInfo } from '../../types/api';
import { formatDateTime, formatRelative } from '../../utils/format';
import { JobModal } from './JobModal';

export function JobsPage({
  jobs,
  settings,
  remotes,
  onReload,
  onToast,
}: {
  jobs: JobWithStatus[];
  settings: GlobalSettings;
  remotes: RemoteInfo[];
  onReload: () => void;
  onToast: (msg: string, isError?: boolean) => void;
}) {
  const [query, setQuery] = useState('');
  const [editingJob, setEditingJob] = useState<Job | null | undefined>(undefined);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return jobs;
    return jobs.filter(
      (j) =>
        j.name.toLowerCase().includes(q) ||
        j.source.toLowerCase().includes(q) ||
        j.destination.toLowerCase().includes(q)
    );
  }, [jobs, query]);

  const handleAction = async (fn: () => Promise<unknown>, successMsg: string) => {
    try {
      await fn();
      onToast(successMsg);
      onReload();
    } catch (err) {
      onToast(err instanceof Error ? err.message : 'Action failed', true);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="relative flex-1 min-w-[200px] max-w-md">
          <Search className="w-4 h-4 text-slate-500 absolute left-3 top-2.5" />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Filter jobs by name, source, or destination..."
            className="w-full pl-9 pr-3 py-2 rounded-xl bg-slate-900 border border-slate-800 text-sm text-slate-100 focus:outline-none focus:border-sky-500"
          />
        </div>
        <button
          type="button"
          onClick={() => setEditingJob(null)}
          className="inline-flex items-center gap-2 px-4 py-2 rounded-xl text-sm font-semibold bg-sky-500 hover:bg-sky-400 text-slate-950 transition-colors"
        >
          <Plus className="w-4 h-4" />
          New Job
        </button>
      </div>

      {filtered.length === 0 ? (
        <div className="p-10 rounded-2xl bg-slate-900/60 border border-slate-800 text-center space-y-2">
          <p className="text-sm text-slate-300">No backup jobs match your filter.</p>
          {jobs.length === 0 && (
            <button
              type="button"
              onClick={() => setEditingJob(null)}
              className="inline-flex items-center gap-1.5 text-sm font-medium text-sky-400 hover:underline"
            >
              Create your first backup job
            </button>
          )}
        </div>
      ) : (
        <div className="space-y-3">
          {filtered.map((job) => (
            <div
              key={job.id}
              className="p-4 rounded-2xl bg-slate-900/90 border border-slate-800 hover:border-slate-700 transition-colors space-y-3"
            >
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div className="min-w-0 space-y-1">
                  <div className="flex flex-wrap items-center gap-2">
                    <button
                      type="button"
                      onClick={() =>
                        void handleAction(
                          () => api.toggleJob(job.id),
                          job.enabled ? 'Job disabled' : 'Job enabled'
                        )
                      }
                      className={`w-9 h-5 rounded-full transition-colors p-0.5 flex items-center ${
                        job.enabled ? 'bg-emerald-500 justify-end' : 'bg-slate-700 justify-start'
                      }`}
                      title={job.enabled ? 'Disable schedule' : 'Enable schedule'}
                      aria-label={job.enabled ? 'Disable job schedule' : 'Enable job schedule'}
                    >
                      <span className="w-4 h-4 rounded-full bg-white shadow" />
                    </button>
                    <h3 className="text-base font-semibold text-slate-100">{job.name}</h3>
                    <span className="px-2 py-0.5 rounded text-[11px] font-mono uppercase bg-slate-800 text-sky-300 border border-slate-700">
                      {job.operation}
                    </span>
                    {job.versioning_enabled && (
                      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] bg-emerald-500/10 text-emerald-300 border border-emerald-500/30">
                        <History className="w-3 h-3" />
                        Versions (
                        {job.version_retention_days > 0
                          ? `${job.version_retention_days}d`
                          : 'Forever'}
                        )
                      </span>
                    )}
                    {(job.abort_if_source_empty ||
                      job.required_marker_file ||
                      job.max_delete !== null) && (
                      <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded text-[11px] bg-amber-500/10 text-amber-300 border border-amber-500/30">
                        <ShieldCheck className="w-3 h-3" />
                        Guarded
                      </span>
                    )}
                  </div>

                  {job.description && <p className="text-xs text-slate-400">{job.description}</p>}

                  <div className="text-xs font-mono text-slate-300 flex flex-wrap items-center gap-1.5 pt-0.5 break-all">
                    <span className="px-2 py-0.5 rounded bg-slate-950 border border-slate-800">
                      {job.source}
                    </span>
                    <span className="text-slate-500">→</span>
                    <span className="px-2 py-0.5 rounded bg-slate-950 border border-slate-800">
                      {job.destination}
                    </span>
                  </div>
                </div>

                <div className="flex flex-wrap items-center gap-1.5">
                  {job.is_running ? (
                    <button
                      type="button"
                      onClick={() =>
                        void handleAction(() => api.stopJob(job.id), 'Stopping job...')
                      }
                      className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold bg-rose-500/20 hover:bg-rose-500/30 text-rose-300 border border-rose-500/40"
                    >
                      <Square className="w-3.5 h-3.5 fill-current" />
                      Stop
                    </button>
                  ) : (
                    <>
                      <button
                        type="button"
                        onClick={() =>
                          void handleAction(
                            () => api.runJob(job.id, false),
                            `Started "${job.name}"`
                          )
                        }
                        className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold bg-emerald-500/15 hover:bg-emerald-500/25 text-emerald-300 border border-emerald-500/30"
                      >
                        <Play className="w-3.5 h-3.5 fill-current" />
                        Run Now
                      </button>
                      <button
                        type="button"
                        onClick={() =>
                          void handleAction(
                            () => api.runJob(job.id, true),
                            `Started dry run for "${job.name}"`
                          )
                        }
                        className="inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-violet-300"
                        title="Dry Run (no destination changes)"
                      >
                        <FlaskConical className="w-3.5 h-3.5" />
                        Dry Run
                      </button>
                    </>
                  )}
                  <button
                    type="button"
                    onClick={() => setEditingJob(job)}
                    className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300"
                    title="Edit"
                  >
                    <Edit3 className="w-4 h-4" />
                  </button>
                  <button
                    type="button"
                    onClick={() =>
                      void handleAction(() => api.cloneJob(job.id), `Cloned "${job.name}"`)
                    }
                    className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300"
                    title="Clone"
                  >
                    <Copy className="w-4 h-4" />
                  </button>
                  <button
                    type="button"
                    disabled={job.is_running}
                    onClick={() => {
                      if (window.confirm(`Delete job "${job.name}"?`)) {
                        void handleAction(() => api.deleteJob(job.id), `Deleted "${job.name}"`);
                      }
                    }}
                    className="p-1.5 rounded-lg bg-slate-800 hover:bg-rose-500/20 text-slate-400 hover:text-rose-300 disabled:opacity-40"
                    title="Delete"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>
              </div>

              <div className="flex flex-wrap items-center justify-between gap-2 pt-2 border-t border-slate-800/70 text-xs text-slate-400">
                <div className="flex flex-wrap items-center gap-3">
                  <span>
                    Schedule:{' '}
                    <code className="text-slate-200">{job.schedule || 'Manual only'}</code>
                  </span>
                  {job.enabled && job.next_run && (
                    <span>
                      Next: <span className="text-sky-300">{formatRelative(job.next_run)}</span> (
                      {formatDateTime(job.next_run)})
                    </span>
                  )}
                </div>
                {job.last_run && (
                  <div className="flex items-center gap-2">
                    <span>Last run:</span>
                    <StatusBadge status={job.last_run.status} dryRun={job.last_run.dry_run} />
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      {editingJob !== undefined && (
        <JobModal
          job={editingJob}
          settings={settings}
          remotes={remotes}
          onSaved={() => {
            onToast(editingJob ? 'Job updated' : 'Job created');
            onReload();
          }}
          onClose={() => setEditingJob(undefined)}
        />
      )}
    </div>
  );
}
