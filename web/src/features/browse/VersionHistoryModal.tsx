import { Calendar, Download, History, Loader2, RotateCcw } from 'lucide-react';
import { useEffect, useState } from 'react';
import { api } from '../../api/client';
import { Modal } from '../../components/ui/Modal';
import type { BrowseEntry, FileVersion } from '../../types/api';
import { formatBytes, formatDateTime } from '../../utils/format';

export function VersionHistoryModal({
  remote,
  entry,
  fullPath,
  onRestoreVersion,
  onClose,
}: {
  remote: string;
  entry: BrowseEntry;
  fullPath: string;
  onRestoreVersion: (versionEntry: BrowseEntry, versionFullPath: string) => void;
  onClose: () => void;
}) {
  const [versions, setVersions] = useState<FileVersion[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError('');
    api
      .listVersions(remote, fullPath)
      .then((res) => {
        if (!cancelled) setVersions(res.versions ?? []);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Failed to load versions');
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [remote, fullPath]);

  return (
    <Modal
      title={`Version Timeline: ${entry.Name}`}
      subtitle={`Archived snapshots for ${remote}:${fullPath}`}
      onClose={onClose}
      maxWidth="max-w-2xl"
      footer={
        <button
          type="button"
          onClick={onClose}
          className="px-4 py-2 rounded-lg text-sm font-medium bg-slate-800 hover:bg-slate-700 text-slate-200"
        >
          Close
        </button>
      }
    >
      {loading ? (
        <div className="flex items-center justify-center py-12 text-sm text-slate-400 gap-2">
          <Loader2 className="w-4 h-4 animate-spin" />
          Scanning backup version folders...
        </div>
      ) : error ? (
        <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-xs text-rose-300">
          {error}
        </div>
      ) : versions.length === 0 ? (
        <div className="py-10 text-center space-y-2">
          <History className="w-8 h-8 text-slate-500 mx-auto" />
          <p className="text-sm text-slate-300">No archived versions found for this item.</p>
          <p className="text-xs text-slate-500 max-w-md mx-auto">
            Previous versions appear here when a job enables <code>--backup-dir</code> versioning
            and this item is later modified or deleted.
          </p>
        </div>
      ) : (
        <div className="space-y-2.5">
          {versions.map((v) => (
            <div
              key={v.source_path}
              className="p-3.5 rounded-xl bg-slate-950 border border-slate-800 flex flex-wrap items-center justify-between gap-3"
            >
              <div className="min-w-0 space-y-1">
                <div className="flex items-center gap-2">
                  <span className="inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md text-xs font-mono font-semibold bg-emerald-500/15 text-emerald-300 border border-emerald-500/30">
                    <Calendar className="w-3 h-3" />
                    {v.date}
                  </span>
                  <span className="text-xs text-slate-400 tabular-nums">
                    {v.is_dir ? 'Folder' : formatBytes(v.size)}
                  </span>
                  {v.mod_time && (
                    <span className="text-xs text-slate-500">{formatDateTime(v.mod_time)}</span>
                  )}
                </div>
                <div className="text-[11px] font-mono text-slate-400 truncate">
                  {remote}:{v.source_path}
                </div>
              </div>

              <div className="flex items-center gap-1.5 shrink-0">
                {!v.is_dir && (
                  <a
                    href={api.downloadUrl(remote, v.source_path)}
                    className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200"
                  >
                    <Download className="w-3.5 h-3.5 text-sky-400" />
                    Download
                  </a>
                )}
                <button
                  type="button"
                  onClick={() => {
                    onClose();
                    onRestoreVersion(
                      {
                        Path: v.source_path,
                        Name: entry.Name,
                        Size: v.size,
                        ModTime: v.mod_time,
                        IsDir: v.is_dir,
                      },
                      v.source_path
                    );
                  }}
                  className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg text-xs font-semibold bg-emerald-500/15 hover:bg-emerald-500/25 text-emerald-300 border border-emerald-500/30"
                >
                  <RotateCcw className="w-3.5 h-3.5" />
                  Restore
                </button>
              </div>
            </div>
          ))}
        </div>
      )}
    </Modal>
  );
}
