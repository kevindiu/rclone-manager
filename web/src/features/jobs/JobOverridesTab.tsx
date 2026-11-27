import { Sparkles } from 'lucide-react';
import type { GlobalSettings, Job } from '../../types/api';

export function JobOverridesTab({
  operation,
  settings,
  transfers,
  setTransfers,
  checkers,
  setCheckers,
  retries,
  setRetries,
  lowLevelRetries,
  setLowLevelRetries,
  timeout,
  setTimeoutVal,
  contimeout,
  setContimeout,
  bwlimit,
  setBwlimit,
  customFlags,
  setCustomFlags,
  fastList,
  setFastList,
  checksum,
  setChecksum,
  updateOnly,
  setUpdateOnly,
  serverSideAcrossConfigs,
  setServerSideAcrossConfigs,
  deleteTiming,
  setDeleteTiming,
  deleteEmptySrcDirs,
  setDeleteEmptySrcDirs,
  symlinkMode,
  setSymlinkMode,
  sizeOnly,
  setSizeOnly,
  ignoreExisting,
  setIgnoreExisting,
  notifyOverride,
  setNotifyOverride,
}: {
  operation: Job['operation'];
  settings: GlobalSettings;
  transfers: string;
  setTransfers: (v: string) => void;
  checkers: string;
  setCheckers: (v: string) => void;
  retries: string;
  setRetries: (v: string) => void;
  lowLevelRetries: string;
  setLowLevelRetries: (v: string) => void;
  timeout: string;
  setTimeoutVal: (v: string) => void;
  contimeout: string;
  setContimeout: (v: string) => void;
  bwlimit: string;
  setBwlimit: (v: string) => void;
  customFlags: string;
  setCustomFlags: (v: string) => void;
  fastList: boolean;
  setFastList: (v: boolean) => void;
  checksum: boolean;
  setChecksum: (v: boolean) => void;
  updateOnly: boolean;
  setUpdateOnly: (v: boolean) => void;
  serverSideAcrossConfigs: boolean;
  setServerSideAcrossConfigs: (v: boolean) => void;
  deleteTiming: NonNullable<Job['delete_timing']>;
  setDeleteTiming: (v: NonNullable<Job['delete_timing']>) => void;
  deleteEmptySrcDirs: boolean;
  setDeleteEmptySrcDirs: (v: boolean) => void;
  symlinkMode: NonNullable<Job['symlink_mode']>;
  setSymlinkMode: (v: NonNullable<Job['symlink_mode']>) => void;
  sizeOnly: boolean;
  setSizeOnly: (v: boolean) => void;
  ignoreExisting: boolean;
  setIgnoreExisting: (v: boolean) => void;
  notifyOverride: NonNullable<Job['notify_override']>;
  setNotifyOverride: (v: NonNullable<Job['notify_override']>) => void;
}) {
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2 text-xs text-slate-400">
        <Sparkles className="w-4 h-4 text-sky-400" />
        Leave numeric/text fields blank to inherit Global Settings defaults.
      </div>

      <div className="p-3.5 rounded-xl bg-slate-950 border border-slate-800 space-y-2.5">
        <div className="text-xs font-semibold uppercase tracking-wider text-sky-400">
          Common rclone Options
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
          <label className="flex items-start gap-2.5 p-2.5 rounded-lg bg-slate-900/70 border border-slate-800 cursor-pointer">
            <input
              type="checkbox"
              checked={fastList}
              onChange={(e) => setFastList(e.target.checked)}
              className="mt-0.5 w-4 h-4 rounded accent-sky-500"
            />
            <div>
              <div className="text-xs font-mono font-medium text-slate-100">--fast-list</div>
              <p className="text-[11px] text-slate-400 mt-0.5">
                Fewer API calls &amp; faster listing on S3, R2, B2, and Google Drive.
              </p>
            </div>
          </label>

          <label className="flex items-start gap-2.5 p-2.5 rounded-lg bg-slate-900/70 border border-slate-800 cursor-pointer">
            <input
              type="checkbox"
              checked={checksum}
              onChange={(e) => setChecksum(e.target.checked)}
              className="mt-0.5 w-4 h-4 rounded accent-sky-500"
            />
            <div>
              <div className="text-xs font-mono font-medium text-slate-100">--checksum</div>
              <p className="text-[11px] text-slate-400 mt-0.5">
                Compare file hashes &amp; sizes instead of modification timestamps.
              </p>
            </div>
          </label>

          <label className="flex items-start gap-2.5 p-2.5 rounded-lg bg-slate-900/70 border border-slate-800 cursor-pointer">
            <input
              type="checkbox"
              checked={updateOnly}
              onChange={(e) => setUpdateOnly(e.target.checked)}
              className="mt-0.5 w-4 h-4 rounded accent-sky-500"
            />
            <div>
              <div className="text-xs font-mono font-medium text-slate-100">--update</div>
              <p className="text-[11px] text-slate-400 mt-0.5">
                Skip destination files that are newer than the source file.
              </p>
            </div>
          </label>

          <label className="flex items-start gap-2.5 p-2.5 rounded-lg bg-slate-900/70 border border-slate-800 cursor-pointer">
            <input
              type="checkbox"
              checked={serverSideAcrossConfigs}
              onChange={(e) => setServerSideAcrossConfigs(e.target.checked)}
              className="mt-0.5 w-4 h-4 rounded accent-sky-500"
            />
            <div>
              <div className="text-xs font-mono font-medium text-slate-100">
                --server-side-across-configs
              </div>
              <p className="text-[11px] text-slate-400 mt-0.5">
                Allow cloud-to-cloud server-side copy/move without downloading locally.
              </p>
            </div>
          </label>

          <label className="flex items-start gap-2.5 p-2.5 rounded-lg bg-slate-900/70 border border-slate-800 cursor-pointer">
            <input
              type="checkbox"
              checked={sizeOnly}
              onChange={(e) => setSizeOnly(e.target.checked)}
              className="mt-0.5 w-4 h-4 rounded accent-sky-500"
            />
            <div>
              <div className="text-xs font-mono font-medium text-slate-100">--size-only</div>
              <p className="text-[11px] text-slate-400 mt-0.5">
                Skip files matched by size alone, ignoring modification timestamps.
              </p>
            </div>
          </label>

          {operation !== 'check' && operation !== 'bisync' && (
            <label className="flex items-start gap-2.5 p-2.5 rounded-lg bg-slate-900/70 border border-slate-800 cursor-pointer">
              <input
                type="checkbox"
                checked={ignoreExisting}
                onChange={(e) => setIgnoreExisting(e.target.checked)}
                className="mt-0.5 w-4 h-4 rounded accent-sky-500"
              />
              <div>
                <div className="text-xs font-mono font-medium text-slate-100">
                  --ignore-existing
                </div>
                <p className="text-[11px] text-slate-400 mt-0.5">
                  Skip all files that already exist on Destination.
                </p>
              </div>
            </label>
          )}

          {operation === 'move' && (
            <label className="flex items-start gap-2.5 p-2.5 rounded-lg bg-slate-900/70 border border-slate-800 cursor-pointer">
              <input
                type="checkbox"
                checked={deleteEmptySrcDirs}
                onChange={(e) => setDeleteEmptySrcDirs(e.target.checked)}
                className="mt-0.5 w-4 h-4 rounded accent-rose-500"
              />
              <div>
                <div className="text-xs font-mono font-medium text-slate-100">
                  --delete-empty-src-dirs
                </div>
                <p className="text-[11px] text-slate-400 mt-0.5">
                  Remove empty directories left behind on Source after moving files.
                </p>
              </div>
            </label>
          )}
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 pt-1">
          {operation === 'sync' && (
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1">
                Sync Delete Timing
              </label>
              <select
                value={deleteTiming}
                onChange={(e) =>
                  setDeleteTiming(e.target.value as NonNullable<Job['delete_timing']>)
                }
                className="w-full px-3 py-2 rounded-lg bg-slate-900 border border-slate-800 text-xs text-slate-100 focus:outline-none focus:border-sky-500"
              >
                <option value="">Default (during)</option>
                <option value="during">--delete-during (while copying)</option>
                <option value="after">--delete-after (safest on flaky net)</option>
                <option value="before">--delete-before (free dest space first)</option>
              </select>
            </div>
          )}

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Symlink Handling
            </label>
            <select
              value={symlinkMode}
              onChange={(e) => setSymlinkMode(e.target.value as NonNullable<Job['symlink_mode']>)}
              className="w-full px-3 py-2 rounded-lg bg-slate-900 border border-slate-800 text-xs text-slate-100 focus:outline-none focus:border-sky-500"
            >
              <option value="">Default (skip symlinks)</option>
              <option value="links">Preserve as .rclonelink (--links)</option>
              <option value="copy_links">Follow &amp; copy targets (--copy-links)</option>
            </select>
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Notification Override
            </label>
            <select
              value={notifyOverride}
              onChange={(e) =>
                setNotifyOverride(e.target.value as NonNullable<Job['notify_override']>)
              }
              className="w-full px-3 py-2 rounded-lg bg-slate-900 border border-slate-800 text-xs text-slate-100 focus:outline-none focus:border-sky-500"
            >
              <option value="">Use global setting ({settings.webhook_on || 'failure'})</option>
              <option value="always">Always notify</option>
              <option value="failure">On failure only</option>
              <option value="off">Never notify (muted)</option>
            </select>
          </div>
        </div>
      </div>

      <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
        <div>
          <label className="block text-xs font-medium text-slate-300 mb-1">Transfers</label>
          <input
            type="number"
            min={1}
            value={transfers}
            onChange={(e) => setTransfers(e.target.value)}
            placeholder={`Global (${settings.transfers})`}
            className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm text-slate-100 focus:outline-none focus:border-sky-500"
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-slate-300 mb-1">Checkers</label>
          <input
            type="number"
            min={1}
            value={checkers}
            onChange={(e) => setCheckers(e.target.value)}
            placeholder={`Global (${settings.checkers})`}
            className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm text-slate-100 focus:outline-none focus:border-sky-500"
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-slate-300 mb-1">Retries</label>
          <input
            type="number"
            min={0}
            value={retries}
            onChange={(e) => setRetries(e.target.value)}
            placeholder={`Global (${settings.retries})`}
            className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm text-slate-100 focus:outline-none focus:border-sky-500"
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-slate-300 mb-1">Low-Level Retries</label>
          <input
            type="number"
            min={0}
            value={lowLevelRetries}
            onChange={(e) => setLowLevelRetries(e.target.value)}
            placeholder={`Global (${settings.low_level_retries})`}
            className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm text-slate-100 focus:outline-none focus:border-sky-500"
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-slate-300 mb-1">Timeout</label>
          <input
            value={timeout}
            onChange={(e) => setTimeoutVal(e.target.value)}
            placeholder={`Global (${settings.timeout})`}
            className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100 focus:outline-none focus:border-sky-500"
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-slate-300 mb-1">Connect Timeout</label>
          <input
            value={contimeout}
            onChange={(e) => setContimeout(e.target.value)}
            placeholder={`Global (${settings.contimeout})`}
            className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100 focus:outline-none focus:border-sky-500"
          />
        </div>
      </div>

      <div>
        <label className="block text-xs font-medium text-slate-300 mb-1">
          Bandwidth Limit Override (<code>--bwlimit</code>)
        </label>
        <input
          value={bwlimit}
          onChange={(e) => setBwlimit(e.target.value)}
          placeholder={
            settings.bwlimit ? `Global (${settings.bwlimit})` : 'e.g. 10M or 08:00,5M 23:00,off'
          }
          className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100 focus:outline-none focus:border-sky-500"
        />
      </div>

      <div>
        <label className="block text-xs font-medium text-slate-300 mb-1">
          Custom rclone Flags{' '}
          <span className="text-slate-500 font-normal">(safely tokenized, no shell execution)</span>
        </label>
        <input
          value={customFlags}
          onChange={(e) => setCustomFlags(e.target.value)}
          placeholder="--tpslimit 10 --drive-chunk-size 64M"
          className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100 focus:outline-none focus:border-sky-500"
        />
      </div>
    </div>
  );
}
