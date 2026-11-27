import { ArrowUpDown, FolderSearch } from 'lucide-react';

export function JobBasicsTab({
  name,
  setName,
  description,
  setDescription,
  operation,
  setOperation,
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
  operation: 'copy' | 'sync' | 'move' | 'check';
  setOperation: (v: 'copy' | 'sync' | 'move' | 'check') => void;
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
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
          <button
            type="button"
            onClick={() => setOperation('copy')}
            className={`p-3 rounded-xl border text-left transition-colors ${
              operation === 'copy'
                ? 'bg-emerald-500/10 border-emerald-500/60 text-slate-100'
                : 'bg-slate-950 border-slate-800 text-slate-300 hover:border-slate-700'
            }`}
          >
            <div className="flex items-center justify-between gap-2">
              <span className="text-xs font-semibold">Copy (Backup Only)</span>
              <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-emerald-500/20 text-emerald-300">
                Safer
              </span>
            </div>
            <p className="text-[11px] text-slate-400 mt-1 leading-relaxed">
              Copies new &amp; updated files from Source to Destination. Never deletes files on
              Destination.
            </p>
          </button>

          <button
            type="button"
            onClick={() => setOperation('sync')}
            className={`p-3 rounded-xl border text-left transition-colors ${
              operation === 'sync'
                ? 'bg-amber-500/10 border-amber-500/60 text-slate-100'
                : 'bg-slate-950 border-slate-800 text-slate-300 hover:border-slate-700'
            }`}
          >
            <div className="flex items-center justify-between gap-2">
              <span className="text-xs font-semibold">Sync (Exact Mirror)</span>
              <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-amber-500/20 text-amber-300">
                Deletes dest
              </span>
            </div>
            <p className="text-[11px] text-slate-400 mt-1 leading-relaxed">
              Makes Destination match Source exactly — deletes files in Destination removed from
              Source.
            </p>
          </button>

          <button
            type="button"
            onClick={() => setOperation('move')}
            className={`p-3 rounded-xl border text-left transition-colors ${
              operation === 'move'
                ? 'bg-rose-500/10 border-rose-500/60 text-slate-100'
                : 'bg-slate-950 border-slate-800 text-slate-300 hover:border-slate-700'
            }`}
          >
            <div className="flex items-center justify-between gap-2">
              <span className="text-xs font-semibold">Move (Transfer &amp; Delete Source)</span>
              <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-rose-500/20 text-rose-300">
                Deletes src
              </span>
            </div>
            <p className="text-[11px] text-slate-400 mt-1 leading-relaxed">
              Moves files to Destination and deletes each verified Source file afterward to free
              local space.
            </p>
          </button>

          <button
            type="button"
            onClick={() => setOperation('check')}
            className={`p-3 rounded-xl border text-left transition-colors ${
              operation === 'check'
                ? 'bg-sky-500/10 border-sky-500/60 text-slate-100'
                : 'bg-slate-950 border-slate-800 text-slate-300 hover:border-slate-700'
            }`}
          >
            <div className="flex items-center justify-between gap-2">
              <span className="text-xs font-semibold">Check (Verify Integrity)</span>
              <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-sky-500/20 text-sky-300">
                Read-only
              </span>
            </div>
            <p className="text-[11px] text-slate-400 mt-1 leading-relaxed">
              Verifies hashes and sizes between Source and Destination without modifying any files.
            </p>
          </button>
        </div>
        {operation === 'sync' && (
          <p className="text-[11px] text-amber-300/90 mt-1.5">
            Tip: Enable Versioning or Safety Guards to protect against accidental deletions.
          </p>
        )}
        {operation === 'move' && (
          <p className="text-[11px] text-rose-300/90 mt-1.5">
            Warning: Source files are deleted as soon as they finish transferring to Destination.
          </p>
        )}
        {operation === 'check' && (
          <p className="text-[11px] text-sky-300/90 mt-1.5">
            Note: Any missing or mismatched files fail the run and appear in run logs.
          </p>
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
