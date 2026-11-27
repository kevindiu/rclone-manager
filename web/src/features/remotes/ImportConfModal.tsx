import { Loader2, Upload } from 'lucide-react';
import { useState } from 'react';
import { api } from '../../api/client';
import { Modal } from '../../components/ui/Modal';

export function ImportConfModal({
  onImported,
  onClose,
}: {
  onImported: () => void;
  onClose: () => void;
}) {
  const [content, setContent] = useState('');
  const [mode, setMode] = useState<'merge' | 'replace'>('merge');
  const [importing, setImporting] = useState(false);
  const [error, setError] = useState('');

  const handleFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setContent(await file.text());
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setImporting(true);
    setError('');
    try {
      await api.importConf(content, mode);
      onImported();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Import failed');
    } finally {
      setImporting(false);
    }
  };

  return (
    <Modal
      title="Import rclone.conf"
      subtitle="Validated by rclone before saving; secrets are masked in the Web UI"
      onClose={onClose}
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
            form="import-conf-form"
            disabled={importing || !content.trim()}
            className="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-semibold bg-sky-500 hover:bg-sky-400 text-slate-950 disabled:opacity-50"
          >
            {importing && <Loader2 className="w-4 h-4 animate-spin" />}
            Import Remotes
          </button>
        </>
      }
    >
      <form id="import-conf-form" onSubmit={handleSubmit} className="space-y-4">
        {error && (
          <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-xs text-rose-300">
            {error}
          </div>
        )}

        <div className="flex flex-wrap items-center justify-between gap-2">
          <label className="inline-flex items-center gap-2 px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-200 cursor-pointer">
            <Upload className="w-3.5 h-3.5 text-sky-400" />
            Load rclone.conf file
            <input type="file" accept=".conf,.ini,.txt" onChange={handleFile} className="hidden" />
          </label>

          <div className="flex items-center gap-3 text-xs text-slate-300">
            <label className="inline-flex items-center gap-1.5 cursor-pointer">
              <input
                type="radio"
                checked={mode === 'merge'}
                onChange={() => setMode('merge')}
                className="accent-sky-500"
              />
              Merge (update/add)
            </label>
            <label className="inline-flex items-center gap-1.5 cursor-pointer">
              <input
                type="radio"
                checked={mode === 'replace'}
                onChange={() => setMode('replace')}
                className="accent-rose-500"
              />
              Replace all
            </label>
          </div>
        </div>

        <textarea
          rows={10}
          value={content}
          onChange={(e) => setContent(e.target.value)}
          placeholder={`[myremote]\ntype = s3\nprovider = Cloudflare\n...`}
          className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-xs font-mono text-slate-100 focus:outline-none focus:border-sky-500"
        />
      </form>
    </Modal>
  );
}
