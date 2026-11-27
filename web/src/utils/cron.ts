export type ScheduleMode = 'manual' | 'interval' | 'daily' | 'weekly' | 'monthly' | 'custom';
export type IntervalUnit = 'minutes' | 'hours';

export interface ParsedSchedule {
  mode: ScheduleMode;
  intervalUnit: IntervalUnit;
  intervalEvery: number;
  hour: number;
  minute: number;
  weekdays: number[];
  dayOfMonth: number;
  customExpr: string;
}

const SHORT_WEEKDAYS = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'] as const;

function parseStrictInt(raw: string, min: number, max: number): number | null {
  if (!/^\d+$/.test(raw)) return null;
  const n = Number(raw);
  if (!Number.isInteger(n) || n < min || n > max) return null;
  return n;
}

function normalizeWeekdays(days: number[]): number[] {
  const unique = new Set<number>();
  for (const d of days) {
    if (!Number.isInteger(d) || d < 0 || d > 7) continue;
    unique.add(d === 7 ? 0 : d);
  }
  return Array.from(unique).sort((a, b) => a - b);
}

function parseWeekdayList(raw: string): number[] | null {
  if (!raw) return null;
  const out: number[] = [];
  for (const part of raw.split(',')) {
    const single = parseStrictInt(part, 0, 7);
    if (single !== null) {
      out.push(single === 7 ? 0 : single);
      continue;
    }
    const rangeMatch = /^(\d+)-(\d+)$/.exec(part);
    if (!rangeMatch) return null;
    const start = parseStrictInt(rangeMatch[1], 0, 7);
    const end = parseStrictInt(rangeMatch[2], 0, 7);
    if (start === null || end === null || start > end) return null;
    for (let d = start; d <= end; d += 1) {
      out.push(d === 7 ? 0 : d);
    }
  }
  const normalized = normalizeWeekdays(out);
  return normalized.length > 0 ? normalized : null;
}

function formatTime(hour: number, minute: number): string {
  return `${String(hour).padStart(2, '0')}:${String(minute).padStart(2, '0')}`;
}

export function formatOrdinal(day: number): string {
  const mod100 = day % 100;
  if (mod100 >= 11 && mod100 <= 13) return `${day}th`;
  switch (day % 10) {
    case 1:
      return `${day}st`;
    case 2:
      return `${day}nd`;
    case 3:
      return `${day}rd`;
    default:
      return `${day}th`;
  }
}

export function parseCronSchedule(expr: string): ParsedSchedule {
  const trimmed = expr.trim();
  const base: ParsedSchedule = {
    mode: 'manual',
    intervalUnit: 'hours',
    intervalEvery: 6,
    hour: 2,
    minute: 0,
    weekdays: [0],
    dayOfMonth: 1,
    customExpr: trimmed,
  };

  if (!trimmed) {
    return base;
  }

  switch (trimmed.toLowerCase()) {
    case '@hourly':
      return { ...base, mode: 'interval', intervalUnit: 'hours', intervalEvery: 1, minute: 0 };
    case '@daily':
    case '@midnight':
      return { ...base, mode: 'daily', hour: 0, minute: 0 };
    case '@weekly':
      return { ...base, mode: 'weekly', weekdays: [0], hour: 0, minute: 0 };
    case '@monthly':
      return { ...base, mode: 'monthly', dayOfMonth: 1, hour: 0, minute: 0 };
    default:
      break;
  }

  const parts = trimmed.split(/\s+/);
  if (parts.length !== 5) {
    return { ...base, mode: 'custom' };
  }

  const [minStr, hrStr, domStr, monStr, dowStr] = parts;
  if (monStr !== '*') {
    return { ...base, mode: 'custom' };
  }

  // Every minute / every N minutes: "* * * * *" or "*/N * * * *"
  if (hrStr === '*' && domStr === '*' && dowStr === '*') {
    if (minStr === '*') {
      return { ...base, mode: 'interval', intervalUnit: 'minutes', intervalEvery: 1 };
    }
    const stepMatch = /^\*\/(\d+)$/.exec(minStr);
    if (stepMatch) {
      const every = parseStrictInt(stepMatch[1], 1, 59);
      if (every !== null) {
        return { ...base, mode: 'interval', intervalUnit: 'minutes', intervalEvery: every };
      }
    }
  }

  const minute = parseStrictInt(minStr, 0, 59);
  if (minute === null) {
    return { ...base, mode: 'custom' };
  }

  // Every hour / every N hours: "M * * * *" or "M */N * * *"
  if (domStr === '*' && dowStr === '*') {
    if (hrStr === '*') {
      return { ...base, mode: 'interval', intervalUnit: 'hours', intervalEvery: 1, minute };
    }
    const stepMatch = /^\*\/(\d+)$/.exec(hrStr);
    if (stepMatch) {
      const every = parseStrictInt(stepMatch[1], 1, 23);
      if (every !== null) {
        return { ...base, mode: 'interval', intervalUnit: 'hours', intervalEvery: every, minute };
      }
    }
  }

  const hour = parseStrictInt(hrStr, 0, 23);
  if (hour === null) {
    return { ...base, mode: 'custom' };
  }

  // Daily: "M H * * *"
  if (domStr === '*' && dowStr === '*') {
    return { ...base, mode: 'daily', hour, minute };
  }

  // Weekly: "M H * * DOW"
  if (domStr === '*' && dowStr !== '*') {
    const weekdays = parseWeekdayList(dowStr);
    if (weekdays) {
      return { ...base, mode: 'weekly', weekdays, hour, minute };
    }
    return { ...base, mode: 'custom' };
  }

  // Monthly: "M H DOM * *"
  if (dowStr === '*') {
    const dayOfMonth = parseStrictInt(domStr, 1, 31);
    if (dayOfMonth !== null) {
      return { ...base, mode: 'monthly', dayOfMonth, hour, minute };
    }
  }

  return { ...base, mode: 'custom' };
}

function formatWeekdaysCron(days: number[]): string {
  const normalized = normalizeWeekdays(days);
  const effective = normalized.length > 0 ? normalized : [0];
  if (effective.length >= 3) {
    let contiguous = true;
    for (let i = 1; i < effective.length; i += 1) {
      if (effective[i] !== effective[i - 1] + 1) {
        contiguous = false;
        break;
      }
    }
    if (contiguous) {
      return `${effective[0]}-${effective[effective.length - 1]}`;
    }
  }
  return effective.join(',');
}

export function buildCronExpression(state: ParsedSchedule): string {
  switch (state.mode) {
    case 'manual':
      return '';
    case 'interval':
      if (state.intervalUnit === 'minutes') {
        const every = Math.max(1, Math.min(59, Math.round(state.intervalEvery || 1)));
        return `*/${every} * * * *`;
      }
      if (state.intervalEvery <= 1) {
        return `${state.minute} * * * *`;
      }
      return `${state.minute} */${Math.max(1, Math.min(23, Math.round(state.intervalEvery)))} * * *`;
    case 'daily':
      return `${state.minute} ${state.hour} * * *`;
    case 'weekly':
      return `${state.minute} ${state.hour} * * ${formatWeekdaysCron(state.weekdays)}`;
    case 'monthly':
      return `${state.minute} ${state.hour} ${Math.max(1, Math.min(31, state.dayOfMonth))} * *`;
    case 'custom':
      return state.customExpr.trim();
  }
}

export function describeCronSchedule(expr: string): string {
  const parsed = parseCronSchedule(expr);
  const timeStr = formatTime(parsed.hour, parsed.minute);

  switch (parsed.mode) {
    case 'manual':
      return 'Manual only';
    case 'interval':
      if (parsed.intervalUnit === 'minutes') {
        return parsed.intervalEvery === 1
          ? 'Every minute'
          : `Every ${parsed.intervalEvery} minutes`;
      }
      if (parsed.intervalEvery === 1) {
        return parsed.minute === 0
          ? 'Every hour'
          : `Every hour at :${String(parsed.minute).padStart(2, '0')}`;
      }
      return parsed.minute === 0
        ? `Every ${parsed.intervalEvery} hours`
        : `Every ${parsed.intervalEvery} hours at :${String(parsed.minute).padStart(2, '0')}`;
    case 'daily':
      return `Daily at ${timeStr}`;
    case 'weekly': {
      const days = normalizeWeekdays(parsed.weekdays);
      const key = days.join(',');
      if (key === '0,1,2,3,4,5,6') {
        return `Daily at ${timeStr}`;
      }
      if (key === '1,2,3,4,5') {
        return `Weekdays (Mon–Fri) at ${timeStr}`;
      }
      if (key === '0,6') {
        return `Weekends (Sat–Sun) at ${timeStr}`;
      }
      const names = days.map((d) => SHORT_WEEKDAYS[d]).join(', ');
      return `Every ${names} at ${timeStr}`;
    }
    case 'monthly':
      return `Monthly on the ${formatOrdinal(parsed.dayOfMonth)} at ${timeStr}`;
    case 'custom':
      return `Custom: ${expr.trim()}`;
  }
}
