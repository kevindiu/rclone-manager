import { Loader2, RefreshCw, Terminal, Trash2 } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';
import { api } from '../../api/client';
import { ConfirmModal } from '../../components/ui/ConfirmModal';
import { StatusBadge } from '../../components/ui/StatusBadge';
import type { JobRun, JobWithStatus } from '../../types/api';
import { formatBytes, formatDateTime, formatDurationMs } from '../../utils/format';

export function HistoryPage({
  jobs,
  refreshSignal,
  onViewLogs,
  onToast,
}: {
  jobs: JobWithStatus[];
  refreshSignal: number;
  onViewLogs: (run: JobRun) => void;
  onToast: (msg: string, isError?: boolean) => void;
}) {
  const [runs, setRuns] = useState<JobRun[]>([]);
  const [total, setTotal] = useState(0);
  const [jobFilter, setJobFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [triggerFilter, setTriggerFilter] = useState('');
  const [offset, setOffset] = useState(0);
  const [loading, setLoading] = useState(false);
  const [confirmDeleteRun, setConfirmDeleteRun] = useState<JobRun | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [confirmClear, setConfirmClear] = useState(false);
  const [clearing, setClearing] = useState(false);
  const limit = 25;

  const loadRuns = useCallback(async () => {
    setLoading(true);
    try {
      const res = await api.listRuns({
        job_id: jobFilter || undefined,
        status: statusFilter || undefined,
        trigger: triggerFilter || undefined,
        limit,
        offset,
      });
      setRuns(res.runs ?? []);
      setTotal(res.total ?? 0);
    } catch (err) {
      onToast(err instanceof Error ? err.message : 'Failed to load runs', true);
    } finally {
      setLoading(false);
    }
  }, [jobFilter, statusFilter, triggerFilter, offset, onToast]);

  useEffect(() => {
    void refreshSignal;
    void loadRuns();
  }, [loadRuns, refreshSignal]);

  const handleDelete = async (run: JobRun) => {
    setDeleting(true);
    try {
      await api.deleteRun(run.id);
      onToast('Run record deleted');
      setConfirmDeleteRun(null);
      void loadRuns();
    } catch (err) {
      onToast(err instanceof Error ? err.message : 'Delete failed', true);
    } finally {
      setDeleting(false);
    }
  };

  const handleClearFiltered = async () => {
    setClearing(true);
    try {
      const res = await api.clearRuns({
        job_id: jobFilter || undefined,
        status: statusFilter || undefined,
        trigger: triggerFilter || undefined,
      });
      onToast(`Cleared ${res.deleted} finished run record(s)`);
      setConfirmClear(false);
      setOffset(0);
      void loadRuns();
    } catch (err) {
      onToast(err instanceof Error ? err.message : 'Clear failed', true);
    } finally {
      setClearing(false);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-2.5">
        <div className="flex flex-wrap items-center gap-2">
          <select
            value={jobFilter}
            onChange={(e) => {
              setJobFilter(e.target.value);
              setOffset(0);
            }}
            className="px-3 py-2 rounded-xl bg-slate-900 border border-slate-800 text-xs sm:text-sm text-slate-100"
          >
            <option value="">All Jobs</option>
            {jobs.map((j) => (
              <option key={j.id} value={j.id}>
                {j.name}
              </option>
            ))}
          </select>

          <select
            value={statusFilter}
            onChange={(e) => {
              setStatusFilter(e.target.value);
              setOffset(0);
            }}
            className="px-3 py-2 rounded-xl bg-slate-900 border border-slate-800 text-xs sm:text-sm text-slate-100"
          >
            <option value="">All Statuses</option>
            <option value="success">Success</option>
            <option value="failed">Failed</option>
            <option value="running">Running</option>
            <option value="cancelled">Cancelled</option>
            <option value="skipped">Skipped</option>
          </select>

          <select
            value={triggerFilter}
            onChange={(e) => {
              setTriggerFilter(e.target.value);
              setOffset(0);
            }}
            className="px-3 py-2 rounded-xl bg-slate-900 border border-slate-800 text-xs sm:text-sm text-slate-100"
          >
            <option value="">All Triggers</option>
            <option value="schedule">Schedule</option>
            <option value="manual">Manual</option>
            <option value="restore">Restore</option>
            <option value="restore_dry_run">Restore (Dry Run)</option>
            <option value="dry_run">Dry Run</option>
          </select>
        </div>

        <div className="flex items-center gap-2">
          <button
            type="button"
            disabled={total === 0}
            onClick={() => setConfirmClear(true)}
            className="inline-flex items-center gap-1.5 px-3 py-2 rounded-xl text-xs font-medium bg-rose-500/15 hover:bg-rose-500/25 text-rose-300 border border-rose-500/30 disabled:opacity-40"
            title="Delete finished runs matching the current filters"
          >
            <Trash2 className="w-3.5 h-3.5" />
            Clear Filtered
          </button>
          <button
            type="button"
            onClick={() => void loadRuns()}
            className="inline-flex items-center gap-1.5 px-3 py-2 rounded-xl text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </button>
        </div>
      </div>

      <div className="rounded-2xl bg-slate-900/90 border border-slate-800 divide-y divide-slate-800/70 overflow-hidden">
        {loading && runs.length === 0 ? (
          <div className="flex items-center justify-center py-12 text-slate-400 text-sm gap-2">
            <Loader2 className="w-4 h-4 animate-spin" />
            Loading run history...
          </div>
        ) : runs.length === 0 ? (
          <div className="py-12 text-center text-sm text-slate-500">No job runs found.</div>
        ) : (
          runs.map((run) => (
            <div key={run.id} className="p-4 flex flex-wrap items-center justify-between gap-3">
              <div className="min-w-0 space-y-1">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="font-semibold text-sm text-slate-100">{run.job_name}</span>
                  <StatusBadge status={run.status} dryRun={run.dry_run} />
                  <span className="text-[11px] px-1.5 py-0.5 rounded bg-slate-800 text-slate-400">
                    {run.trigger}
                  </span>
                </div>
                <div className="text-xs text-slate-400 flex flex-wrap gap-x-3 gap-y-0.5 tabular-nums">
                  <span>{formatDateTime(run.started_at)}</span>
                  <span>Duration: {formatDurationMs(run.duration_ms)}</span>
                  <span>Transferred: {formatBytes(run.bytes_transferred)}</span>
                  <span>Files: {run.files_transferred}</span>
                  {run.files_deleted > 0 && <span>Deleted: {run.files_deleted}</span>}
                  {run.errors_count > 0 && (
                    <span className="text-rose-400">Errors: {run.errors_count}</span>
                  )}
                </div>
                {run.error_message && (
                  <p className="text-xs text-rose-300 break-words">{run.error_message}</p>
                )}
              </div>

              <div className="flex items-center gap-1.5 shrink-0">
                <button
                  type="button"
                  onClick={() => onViewLogs(run)}
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200"
                >
                  <Terminal className="w-3.5 h-3.5 text-sky-400" />
                  Logs
                </button>
                {run.status !== 'running' && run.status !== 'queued' && (
                  <button
                    type="button"
                    onClick={() => setConfirmDeleteRun(run)}
                    className="p-1.5 rounded-lg bg-slate-800 hover:bg-rose-500/20 text-slate-400 hover:text-rose-300"
                    title="Delete run"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                )}
              </div>
            </div>
          ))
        )}
      </div>

      {total > limit && (
        <div className="flex items-center justify-between text-xs text-slate-400">
          <span>
            Showing {offset + 1}–{Math.min(offset + limit, total)} of {total}
          </span>
          <div className="flex gap-2">
            <button
              type="button"
              disabled={offset === 0}
              onClick={() => setOffset((o) => Math.max(0, o - limit))}
              className="px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 disabled:opacity-40"
            >
              Previous
            </button>
            <button
              type="button"
              disabled={offset + limit >= total}
              onClick={() => setOffset((o) => o + limit)}
              className="px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 disabled:opacity-40"
            >
              Next
            </button>
          </div>
        </div>
      )}

      {confirmDeleteRun && (
        <ConfirmModal
          title="Delete Run Record"
          message={`Delete history record and log file for "${confirmDeleteRun.job_name}" (${formatDateTime(confirmDeleteRun.started_at)})?`}
          busy={deleting}
          onCancel={() => setConfirmDeleteRun(null)}
          onConfirm={() => void handleDelete(confirmDeleteRun)}
        />
      )}

      {confirmClear && (
        <ConfirmModal
          title="Clear Filtered History"
          message="Permanently delete all completed runs and log files matching the current filters? Active or queued runs will be preserved."
          confirmLabel="Clear History"
          busy={clearing}
          onCancel={() => setConfirmClear(false)}
          onConfirm={() => void handleClearFiltered()}
        />
      )}
    </div>
  );
}
