import { Sparkles } from 'lucide-react';
import type { GlobalSettings } from '../../types/api';

export function JobOverridesTab({
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
}: {
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
}) {
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2 text-xs text-slate-400">
        <Sparkles className="w-4 h-4 text-sky-400" />
        Leave fields blank to inherit Global Settings defaults.
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
          placeholder="--fast-list --tpslimit 10 --drive-chunk-size 64M"
          className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100 focus:outline-none focus:border-sky-500"
        />
      </div>
    </div>
  );
}
