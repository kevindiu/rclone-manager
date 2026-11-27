import {
  Activity,
  CheckCircle2,
  Edit3,
  Eraser,
  FolderOpen,
  HardDrive,
  Layers,
  Loader2,
  Lock,
  Plus,
  Trash2,
  Upload,
  XCircle,
} from 'lucide-react';
import { useState } from 'react';
import { api } from '../../api/client';
import { ConfirmModal } from '../../components/ui/ConfirmModal';
import type { ProviderInfo, RemoteAbout, RemoteInfo, RemoteTestResult } from '../../types/api';
import { formatBytes } from '../../utils/format';
import { CryptWizardModal } from './CryptWizardModal';
import { DedupeModal } from './DedupeModal';
import { ImportConfModal } from './ImportConfModal';
import { RemoteModal } from './RemoteModal';

export function RemotesPage({
  remotes,
  providers,
  onReload,
  onBrowseRemote,
  onToast,
}: {
  remotes: RemoteInfo[];
  providers: ProviderInfo[];
  onReload: () => void;
  onBrowseRemote: (name: string) => void;
  onToast: (msg: string, isError?: boolean) => void;
}) {
  const [modalRemote, setModalRemote] = useState<RemoteInfo | null | undefined>(undefined);
  const [showImport, setShowImport] = useState(false);
  const [testing, setTesting] = useState<Record<string, boolean>>({});
  const [testResults, setTestResults] = useState<Record<string, RemoteTestResult>>({});
  const [aboutResults, setAboutResults] = useState<Record<string, RemoteAbout>>({});
  const [loadingAbout, setLoadingAbout] = useState<Record<string, boolean>>({});
  const [confirmDeleteRemote, setConfirmDeleteRemote] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [confirmCleanupRemote, setConfirmCleanupRemote] = useState<string | null>(null);
  const [cleaning, setCleaning] = useState(false);
  const [showCryptWizard, setShowCryptWizard] = useState(false);
  const [dedupeRemote, setDedupeRemote] = useState<string | null>(null);

  const runTest = async (name: string) => {
    setTesting((p) => ({ ...p, [name]: true }));
    try {
      const res = await api.testRemote(name);
      setTestResults((p) => ({ ...p, [name]: res }));
    } catch (err) {
      setTestResults((p) => ({
        ...p,
        [name]: {
          remote: name,
          ok: false,
          latency_ms: 0,
          error: err instanceof Error ? err.message : 'Test failed',
        },
      }));
    } finally {
      setTesting((p) => ({ ...p, [name]: false }));
    }
  };

  const runAbout = async (name: string) => {
    setLoadingAbout((p) => ({ ...p, [name]: true }));
    try {
      const res = await api.aboutRemote(name);
      setAboutResults((p) => ({ ...p, [name]: res }));
    } catch {
      // ignore
    } finally {
      setLoadingAbout((p) => ({ ...p, [name]: false }));
    }
  };

  const handleDelete = async (name: string) => {
    setDeleting(true);
    try {
      await api.deleteRemote(name);
      onToast(`Deleted remote "${name}"`);
      setConfirmDeleteRemote(null);
      onReload();
    } catch (err) {
      onToast(err instanceof Error ? err.message : 'Delete failed', true);
    } finally {
      setDeleting(false);
    }
  };

  const handleCleanup = async (name: string) => {
    setCleaning(true);
    try {
      await api.cleanupRemote(name);
      onToast(`Emptied trash on "${name}"`);
      setConfirmCleanupRemote(null);
    } catch (err) {
      onToast(
        err instanceof Error ? err.message : 'Cleanup failed or unsupported on this backend',
        true
      );
    } finally {
      setCleaning(false);
    }
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="text-base font-semibold text-slate-100">Configured Remotes</h2>
          <p className="text-xs text-slate-400">
            Secrets, tokens, and crypt passphrases are obscured in <code>rclone.conf</code> and
            masked in UI.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <button
            type="button"
            onClick={() => setShowCryptWizard(true)}
            className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-xl text-xs sm:text-sm font-medium bg-emerald-500/15 hover:bg-emerald-500/25 text-emerald-300 border border-emerald-500/30"
          >
            <Lock className="w-4 h-4" />
            Encrypted Remote
          </button>
          <button
            type="button"
            onClick={() => setShowImport(true)}
            className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-xl text-xs sm:text-sm font-medium bg-slate-800 hover:bg-slate-700 text-slate-200"
          >
            <Upload className="w-4 h-4 text-sky-400" />
            Import rclone.conf
          </button>
          <button
            type="button"
            onClick={() => setModalRemote(null)}
            className="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl text-xs sm:text-sm font-semibold bg-sky-500 hover:bg-sky-400 text-slate-950"
          >
            <Plus className="w-4 h-4" />
            Add Remote
          </button>
        </div>
      </div>

      {remotes.length === 0 ? (
        <div className="p-10 rounded-2xl bg-slate-900/60 border border-slate-800 text-center space-y-2">
          <HardDrive className="w-8 h-8 text-slate-500 mx-auto" />
          <p className="text-sm text-slate-300">No remotes configured yet.</p>
          <p className="text-xs text-slate-500">
            Add any S3, R2, Google Drive, OneDrive, Dropbox, SFTP, WebDAV, or crypt remote.
          </p>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {remotes.map((rem) => {
            const test = testResults[rem.name];
            const about = aboutResults[rem.name];
            const configEntries = Object.entries(rem.config).filter(([k]) => k !== 'type');
            return (
              <div
                key={rem.name}
                className="p-4 rounded-2xl bg-slate-900/90 border border-slate-800 space-y-3 flex flex-col justify-between"
              >
                <div className="space-y-3">
                  <div className="flex items-start justify-between gap-2">
                    <div>
                      <div className="flex items-center gap-2">
                        <h3 className="text-base font-semibold text-slate-100">{rem.name}</h3>
                        <span className="px-2 py-0.5 rounded text-[11px] font-mono bg-sky-500/15 text-sky-300 border border-sky-500/30">
                          {rem.type}
                        </span>
                      </div>
                      {rem.description && (
                        <p className="text-xs text-slate-400 mt-0.5">{rem.description}</p>
                      )}
                    </div>
                    <div className="flex items-center gap-1">
                      <button
                        type="button"
                        onClick={() => onBrowseRemote(rem.name)}
                        className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-sky-400"
                        title="Browse files"
                      >
                        <FolderOpen className="w-4 h-4" />
                      </button>
                      <button
                        type="button"
                        onClick={() => setModalRemote(rem)}
                        className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300"
                        title="Edit remote"
                      >
                        <Edit3 className="w-4 h-4" />
                      </button>
                      <button
                        type="button"
                        onClick={() => setConfirmDeleteRemote(rem.name)}
                        className="p-1.5 rounded-lg bg-slate-800 hover:bg-rose-500/20 text-slate-400 hover:text-rose-300"
                        title="Delete remote"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>

                  {configEntries.length > 0 && (
                    <div className="p-2.5 rounded-xl bg-slate-950 border border-slate-800/90 space-y-1 text-xs font-mono">
                      {configEntries.slice(0, 6).map(([k, v]) => (
                        <div key={k} className="flex items-center justify-between gap-3">
                          <span className="text-slate-400 shrink-0">{k}</span>
                          <span className="text-slate-200 truncate">{v}</span>
                        </div>
                      ))}
                      {configEntries.length > 6 && (
                        <div className="text-[11px] text-slate-500">
                          +{configEntries.length - 6} more option(s)
                        </div>
                      )}
                    </div>
                  )}

                  {test && (
                    <div
                      className={`p-2.5 rounded-xl border text-xs flex items-start gap-2 ${
                        test.ok
                          ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
                          : 'bg-rose-500/10 border-rose-500/30 text-rose-300'
                      }`}
                    >
                      {test.ok ? (
                        <CheckCircle2 className="w-4 h-4 shrink-0 mt-0.5" />
                      ) : (
                        <XCircle className="w-4 h-4 shrink-0 mt-0.5" />
                      )}
                      <div className="min-w-0">
                        {test.ok ? (
                          <span>
                            {test.message} ({test.latency_ms}ms)
                          </span>
                        ) : (
                          <span className="break-words">{test.error}</span>
                        )}
                      </div>
                    </div>
                  )}

                  {about && (
                    <div className="p-2.5 rounded-xl bg-slate-950 border border-slate-800 text-xs">
                      {about.supported ? (
                        <div className="space-y-1 tabular-nums">
                          <div className="flex justify-between text-slate-300">
                            <span>Used: {formatBytes(about.used)}</span>
                            <span>Free: {formatBytes(about.free)}</span>
                            <span>Total: {formatBytes(about.total)}</span>
                          </div>
                          {about.total && about.used !== undefined && (
                            <div className="w-full h-1.5 rounded-full bg-slate-800 overflow-hidden">
                              <div
                                className="h-full bg-emerald-400"
                                style={{
                                  width: `${Math.min(100, (about.used / about.total) * 100)}%`,
                                }}
                              />
                            </div>
                          )}
                        </div>
                      ) : (
                        <span className="text-slate-500">
                          Quota (<code>rclone about</code>) not supported on this backend.
                        </span>
                      )}
                    </div>
                  )}
                </div>

                <div className="flex flex-wrap gap-2 pt-2 border-t border-slate-800/70">
                  <button
                    type="button"
                    disabled={testing[rem.name]}
                    onClick={() => void runTest(rem.name)}
                    className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 disabled:opacity-50"
                  >
                    {testing[rem.name] ? (
                      <Loader2 className="w-3.5 h-3.5 animate-spin" />
                    ) : (
                      <Activity className="w-3.5 h-3.5 text-emerald-400" />
                    )}
                    Test Connectivity
                  </button>
                  <button
                    type="button"
                    disabled={loadingAbout[rem.name]}
                    onClick={() => void runAbout(rem.name)}
                    className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 disabled:opacity-50"
                  >
                    {loadingAbout[rem.name] ? (
                      <Loader2 className="w-3.5 h-3.5 animate-spin" />
                    ) : (
                      <HardDrive className="w-3.5 h-3.5 text-sky-400" />
                    )}
                    Check Quota
                  </button>
                  <button
                    type="button"
                    onClick={() => setConfirmCleanupRemote(rem.name)}
                    className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200"
                    title="Run rclone cleanup (empties trash on supported backends)"
                  >
                    <Eraser className="w-3.5 h-3.5 text-amber-400" />
                    Empty Trash
                  </button>
                  <button
                    type="button"
                    onClick={() => setDedupeRemote(rem.name)}
                    className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200"
                    title="Find and resolve duplicate filenames via rclone dedupe"
                  >
                    <Layers className="w-3.5 h-3.5 text-violet-400" />
                    Dedupe
                  </button>
                </div>
              </div>
            );
          })}
        </div>
      )}

      {modalRemote !== undefined && (
        <RemoteModal
          providers={providers}
          remotes={remotes}
          existing={modalRemote}
          onSaved={() => {
            onToast(modalRemote ? 'Remote updated' : 'Remote created');
            onReload();
          }}
          onClose={() => setModalRemote(undefined)}
        />
      )}

      {showImport && (
        <ImportConfModal
          onImported={() => {
            onToast('Imported rclone.conf');
            onReload();
          }}
          onClose={() => setShowImport(false)}
        />
      )}

      {confirmDeleteRemote && (
        <ConfirmModal
          title="Delete Remote"
          message={`Delete remote "${confirmDeleteRemote}" from rclone.conf? Remotes still referenced by backup jobs cannot be deleted.`}
          busy={deleting}
          onCancel={() => setConfirmDeleteRemote(null)}
          onConfirm={() => void handleDelete(confirmDeleteRemote)}
        />
      )}

      {confirmCleanupRemote && (
        <ConfirmModal
          title="Empty Remote Trash"
          message={`Run "rclone cleanup" on "${confirmCleanupRemote}"? This permanently empties trash or old backend versions where supported (e.g. Google Drive, B2).`}
          confirmLabel="Empty Trash"
          busy={cleaning}
          onCancel={() => setConfirmCleanupRemote(null)}
          onConfirm={() => void handleCleanup(confirmCleanupRemote)}
        />
      )}

      {showCryptWizard && (
        <CryptWizardModal
          remotes={remotes}
          onCreated={(createdName) => {
            onToast(`Created encrypted remote "${createdName}"`);
            onReload();
          }}
          onClose={() => setShowCryptWizard(false)}
        />
      )}

      {dedupeRemote && (
        <DedupeModal remoteName={dedupeRemote} onClose={() => setDedupeRemote(null)} />
      )}
    </div>
  );
}
