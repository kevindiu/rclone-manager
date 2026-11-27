import { AlertTriangle, FlaskConical, FolderSearch, Loader2, RotateCcw } from 'lucide-react';
import { useMemo, useState } from 'react';
import { api } from '../../api/client';
import { Modal } from '../../components/ui/Modal';
import type {
  BrowseEntry,
  JobRun,
  JobWithStatus,
  RemoteInfo,
  RestoreConflictMode,
} from '../../types/api';
import { computeRestoreSuggestions, findSyncJobMatchingTarget } from '../../utils/format';
import { PathPickerModal } from '../jobs/PathPickerModal';

export function RestoreModal({
  remote,
  entry,
  fullPath,
  remotes,
  jobs,
  onStarted,
  onClose,
}: {
  remote: string;
  entry: BrowseEntry;
  fullPath: string;
  remotes: RemoteInfo[];
  jobs: JobWithStatus[];
  onStarted: (run: JobRun) => void;
  onClose: () => void;
}) {
  const suggestions = useMemo(
    () => computeRestoreSuggestions(remote, fullPath, jobs),
    [remote, fullPath, jobs]
  );

  const [targetPath, setTargetPath] = useState(
    () => suggestions.find((s) => s.recommended)?.target ?? suggestions[0]?.target ?? ''
  );
  const [conflictMode, setConflictMode] = useState<RestoreConflictMode>('overwrite');
  const [showPicker, setShowPicker] = useState(false);
  const [submitting, setSubmitting] = useState<'restore' | 'dry' | null>(null);
  const [error, setError] = useState('');

  const syncWarningJob = useMemo(
    () => findSyncJobMatchingTarget(targetPath, jobs),
    [targetPath, jobs]
  );

  const launchRestore = async (dryRun: boolean) => {
    setSubmitting(dryRun ? 'dry' : 'restore');
    setError('');
    try {
      const run = await api.restore(remote, fullPath, targetPath.trim(), entry.IsDir, {
        conflict_mode: conflictMode,
        dry_run: dryRun,
      });
      onStarted(run);
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to start restore');
    } finally {
      setSubmitting(null);
    }
  };

  const handlePickedFolder = (pickedDir: string) => {
    if (entry.IsDir) {
      const clean = pickedDir.endsWith(':')
        ? `${pickedDir}${entry.Name}`
        : `${pickedDir.replace(/\/+$/, '')}/${entry.Name}`;
      setTargetPath(clean);
    } else {
      const clean = pickedDir.endsWith(':')
        ? `${pickedDir}${entry.Name}`
        : `${pickedDir.replace(/\/+$/, '')}/${entry.Name}`;
      setTargetPath(clean);
    }
  };

  return (
    <>
      <Modal
        title={`Restore ${entry.IsDir ? 'Folder' : 'File'}`}
        subtitle={`From ${remote}:${fullPath}`}
        onClose={onClose}
        maxWidth="max-w-xl"
        footer={
          <>
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 rounded-lg text-sm font-medium text-slate-300 hover:bg-slate-800"
            >
              Cancel
            </button>
            <button
              type="button"
              disabled={submitting !== null || !targetPath.trim()}
              onClick={() => void launchRestore(true)}
              className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-lg text-sm font-medium bg-slate-800 hover:bg-slate-700 text-violet-300 disabled:opacity-50"
            >
              {submitting === 'dry' ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : (
                <FlaskConical className="w-4 h-4" />
              )}
              Dry Run
            </button>
            <button
              type="submit"
              form="restore-form"
              disabled={submitting !== null || !targetPath.trim()}
              className="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-semibold bg-emerald-500 hover:bg-emerald-400 text-slate-950 disabled:opacity-50"
            >
              {submitting === 'restore' ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : (
                <RotateCcw className="w-4 h-4" />
              )}
              Restore Now
            </button>
          </>
        }
      >
        <form
          id="restore-form"
          onSubmit={(e) => {
            e.preventDefault();
            void launchRestore(false);
          }}
          className="space-y-4"
        >
          {error && (
            <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-xs text-rose-300">
              {error}
            </div>
          )}

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">Source</label>
            <div className="px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-xs font-mono text-slate-300 break-all">
              {remote}:{fullPath}
            </div>
          </div>

          {suggestions.length > 0 && (
            <div>
              <div className="text-xs font-medium text-slate-400 mb-1.5">
                Quick Target Suggestions
              </div>
              <div className="flex flex-wrap gap-1.5">
                {suggestions.map((s) => (
                  <button
                    key={s.target}
                    type="button"
                    onClick={() => setTargetPath(s.target)}
                    className={`px-2.5 py-1.5 rounded-lg text-xs border transition-colors text-left ${
                      targetPath === s.target
                        ? 'bg-emerald-500/15 border-emerald-500/50 text-emerald-300'
                        : 'bg-slate-950 border-slate-800 text-slate-300 hover:border-slate-700'
                    }`}
                  >
                    <span className="font-medium">{s.label}</span>
                    {s.recommended && (
                      <span className="ml-1.5 px-1.5 py-0.5 rounded text-[10px] bg-emerald-500/20 text-emerald-300">
                        Recommended
                      </span>
                    )}
                  </button>
                ))}
              </div>
            </div>
          )}

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Target Path (remote:path or local path)
            </label>
            <div className="flex gap-2">
              <input
                required
                value={targetPath}
                onChange={(e) => setTargetPath(e.target.value)}
                placeholder="/restore/path or remote:path"
                className="flex-1 px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100 focus:outline-none focus:border-emerald-500"
              />
              <button
                type="button"
                onClick={() => setShowPicker(true)}
                className="inline-flex items-center gap-1.5 px-3 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-200 shrink-0"
              >
                <FolderSearch className="w-4 h-4 text-sky-400" />
                Browse
              </button>
            </div>
          </div>

          {syncWarningJob && (
            <div className="p-3 rounded-xl bg-amber-500/10 border border-amber-500/30 text-xs text-amber-200 flex items-start gap-2">
              <AlertTriangle className="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
              <div>
                <span className="font-semibold">Sync destination warning:</span> Target is inside{' '}
                <code className="text-amber-300">{syncWarningJob.name}</code>&apos;s sync
                destination. If that job runs Local → Cloud sync, restore to the original local
                source instead or the next sync will move/delete this file again.
              </div>
            </div>
          )}

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1.5">
              Conflict Handling
            </label>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-2">
              {(
                [
                  {
                    value: 'overwrite',
                    title: 'Overwrite',
                    desc: 'Replace existing target file(s)',
                  },
                  {
                    value: 'skip',
                    title: 'Skip existing',
                    desc: 'Uses --ignore-existing',
                  },
                  {
                    value: 'newer_only',
                    title: 'Keep newer',
                    desc: 'Uses --update (skip newer target)',
                  },
                ] as const
              ).map((opt) => (
                <button
                  key={opt.value}
                  type="button"
                  onClick={() => setConflictMode(opt.value)}
                  className={`p-2.5 rounded-xl border text-left transition-colors ${
                    conflictMode === opt.value
                      ? 'bg-sky-500/15 border-sky-500/50 text-sky-200'
                      : 'bg-slate-950 border-slate-800 text-slate-300 hover:border-slate-700'
                  }`}
                >
                  <div className="text-xs font-semibold">{opt.title}</div>
                  <div className="text-[11px] text-slate-400 mt-0.5">{opt.desc}</div>
                </button>
              ))}
            </div>
          </div>
        </form>
      </Modal>

      {showPicker && (
        <PathPickerModal
          remotes={remotes}
          initialValue={targetPath}
          onSelect={handlePickedFolder}
          onClose={() => setShowPicker(false)}
        />
      )}
    </>
  );
}
