import { Calendar, Clock } from 'lucide-react';
import { useEffect, useState } from 'react';
import {
  buildCronExpression,
  describeCronSchedule,
  formatOrdinal,
  type IntervalUnit,
  type ParsedSchedule,
  parseCronSchedule,
  type ScheduleMode,
} from '../../utils/cron';
import { formatDateTime } from '../../utils/format';

const MODE_OPTIONS: Array<{ id: ScheduleMode; label: string }> = [
  { id: 'manual', label: 'Manual' },
  { id: 'interval', label: 'Interval' },
  { id: 'daily', label: 'Daily' },
  { id: 'weekly', label: 'Weekly' },
  { id: 'monthly', label: 'Monthly' },
  { id: 'custom', label: 'Custom' },
];

const WEEKDAY_BUTTONS = [
  { value: 0, short: 'Sun' },
  { value: 1, short: 'Mon' },
  { value: 2, short: 'Tue' },
  { value: 3, short: 'Wed' },
  { value: 4, short: 'Thu' },
  { value: 5, short: 'Fri' },
  { value: 6, short: 'Sat' },
];

const HOURS = Array.from({ length: 24 }, (_, i) => i);
const MINUTES = Array.from({ length: 60 }, (_, i) => i);
const MONTH_DAYS = Array.from({ length: 31 }, (_, i) => i + 1);
const MINUTE_INTERVAL_CHIPS = [5, 10, 15, 20, 30];
const HOUR_INTERVAL_CHIPS = [1, 2, 3, 4, 6, 8, 12];
const DAILY_TIME_CHIPS = [
  { label: '00:00', hour: 0, minute: 0 },
  { label: '02:00', hour: 2, minute: 0 },
  { label: '03:00', hour: 3, minute: 0 },
  { label: '12:00', hour: 12, minute: 0 },
  { label: '22:00', hour: 22, minute: 0 },
];

const CUSTOM_PRESETS = [
  { label: 'Every 15m', expr: '*/15 * * * *' },
  { label: 'Hourly', expr: '0 * * * *' },
  { label: 'Daily 02:00', expr: '0 2 * * *' },
  { label: 'Weekdays 03:00', expr: '0 3 * * 1-5' },
  { label: '1st of month', expr: '0 4 1 * *' },
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
  const [state, setState] = useState<ParsedSchedule>(() => parseCronSchedule(schedule));

  useEffect(() => {
    const trimmed = schedule.trim();
    if (buildCronExpression(state) !== trimmed) {
      setState(parseCronSchedule(trimmed));
    }
  }, [schedule, state]);

  const updateState = (updater: (prev: ParsedSchedule) => ParsedSchedule) => {
    setState((prev) => {
      const next = updater(prev);
      setSchedule(buildCronExpression(next));
      return next;
    });
  };

  const handleModeChange = (nextMode: ScheduleMode) => {
    updateState((prev) => {
      if (nextMode === 'custom') {
        const currentExpr = buildCronExpression(prev) || schedule.trim() || '0 2 * * *';
        return { ...prev, mode: 'custom', customExpr: currentExpr };
      }
      if (nextMode === 'interval' && prev.intervalUnit === 'hours' && prev.intervalEvery < 1) {
        return { ...prev, mode: nextMode, intervalEvery: 6 };
      }
      return { ...prev, mode: nextMode };
    });
  };

  const toggleWeekday = (day: number) => {
    updateState((prev) => {
      const exists = prev.weekdays.includes(day);
      if (exists && prev.weekdays.length === 1) {
        return prev;
      }
      const weekdays = exists
        ? prev.weekdays.filter((d) => d !== day)
        : [...prev.weekdays, day].sort((a, b) => a - b);
      return { ...prev, mode: 'weekly', weekdays };
    });
  };

  const renderTimePicker = (mode: 'daily' | 'weekly' | 'monthly') => (
    <div className="inline-flex items-center gap-1.5 text-xs">
      <span className="text-slate-400">at</span>
      <select
        value={state.hour}
        onChange={(e) => {
          const hour = Number(e.target.value);
          updateState((prev) => ({ ...prev, mode, hour }));
        }}
        aria-label="Schedule hour"
        className="px-2.5 py-1.5 rounded-lg bg-slate-900 border border-slate-800 font-mono text-slate-100 focus:outline-none focus:border-sky-500"
      >
        {HOURS.map((h) => (
          <option key={h} value={h}>
            {String(h).padStart(2, '0')}
          </option>
        ))}
      </select>
      <span className="text-slate-500">:</span>
      <select
        value={state.minute}
        onChange={(e) => {
          const minute = Number(e.target.value);
          updateState((prev) => ({ ...prev, mode, minute }));
        }}
        aria-label="Schedule minute"
        className="px-2.5 py-1.5 rounded-lg bg-slate-900 border border-slate-800 font-mono text-slate-100 focus:outline-none focus:border-sky-500"
      >
        {MINUTES.map((m) => (
          <option key={m} value={m}>
            {String(m).padStart(2, '0')}
          </option>
        ))}
      </select>
    </div>
  );

  const weekdayKey = [...state.weekdays].sort((a, b) => a - b).join(',');

  return (
    <div className="space-y-4">
      <div
        role="tablist"
        aria-label="Schedule frequency"
        className="grid grid-cols-3 sm:grid-cols-6 gap-1 p-1 rounded-xl bg-slate-950 border border-slate-800"
      >
        {MODE_OPTIONS.map((opt) => {
          const active = state.mode === opt.id;
          return (
            <button
              key={opt.id}
              type="button"
              role="tab"
              aria-selected={active}
              onClick={() => handleModeChange(opt.id)}
              className={`px-2.5 py-1.5 rounded-lg text-xs font-medium transition-colors ${
                active
                  ? 'bg-sky-500/20 text-sky-300 border border-sky-500/40'
                  : 'text-slate-400 hover:text-slate-200 border border-transparent'
              }`}
            >
              {opt.label}
            </button>
          );
        })}
      </div>

      {state.mode === 'manual' && (
        <div className="p-4 rounded-xl bg-slate-950 border border-slate-800 text-xs text-slate-400">
          This job will not run automatically. Trigger it anytime using{' '}
          <span className="text-slate-200 font-medium">Run Now</span> from the Jobs page.
        </div>
      )}

      {state.mode === 'interval' && (
        <div className="p-4 rounded-xl bg-slate-950 border border-slate-800 space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="inline-flex rounded-lg bg-slate-900 border border-slate-800 p-0.5 text-xs">
              {(['minutes', 'hours'] as IntervalUnit[]).map((unit) => {
                const active = state.intervalUnit === unit;
                return (
                  <button
                    key={unit}
                    type="button"
                    onClick={() =>
                      updateState((prev) => ({
                        ...prev,
                        mode: 'interval',
                        intervalUnit: unit,
                        intervalEvery:
                          unit === 'minutes'
                            ? MINUTE_INTERVAL_CHIPS.includes(prev.intervalEvery)
                              ? prev.intervalEvery
                              : 15
                            : HOUR_INTERVAL_CHIPS.includes(prev.intervalEvery)
                              ? prev.intervalEvery
                              : 6,
                      }))
                    }
                    className={`px-3 py-1 rounded-md capitalize transition-colors ${
                      active ? 'bg-sky-500/20 text-sky-300' : 'text-slate-400 hover:text-slate-200'
                    }`}
                  >
                    {unit}
                  </button>
                );
              })}
            </div>

            {state.intervalUnit === 'hours' && (
              <div className="inline-flex items-center gap-1.5 text-xs">
                <span className="text-slate-400">at minute</span>
                <select
                  value={[0, 15, 30, 45].includes(state.minute) ? state.minute : 0}
                  onChange={(e) =>
                    updateState((prev) => ({
                      ...prev,
                      mode: 'interval',
                      minute: Number(e.target.value),
                    }))
                  }
                  aria-label="Hourly offset minute"
                  className="px-2 py-1 rounded-lg bg-slate-900 border border-slate-800 font-mono text-slate-100"
                >
                  {[0, 15, 30, 45].map((m) => (
                    <option key={m} value={m}>
                      :{String(m).padStart(2, '0')}
                    </option>
                  ))}
                </select>
              </div>
            )}
          </div>

          <div className="flex flex-wrap gap-1.5">
            {(state.intervalUnit === 'minutes' ? MINUTE_INTERVAL_CHIPS : HOUR_INTERVAL_CHIPS).map(
              (n) => {
                const active = state.intervalEvery === n;
                return (
                  <button
                    key={n}
                    type="button"
                    onClick={() =>
                      updateState((prev) => ({ ...prev, mode: 'interval', intervalEvery: n }))
                    }
                    className={`px-3 py-1.5 rounded-lg text-xs font-medium border transition-colors ${
                      active
                        ? 'bg-sky-500/20 border-sky-500/50 text-sky-300'
                        : 'bg-slate-900 border-slate-800 hover:bg-slate-800 text-slate-300'
                    }`}
                  >
                    {state.intervalUnit === 'minutes'
                      ? `Every ${n}m`
                      : n === 1
                        ? 'Hourly'
                        : `Every ${n}h`}
                  </button>
                );
              }
            )}
          </div>
        </div>
      )}

      {state.mode === 'daily' && (
        <div className="p-4 rounded-xl bg-slate-950 border border-slate-800 space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <span className="text-xs font-medium text-slate-300">Run every day</span>
            {renderTimePicker('daily')}
          </div>
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="text-[11px] text-slate-500 mr-1">Quick times:</span>
            {DAILY_TIME_CHIPS.map((t) => {
              const active = state.hour === t.hour && state.minute === t.minute;
              return (
                <button
                  key={t.label}
                  type="button"
                  onClick={() =>
                    updateState((prev) => ({
                      ...prev,
                      mode: 'daily',
                      hour: t.hour,
                      minute: t.minute,
                    }))
                  }
                  className={`px-2.5 py-1 rounded-md text-xs font-mono border transition-colors ${
                    active
                      ? 'bg-sky-500/20 border-sky-500/50 text-sky-300'
                      : 'bg-slate-900 border-slate-800 hover:bg-slate-800 text-slate-300'
                  }`}
                >
                  {t.label}
                </button>
              );
            })}
          </div>
        </div>
      )}

      {state.mode === 'weekly' && (
        <div className="p-4 rounded-xl bg-slate-950 border border-slate-800 space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="flex items-center gap-1.5 text-xs">
              <button
                type="button"
                onClick={() =>
                  updateState((prev) => ({ ...prev, mode: 'weekly', weekdays: [1, 2, 3, 4, 5] }))
                }
                className={`px-2.5 py-1 rounded-md border transition-colors ${
                  weekdayKey === '1,2,3,4,5'
                    ? 'bg-sky-500/20 border-sky-500/50 text-sky-300'
                    : 'bg-slate-900 border-slate-800 hover:bg-slate-800 text-slate-300'
                }`}
              >
                Weekdays
              </button>
              <button
                type="button"
                onClick={() =>
                  updateState((prev) => ({ ...prev, mode: 'weekly', weekdays: [0, 6] }))
                }
                className={`px-2.5 py-1 rounded-md border transition-colors ${
                  weekdayKey === '0,6'
                    ? 'bg-sky-500/20 border-sky-500/50 text-sky-300'
                    : 'bg-slate-900 border-slate-800 hover:bg-slate-800 text-slate-300'
                }`}
              >
                Weekends
              </button>
            </div>
            {renderTimePicker('weekly')}
          </div>

          <div className="grid grid-cols-7 gap-1.5">
            {WEEKDAY_BUTTONS.map((d) => {
              const active = state.weekdays.includes(d.value);
              return (
                <button
                  key={d.value}
                  type="button"
                  aria-pressed={active}
                  onClick={() => toggleWeekday(d.value)}
                  className={`py-2 rounded-lg text-xs font-medium border transition-colors ${
                    active
                      ? 'bg-sky-500/20 border-sky-500/50 text-sky-300'
                      : 'bg-slate-900 border-slate-800 hover:bg-slate-800 text-slate-400'
                  }`}
                >
                  {d.short}
                </button>
              );
            })}
          </div>
        </div>
      )}

      {state.mode === 'monthly' && (
        <div className="p-4 rounded-xl bg-slate-950 border border-slate-800 space-y-2.5">
          <div className="flex flex-wrap items-center justify-between gap-2 text-xs">
            <div className="inline-flex items-center gap-2">
              <span className="text-slate-300">Run on the</span>
              <select
                value={state.dayOfMonth}
                onChange={(e) =>
                  updateState((prev) => ({
                    ...prev,
                    mode: 'monthly',
                    dayOfMonth: Number(e.target.value),
                  }))
                }
                aria-label="Day of month"
                className="px-2.5 py-1.5 rounded-lg bg-slate-900 border border-slate-800 font-mono text-slate-100 focus:outline-none focus:border-sky-500"
              >
                {MONTH_DAYS.map((d) => (
                  <option key={d} value={d}>
                    {formatOrdinal(d)}
                  </option>
                ))}
              </select>
              <span className="text-slate-400">of every month</span>
            </div>
            {renderTimePicker('monthly')}
          </div>
          {state.dayOfMonth > 28 && (
            <p className="text-[11px] text-amber-300/90">
              Note: Months with fewer than {state.dayOfMonth} days will be skipped by standard cron.
            </p>
          )}
        </div>
      )}

      {state.mode === 'custom' && (
        <div className="p-4 rounded-xl bg-slate-950 border border-slate-800 space-y-2.5">
          <label className="block text-xs font-medium text-slate-300">
            5-field Cron Expression ({timezone})
          </label>
          <input
            value={state.customExpr}
            onChange={(e) =>
              updateState((prev) => ({
                ...prev,
                mode: 'custom',
                customExpr: e.target.value,
              }))
            }
            placeholder="0 2 * * *"
            className="w-full px-3 py-2 rounded-lg bg-slate-900 border border-slate-800 text-sm font-mono text-slate-100 focus:outline-none focus:border-sky-500"
          />
          <div className="text-[11px] font-mono text-slate-500">
            Format: Minute (0–59) · Hour (0–23) · Day (1–31) · Month (1–12) · Weekday (0–6, Sun=0)
          </div>
          <div className="flex flex-wrap gap-1.5 pt-1">
            {CUSTOM_PRESETS.map((p) => {
              const active = state.customExpr.trim() === p.expr;
              return (
                <button
                  key={p.label}
                  type="button"
                  onClick={() =>
                    updateState((prev) => ({ ...prev, mode: 'custom', customExpr: p.expr }))
                  }
                  className={`px-2.5 py-1 rounded-md text-xs border transition-colors ${
                    active
                      ? 'bg-sky-500/20 border-sky-500/50 text-sky-300'
                      : 'bg-slate-900 border-slate-800 hover:bg-slate-800 text-slate-300'
                  }`}
                >
                  {p.label}
                </button>
              );
            })}
          </div>
        </div>
      )}

      <div className="flex flex-wrap items-center justify-between gap-2 px-3.5 py-2.5 rounded-xl bg-slate-950 border border-slate-800 text-xs">
        <div className="flex items-center gap-2 text-slate-200 min-w-0">
          <Clock className="w-3.5 h-3.5 text-sky-400 shrink-0" />
          <span className="font-medium truncate">{describeCronSchedule(schedule)}</span>
          <span className="text-slate-500 shrink-0">({timezone})</span>
        </div>
        {schedule.trim() !== '' && state.mode !== 'custom' && (
          <button
            type="button"
            onClick={() => handleModeChange('custom')}
            title="Edit raw cron expression"
            className="px-2 py-0.5 rounded bg-slate-900 border border-slate-800 hover:border-sky-500/50 font-mono text-[11px] text-slate-400 hover:text-sky-300 transition-colors"
          >
            {schedule.trim()}
          </button>
        )}
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
