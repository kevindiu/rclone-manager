import { Square } from 'lucide-react';
import type { ActiveRunSnapshot } from '../../types/api';
import { formatBytes, formatETA, formatSpeed } from '../../utils/format';
import { StatusBadge } from './StatusBadge';

export function ActiveRunCard({
  run,
  onStop,
  variant = 'backup',
}: {
  run: ActiveRunSnapshot;
  onStop: (jobId: string) => void;
  variant?: 'backup' | 'restore';
}) {
  const p = run.progress;
  const pct = Math.min(100, Math.max(0, p?.percentage ?? 0));
  const isRestore = variant === 'restore';

  return (
    <div
      className={`${isRestore ? 'p-4' : 'p-4 sm:p-5'} rounded-2xl bg-slate-900 border ${
        isRestore ? 'border-emerald-500/40' : 'border-sky-500/40'
      } shadow-lg ${isRestore ? 'space-y-2.5' : 'space-y-3'}`}
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className={`flex items-center ${isRestore ? 'gap-2' : 'gap-2.5'} min-w-0`}>
          <span className={`font-semibold ${isRestore ? 'text-sm' : ''} text-slate-100 truncate`}>
            {run.job_name}
          </span>
          <StatusBadge status={run.status} dryRun={run.dry_run} />
        </div>
        <button
          type="button"
          onClick={() => onStop(run.job_id)}
          className={`inline-flex items-center gap-1.5 px-3 ${
            isRestore ? 'py-1' : 'py-1.5'
          } rounded-lg text-xs font-semibold bg-rose-500/15 hover:bg-rose-500/25 text-rose-300 border border-rose-500/30`}
        >
          <Square className={`${isRestore ? 'w-3 h-3' : 'w-3.5 h-3.5'} fill-current`} />
          Stop
        </button>
      </div>

      <div>
        <div
          className={`flex flex-wrap items-center justify-between text-xs text-slate-300 ${
            isRestore ? 'mb-2' : 'mb-1.5'
          } tabular-nums gap-2`}
        >
          <span>
            {formatBytes(p?.bytes)} / {formatBytes(p?.total_bytes)} ({pct.toFixed(1)}%)
          </span>
          <span className="flex items-center gap-3">
            <span className="text-emerald-400 font-medium">{formatSpeed(p?.speed)}</span>
            <span className="text-slate-400">ETA: {formatETA(p?.eta)}</span>
            {!isRestore && (
              <span className="text-slate-400">
                Files: {p?.transfers ?? 0}/{p?.total_transfers ?? 0}
              </span>
            )}
          </span>
        </div>
        <div
          className={`w-full ${isRestore ? 'h-2' : 'h-2.5'} rounded-full bg-slate-800 overflow-hidden`}
        >
          <div
            className={`h-full bg-gradient-to-r ${
              isRestore ? 'from-emerald-500 to-sky-400' : 'from-sky-500 to-emerald-400'
            } transition-all duration-300`}
            style={{ width: `${pct}%` }}
          />
        </div>
      </div>

      {!isRestore && p?.transferring && p.transferring.length > 0 && (
        <div className="pt-2 border-t border-slate-800/80 space-y-1.5">
          <div className="text-[11px] font-medium text-slate-400">
            Currently Transferring ({p.transferring.length})
          </div>
          {p.transferring.slice(0, 6).map((f) => (
            <div key={f.name} className="text-xs flex items-center gap-3">
              <span className="font-mono text-slate-200 truncate flex-1">{f.name}</span>
              <span className="text-slate-400 tabular-nums shrink-0">{formatSpeed(f.speed)}</span>
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
}
