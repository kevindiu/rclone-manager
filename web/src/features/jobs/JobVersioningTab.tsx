import { FolderSearch } from 'lucide-react';

export function JobVersioningTab({
  versioningEnabled,
  setVersioningEnabled,
  backupDirBase,
  setBackupDirBase,
  versionFormat,
  setVersionFormat,
  versionRetentionDays,
  setVersionRetentionDays,
  derivedDefaultBackupDir,
  onPickBackupDir,
}: {
  versioningEnabled: boolean;
  setVersioningEnabled: (v: boolean) => void;
  backupDirBase: string;
  setBackupDirBase: (v: string) => void;
  versionFormat: string;
  setVersionFormat: (v: string) => void;
  versionRetentionDays: number;
  setVersionRetentionDays: (v: number) => void;
  derivedDefaultBackupDir: string;
  onPickBackupDir: () => void;
}) {
  return (
    <div className="space-y-4">
      <label className="flex items-start gap-3 p-3.5 rounded-xl bg-slate-950 border border-slate-800 cursor-pointer">
        <input
          type="checkbox"
          checked={versioningEnabled}
          onChange={(e) => setVersioningEnabled(e.target.checked)}
          className="mt-0.5 w-4 h-4 rounded accent-emerald-500"
        />
        <div>
          <div className="text-sm font-medium text-slate-100">
            Enable Incremental Delete Protection (<code>--backup-dir</code>)
          </div>
          <p className="text-xs text-slate-400 mt-0.5">
            Moves modified or deleted destination files into dated version folders instead of
            permanently overwriting them, without duplicating unchanged data.
          </p>
        </div>
      </label>

      {versioningEnabled && (
        <div className="space-y-3.5 pl-1">
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Backup Dir Base{' '}
              <span className="text-slate-500 font-normal">
                (blank defaults to <code>{derivedDefaultBackupDir}</code>)
              </span>
            </label>
            <div className="flex gap-2">
              <input
                value={backupDirBase}
                onChange={(e) => setBackupDirBase(e.target.value)}
                placeholder={derivedDefaultBackupDir}
                className="flex-1 px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100 focus:outline-none focus:border-sky-500"
              />
              <button
                type="button"
                onClick={onPickBackupDir}
                className="inline-flex items-center gap-1.5 px-3 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-200 shrink-0"
              >
                <FolderSearch className="w-4 h-4 text-sky-400" />
                Browse
              </button>
            </div>
            <p className="text-[11px] text-slate-500 mt-1">
              If located inside the destination, rclone-manager automatically excludes it so sync
              never deletes archived versions.
            </p>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1">
                Version Folder Retention (Days)
              </label>
              <input
                type="number"
                min={0}
                value={versionRetentionDays}
                onChange={(e) => setVersionRetentionDays(Number(e.target.value))}
                className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm text-slate-100 focus:outline-none focus:border-sky-500"
              />
              <div className="flex flex-wrap gap-1.5 mt-1.5">
                {[
                  { l: 'Keep Forever (0)', v: 0 },
                  { l: '7d', v: 7 },
                  { l: '30d', v: 30 },
                  { l: '90d', v: 90 },
                ].map((p) => (
                  <button
                    key={p.v}
                    type="button"
                    onClick={() => setVersionRetentionDays(p.v)}
                    className="px-2 py-0.5 rounded text-[11px] bg-slate-800 hover:bg-slate-700 text-slate-300"
                  >
                    {p.l}
                  </button>
                ))}
              </div>
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1">
                Date Folder Layout (Go format)
              </label>
              <input
                value={versionFormat}
                onChange={(e) => setVersionFormat(e.target.value)}
                placeholder="2006-01-02"
                className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100 focus:outline-none focus:border-sky-500"
              />
              <p className="text-[11px] text-slate-500 mt-1">
                Default <code>2006-01-02</code> produces <code>YYYY-MM-DD</code>.
              </p>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
