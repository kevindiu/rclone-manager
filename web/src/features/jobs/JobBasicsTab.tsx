import { FolderSearch } from 'lucide-react';

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
  operation: 'copy' | 'sync';
  setOperation: (v: 'copy' | 'sync') => void;
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
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <div className="sm:col-span-2">
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
          <label className="block text-xs font-medium text-slate-300 mb-1">Operation</label>
          <select
            value={operation}
            onChange={(e) => setOperation(e.target.value as 'copy' | 'sync')}
            className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm text-slate-100 focus:outline-none focus:border-sky-500"
          >
            <option value="sync">sync (mirror + deletes)</option>
            <option value="copy">copy (additive only)</option>
          </select>
        </div>
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
            (local path like /data/photos or remote:path)
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
        <label className="block text-xs font-medium text-slate-300 mb-1">Destination Path *</label>
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
