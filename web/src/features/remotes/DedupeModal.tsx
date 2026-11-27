import { FlaskConical, Layers, Loader2 } from 'lucide-react';
import { useState } from 'react';
import { api } from '../../api/client';
import { Modal } from '../../components/ui/Modal';
import type { DedupeResult } from '../../types/api';

export function DedupeModal({ remoteName, onClose }: { remoteName: string; onClose: () => void }) {
  const [subpath, setSubpath] = useState('');
  const [mode, setMode] = useState<
    'newest' | 'oldest' | 'largest' | 'smallest' | 'rename' | 'skip'
  >('newest');
  const [running, setRunning] = useState<'dry' | 'apply' | null>(null);
  const [error, setError] = useState('');
  const [result, setResult] = useState<DedupeResult | null>(null);

  const execute = async (dryRun: boolean) => {
    setRunning(dryRun ? 'dry' : 'apply');
    setError('');
    try {
      const res = await api.dedupeRemote(remoteName, {
        path: subpath.trim(),
        mode,
        dry_run: dryRun,
      });
      setResult(res);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Dedupe failed');
    } finally {
      setRunning(null);
    }
  };

  return (
    <Modal
      title={`Deduplicate Remote: ${remoteName}`}
      subtitle="Find and resolve duplicate identically-named files (useful on Google Drive)"
      onClose={() => running === null && onClose()}
      maxWidth="max-w-xl"
      footer={
        <>
          <button
            type="button"
            disabled={running !== null}
            onClick={onClose}
            className="px-4 py-2 rounded-lg text-sm font-medium text-slate-300 hover:bg-slate-800 disabled:opacity-50"
          >
            Close
          </button>
          <button
            type="button"
            disabled={running !== null}
            onClick={() => void execute(true)}
            className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-lg text-sm font-medium bg-slate-800 hover:bg-slate-700 text-violet-300 disabled:opacity-50"
          >
            {running === 'dry' ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <FlaskConical className="w-4 h-4" />
            )}
            Dry Run
          </button>
          <button
            type="button"
            disabled={running !== null}
            onClick={() => void execute(false)}
            className="inline-flex items-center gap-1.5 px-4 py-2 rounded-lg text-sm font-semibold bg-amber-500 hover:bg-amber-400 text-slate-950 disabled:opacity-50"
          >
            {running === 'apply' ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <Layers className="w-4 h-4" />
            )}
            Run Dedupe
          </button>
        </>
      }
    >
      <div className="space-y-4">
        {error && (
          <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-xs text-rose-300">
            {error}
          </div>
        )}

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Subpath <span className="text-slate-500 font-normal">(blank = entire remote)</span>
            </label>
            <input
              value={subpath}
              onChange={(e) => setSubpath(e.target.value)}
              placeholder="e.g. Photos/2026"
              className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-xs font-mono text-slate-100 focus:outline-none focus:border-sky-500"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Dedupe Strategy (<code>--dedupe-mode</code>)
            </label>
            <select
              value={mode}
              onChange={(e) =>
                setMode(
                  e.target.value as 'newest' | 'oldest' | 'largest' | 'smallest' | 'rename' | 'skip'
                )
              }
              className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-xs text-slate-100 focus:outline-none focus:border-sky-500"
            >
              <option value="newest">newest — keep newest, delete older copies</option>
              <option value="oldest">oldest — keep oldest, delete newer copies</option>
              <option value="largest">largest — keep largest copy</option>
              <option value="smallest">smallest — keep smallest copy</option>
              <option value="rename">rename — append numeric suffix (safe)</option>
              <option value="skip">skip — report only, leave files untouched</option>
            </select>
          </div>
        </div>

        <p className="text-[11px] text-slate-400">
          Identical duplicates with matching hashes are automatically collapsed where supported.
          Always test with <strong>Dry Run</strong> before deleting or renaming files.
        </p>

        {result && (
          <div className="p-3 rounded-xl bg-slate-950 border border-slate-800 space-y-2">
            <div className="flex items-center justify-between text-xs">
              <span className="font-semibold text-slate-200">
                Output ({result.mode} · {result.dry_run ? 'Dry Run' : 'Applied'})
              </span>
            </div>
            <pre className="max-h-60 overflow-auto text-[11px] font-mono text-slate-300 whitespace-pre-wrap break-words">
              {result.output || 'Completed with no duplicate objects reported.'}
            </pre>
          </div>
        )}
      </div>
    </Modal>
  );
}
