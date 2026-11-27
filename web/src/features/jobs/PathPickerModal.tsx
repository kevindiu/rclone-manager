import {
  ChevronRight,
  CornerLeftUp,
  Folder,
  FolderRoot,
  HardDrive,
  Loader2,
  Search,
} from 'lucide-react';
import { useEffect, useMemo, useState } from 'react';
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
  const trimmedInit = initialValue?.trim() ?? '';
  const initIsLocal =
    trimmedInit.startsWith('/') || (!trimmedInit.includes(':') && remotes.length === 0);

  const [mode, setMode] = useState<'local' | 'remote'>(initIsLocal ? 'local' : 'remote');
  const [localPath, setLocalPath] = useState<string>(() =>
    trimmedInit.startsWith('/') ? trimmedInit : '/'
  );
  const [selectedRemote, setSelectedRemote] = useState<string>(() => {
    if (trimmedInit.includes(':') && !trimmedInit.startsWith('/')) {
      return trimmedInit.split(':')[0];
    }
    return remotes[0]?.name ?? '';
  });
  const [subpath, setSubpath] = useState<string>(() => {
    if (trimmedInit.includes(':') && !trimmedInit.startsWith('/')) {
      return trimmedInit.split(':').slice(1).join(':').replace(/^\/+/, '');
    }
    return '';
  });
  const [entries, setEntries] = useState<BrowseEntry[]>([]);
  const [filter, setFilter] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;
    setFilter('');

    if (mode === 'local') {
      setLoading(true);
      setError('');
      api
        .browseLocal(localPath || '/')
        .then((res) => {
          if (!cancelled) {
            setEntries(res.entries ?? []);
          }
        })
        .catch((err) => {
          if (!cancelled) {
            setError(err instanceof Error ? err.message : 'Failed to list local directory');
            setEntries([]);
          }
        })
        .finally(() => {
          if (!cancelled) setLoading(false);
        });
      return () => {
        cancelled = true;
      };
    }

    if (!selectedRemote) {
      setEntries([]);
      setError('');
      return;
    }

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
  }, [mode, localPath, selectedRemote, subpath]);

  const currentPath =
    mode === 'local' ? localPath || '/' : selectedRemote ? `${selectedRemote}:${subpath}` : '';

  const crumbs =
    mode === 'local'
      ? (localPath || '/').split('/').filter(Boolean)
      : subpath
        ? subpath.split('/').filter(Boolean)
        : [];

  const handleSelectEntry = (entry: BrowseEntry) => {
    if (mode === 'local') {
      const next =
        entry.Path ||
        (localPath === '/' ? `/${entry.Name}` : `${localPath.replace(/\/+$/, '')}/${entry.Name}`);
      setLocalPath(next);
    } else {
      setSubpath(subpath ? `${subpath}/${entry.Name}` : entry.Name);
    }
  };

  const canGoUp = (mode === 'local' && localPath !== '/') || (mode === 'remote' && subpath !== '');
  const handleGoUp = () => {
    if (mode === 'local') {
      const parentParts = (localPath || '/').split('/').filter(Boolean).slice(0, -1);
      setLocalPath(parentParts.length === 0 ? '/' : `/${parentParts.join('/')}`);
    } else {
      const parentParts = subpath.split('/').filter(Boolean).slice(0, -1);
      setSubpath(parentParts.join('/'));
    }
  };

  const visibleEntries = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return entries;
    return entries.filter((e) => e.Name.toLowerCase().includes(q));
  }, [entries, filter]);

  return (
    <Modal
      title="Pick Folder Path"
      subtitle="Choose a local directory or a configured remote folder"
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
            disabled={!currentPath}
            onClick={() => {
              onSelect(currentPath);
              onClose();
            }}
            className="px-4 py-2 rounded-lg text-sm font-medium bg-sky-500 hover:bg-sky-400 text-slate-950 disabled:opacity-50"
          >
            Use &quot;{currentPath || '...'}&quot;
          </button>
        </>
      }
    >
      <div className="space-y-3">
        <div>
          <label className="block text-xs font-medium text-slate-400 mb-1.5">Location</label>
          <div className="flex flex-wrap gap-1.5">
            <button
              type="button"
              onClick={() => setMode('local')}
              className={`inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-xs font-medium border transition-colors ${
                mode === 'local'
                  ? 'bg-emerald-500/20 border-emerald-500 text-emerald-300'
                  : 'bg-slate-800/70 border-slate-700 text-slate-300 hover:border-slate-600'
              }`}
            >
              <FolderRoot className="w-3.5 h-3.5" />
              Local Folder
            </button>

            {remotes.map((r) => (
              <button
                key={r.name}
                type="button"
                onClick={() => {
                  setMode('remote');
                  setSelectedRemote(r.name);
                  setSubpath('');
                }}
                className={`inline-flex items-center gap-1.5 px-2.5 py-1.5 rounded-lg text-xs font-medium border transition-colors ${
                  mode === 'remote' && selectedRemote === r.name
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
            onClick={() => (mode === 'local' ? setLocalPath('/') : setSubpath(''))}
            className="text-sky-400 hover:underline font-mono shrink-0"
          >
            {mode === 'local' ? '/' : `${selectedRemote}:`}
          </button>
          {crumbs.map((c, idx) => {
            const target =
              mode === 'local'
                ? `/${crumbs.slice(0, idx + 1).join('/')}`
                : crumbs.slice(0, idx + 1).join('/');
            return (
              <span key={target} className="inline-flex items-center gap-1 shrink-0">
                <ChevronRight className="w-3.5 h-3.5 text-slate-600" />
                <button
                  type="button"
                  onClick={() => (mode === 'local' ? setLocalPath(target) : setSubpath(target))}
                  className="text-slate-200 hover:text-sky-400 font-mono"
                >
                  {c}
                </button>
              </span>
            );
          })}
        </div>

        {error && (
          <div className="p-2.5 rounded-lg bg-rose-500/10 border border-rose-500/30 text-xs text-rose-300 flex items-center justify-between gap-2">
            <span>{error}</span>
            {mode === 'local' && localPath !== '/' && (
              <button
                type="button"
                onClick={() => setLocalPath('/')}
                className="px-2 py-1 rounded bg-rose-500/20 hover:bg-rose-500/30 text-rose-200 shrink-0"
              >
                Go to /
              </button>
            )}
          </div>
        )}

        <div className="relative">
          <Search className="w-3.5 h-3.5 text-slate-500 absolute left-3 top-2.5" />
          <input
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="Filter folders in current location..."
            className="w-full pl-8 pr-3 py-1.5 rounded-lg bg-slate-950 border border-slate-800 text-xs text-slate-100 focus:outline-none focus:border-sky-500"
          />
        </div>

        <div className="border border-slate-800 rounded-xl bg-slate-950/60 divide-y divide-slate-800/70 max-h-64 overflow-y-auto">
          {canGoUp && !loading && (
            <button
              type="button"
              onClick={handleGoUp}
              className="w-full flex items-center gap-2.5 px-3.5 py-2 text-left text-xs text-slate-400 hover:text-slate-200 hover:bg-slate-800/50 transition-colors"
            >
              <CornerLeftUp className="w-3.5 h-3.5" />
              <span className="font-mono">.. (Parent folder)</span>
            </button>
          )}
          {loading ? (
            <div className="flex items-center justify-center py-8 text-slate-400 text-sm gap-2">
              <Loader2 className="w-4 h-4 animate-spin" />
              Loading folders...
            </div>
          ) : visibleEntries.length === 0 ? (
            <div className="py-8 text-center text-xs text-slate-500">
              {entries.length === 0
                ? 'No subdirectories here. You can select this folder directly.'
                : 'No folders match your filter.'}
            </div>
          ) : (
            visibleEntries.map((entry) => (
              <button
                key={entry.Path}
                type="button"
                onClick={() => handleSelectEntry(entry)}
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
    </Modal>
  );
}
