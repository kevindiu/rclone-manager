import { ShieldAlert } from 'lucide-react';

export function JobGuardsTab({
  abortIfSourceEmpty,
  setAbortIfSourceEmpty,
  requiredMarkerFile,
  setRequiredMarkerFile,
  maxDelete,
  setMaxDelete,
  maxTransfer,
  setMaxTransfer,
  cutoffMode,
  setCutoffMode,
  minAge,
  setMinAge,
  maxAge,
  setMaxAge,
  minSize,
  setMinSize,
  maxSize,
  setMaxSize,
  excludesText,
  setExcludesText,
  includesText,
  setIncludesText,
  filtersText,
  setFiltersText,
}: {
  abortIfSourceEmpty: boolean;
  setAbortIfSourceEmpty: (v: boolean) => void;
  requiredMarkerFile: string;
  setRequiredMarkerFile: (v: string) => void;
  maxDelete: string;
  setMaxDelete: (v: string) => void;
  maxTransfer: string;
  setMaxTransfer: (v: string) => void;
  cutoffMode: 'soft' | 'hard' | 'cautious' | '';
  setCutoffMode: (v: 'soft' | 'hard' | 'cautious') => void;
  minAge: string;
  setMinAge: (v: string) => void;
  maxAge: string;
  setMaxAge: (v: string) => void;
  minSize: string;
  setMinSize: (v: string) => void;
  maxSize: string;
  setMaxSize: (v: string) => void;
  excludesText: string;
  setExcludesText: (v: string) => void;
  includesText: string;
  setIncludesText: (v: string) => void;
  filtersText: string;
  setFiltersText: (v: string) => void;
}) {
  return (
    <div className="space-y-4">
      <div className="p-3.5 rounded-xl bg-slate-950 border border-slate-800 space-y-3">
        <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-wider text-amber-400">
          <ShieldAlert className="w-4 h-4" />
          Pre-flight &amp; Delete Safety Guards
        </div>

        <label className="flex items-center gap-2.5 cursor-pointer">
          <input
            type="checkbox"
            checked={abortIfSourceEmpty}
            onChange={(e) => setAbortIfSourceEmpty(e.target.checked)}
            className="w-4 h-4 rounded accent-amber-500"
          />
          <span className="text-sm text-slate-200">
            Abort if source is empty (prevents wiping destination on unmounted drives)
          </span>
        </label>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Required Marker File (in source)
            </label>
            <input
              value={requiredMarkerFile}
              onChange={(e) => setRequiredMarkerFile(e.target.value)}
              placeholder="e.g. .backup-mounted"
              className="w-full px-3 py-2 rounded-lg bg-slate-900 border border-slate-800 text-sm font-mono text-slate-100 focus:outline-none focus:border-sky-500"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Max Delete Guard (<code>--max-delete</code>)
            </label>
            <input
              type="number"
              min={0}
              value={maxDelete}
              onChange={(e) => setMaxDelete(e.target.value)}
              placeholder="Unlimited"
              className="w-full px-3 py-2 rounded-lg bg-slate-900 border border-slate-800 text-sm text-slate-100 focus:outline-none focus:border-sky-500"
            />
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
        <div>
          <label className="block text-xs font-medium text-slate-300 mb-1">
            Daily Quota Cap (<code>--max-transfer</code>)
          </label>
          <input
            value={maxTransfer}
            onChange={(e) => setMaxTransfer(e.target.value)}
            placeholder="e.g. 700G (blank = none)"
            className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100 focus:outline-none focus:border-sky-500"
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-slate-300 mb-1">
            Cutoff Mode (<code>--cutoff-mode</code>)
          </label>
          <select
            value={cutoffMode}
            onChange={(e) => setCutoffMode(e.target.value as 'soft' | 'hard' | 'cautious')}
            className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm text-slate-100 focus:outline-none focus:border-sky-500"
          >
            <option value="soft">soft (finish active files cleanly)</option>
            <option value="hard">hard (stop immediately)</option>
            <option value="cautious">cautious (check before starting file)</option>
          </select>
        </div>
      </div>

      <div className="p-3.5 rounded-xl bg-slate-950 border border-slate-800 space-y-2.5">
        <div className="text-xs font-semibold uppercase tracking-wider text-sky-400">
          Age &amp; Size Filters
        </div>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Min Age (<code>--min-age</code>)
            </label>
            <input
              value={minAge}
              onChange={(e) => setMinAge(e.target.value)}
              placeholder="e.g. 24h"
              className="w-full px-3 py-2 rounded-lg bg-slate-900 border border-slate-800 text-xs font-mono text-slate-100 focus:outline-none focus:border-sky-500"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Max Age (<code>--max-age</code>)
            </label>
            <input
              value={maxAge}
              onChange={(e) => setMaxAge(e.target.value)}
              placeholder="e.g. 30d"
              className="w-full px-3 py-2 rounded-lg bg-slate-900 border border-slate-800 text-xs font-mono text-slate-100 focus:outline-none focus:border-sky-500"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Min Size (<code>--min-size</code>)
            </label>
            <input
              value={minSize}
              onChange={(e) => setMinSize(e.target.value)}
              placeholder="e.g. 10M"
              className="w-full px-3 py-2 rounded-lg bg-slate-900 border border-slate-800 text-xs font-mono text-slate-100 focus:outline-none focus:border-sky-500"
            />
          </div>
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Max Size (<code>--max-size</code>)
            </label>
            <input
              value={maxSize}
              onChange={(e) => setMaxSize(e.target.value)}
              placeholder="e.g. 2G"
              className="w-full px-3 py-2 rounded-lg bg-slate-900 border border-slate-800 text-xs font-mono text-slate-100 focus:outline-none focus:border-sky-500"
            />
          </div>
        </div>
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <div>
          <label className="block text-xs font-medium text-slate-300 mb-1">
            Exclude Patterns (one per line)
          </label>
          <textarea
            rows={4}
            value={excludesText}
            onChange={(e) => setExcludesText(e.target.value)}
            placeholder={`.DS_Store\n*.tmp\nnode_modules/**`}
            className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-xs font-mono text-slate-100 focus:outline-none focus:border-sky-500"
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-slate-300 mb-1">
            Include Patterns (one per line)
          </label>
          <textarea
            rows={4}
            value={includesText}
            onChange={(e) => setIncludesText(e.target.value)}
            placeholder={`*.jpg\n*.raw`}
            className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-xs font-mono text-slate-100 focus:outline-none focus:border-sky-500"
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-slate-300 mb-1">
            Filter Rules (<code>--filter</code>)
          </label>
          <textarea
            rows={4}
            value={filtersText}
            onChange={(e) => setFiltersText(e.target.value)}
            placeholder={`+ /important/**\n- *.cache`}
            className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-xs font-mono text-slate-100 focus:outline-none focus:border-sky-500"
          />
        </div>
      </div>
    </div>
  );
}
