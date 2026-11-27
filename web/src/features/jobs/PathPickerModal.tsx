import { ChevronRight, Folder, HardDrive, Loader2 } from 'lucide-react';
import { useEffect, useState } from 'react';
import { api } from '../../api/client';
import { Modal } from '../../components/ui/Modal';
import type { BrowseEntry, RemoteInfo } from '../../types/api';

interface PathPickerModalProps {
  remotes: RemoteInfo[];
  initialValue?: string;
  onSelect: (rclonePath: string) => void;
  onClose: () => void;
}

export function PathPickerModal({
  remotes,
  initialValue,
  onSelect,
  onClose,
}: PathPickerModalProps) {
  const [selectedRemote, setSelectedRemote] = useState<string>(() => {
    if (initialValue?.includes(':') && !initialValue.startsWith('/')) {
      return initialValue.split(':')[0];
    }
    return remotes[0]?.name ?? '';
  });
  const [subpath, setSubpath] = useState<string>(() => {
    if (initialValue?.includes(':') && !initialValue.startsWith('/')) {
      return initialValue.split(':').slice(1).join(':').replace(/^\/+/, '');
    }
    return '';
  });
  const [entries, setEntries] = useState<BrowseEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    if (!selectedRemote) return;
    let cancelled = false;
    setLoading(true);
    setError('');
    api
      .browse(selectedRemote, subpath)
      .then((res) => {
        if (!cancelled) {
          setEntries((res.entries ?? []).filter((e) => e.IsDir));
        }
      })
      .catch((err) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Failed to list directory');
          setEntries([]);
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [selectedRemote, subpath]);

  const currentRclonePath = selectedRemote ? `${selectedRemote}:${subpath}` : '';
  const crumbs = subpath ? subpath.split('/').filter(Boolean) : [];

  return (
    <Modal
      title="Pick Remote Path"
      subtitle="Select a configured remote and folder"
      onClose={onClose}
      maxWidth="max-w-lg"
      footer={
        <>
          <button
            type="button"
            onClick={onClose}
            className="px-3.5 py-2 rounded-lg text-sm font-medium text-slate-300 hover:bg-slate-800"
          >
            Cancel
          </button>
          <button
            type="button"
            disabled={!selectedRemote}
            onClick={() => {
              onSelect(currentRclonePath);
              onClose();
            }}
            className="px-4 py-2 rounded-lg text-sm font-medium bg-sky-500 hover:bg-sky-400 text-slate-950 disabled:opacity-50"
          >
            Use &quot;{currentRclonePath || '...'}&quot;
          </button>
        </>
      }
    >
      {remotes.length === 0 ? (
        <p className="text-sm text-slate-400">
          No remotes configured yet. Add a remote first or enter a local filesystem path manually.
        </p>
      ) : (
        <div className="space-y-3">
          <div>
            <label className="block text-xs font-medium text-slate-400 mb-1.5">Remote</label>
            <div className="flex flex-wrap gap-1.5">
              {remotes.map((r) => (
                <button
                  key={r.name}
                  type="button"
                  onClick={() => {
                    setSelectedRemote(r.name);
                    setSubpath('');
                  }}
                  className={`inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-xs font-medium border transition-colors ${
                    selectedRemote === r.name
                      ? 'bg-sky-500/20 border-sky-500 text-sky-300'
                      : 'bg-slate-800/70 border-slate-700 text-slate-300 hover:border-slate-600'
                  }`}
                >
                  <HardDrive className="w-3.5 h-3.5" />
                  {r.name}
                  <span className="text-[10px] text-slate-400">({r.type})</span>
                </button>
              ))}
            </div>
          </div>

          <div className="flex items-center gap-1 text-xs bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 overflow-x-auto">
            <button
              type="button"
              onClick={() => setSubpath('')}
              className="text-sky-400 hover:underline font-mono shrink-0"
            >
              {selectedRemote}:
            </button>
            {crumbs.map((c, idx) => {
              const target = crumbs.slice(0, idx + 1).join('/');
              return (
                <span key={target} className="inline-flex items-center gap-1 shrink-0">
                  <ChevronRight className="w-3.5 h-3.5 text-slate-600" />
                  <button
                    type="button"
                    onClick={() => setSubpath(target)}
                    className="text-slate-200 hover:text-sky-400 font-mono"
                  >
                    {c}
                  </button>
                </span>
              );
            })}
          </div>

          {error && (
            <div className="p-2.5 rounded-lg bg-rose-500/10 border border-rose-500/30 text-xs text-rose-300">
              {error}
            </div>
          )}

          <div className="border border-slate-800 rounded-xl bg-slate-950/60 divide-y divide-slate-800/70 max-h-64 overflow-y-auto">
            {loading ? (
              <div className="flex items-center justify-center py-8 text-slate-400 text-sm gap-2">
                <Loader2 className="w-4 h-4 animate-spin" />
                Loading folders...
              </div>
            ) : entries.length === 0 ? (
              <div className="py-8 text-center text-xs text-slate-500">
                No subdirectories here. You can select this folder directly.
              </div>
            ) : (
              entries.map((entry) => (
                <button
                  key={entry.Path}
                  type="button"
                  onClick={() => setSubpath(subpath ? `${subpath}/${entry.Name}` : entry.Name)}
                  className="w-full flex items-center justify-between px-3.5 py-2.5 text-left hover:bg-slate-800/50 transition-colors"
                >
                  <span className="inline-flex items-center gap-2.5 text-sm text-slate-200">
                    <Folder className="w-4 h-4 text-sky-400" />
                    {entry.Name}
                  </span>
                  <ChevronRight className="w-4 h-4 text-slate-500" />
                </button>
              ))
            )}
          </div>
        </div>
      )}
    </Modal>
  );
}
