export function StatusBadge({ status, dryRun }: { status: string; dryRun?: boolean }) {
  const styles: Record<string, string> = {
    success: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/30',
    running: 'bg-sky-500/15 text-sky-400 border-sky-500/30 animate-pulse',
    queued: 'bg-amber-500/15 text-amber-400 border-amber-500/30',
    failed: 'bg-rose-500/15 text-rose-400 border-rose-500/30',
    cancelled: 'bg-slate-500/20 text-slate-300 border-slate-500/30',
    skipped: 'bg-amber-500/15 text-amber-300 border-amber-500/30',
  };

  return (
    <span className="inline-flex items-center gap-1.5">
      <span
        className={`inline-flex items-center px-2 py-0.5 rounded-full text-xs font-medium border capitalize ${
          styles[status] ?? 'bg-slate-700/40 text-slate-300 border-slate-600'
        }`}
      >
        {status}
      </span>
      {dryRun && (
        <span className="inline-flex items-center px-1.5 py-0.5 rounded text-[11px] font-medium bg-violet-500/15 text-violet-300 border border-violet-500/30">
          Dry Run
        </span>
      )}
    </span>
  );
}
