import type { JobWithStatus } from '../types/api';

export function formatBytes(bytes?: number | null): string {
  if (bytes === undefined || bytes === null || Number.isNaN(bytes)) return '—';
  if (bytes === 0) return '0 B';
  const units = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB'];
  const i = Math.min(units.length - 1, Math.floor(Math.log(Math.abs(bytes)) / Math.log(1024)));
  const val = bytes / 1024 ** i;
  return `${val >= 100 || i === 0 ? val.toFixed(0) : val.toFixed(val >= 10 ? 1 : 2)} ${units[i]}`;
}

export function formatSpeed(bytesPerSec?: number | null): string {
  if (!bytesPerSec || bytesPerSec <= 0) return '0 B/s';
  return `${formatBytes(bytesPerSec)}/s`;
}

export function formatDurationMs(ms?: number | null): string {
  if (!ms || ms < 0) return '0s';
  const totalSec = Math.round(ms / 1000);
  if (totalSec < 60) return `${totalSec}s`;
  const m = Math.floor(totalSec / 60);
  const s = totalSec % 60;
  if (m < 60) return `${m}m ${s}s`;
  const h = Math.floor(m / 60);
  return `${h}h ${m % 60}m`;
}

export function formatETA(seconds?: number | null): string {
  if (seconds === undefined || seconds === null || seconds < 0) return '—';
  if (seconds < 60) return `${seconds}s`;
  const m = Math.floor(seconds / 60);
  const s = seconds % 60;
  if (m < 60) return `${m}m ${s}s`;
  const h = Math.floor(m / 60);
  return `${h}h ${m % 60}m`;
}

export function formatDateTime(iso?: string | null): string {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  });
}

export function formatRelative(iso?: string | null): string {
  if (!iso) return '—';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  const diffSec = Math.round((d.getTime() - Date.now()) / 1000);
  const abs = Math.abs(diffSec);
  const suffix = diffSec >= 0 ? 'from now' : 'ago';
  if (abs < 60) return `${abs}s ${suffix}`;
  const mins = Math.round(abs / 60);
  if (mins < 60) return `${mins}m ${suffix}`;
  const hrs = Math.round(mins / 60);
  if (hrs < 48) return `${hrs}h ${suffix}`;
  const days = Math.round(hrs / 24);
  return `${days}d ${suffix}`;
}

export function suggestRestoreTarget(remote: string, sourcePath: string): string {
  const normalized = sourcePath.replace(/^\/+/, '');
  const versionMatch = normalized.match(/^_versions\/[^/]+\/(.+)$/);
  if (versionMatch) {
    return `${remote}:${versionMatch[1]}`;
  }
  const nestedVersionMatch = normalized.match(/^(.+?)\/_versions\/[^/]+\/(.+)$/);
  if (nestedVersionMatch) {
    return `${remote}:${nestedVersionMatch[1]}/${nestedVersionMatch[2]}`;
  }
  return `${remote}:${normalized}`;
}

function normalizeRcloneBase(p: string): string {
  const trimmed = p.trim();
  if (trimmed.endsWith(':')) return trimmed;
  return trimmed.replace(/\/+$/, '');
}

function joinRcloneSub(base: string, rel: string): string {
  const cleanRel = rel.replace(/^\/+/, '');
  if (!cleanRel) return base;
  if (base.endsWith(':')) return `${base}${cleanRel}`;
  return `${base.replace(/\/+$/, '')}/${cleanRel}`;
}

function matchPathUnderBase(fullTarget: string, base: string): string | null {
  const normBase = normalizeRcloneBase(base);
  const normFull = normalizeRcloneBase(fullTarget);
  if (normBase.endsWith(':')) {
    if (normFull.startsWith(normBase) && normFull.length > normBase.length) {
      return normFull.slice(normBase.length).replace(/^\/+/, '');
    }
    return null;
  }
  const prefix = `${normBase}/`;
  if (normFull.startsWith(prefix)) {
    return normFull.slice(prefix.length);
  }
  return null;
}

export interface RestoreSuggestion {
  label: string;
  target: string;
  recommended?: boolean;
  syncJobName?: string;
}

export function computeRestoreSuggestions(
  remote: string,
  sourcePath: string,
  jobs: JobWithStatus[]
): RestoreSuggestion[] {
  const cloudOriginal = suggestRestoreTarget(remote, sourcePath);
  const fullSrc = `${remote}:${sourcePath.replace(/^\/+/, '')}`;
  const suggestions: RestoreSuggestion[] = [];
  const seen = new Set<string>();

  for (const job of jobs) {
    // Check if sourcePath is inside a custom backup_dir_base/<date>/<rel>
    if (job.versioning_enabled && job.backup_dir_base) {
      const underBackup = matchPathUnderBase(fullSrc, job.backup_dir_base);
      if (underBackup) {
        const idx = underBackup.indexOf('/');
        if (idx > 0) {
          const rel = underBackup.slice(idx + 1);
          const sourceTarget = joinRcloneSub(job.source, rel);
          if (!seen.has(sourceTarget)) {
            seen.add(sourceTarget);
            suggestions.push({
              label: `Original Source (${job.name})`,
              target: sourceTarget,
              recommended: job.operation === 'sync',
            });
          }
        }
      }
    }

    // Check if logical cloud path matches job.destination
    const relFromDest = matchPathUnderBase(cloudOriginal, job.destination);
    if (relFromDest !== null) {
      const sourceTarget = joinRcloneSub(job.source, relFromDest);
      if (!seen.has(sourceTarget)) {
        seen.add(sourceTarget);
        suggestions.push({
          label: `Original Source (${job.name})`,
          target: sourceTarget,
          recommended: job.operation === 'sync',
        });
      }
    }
  }

  const syncMatch = findSyncJobMatchingTarget(cloudOriginal, jobs);
  if (!seen.has(cloudOriginal)) {
    seen.add(cloudOriginal);
    suggestions.push({
      label: 'Original Cloud Location',
      target: cloudOriginal,
      syncJobName: syncMatch?.name,
    });
  }

  return suggestions;
}

export function findSyncJobMatchingTarget(
  targetPath: string,
  jobs: JobWithStatus[]
): JobWithStatus | null {
  const trimmed = targetPath.trim();
  if (!trimmed) return null;
  for (const job of jobs) {
    if (job.operation !== 'sync') continue;
    if (matchPathUnderBase(trimmed, job.destination) !== null) {
      return job;
    }
  }
  return null;
}

export const COMMON_TIMEZONES = [
  'UTC',
  'America/New_York',
  'America/Chicago',
  'America/Denver',
  'America/Los_Angeles',
  'America/Toronto',
  'Europe/London',
  'Europe/Berlin',
  'Europe/Paris',
  'Europe/Amsterdam',
  'Asia/Tokyo',
  'Asia/Seoul',
  'Asia/Singapore',
  'Asia/Shanghai',
  'Australia/Sydney',
];
