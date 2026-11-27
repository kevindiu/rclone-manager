import { Calendar } from 'lucide-react';
import { useState } from 'react';
import { formatDateTime } from '../../utils/format';

const CRON_PRESETS = [
  { label: 'Every 15m', expr: '*/15 * * * *' },
  { label: 'Hourly', expr: '0 * * * *' },
  { label: 'Daily 02:00', expr: '0 2 * * *' },
  { label: 'Weekly Sun 03:00', expr: '0 3 * * 0' },
  { label: 'Manual only', expr: '' },
];

const WEEKDAYS = [
  { label: 'Sunday', value: '0' },
  { label: 'Monday', value: '1' },
  { label: 'Tuesday', value: '2' },
  { label: 'Wednesday', value: '3' },
  { label: 'Thursday', value: '4' },
  { label: 'Friday', value: '5' },
  { label: 'Saturday', value: '6' },
];

const HOURS = Array.from({ length: 24 }, (_, i) => String(i));

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
  const [freq, setFreq] = useState<'daily' | 'weekly' | 'hours'>('daily');
  const [hour, setHour] = useState('2');
  const [minute, setMinute] = useState('0');
  const [weekday, setWeekday] = useState('0');
  const [everyHours, setEveryHours] = useState('6');

  const applySimple = (
    nextFreq = freq,
    nextHour = hour,
    nextMinute = minute,
    nextWeekday = weekday,
    nextEveryHours = everyHours
  ) => {
    if (nextFreq === 'daily') {
      setSchedule(`${nextMinute} ${nextHour} * * *`);
    } else if (nextFreq === 'weekly') {
      setSchedule(`${nextMinute} ${nextHour} * * ${nextWeekday}`);
    } else {
      setSchedule(`${nextMinute} */${nextEveryHours} * * *`);
    }
  };

  return (
    <div className="space-y-4">
      <div className="p-3.5 rounded-xl bg-slate-950 border border-slate-800 space-y-3">
        <div className="text-xs font-medium text-slate-300">Quick Schedule Builder</div>
        <div className="flex flex-wrap items-center gap-2 text-xs">
          <select
            value={freq}
            onChange={(e) => {
              const v = e.target.value as 'daily' | 'weekly' | 'hours';
              setFreq(v);
              applySimple(v, hour, minute, weekday, everyHours);
            }}
            className="px-2.5 py-1.5 rounded-lg bg-slate-900 border border-slate-800 text-slate-100"
          >
            <option value="daily">Daily</option>
            <option value="weekly">Weekly</option>
            <option value="hours">Every N hours</option>
          </select>

          {freq === 'weekly' && (
            <select
              value={weekday}
              onChange={(e) => {
                setWeekday(e.target.value);
                applySimple(freq, hour, minute, e.target.value, everyHours);
              }}
              className="px-2.5 py-1.5 rounded-lg bg-slate-900 border border-slate-800 text-slate-100"
            >
              {WEEKDAYS.map((w) => (
                <option key={w.value} value={w.value}>
                  on {w.label}
                </option>
              ))}
            </select>
          )}

          {freq === 'hours' ? (
            <select
              value={everyHours}
              onChange={(e) => {
                setEveryHours(e.target.value);
                applySimple(freq, hour, minute, weekday, e.target.value);
              }}
              className="px-2.5 py-1.5 rounded-lg bg-slate-900 border border-slate-800 text-slate-100"
            >
              {[1, 2, 3, 4, 6, 8, 12].map((h) => (
                <option key={h} value={String(h)}>
                  Every {h}h
                </option>
              ))}
            </select>
          ) : (
            <div className="inline-flex items-center gap-1.5">
              <span className="text-slate-400">at</span>
              <select
                value={hour}
                onChange={(e) => {
                  setHour(e.target.value);
                  applySimple(freq, e.target.value, minute, weekday, everyHours);
                }}
                aria-label="Schedule hour"
                className="px-2 py-1.5 rounded-lg bg-slate-900 border border-slate-800 font-mono text-slate-100"
              >
                {HOURS.map((h) => (
                  <option key={h} value={h}>
                    {h.padStart(2, '0')}
                  </option>
                ))}
              </select>
              <span className="text-slate-500">:</span>
              <select
                value={minute}
                onChange={(e) => {
                  setMinute(e.target.value);
                  applySimple(freq, hour, e.target.value, weekday, everyHours);
                }}
                aria-label="Schedule minute"
                className="px-2 py-1.5 rounded-lg bg-slate-900 border border-slate-800 font-mono text-slate-100"
              >
                {[0, 15, 30, 45].map((m) => (
                  <option key={m} value={String(m)}>
                    {String(m).padStart(2, '0')}
                  </option>
                ))}
              </select>
            </div>
          )}
        </div>
      </div>

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
          {CRON_PRESETS.map((p) => {
            const active = schedule.trim() === p.expr;
            return (
              <button
                key={p.label}
                type="button"
                onClick={() => setSchedule(p.expr)}
                className={`px-2.5 py-1 rounded-md text-xs border transition-colors ${
                  active
                    ? 'bg-sky-500/20 border-sky-500/50 text-sky-300'
                    : 'bg-slate-800 border-transparent hover:bg-slate-700 text-slate-300'
                }`}
              >
                {p.label}
              </button>
            );
          })}
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
