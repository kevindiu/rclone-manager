import { Download, Loader2, Radio, Search } from 'lucide-react';
import { useEffect, useMemo, useRef, useState } from 'react';
import { api } from '../../api/client';
import { Modal } from '../../components/ui/Modal';
import { StatusBadge } from '../../components/ui/StatusBadge';
import type { JobRun, ParsedLogLine } from '../../types/api';

const isActiveStatus = (status: string) => status === 'running' || status === 'queued';

export function LogViewerModal({ run, onClose }: { run: JobRun; onClose: () => void }) {
  const [currentRun, setCurrentRun] = useState<JobRun>(run);
  const [lines, setLines] = useState<ParsedLogLine[]>([]);
  const [raw, setRaw] = useState('');
  const [truncated, setTruncated] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');
  const [hideStats, setHideStats] = useState(true);
  const [rawView, setRawView] = useState(false);
  const [liveTail, setLiveTail] = useState(() => isActiveStatus(run.status));

  const parsedScrollRef = useRef<HTMLDivElement | null>(null);
  const rawScrollRef = useRef<HTMLPreElement | null>(null);
  const stickToBottomRef = useRef(true);

  useEffect(() => {
    setCurrentRun(run);
    setLiveTail(isActiveStatus(run.status));
    stickToBottomRef.current = true;
  }, [run]);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError('');
    api
      .getRunLogs(run.id)
      .then((res) => {
        if (!cancelled) {
          setLines(res.lines ?? []);
          setRaw(res.raw ?? '');
          setTruncated(Boolean(res.truncated));
        }
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Failed to load logs');
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [run.id]);

  const active = isActiveStatus(currentRun.status);

  useEffect(() => {
    if (!liveTail || !active) return;
    let cancelled = false;
    let inFlight = false;

    const timer = window.setInterval(() => {
      if (cancelled || inFlight) return;
      inFlight = true;
      void Promise.all([api.getRun(run.id), api.getRunLogs(run.id)])
        .then(([latestRun, logRes]) => {
          if (cancelled) return;
          setCurrentRun(latestRun);
          setLines(logRes.lines ?? []);
          setRaw(logRes.raw ?? '');
          setTruncated(Boolean(logRes.truncated));
          setError('');
          if (!isActiveStatus(latestRun.status)) {
            setLiveTail(false);
          } else if (stickToBottomRef.current) {
            window.requestAnimationFrame(() => {
              const el = rawView ? rawScrollRef.current : parsedScrollRef.current;
              if (el) el.scrollTop = el.scrollHeight;
            });
          }
        })
        .catch(() => {
          // ignore transient poll errors; next tick will retry
        })
        .finally(() => {
          inFlight = false;
        });
    }, 2000);

    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [liveTail, active, run.id, rawView]);

  const handleLogScroll = (el: HTMLElement | null) => {
    if (!el) return;
    stickToBottomRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 48;
  };

  const filteredLines = useMemo(() => {
    const q = query.trim().toLowerCase();
    return lines
      .map((l, lineNo) => ({ ...l, lineNo }))
      .filter((l) => {
        if (hideStats && l.is_stats) return false;
        if (!q) return true;
        return (
          l.msg.toLowerCase().includes(q) ||
          (l.object ?? '').toLowerCase().includes(q) ||
          (l.level ?? '').toLowerCase().includes(q)
        );
      });
  }, [lines, query, hideStats]);

  const levelColor = (lvl?: string) => {
    switch (lvl) {
      case 'error':
      case 'critical':
        return 'text-rose-400';
      case 'warning':
        return 'text-amber-400';
      case 'notice':
        return 'text-sky-400';
      default:
        return 'text-slate-400';
    }
  };

  return (
    <Modal
      title={`Run Logs: ${currentRun.job_name}`}
      subtitle={`Run ID ${currentRun.id}`}
      onClose={onClose}
      maxWidth="max-w-4xl"
      footer={
        <>
          <a
            href={`/api/runs/${encodeURIComponent(currentRun.id)}/logs?download=1`}
            className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200"
          >
            <Download className="w-3.5 h-3.5" />
            Download Log
          </a>
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 rounded-lg text-sm font-medium bg-sky-500 hover:bg-sky-400 text-slate-950"
          >
            Close
          </button>
        </>
      }
    >
      <div className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <StatusBadge status={currentRun.status} dryRun={currentRun.dry_run} />
          <div className="flex flex-wrap items-center gap-2">
            {active && (
              <button
                type="button"
                onClick={() => setLiveTail((v) => !v)}
                className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium border transition-colors ${
                  liveTail
                    ? 'bg-emerald-500/15 border-emerald-500/40 text-emerald-300'
                    : 'bg-slate-800 border-slate-700 text-slate-300 hover:bg-slate-700'
                }`}
              >
                <Radio
                  className={`w-3.5 h-3.5 ${liveTail ? 'animate-pulse text-emerald-400' : ''}`}
                />
                {liveTail ? 'Live' : 'Paused'}
              </button>
            )}
            <label className="inline-flex items-center gap-1.5 text-xs text-slate-300 cursor-pointer">
              <input
                type="checkbox"
                checked={hideStats}
                onChange={(e) => setHideStats(e.target.checked)}
                className="accent-sky-500"
              />
              Hide periodic stats
            </label>
            <button
              type="button"
              onClick={() => setRawView((v) => !v)}
              className="px-2.5 py-1 rounded-md text-xs bg-slate-800 hover:bg-slate-700 text-slate-200"
            >
              {rawView ? 'Parsed View' : 'Raw JSON'}
            </button>
          </div>
        </div>

        {currentRun.command_preview && (
          <div className="p-2.5 rounded-lg bg-slate-950 border border-slate-800 text-[11px] font-mono text-slate-300 break-all">
            $ {currentRun.command_preview}
          </div>
        )}

        {currentRun.pruned_versions && currentRun.pruned_versions.length > 0 && (
          <div className="p-2.5 rounded-lg bg-emerald-500/10 border border-emerald-500/25 text-xs text-emerald-300">
            Pruned expired version folders: {currentRun.pruned_versions.join(', ')}
          </div>
        )}

        {truncated && (
          <div className="p-2.5 rounded-lg bg-amber-500/10 border border-amber-500/30 text-xs text-amber-300">
            Showing the latest log tail. Use &quot;Download Log&quot; for the complete file.
          </div>
        )}

        {!rawView && (
          <div className="relative">
            <Search className="w-4 h-4 text-slate-500 absolute left-3 top-2.5" />
            <input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              placeholder="Filter log messages or object paths..."
              className="w-full pl-9 pr-3 py-1.5 rounded-lg bg-slate-950 border border-slate-800 text-xs text-slate-100 focus:outline-none focus:border-sky-500"
            />
          </div>
        )}

        {loading ? (
          <div className="flex items-center justify-center py-12 text-slate-400 text-sm gap-2">
            <Loader2 className="w-4 h-4 animate-spin" />
            Loading log file...
          </div>
        ) : error ? (
          <div className="p-3 rounded-lg bg-rose-500/10 border border-rose-500/30 text-xs text-rose-300">
            {error}
          </div>
        ) : rawView ? (
          <pre
            ref={rawScrollRef}
            onScroll={(e) => handleLogScroll(e.currentTarget)}
            className="p-3 rounded-xl bg-slate-950 border border-slate-800 text-[11px] font-mono text-slate-300 overflow-x-auto max-h-[48vh] whitespace-pre-wrap"
          >
            {raw || '(empty log)'}
          </pre>
        ) : (
          <div
            ref={parsedScrollRef}
            onScroll={(e) => handleLogScroll(e.currentTarget)}
            className="rounded-xl bg-slate-950 border border-slate-800 p-3 max-h-[48vh] overflow-y-auto font-mono text-[11px] space-y-1"
          >
            {filteredLines.length === 0 ? (
              <div className="text-slate-500 py-4 text-center">No matching log lines.</div>
            ) : (
              filteredLines.map((l) => (
                <div key={l.lineNo} className="flex items-start gap-2 leading-relaxed break-words">
                  <span className={`uppercase font-semibold shrink-0 w-14 ${levelColor(l.level)}`}>
                    {l.level || 'info'}
                  </span>
                  <div className="min-w-0 text-slate-200">
                    {l.object && <span className="text-sky-300 mr-1.5">[{l.object}]</span>}
                    <span className="whitespace-pre-wrap">{l.msg}</span>
                  </div>
                </div>
              ))
            )}
          </div>
        )}
      </div>
    </Modal>
  );
}
