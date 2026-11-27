import { Calendar } from 'lucide-react';
import { formatDateTime } from '../../utils/format';

const CRON_PRESETS = [
  { label: 'Every 15m', expr: '*/15 * * * *' },
  { label: 'Hourly', expr: '0 * * * *' },
  { label: 'Daily 02:00', expr: '0 2 * * *' },
  { label: 'Weekly Sun 03:00', expr: '0 3 * * 0' },
  { label: 'Manual only', expr: '' },
];

export function JobScheduleTab({
  schedule,
  setSchedule,
  timezone,
  cronError,
  cronPreview,
}: {
  schedule: string;
  setSchedule: (v: string) => void;
  timezone: string;
  cronError: string;
  cronPreview: string[];
}) {
  return (
    <div className="space-y-4">
      <div>
        <label className="block text-xs font-medium text-slate-300 mb-1">
          Cron Expression{' '}
          <span className="text-slate-500 font-normal">
            (5-field standard cron in {timezone}; leave blank for manual only)
          </span>
        </label>
        <input
          value={schedule}
          onChange={(e) => setSchedule(e.target.value)}
          placeholder="0 2 * * *"
          className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100 focus:outline-none focus:border-sky-500"
        />
        <div className="flex flex-wrap gap-1.5 mt-2">
          {CRON_PRESETS.map((p) => (
            <button
              key={p.label}
              type="button"
              onClick={() => setSchedule(p.expr)}
              className="px-2.5 py-1 rounded-md text-xs bg-slate-800 hover:bg-slate-700 text-slate-300"
            >
              {p.label}
            </button>
          ))}
        </div>
      </div>

      {cronError && (
        <div className="p-2.5 rounded-lg bg-rose-500/10 border border-rose-500/30 text-xs text-rose-300">
          {cronError}
        </div>
      )}

      {cronPreview.length > 0 && (
        <div className="p-3.5 rounded-xl bg-slate-950 border border-slate-800 space-y-2">
          <div className="flex items-center gap-1.5 text-xs font-medium text-sky-400">
            <Calendar className="w-4 h-4" />
            Next 5 scheduled runs ({timezone})
          </div>
          <ul className="space-y-1 text-xs text-slate-300 font-mono">
            {cronPreview.map((ts) => (
              <li key={ts}>• {formatDateTime(ts)}</li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}
