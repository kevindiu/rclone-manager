import {
  ChevronRight,
  CornerLeftUp,
  Download,
  Eye,
  File,
  Folder,
  FolderPlus,
  HardDrive,
  History,
  Loader2,
  Pencil,
  RefreshCw,
  RotateCcw,
  Search,
  Trash2,
  Upload,
  X,
} from 'lucide-react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import { api } from '../../api/client';
import { ActiveRunCard } from '../../components/ui/ActiveRunCard';
import { ConfirmModal } from '../../components/ui/ConfirmModal';
import { Modal } from '../../components/ui/Modal';
import type { Tab } from '../../hooks/useRoute';
import type {
  ActiveRunSnapshot,
  BrowseEntry,
  DirSize,
  JobRun,
  JobWithStatus,
  RemoteInfo,
} from '../../types/api';
import { formatBytes, formatDateTime } from '../../utils/format';
import { PathPickerModal } from '../jobs/PathPickerModal';
import { FilePreviewModal, isPreviewableFile } from './FilePreviewModal';
import { RestoreModal } from './RestoreModal';
import { VersionHistoryModal } from './VersionHistoryModal';

export function BrowsePage({
  remotes,
  jobs,
  activeRuns,
  initialRemote,
  onNavigate,
  onViewActiveLogs,
  onToast,
}: {
  remotes: RemoteInfo[];
  jobs: JobWithStatus[];
  activeRuns: ActiveRunSnapshot[];
  initialRemote?: string;
  onNavigate: (tab: Tab) => void;
  onViewActiveLogs: (run: ActiveRunSnapshot) => void;
  onToast: (msg: string, isError?: boolean) => void;
}) {
  const [remote, setRemote] = useState(initialRemote || remotes[0]?.name || '');
  const [subpath, setSubpath] = useState('');
  const [entries, setEntries] = useState<BrowseEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [query, setQuery] = useState('');

  const [dirSize, setDirSize] = useState<DirSize | null>(null);
  const [loadingSize, setLoadingSize] = useState(false);
  const [uploading, setUploading] = useState(false);

  const [showMkdir, setShowMkdir] = useState(false);
  const [newFolderName, setNewFolderName] = useState('');
  const [creatingDir, setCreatingDir] = useState(false);

  const [renameTarget, setRenameTarget] = useState<{
    entry: BrowseEntry;
    fullPath: string;
  } | null>(null);
  const [renameValue, setRenameValue] = useState('');
  const [renaming, setRenaming] = useState(false);

  const [deleteTarget, setDeleteTarget] = useState<{
    entry: BrowseEntry;
    fullPath: string;
  } | null>(null);
  const [deleting, setDeleting] = useState(false);

  const [restoreTarget, setRestoreTarget] = useState<{
    entry: BrowseEntry;
    fullPath: string;
  } | null>(null);
  const [versionTarget, setVersionTarget] = useState<{
    entry: BrowseEntry;
    fullPath: string;
  } | null>(null);
  const [previewTarget, setPreviewTarget] = useState<{
    entry: BrowseEntry;
    fullPath: string;
  } | null>(null);
  const [selectedPaths, setSelectedPaths] = useState<Set<string>>(() => new Set());
  const [confirmBulkDelete, setConfirmBulkDelete] = useState(false);
  const [bulkDeleting, setBulkDeleting] = useState(false);
  const [showBulkRestorePicker, setShowBulkRestorePicker] = useState(false);

  useEffect(() => {
    if (initialRemote) {
      setRemote(initialRemote);
      setSubpath('');
    } else if (!remote && remotes[0]?.name) {
      setRemote(remotes[0].name);
    }
  }, [initialRemote, remotes, remote]);

  const loadDir = useCallback(
    async (targetRemote = remote, targetPath = subpath) => {
      if (!targetRemote) return;
      setLoading(true);
      setError('');
      try {
        const res = await api.browse(targetRemote, targetPath);
        setEntries(res.entries ?? []);
      } catch (err) {
        setError(err instanceof Error ? err.message : 'Failed to browse remote');
        setEntries([]);
      } finally {
        setLoading(false);
      }
    },
    [remote, subpath]
  );

  useEffect(() => {
    setDirSize(null);
    setSelectedPaths(new Set());
    if (remote) void loadDir(remote, subpath);
  }, [remote, subpath, loadDir]);

  const activeRestores = useMemo(
    () => activeRuns.filter((r) => r.trigger === 'restore' || r.trigger === 'restore_dry_run'),
    [activeRuns]
  );

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return entries;
    return entries.filter((e) => e.Name.toLowerCase().includes(q));
  }, [entries, query]);

  const crumbs = subpath ? subpath.split('/').filter(Boolean) : [];
  const fullEntryPath = useCallback(
    (name: string) => (subpath ? `${subpath}/${name}` : name),
    [subpath]
  );
  const insideVersionsDir =
    subpath === '_versions' || subpath.startsWith('_versions/') || subpath.includes('/_versions');

  const selectedEntries = useMemo(
    () =>
      filtered
        .map((entry) => ({ entry, fullPath: fullEntryPath(entry.Name) }))
        .filter((item) => selectedPaths.has(item.fullPath)),
    [filtered, fullEntryPath, selectedPaths]
  );

  const allFilteredSelected =
    filtered.length > 0 && filtered.every((e) => selectedPaths.has(fullEntryPath(e.Name)));

  const toggleSelectAll = () => {
    if (allFilteredSelected) {
      setSelectedPaths(new Set());
      return;
    }
    setSelectedPaths(new Set(filtered.map((e) => fullEntryPath(e.Name))));
  };

  const toggleSelectOne = (fullPath: string) => {
    setSelectedPaths((prev) => {
      const next = new Set(prev);
      if (next.has(fullPath)) next.delete(fullPath);
      else next.add(fullPath);
      return next;
    });
  };

  const handleBulkDeleteConfirm = async () => {
    if (!remote || selectedEntries.length === 0) return;
    setBulkDeleting(true);
    try {
      const res = await api.browseBulkDelete(
        remote,
        selectedEntries.map((item) => ({ path: item.fullPath, is_dir: item.entry.IsDir }))
      );
      if (res.failed.length > 0) {
        onToast(`Deleted ${res.deleted} item(s); ${res.failed.length} failed`, true);
      } else {
        onToast(`Deleted ${res.deleted} selected item(s)`);
      }
      setConfirmBulkDelete(false);
      setSelectedPaths(new Set());
      void loadDir(remote, subpath);
    } catch (err) {
      onToast(err instanceof Error ? err.message : 'Bulk delete failed', true);
    } finally {
      setBulkDeleting(false);
    }
  };

  const handleBulkRestoreTarget = async (pickedDir: string) => {
    if (!remote || selectedEntries.length === 0) return;
    let started = 0;
    let failed = 0;
    for (const item of selectedEntries) {
      const target = pickedDir.endsWith(':')
        ? `${pickedDir}${item.entry.Name}`
        : `${pickedDir.replace(/\/+$/, '')}/${item.entry.Name}`;
      try {
        await api.restore(remote, item.fullPath, target, item.entry.IsDir, {
          conflict_mode: 'overwrite',
        });
        started++;
      } catch {
        failed++;
      }
    }
    setSelectedPaths(new Set());
    if (failed > 0) {
      onToast(`Started ${started} restore(s); ${failed} failed to start`, true);
    } else {
      onToast(`Started ${started} restore job(s) to ${pickedDir}`);
    }
  };

  const handleRestoreStarted = (run: JobRun) => {
    onToast(
      run.dry_run
        ? `Started dry-run restore (${run.job_name})`
        : `Restore started (${run.job_name})`
    );
  };

  const handleCalculateSize = async () => {
    if (!remote) return;
    setLoadingSize(true);
    try {
      const sz = await api.browseSize(remote, subpath);
      setDirSize(sz);
    } catch (err) {
      onToast(err instanceof Error ? err.message : 'Failed to calculate folder size', true);
    } finally {
      setLoadingSize(false);
    }
  };

  const handleUploadFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const input = e.currentTarget;
    const file = input.files?.[0];
    if (!file || !remote) return;
    setUploading(true);
    try {
      const res = await api.browseUpload(remote, subpath, file);
      onToast(`Uploaded "${res.name}"`);
      void loadDir(remote, subpath);
    } catch (err) {
      onToast(err instanceof Error ? err.message : 'Upload failed', true);
    } finally {
      setUploading(false);
      input.value = '';
    }
  };

  const handleCreateFolder = async (e: React.FormEvent) => {
    e.preventDefault();
    const clean = newFolderName.trim().replace(/^\/+|\/+$/g, '');
    if (!clean || !remote) return;
    setCreatingDir(true);
    try {
      const target = subpath ? `${subpath}/${clean}` : clean;
      await api.browseMkdir(remote, target);
      onToast(`Created folder "${clean}"`);
      setShowMkdir(false);
      setNewFolderName('');
      void loadDir(remote, subpath);
    } catch (err) {
      onToast(err instanceof Error ? err.message : 'Failed to create folder', true);
    } finally {
      setCreatingDir(false);
    }
  };

  const handleRenameSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!renameTarget || !remote) return;
    const newName = renameValue.trim();
    if (!newName) return;
    setRenaming(true);
    try {
      await api.browseRename(remote, renameTarget.fullPath, newName);
      onToast(`Renamed to "${newName}"`);
      setRenameTarget(null);
      void loadDir(remote, subpath);
    } catch (err) {
      onToast(err instanceof Error ? err.message : 'Rename failed', true);
    } finally {
      setRenaming(false);
    }
  };

  const handleDeleteConfirm = async () => {
    if (!deleteTarget || !remote) return;
    setDeleting(true);
    try {
      await api.browseDelete(remote, deleteTarget.fullPath, deleteTarget.entry.IsDir);
      onToast(`Deleted "${deleteTarget.entry.Name}"`);
      setDeleteTarget(null);
      void loadDir(remote, subpath);
    } catch (err) {
      onToast(err instanceof Error ? err.message : 'Delete failed', true);
    } finally {
      setDeleting(false);
    }
  };

  return (
    <div className="space-y-4">
      {activeRestores.length > 0 && (
        <div className="space-y-2.5">
          {activeRestores.map((run) => (
            <ActiveRunCard
              key={run.run_id}
              run={run}
              variant="restore"
              onViewLogs={onViewActiveLogs}
              onStop={(jobId) => {
                void api
                  .stopJob(jobId)
                  .then(() => onToast('Stopping restore...'))
                  .catch((err) =>
                    onToast(err instanceof Error ? err.message : 'Failed to stop restore', true)
                  );
              }}
            />
          ))}
        </div>
      )}

      <div className="flex flex-wrap items-center gap-2">
        <select
          value={remote}
          onChange={(e) => {
            setRemote(e.target.value);
            setSubpath('');
          }}
          className="px-3 py-2 rounded-xl bg-slate-900 border border-slate-800 text-sm text-slate-100 focus:outline-none focus:border-sky-500"
        >
          {remotes.length === 0 && <option value="">No remotes configured</option>}
          {remotes.map((r) => (
            <option key={r.name} value={r.name}>
              {r.name} ({r.type})
            </option>
          ))}
        </select>

        <div className="relative flex-1 min-w-[180px]">
          <Search className="w-4 h-4 text-slate-500 absolute left-3 top-2.5" />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Filter current folder..."
            className="w-full pl-9 pr-3 py-2 rounded-xl bg-slate-900 border border-slate-800 text-sm text-slate-100 focus:outline-none focus:border-sky-500"
          />
        </div>

        <div className="flex flex-wrap items-center gap-1.5">
          <div
            className={`relative overflow-hidden inline-flex items-center gap-1.5 px-3 py-2 rounded-xl text-xs font-medium bg-sky-500/15 hover:bg-sky-500/25 text-sky-300 border border-sky-500/30 ${
              uploading || !remote ? 'opacity-50 pointer-events-none' : 'cursor-pointer'
            }`}
          >
            {uploading ? (
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
            ) : (
              <Upload className="w-3.5 h-3.5" />
            )}
            <span>Upload</span>
            <input
              type="file"
              aria-label="Upload file to current folder"
              disabled={uploading || !remote}
              onChange={(e) => void handleUploadFile(e)}
              className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
            />
          </div>

          <button
            type="button"
            disabled={!remote}
            onClick={() => {
              setNewFolderName('');
              setShowMkdir(true);
            }}
            className="inline-flex items-center gap-1.5 px-3 py-2 rounded-xl text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 disabled:opacity-50"
          >
            <FolderPlus className="w-3.5 h-3.5 text-emerald-400" />
            New Folder
          </button>

          <button
            type="button"
            disabled={loadingSize || !remote}
            onClick={() => void handleCalculateSize()}
            className="inline-flex items-center gap-1.5 px-3 py-2 rounded-xl text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 disabled:opacity-50"
            title="Calculate total folder size via rclone size"
          >
            {loadingSize ? (
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
            ) : (
              <HardDrive className="w-3.5 h-3.5 text-amber-400" />
            )}
            Size
          </button>

          <button
            type="button"
            onClick={() => void loadDir()}
            disabled={loading || !remote}
            className="inline-flex items-center gap-1.5 px-3 py-2 rounded-xl text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 disabled:opacity-50"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </button>
        </div>
      </div>

      {remote && (
        <div className="flex items-center justify-between gap-2 px-3.5 py-2.5 rounded-xl bg-slate-900 border border-slate-800 text-xs">
          <div className="flex items-center gap-1 overflow-x-auto">
            <button
              type="button"
              onClick={() => setSubpath('')}
              className="font-mono font-semibold text-sky-400 hover:underline shrink-0"
            >
              {remote}:
            </button>
            {crumbs.map((c, idx) => {
              const p = crumbs.slice(0, idx + 1).join('/');
              return (
                <span key={p} className="inline-flex items-center gap-1 shrink-0">
                  <ChevronRight className="w-3.5 h-3.5 text-slate-600" />
                  <button
                    type="button"
                    onClick={() => setSubpath(p)}
                    className="font-mono text-slate-200 hover:text-sky-400"
                  >
                    {c}
                  </button>
                </span>
              );
            })}
          </div>

          {dirSize && (
            <div className="flex items-center gap-2 shrink-0 px-2 py-0.5 rounded-md bg-amber-500/10 border border-amber-500/30 text-amber-300 tabular-nums">
              <span>
                {dirSize.count} file(s) · {formatBytes(dirSize.bytes)}
              </span>
              <button
                type="button"
                onClick={() => setDirSize(null)}
                className="text-amber-300/70 hover:text-amber-200"
                aria-label="Dismiss folder size"
              >
                <X className="w-3 h-3" />
              </button>
            </div>
          )}
        </div>
      )}

      {error && (
        <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-xs text-rose-300">
          {error}
        </div>
      )}

      {selectedEntries.length > 0 && (
        <div className="flex flex-wrap items-center justify-between gap-2 px-3.5 py-2.5 rounded-xl bg-sky-500/10 border border-sky-500/30 text-xs">
          <span className="font-medium text-sky-200">{selectedEntries.length} selected</span>
          <div className="flex flex-wrap items-center gap-2">
            <button
              type="button"
              onClick={() => setShowBulkRestorePicker(true)}
              className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg bg-emerald-500/20 hover:bg-emerald-500/30 text-emerald-300 font-medium"
            >
              <RotateCcw className="w-3.5 h-3.5" />
              Restore Selected
            </button>
            <button
              type="button"
              onClick={() => setConfirmBulkDelete(true)}
              className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg bg-rose-500/20 hover:bg-rose-500/30 text-rose-300 font-medium"
            >
              <Trash2 className="w-3.5 h-3.5" />
              Delete Selected
            </button>
            <button
              type="button"
              onClick={() => setSelectedPaths(new Set())}
              className="px-2.5 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300"
            >
              Clear
            </button>
          </div>
        </div>
      )}

      <div className="rounded-2xl bg-slate-900/90 border border-slate-800 divide-y divide-slate-800/70 overflow-hidden">
        {loading ? (
          <div className="flex items-center justify-center py-12 text-sm text-slate-400 gap-2">
            <Loader2 className="w-4 h-4 animate-spin" />
            Listing remote contents...
          </div>
        ) : !remote ? (
          <div className="py-12 text-center space-y-3">
            <p className="text-sm text-slate-400">Configure a remote first to browse files.</p>
            <button
              type="button"
              onClick={() => onNavigate('remotes')}
              className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-xl text-xs font-semibold bg-sky-500 hover:bg-sky-400 text-slate-950"
            >
              Go to Remotes
            </button>
          </div>
        ) : filtered.length === 0 ? (
          <>
            {subpath !== '' && !query.trim() && (
              <button
                type="button"
                onClick={() => setSubpath(crumbs.slice(0, -1).join('/'))}
                className="w-full px-4 py-2.5 flex items-center gap-2.5 text-left text-xs text-slate-400 hover:text-slate-200 hover:bg-slate-800/40 transition-colors"
              >
                <CornerLeftUp className="w-3.5 h-3.5" />
                <span className="font-mono">.. (Parent folder)</span>
              </button>
            )}
            <div className="py-12 text-center text-sm text-slate-500">This directory is empty.</div>
          </>
        ) : (
          <>
            <div className="px-4 py-2 bg-slate-950/60 flex items-center justify-between text-xs text-slate-400">
              <label className="inline-flex items-center gap-2.5 cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={allFilteredSelected}
                  onChange={toggleSelectAll}
                  className="w-4 h-4 rounded accent-sky-500"
                />
                <span>Select all ({filtered.length})</span>
              </label>
            </div>
            {subpath !== '' && !query.trim() && (
              <button
                type="button"
                onClick={() => setSubpath(crumbs.slice(0, -1).join('/'))}
                className="w-full px-4 py-2.5 flex items-center gap-2.5 text-left text-xs text-slate-400 hover:text-slate-200 hover:bg-slate-800/40 transition-colors"
              >
                <CornerLeftUp className="w-3.5 h-3.5" />
                <span className="font-mono">.. (Parent folder)</span>
              </button>
            )}
            {filtered.map((entry) => {
              const fPath = fullEntryPath(entry.Name);
              const isVersionDir = entry.IsDir && entry.Name === '_versions';
              const previewable = isPreviewableFile(entry);
              const isSelected = selectedPaths.has(fPath);
              return (
                <div
                  key={entry.Path || entry.Name}
                  className={`px-4 py-3 flex items-center justify-between gap-3 hover:bg-slate-800/40 transition-colors ${
                    isSelected ? 'bg-sky-500/5' : ''
                  }`}
                >
                  <div className="flex items-center gap-3 min-w-0 flex-1">
                    <input
                      type="checkbox"
                      aria-label={`Select ${entry.Name}`}
                      checked={isSelected}
                      onChange={() => toggleSelectOne(fPath)}
                      className="w-4 h-4 rounded accent-sky-500 shrink-0"
                    />
                    {entry.IsDir ? (
                      <Folder
                        className={`w-4 h-4 shrink-0 ${
                          isVersionDir ? 'text-emerald-400' : 'text-sky-400'
                        }`}
                      />
                    ) : (
                      <File className="w-4 h-4 text-slate-400 shrink-0" />
                    )}
                    <div className="min-w-0">
                      {entry.IsDir ? (
                        <button
                          type="button"
                          onClick={() => setSubpath(fPath)}
                          className="text-sm font-medium text-slate-100 hover:text-sky-400 truncate flex items-center gap-2 text-left"
                        >
                          <span className="truncate">{entry.Name}</span>
                          {isVersionDir && (
                            <span className="inline-flex items-center gap-1 px-1.5 py-0.5 rounded text-[10px] bg-emerald-500/15 text-emerald-300 border border-emerald-500/30">
                              <History className="w-2.5 h-2.5" />
                              Recovery Versions
                            </span>
                          )}
                        </button>
                      ) : previewable ? (
                        <button
                          type="button"
                          onClick={() => setPreviewTarget({ entry, fullPath: fPath })}
                          className="text-sm text-slate-100 hover:text-sky-400 truncate text-left block max-w-full"
                          title="Preview file"
                        >
                          {entry.Name}
                        </button>
                      ) : (
                        <div className="text-sm text-slate-100 truncate">{entry.Name}</div>
                      )}
                      <div className="text-[11px] text-slate-500 tabular-nums">
                        {entry.IsDir ? 'Folder' : formatBytes(entry.Size)}
                        {entry.ModTime && ` · ${formatDateTime(entry.ModTime)}`}
                      </div>
                    </div>
                  </div>

                  <div className="flex items-center gap-1.5 shrink-0">
                    {previewable && (
                      <button
                        type="button"
                        onClick={() => setPreviewTarget({ entry, fullPath: fPath })}
                        className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-sky-300"
                        title="Preview file"
                      >
                        <Eye className="w-3.5 h-3.5" />
                      </button>
                    )}
                    {!isVersionDir && !insideVersionsDir && (
                      <button
                        type="button"
                        onClick={() => setVersionTarget({ entry, fullPath: fPath })}
                        className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-amber-300"
                        title="View archived versions"
                      >
                        <History className="w-3.5 h-3.5" />
                        <span className="hidden sm:inline">Versions</span>
                      </button>
                    )}
                    {!entry.IsDir && (
                      <a
                        href={api.downloadUrl(remote, fPath)}
                        className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200"
                        title="Download file"
                      >
                        <Download className="w-3.5 h-3.5 text-sky-400" />
                        <span className="hidden sm:inline">Download</span>
                      </a>
                    )}
                    <button
                      type="button"
                      onClick={() => setRestoreTarget({ entry, fullPath: fPath })}
                      className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-emerald-300"
                      title="Restore to path"
                    >
                      <RotateCcw className="w-3.5 h-3.5" />
                      <span className="hidden sm:inline">Restore</span>
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        setRenameTarget({ entry, fullPath: fPath });
                        setRenameValue(entry.Name);
                      }}
                      className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300"
                      title="Rename"
                    >
                      <Pencil className="w-3.5 h-3.5" />
                    </button>
                    <button
                      type="button"
                      onClick={() => setDeleteTarget({ entry, fullPath: fPath })}
                      className="p-1.5 rounded-lg bg-slate-800 hover:bg-rose-500/20 text-slate-400 hover:text-rose-300"
                      title="Delete"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                </div>
              );
            })}
          </>
        )}
      </div>

      {showMkdir && (
        <Modal
          title="New Folder"
          subtitle={`Inside ${remote}:${subpath}`}
          onClose={() => !creatingDir && setShowMkdir(false)}
          maxWidth="max-w-md"
          footer={
            <>
              <button
                type="button"
                disabled={creatingDir}
                onClick={() => setShowMkdir(false)}
                className="px-4 py-2 rounded-lg text-sm font-medium text-slate-300 hover:bg-slate-800 disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                type="submit"
                form="mkdir-form"
                disabled={creatingDir || !newFolderName.trim()}
                className="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-semibold bg-sky-500 hover:bg-sky-400 text-slate-950 disabled:opacity-50"
              >
                {creatingDir && <Loader2 className="w-4 h-4 animate-spin" />}
                Create
              </button>
            </>
          }
        >
          <form id="mkdir-form" onSubmit={handleCreateFolder}>
            <label className="block text-xs font-medium text-slate-300 mb-1">Folder Name</label>
            <input
              required
              value={newFolderName}
              onChange={(e) => setNewFolderName(e.target.value)}
              placeholder="e.g. backups-2026"
              className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100 focus:outline-none focus:border-sky-500"
            />
          </form>
        </Modal>
      )}

      {renameTarget && (
        <Modal
          title={`Rename ${renameTarget.entry.IsDir ? 'Folder' : 'File'}`}
          subtitle={`${remote}:${renameTarget.fullPath}`}
          onClose={() => !renaming && setRenameTarget(null)}
          maxWidth="max-w-md"
          footer={
            <>
              <button
                type="button"
                disabled={renaming}
                onClick={() => setRenameTarget(null)}
                className="px-4 py-2 rounded-lg text-sm font-medium text-slate-300 hover:bg-slate-800 disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                type="submit"
                form="rename-form"
                disabled={
                  renaming || !renameValue.trim() || renameValue.trim() === renameTarget.entry.Name
                }
                className="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-semibold bg-sky-500 hover:bg-sky-400 text-slate-950 disabled:opacity-50"
              >
                {renaming && <Loader2 className="w-4 h-4 animate-spin" />}
                Rename
              </button>
            </>
          }
        >
          <form id="rename-form" onSubmit={handleRenameSubmit}>
            <label className="block text-xs font-medium text-slate-300 mb-1">New Name</label>
            <input
              required
              value={renameValue}
              onChange={(e) => setRenameValue(e.target.value)}
              className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100 focus:outline-none focus:border-sky-500"
            />
          </form>
        </Modal>
      )}

      {deleteTarget && (
        <ConfirmModal
          title={deleteTarget.entry.IsDir ? 'Delete Folder Recursively' : 'Delete File'}
          message={
            deleteTarget.entry.IsDir
              ? `Permanently delete folder "${deleteTarget.entry.Name}" and ALL files inside it from "${remote}"?`
              : `Permanently delete file "${deleteTarget.entry.Name}" from "${remote}"?`
          }
          busy={deleting}
          onCancel={() => setDeleteTarget(null)}
          onConfirm={() => void handleDeleteConfirm()}
        />
      )}

      {restoreTarget && (
        <RestoreModal
          remote={remote}
          entry={restoreTarget.entry}
          fullPath={restoreTarget.fullPath}
          remotes={remotes}
          jobs={jobs}
          onStarted={handleRestoreStarted}
          onClose={() => setRestoreTarget(null)}
        />
      )}

      {versionTarget && (
        <VersionHistoryModal
          remote={remote}
          entry={versionTarget.entry}
          fullPath={versionTarget.fullPath}
          onRestoreVersion={(vEntry, vPath) => setRestoreTarget({ entry: vEntry, fullPath: vPath })}
          onClose={() => setVersionTarget(null)}
        />
      )}

      {previewTarget && (
        <FilePreviewModal
          remote={remote}
          entry={previewTarget.entry}
          fullPath={previewTarget.fullPath}
          onClose={() => setPreviewTarget(null)}
        />
      )}

      {confirmBulkDelete && (
        <ConfirmModal
          title="Delete Selected Items"
          message={`Permanently delete ${selectedEntries.length} selected item(s) from "${remote}"? Selected folders will be deleted recursively.`}
          busy={bulkDeleting}
          onCancel={() => setConfirmBulkDelete(false)}
          onConfirm={() => void handleBulkDeleteConfirm()}
        />
      )}

      {showBulkRestorePicker && (
        <PathPickerModal
          remotes={remotes}
          onSelect={(pickedDir) => void handleBulkRestoreTarget(pickedDir)}
          onClose={() => setShowBulkRestorePicker(false)}
        />
      )}
    </div>
  );
}
