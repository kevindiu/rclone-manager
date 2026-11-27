import { BarChart3 } from 'lucide-react';
import { useState } from 'react';
import type { DailyRunStat } from '../../types/api';
import { formatBytes } from '../../utils/format';

export function StatsChart({ stats, loading }: { stats: DailyRunStat[]; loading?: boolean }) {
  const [hovered, setHovered] = useState<DailyRunStat | null>(null);

  const totalBytes = stats.reduce((acc, s) => acc + s.bytes_transferred, 0);
  const totalSuccess = stats.reduce((acc, s) => acc + s.success_count, 0);
  const totalFailed = stats.reduce((acc, s) => acc + s.failed_count, 0);
  const maxBytes = Math.max(1, ...stats.map((s) => s.bytes_transferred));

  const chartWidth = 640;
  const chartHeight = 132;
  const barGap = 8;
  const count = Math.max(1, stats.length);
  const slotW = chartWidth / count;
  const barW = Math.max(10, slotW - barGap);

  return (
    <section className="p-4 rounded-2xl bg-slate-900/90 border border-slate-800 space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-center gap-2">
          <BarChart3 className="w-4 h-4 text-sky-400" />
          <h2 className="text-sm font-semibold text-slate-200">
            Last {stats.length || 14} Days Transfer Activity
          </h2>
        </div>

        <div className="flex flex-wrap items-center gap-3 text-xs tabular-nums">
          <span className="text-slate-300">
            Volume: <strong className="text-sky-300">{formatBytes(totalBytes)}</strong>
          </span>
          <span className="text-emerald-400">{totalSuccess} succeeded</span>
          {totalFailed > 0 && <span className="text-rose-400">{totalFailed} failed</span>}
        </div>
      </div>

      {loading && stats.length === 0 ? (
        <div className="h-36 flex items-center justify-center text-xs text-slate-500">
          Loading transfer metrics...
        </div>
      ) : (
        <div className="space-y-2">
          <svg
            viewBox={`0 0 ${chartWidth} ${chartHeight}`}
            className="w-full h-36 overflow-visible"
            role="img"
            aria-label="14-day transfer volume and run status bar chart"
          >
            <title>Daily transfer volume and run outcomes</title>
            {[0.25, 0.5, 0.75, 1].map((frac) => {
              const y = chartHeight - frac * (chartHeight - 16);
              return (
                <line
                  key={frac}
                  x1={0}
                  x2={chartWidth}
                  y1={y}
                  y2={y}
                  stroke="#1e293b"
                  strokeDasharray="3 3"
                  strokeWidth={1}
                />
              );
            })}

            {stats.map((s, idx) => {
              const x = idx * slotW + barGap / 2;
              const usableH = chartHeight - 20;
              const hasActivity =
                s.bytes_transferred > 0 ||
                s.success_count > 0 ||
                s.failed_count > 0 ||
                s.other_count > 0;
              const rawH =
                s.bytes_transferred > 0
                  ? Math.max(6, (s.bytes_transferred / maxBytes) * usableH)
                  : hasActivity
                    ? 5
                    : 2;
              const y = chartHeight - 4 - rawH;
              const fill = s.failed_count > 0 ? '#f43f5e' : hasActivity ? '#38bdf8' : '#1e293b';

              return (
                <g key={s.date}>
                  <title>
                    {`${s.date}: ${formatBytes(s.bytes_transferred)} · ${s.success_count} ok, ${s.failed_count} failed`}
                  </title>
                  <rect
                    x={x}
                    y={y}
                    width={barW}
                    height={rawH}
                    rx={4}
                    fill={fill}
                    opacity={hovered?.date === s.date ? 1 : 0.85}
                    onMouseEnter={() => setHovered(s)}
                    onMouseLeave={() => setHovered((cur) => (cur?.date === s.date ? null : cur))}
                  />
                </g>
              );
            })}
          </svg>

          <div className="flex items-center justify-between text-[11px] text-slate-500 tabular-nums">
            <span>{stats[0]?.date ?? ''}</span>
            <span className="text-slate-300 font-mono">
              {hovered
                ? `${hovered.date} — ${formatBytes(hovered.bytes_transferred)} (${hovered.success_count} ok${hovered.failed_count > 0 ? `, ${hovered.failed_count} failed` : ''})`
                : 'Hover a bar for daily breakdown'}
            </span>
            <span>{stats[stats.length - 1]?.date ?? ''}</span>
          </div>
        </div>
      )}
    </section>
  );
}
