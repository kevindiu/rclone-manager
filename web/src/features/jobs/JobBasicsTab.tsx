import { ArrowUpDown, FolderSearch } from 'lucide-react';
import type { Job } from '../../types/api';

export function JobBasicsTab({
  name,
  setName,
  description,
  setDescription,
  operation,
  setOperation,
  bisyncConflictResolve,
  setBisyncConflictResolve,
  source,
  setSource,
  destination,
  setDestination,
  enabled,
  setEnabled,
  onPickPath,
}: {
  name: string;
  setName: (v: string) => void;
  description: string;
  setDescription: (v: string) => void;
  operation: Job['operation'];
  setOperation: (v: Job['operation']) => void;
  bisyncConflictResolve: NonNullable<Job['bisync_conflict_resolve']>;
  setBisyncConflictResolve: (v: NonNullable<Job['bisync_conflict_resolve']>) => void;
  source: string;
  setSource: (v: string) => void;
  destination: string;
  setDestination: (v: string) => void;
  enabled: boolean;
  setEnabled: (v: boolean) => void;
  onPickPath: (target: 'source' | 'destination') => void;
}) {
  return (
    <div className="space-y-4">
      <div>
        <label className="block text-xs font-medium text-slate-300 mb-1">Job Name *</label>
        <input
          required
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="e.g. Photos to Cloudflare R2"
          className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm text-slate-100 focus:outline-none focus:border-sky-500"
        />
      </div>

      <div>
        <label className="block text-xs font-medium text-slate-300 mb-1.5">Operation Mode</label>
        <div className="grid grid-cols-5 gap-1 p-1 rounded-xl bg-slate-950 border border-slate-800">
          {(
            [
              {
                id: 'copy',
                label: 'Copy',
                activeClass: 'bg-emerald-500/20 text-emerald-300 border-emerald-500/50',
              },
              {
                id: 'sync',
                label: 'Sync',
                activeClass: 'bg-amber-500/20 text-amber-300 border-amber-500/50',
              },
              {
                id: 'move',
                label: 'Move',
                activeClass: 'bg-rose-500/20 text-rose-300 border-rose-500/50',
              },
              {
                id: 'check',
                label: 'Check',
                activeClass: 'bg-sky-500/20 text-sky-300 border-sky-500/50',
              },
              {
                id: 'bisync',
                label: 'Bisync',
                activeClass: 'bg-violet-500/20 text-violet-300 border-violet-500/50',
              },
            ] as const
          ).map((op) => {
            const active = operation === op.id;
            return (
              <button
                key={op.id}
                type="button"
                onClick={() => setOperation(op.id)}
                className={`py-1.5 px-2 rounded-lg text-xs font-semibold border transition-colors ${
                  active
                    ? op.activeClass
                    : 'border-transparent text-slate-400 hover:text-slate-200 hover:bg-slate-900'
                }`}
              >
                {op.label}
              </button>
            );
          })}
        </div>

        <p className="text-[11px] text-slate-400 mt-1.5">
          {operation === 'copy' &&
            'Safer backup: copies new & changed files from Source to Destination without deleting anything.'}
          {operation === 'sync' && (
            <span className="text-amber-300/90">
              Exact mirror: deletes files in Destination missing from Source. Enable Versioning or
              Guards to stay safe.
            </span>
          )}
          {operation === 'move' && (
            <span className="text-rose-300/90">
              Transfer &amp; delete: moves files to Destination and deletes each verified Source
              file afterward.
            </span>
          )}
          {operation === 'check' && (
            <span className="text-sky-300/90">
              Read-only check: compares hashes and sizes without modifying any files.
            </span>
          )}
          {operation === 'bisync' && (
            <span className="text-violet-300/90">
              Two-way sync: syncs changes both ways (run Resync from Jobs menu on first setup).
            </span>
          )}
        </p>

        {operation === 'bisync' && (
          <div className="mt-2">
            <label
              htmlFor="job-bisync-conflict"
              className="block text-xs font-medium text-slate-300 mb-1"
            >
              Conflict Resolution (<code>--conflict-resolve</code>)
            </label>
            <select
              id="job-bisync-conflict"
              value={bisyncConflictResolve}
              onChange={(e) =>
                setBisyncConflictResolve(
                  e.target.value as NonNullable<Job['bisync_conflict_resolve']>
                )
              }
              className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-xs text-slate-100 focus:outline-none focus:border-violet-500"
            >
              <option value="">Default (none — fail / rename on conflict)</option>
              <option value="newer">newer (keep most recently modified)</option>
              <option value="older">older (keep oldest)</option>
              <option value="larger">larger (keep largest file)</option>
              <option value="smaller">smaller (keep smallest file)</option>
              <option value="path1">path1 (always prefer Source)</option>
              <option value="path2">path2 (always prefer Destination)</option>
              <option value="none">none (fail run on conflict)</option>
            </select>
          </div>
        )}
      </div>

      <div>
        <label className="block text-xs font-medium text-slate-300 mb-1">Description</label>
        <input
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          placeholder="Optional note about this backup job"
          className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm text-slate-100 focus:outline-none focus:border-sky-500"
        />
      </div>

      <div>
        <label className="block text-xs font-medium text-slate-300 mb-1">
          Source Path *{' '}
          <span className="text-slate-500 font-normal">
            (local folder like /data/photos or remote:path)
          </span>
        </label>
        <div className="flex gap-2">
          <input
            required
            value={source}
            onChange={(e) => setSource(e.target.value)}
            placeholder="/data/photos or gdrive:Photos"
            className="flex-1 px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100 focus:outline-none focus:border-sky-500"
          />
          <button
            type="button"
            onClick={() => onPickPath('source')}
            className="inline-flex items-center gap-1.5 px-3 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-200 shrink-0"
          >
            <FolderSearch className="w-4 h-4 text-sky-400" />
            Browse
          </button>
        </div>
      </div>

      <div>
        <div className="flex items-center justify-between gap-2 mb-1">
          <label className="block text-xs font-medium text-slate-300">
            Destination Path *{' '}
            <span className="text-slate-500 font-normal">
              (remote:path or local folder like /mnt/backup)
            </span>
          </label>
          {(source.trim() || destination.trim()) && (
            <button
              type="button"
              onClick={() => {
                const prevSource = source;
                setSource(destination);
                setDestination(prevSource);
              }}
              className="inline-flex items-center gap-1 text-[11px] font-medium text-sky-400 hover:text-sky-300 shrink-0"
              title="Swap source and destination paths"
            >
              <ArrowUpDown className="w-3 h-3" />
              Swap
            </button>
          )}
        </div>
        <div className="flex gap-2">
          <input
            required
            value={destination}
            onChange={(e) => setDestination(e.target.value)}
            placeholder="r2:backups/photos or /mnt/backup"
            className="flex-1 px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100 focus:outline-none focus:border-sky-500"
          />
          <button
            type="button"
            onClick={() => onPickPath('destination')}
            className="inline-flex items-center gap-1.5 px-3 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-200 shrink-0"
          >
            <FolderSearch className="w-4 h-4 text-sky-400" />
            Browse
          </button>
        </div>
      </div>

      <label className="flex items-center gap-2.5 pt-1 cursor-pointer">
        <input
          type="checkbox"
          checked={enabled}
          onChange={(e) => setEnabled(e.target.checked)}
          className="w-4 h-4 rounded accent-sky-500"
        />
        <span className="text-sm text-slate-200">Job enabled (participates in scheduled runs)</span>
      </label>
    </div>
  );
}
