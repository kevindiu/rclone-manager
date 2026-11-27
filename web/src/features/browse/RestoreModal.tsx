import { Loader2, RotateCcw } from 'lucide-react';
import { useState } from 'react';
import { api } from '../../api/client';
import { Modal } from '../../components/ui/Modal';
import type { BrowseEntry } from '../../types/api';
import { suggestRestoreTarget } from '../../utils/format';

export function RestoreModal({
  remote,
  entry,
  fullPath,
  onRestored,
  onClose,
}: {
  remote: string;
  entry: BrowseEntry;
  fullPath: string;
  onRestored: (msg: string) => void;
  onClose: () => void;
}) {
  const [targetPath, setTargetPath] = useState(() => suggestRestoreTarget(remote, fullPath));
  const [restoring, setRestoring] = useState(false);
  const [error, setError] = useState('');

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setRestoring(true);
    setError('');
    try {
      await api.restore(remote, fullPath, targetPath.trim(), entry.IsDir);
      onRestored(`Restored ${entry.Name} to ${targetPath.trim()}`);
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Restore failed');
    } finally {
      setRestoring(false);
    }
  };

  return (
    <Modal
      title={`Restore ${entry.IsDir ? 'Folder' : 'File'}`}
      subtitle={`From ${remote}:${fullPath}`}
      onClose={onClose}
      maxWidth="max-w-lg"
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
            type="submit"
            form="restore-form"
            disabled={restoring || !targetPath.trim()}
            className="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-semibold bg-emerald-500 hover:bg-emerald-400 text-slate-950 disabled:opacity-50"
          >
            {restoring ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <RotateCcw className="w-4 h-4" />
            )}
            Restore Now
          </button>
        </>
      }
    >
      <form id="restore-form" onSubmit={handleSubmit} className="space-y-4">
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
        <div>
          <label className="block text-xs font-medium text-slate-300 mb-1">
            Target Path (remote:path or local path)
          </label>
          <input
            required
            value={targetPath}
            onChange={(e) => setTargetPath(e.target.value)}
            className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100 focus:outline-none focus:border-emerald-500"
          />
          <p className="text-[11px] text-slate-500 mt-1">
            For <code>_versions/YYYY-MM-DD/</code> items, the original path is pre-filled
            automatically.
          </p>
        </div>
      </form>
    </Modal>
  );
}
