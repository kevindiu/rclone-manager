import assert from 'node:assert/strict';
import test from 'node:test';
import type { JobWithStatus } from '../src/types/api.ts';
import {
  computeRestoreSuggestions,
  findSyncJobMatchingTarget,
  formatBytes,
  formatDurationMs,
  formatETA,
  formatSpeed,
  suggestRestoreTarget,
} from '../src/utils/format.ts';

function makeJob(overrides: Partial<JobWithStatus>): JobWithStatus {
  return {
    id: 'job-1',
    name: 'Test Job',
    enabled: true,
    operation: 'copy',
    source: '/local/docs',
    destination: 'r2:docs',
    versioning_enabled: true,
    version_retention_days: 30,
    abort_if_source_empty: false,
    is_running: false,
    ...overrides,
  };
}

test('formatting helpers handle null, zero, and scaling', () => {
  assert.equal(formatBytes(null), '—');
  assert.equal(formatBytes(undefined), '—');
  assert.equal(formatBytes(0), '0 B');
  assert.equal(formatBytes(512), '512 B');
  assert.equal(formatBytes(1536), '1.50 KiB');
  assert.equal(formatBytes(1024 ** 3), '1.00 GiB');
  assert.equal(formatBytes(1024 ** 4), '1.00 TiB');

  assert.equal(formatSpeed(0), '0 B/s');
  assert.equal(formatSpeed(-5), '0 B/s');
  assert.equal(formatSpeed(2048), '2.00 KiB/s');

  assert.equal(formatDurationMs(0), '0s');
  assert.equal(formatDurationMs(-10), '0s');
  assert.equal(formatDurationMs(45000), '45s');
  assert.equal(formatDurationMs(125000), '2m 5s');
  assert.equal(formatDurationMs(3720000), '1h 2m');

  assert.equal(formatETA(null), '—');
  assert.equal(formatETA(-1), '—');
  assert.equal(formatETA(30), '30s');
  assert.equal(formatETA(90), '1m 30s');
  assert.equal(formatETA(3660), '1h 1m');
});

test('suggestRestoreTarget strips root and nested _versions/<date>', () => {
  assert.equal(suggestRestoreTarget('r2', '_versions/2026-11-26/a/b.txt'), 'r2:a/b.txt');
  assert.equal(
    suggestRestoreTarget('r2', 'photos/_versions/2026-11-26/img.jpg'),
    'r2:photos/img.jpg'
  );
  assert.equal(suggestRestoreTarget('r2', '/docs/a.txt'), 'r2:docs/a.txt');
});

test('findSyncJobMatchingTarget avoids sibling prefix false positives', () => {
  const jobs: JobWithStatus[] = [
    makeJob({ id: 'sync-docs', name: 'Sync Docs', operation: 'sync', destination: 'r2:docs' }),
    makeJob({
      id: 'copy-photos',
      name: 'Copy Photos',
      operation: 'copy',
      destination: 'r2:photos',
    }),
  ];

  assert.equal(findSyncJobMatchingTarget('r2:docs/sub/report.pdf', jobs)?.id, 'sync-docs');
  assert.equal(findSyncJobMatchingTarget('r2:docs2/report.pdf', jobs), null);
  assert.equal(findSyncJobMatchingTarget('r2:photos/a.jpg', jobs), null);

  const rootSyncJobs = [
    makeJob({ id: 'sync-root', name: 'Root Sync', operation: 'sync', destination: 'b2:' }),
  ];
  assert.equal(findSyncJobMatchingTarget('b2:anything/file.txt', rootSyncJobs)?.id, 'sync-root');
});

test('computeRestoreSuggestions recommends source for sync jobs and warns on cloud target', () => {
  const jobs: JobWithStatus[] = [
    makeJob({
      id: 'j-sync',
      name: 'Docs Sync',
      operation: 'sync',
      source: '/nas/docs',
      destination: 'r2:backups/docs',
      backup_dir_base: 'r2:archive/docs-ver',
    }),
    makeJob({
      id: 'j-copy',
      name: 'Photos Copy',
      operation: 'copy',
      source: '/nas/photos',
      destination: 'r2:photos',
    }),
  ];

  // Version inside custom backup_dir_base for a sync job recommends original source + flags sync warning on cloud target.
  const syncSuggestions = computeRestoreSuggestions(
    'r2',
    'archive/docs-ver/2026-11-26/tax.pdf',
    jobs
  );
  assert.equal(syncSuggestions[0]?.target, '/nas/docs/tax.pdf');
  assert.equal(syncSuggestions[0]?.recommended, true);

  const fromDestSuggestions = computeRestoreSuggestions(
    'r2',
    'backups/docs/_versions/2026-11-26/tax.pdf',
    jobs
  );
  assert.equal(fromDestSuggestions[0]?.target, '/nas/docs/tax.pdf');
  assert.equal(fromDestSuggestions[0]?.recommended, true);
  assert.equal(fromDestSuggestions[1]?.target, 'r2:backups/docs/tax.pdf');
  assert.equal(fromDestSuggestions[1]?.syncJobName, 'Docs Sync');

  // Copy job gives non-recommended source suggestion and no sync warning on cloud target.
  const copySuggestions = computeRestoreSuggestions('r2', 'photos/trip.jpg', jobs);
  assert.equal(copySuggestions[0]?.target, '/nas/photos/trip.jpg');
  assert.equal(copySuggestions[0]?.recommended, false);
  assert.equal(copySuggestions[1]?.target, 'r2:photos/trip.jpg');
  assert.equal(copySuggestions[1]?.syncJobName, undefined);
});
