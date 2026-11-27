import {
  Check,
  ChevronDown,
  ChevronRight,
  Copy,
  FolderSearch,
  KeyRound,
  Layers,
  Loader2,
  Plus,
  Search,
} from 'lucide-react';
import { useCallback, useMemo, useState } from 'react';
import { api } from '../../api/client';
import { Modal } from '../../components/ui/Modal';
import type { ProviderInfo, ProviderOption, RemoteInfo } from '../../types/api';
import { PathPickerModal } from '../jobs/PathPickerModal';

interface RemoteModalProps {
  providers: ProviderInfo[];
  remotes: RemoteInfo[];
  existing?: RemoteInfo | null;
  onSaved: () => void;
  onClose: () => void;
}

const POPULAR_PROVIDERS = [
  { name: 's3', label: 'S3 / R2 / B2 / MinIO' },
  { name: 'drive', label: 'Google Drive' },
  { name: 'onedrive', label: 'Microsoft OneDrive' },
  { name: 'dropbox', label: 'Dropbox' },
  { name: 'b2', label: 'Backblaze B2 Native' },
  { name: 'sftp', label: 'SFTP / SSH' },
  { name: 'webdav', label: 'WebDAV' },
];

const WRAPPER_PROVIDERS = [
  { name: 'crypt', label: 'Crypt (Encryption)' },
  { name: 'alias', label: 'Alias (Remote / Folder Shortcut)' },
  { name: 'union', label: 'Union (Merge Multiple Remotes)' },
  { name: 'combine', label: 'Combine (Mount Remotes as Folders)' },
  { name: 'chunker', label: 'Chunker (Split Large Files)' },
  { name: 'compress', label: 'Compress (Gzip / Zstd)' },
];

const SINGLE_REMOTE_PROVIDERS = new Set([
  'crypt',
  'alias',
  'chunker',
  'compress',
  'hasher',
  'cache',
]);

export function RemoteModal({ providers, remotes, existing, onSaved, onClose }: RemoteModalProps) {
  const isEdit = Boolean(existing);
  const [name, setName] = useState(existing?.name ?? '');
  const [selectedType, setSelectedType] = useState(existing?.type ?? '');
  const [providerQuery, setProviderQuery] = useState('');
  const [values, setValues] = useState<Record<string, string>>(() => {
    if (!existing) return {};
    const copy: Record<string, string> = {};
    for (const [k, v] of Object.entries(existing.config)) {
      if (k !== 'type') copy[k] = v;
    }
    return copy;
  });
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState('');
  const [copiedCmd, setCopiedCmd] = useState(false);
  const [pickerOpt, setPickerOpt] = useState<{
    optName: string;
    mode: 'replace' | 'append-union' | 'append-combine';
  } | null>(null);
  const [quickUpstreamRemote, setQuickUpstreamRemote] = useState('');

  const selectableRemotes = useMemo(
    () => remotes.filter((r) => !existing || r.name !== existing.name),
    [remotes, existing]
  );

  const activeProvider = useMemo(
    () => providers.find((p) => p.Name === selectedType),
    [providers, selectedType]
  );

  const filteredProviders = useMemo(() => {
    const q = providerQuery.trim().toLowerCase();
    if (!q) return providers;
    return providers.filter(
      (p) => p.Name.toLowerCase().includes(q) || p.Description.toLowerCase().includes(q)
    );
  }, [providers, providerQuery]);

  const subProvider = values.provider ?? '';

  const matchesSubProvider = useCallback(
    (optProvider?: string) => {
      if (!optProvider || !subProvider) return true;
      const list = optProvider.split(',').map((s) => s.trim());
      if (list.includes(subProvider)) return true;
      if (list.every((s) => s.startsWith('!'))) {
        return !list.includes(`!${subProvider}`);
      }
      return false;
    },
    [subProvider]
  );

  const { basicOptions, advancedOptions } = useMemo(() => {
    const opts = activeProvider?.Options ?? [];
    const visible = opts.filter((o) => {
      if (!matchesSubProvider(o.Provider)) return false;
      if (
        activeProvider?.is_oauth &&
        (o.Name === 'token' || o.Name === 'client_id' || o.Name === 'client_secret')
      ) {
        return false;
      }
      return true;
    });
    return {
      basicOptions: visible.filter((o) => !o.Advanced),
      advancedOptions: visible.filter((o) => o.Advanced),
    };
  }, [activeProvider, matchesSubProvider]);

  const handleCopyAuthCmd = () => {
    const cmd = `rclone authorize "${selectedType}"`;
    navigator.clipboard?.writeText(cmd).then(() => {
      setCopiedCmd(true);
      window.setTimeout(() => setCopiedCmd(false), 2000);
    });
  };

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedType) {
      setError('Select a storage provider first');
      return;
    }
    if (activeProvider?.is_oauth && values.token && values.token !== '••••••••') {
      try {
        JSON.parse(values.token);
      } catch {
        setError('OAuth token must be valid JSON as output by `rclone authorize`');
        return;
      }
    }

    setSaving(true);
    setError('');
    try {
      if (isEdit && existing) {
        await api.updateRemote(existing.name, values);
      } else {
        await api.createRemote(name.trim(), selectedType, values);
      }
      onSaved();
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save remote');
    } finally {
      setSaving(false);
    }
  };

  const isSingleRemoteOption = (opt: ProviderOption) =>
    opt.Name === 'remote' &&
    (SINGLE_REMOTE_PROVIDERS.has(selectedType) ||
      opt.Help.toLowerCase().includes('remote:path') ||
      opt.Help.toLowerCase().includes('remote to'));

  const isMultiUpstreamOption = (opt: ProviderOption) =>
    (selectedType === 'union' || selectedType === 'combine') && opt.Name === 'upstreams';

  const appendUpstreamToken = (optName: string, pickedPath: string, isCombine: boolean) => {
    const clean = pickedPath.trim();
    if (!clean) return;
    let token = clean;
    if (isCombine) {
      const baseName = clean.includes(':')
        ? clean.split(':')[0] || 'remote'
        : clean.split('/').filter(Boolean).pop() || 'local';
      token = `${baseName}=${clean}`;
    } else if (clean.includes(' ') && !clean.startsWith('"')) {
      token = `"${clean}"`;
    }
    setValues((prev) => {
      const cur = (prev[optName] ?? '').trim();
      return { ...prev, [optName]: cur ? `${cur} ${token}` : token };
    });
  };

  const renderOptionInput = (opt: ProviderOption) => {
    const val = values[opt.Name] ?? '';
    const isSecret = opt.IsPassword || opt.Sensitive;
    const relevantExamples = (opt.Examples ?? []).filter((ex) => matchesSubProvider(ex.Provider));

    if (isSingleRemoteOption(opt)) {
      const parsedRemoteName = val.includes(':') && !val.startsWith('/') ? val.split(':')[0] : '';
      const existingSubpath =
        val.includes(':') && !val.startsWith('/') ? val.split(':').slice(1).join(':') : '';
      const selectedKnownRemote = selectableRemotes.some((r) => r.name === parsedRemoteName)
        ? parsedRemoteName
        : '';

      return (
        <div key={opt.Name} className="space-y-1.5">
          <div className="flex items-center justify-between gap-2">
            <label className="text-xs font-mono font-medium text-slate-200">
              {opt.Name} {opt.Required && <span className="text-rose-400">*</span>}
            </label>
            <span className="text-[11px] text-slate-400">
              Pick an existing remote or browse folder
            </span>
          </div>

          <div className="flex flex-wrap sm:flex-nowrap gap-2">
            <select
              aria-label={`Select source remote for ${opt.Name}`}
              value={selectedKnownRemote}
              onChange={(e) => {
                const picked = e.target.value;
                if (!picked) return;
                const nextVal = existingSubpath ? `${picked}:${existingSubpath}` : `${picked}:`;
                setValues((prev) => ({ ...prev, [opt.Name]: nextVal }));
              }}
              className="flex-1 min-w-[180px] px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-xs text-slate-100 focus:outline-none focus:border-sky-500"
            >
              <option value="">
                {selectableRemotes.length > 0
                  ? '-- Select an existing remote (or browse / type below) --'
                  : '-- No existing remotes yet (browse local or type below) --'}
              </option>
              {selectableRemotes.map((r) => (
                <option key={r.name} value={r.name}>
                  {r.name} ({r.type})
                </option>
              ))}
            </select>
            <button
              type="button"
              onClick={() => setPickerOpt({ optName: opt.Name, mode: 'replace' })}
              className="inline-flex items-center gap-1.5 px-3 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-200 shrink-0"
            >
              <FolderSearch className="w-4 h-4 text-sky-400" />
              Browse
            </button>
          </div>

          <input
            type="text"
            required={opt.Required && !isEdit}
            value={val}
            onChange={(e) => setValues((prev) => ({ ...prev, [opt.Name]: e.target.value }))}
            placeholder={opt.DefaultStr || 'e.g. gdrive:encrypted or /data/folder'}
            className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-xs font-mono text-slate-100 focus:outline-none focus:border-sky-500"
          />
          {opt.Help && (
            <p className="text-[11px] text-slate-400 mt-1 line-clamp-2">
              {opt.Help.split('\n')[0]}
            </p>
          )}
        </div>
      );
    }

    if (isMultiUpstreamOption(opt)) {
      const isCombine = selectedType === 'combine';
      return (
        <div key={opt.Name} className="space-y-1.5">
          <div className="flex items-center justify-between gap-2">
            <label className="text-xs font-mono font-medium text-slate-200">
              {opt.Name} {opt.Required && <span className="text-rose-400">*</span>}
            </label>
            <span className="text-[11px] text-slate-400">
              Add multiple remotes (space-separated)
            </span>
          </div>

          <div className="flex flex-wrap sm:flex-nowrap gap-2">
            <select
              aria-label="Pick remote to add to upstreams"
              value={quickUpstreamRemote}
              onChange={(e) => setQuickUpstreamRemote(e.target.value)}
              className="flex-1 min-w-[160px] px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-xs text-slate-100 focus:outline-none focus:border-sky-500"
            >
              <option value="">-- Pick remote to append --</option>
              {selectableRemotes.map((r) => (
                <option key={r.name} value={r.name}>
                  {r.name} ({r.type})
                </option>
              ))}
            </select>
            <button
              type="button"
              disabled={!quickUpstreamRemote}
              onClick={() => {
                appendUpstreamToken(opt.Name, `${quickUpstreamRemote}:`, isCombine);
                setQuickUpstreamRemote('');
              }}
              className="inline-flex items-center gap-1 px-3 py-2 rounded-lg bg-sky-500/15 hover:bg-sky-500/25 text-xs font-medium text-sky-300 border border-sky-500/30 disabled:opacity-40 shrink-0"
            >
              <Plus className="w-3.5 h-3.5" />
              Add
            </button>
            <button
              type="button"
              onClick={() =>
                setPickerOpt({
                  optName: opt.Name,
                  mode: isCombine ? 'append-combine' : 'append-union',
                })
              }
              className="inline-flex items-center gap-1.5 px-3 py-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs font-medium text-slate-200 shrink-0"
            >
              <FolderSearch className="w-4 h-4 text-sky-400" />
              Browse
            </button>
          </div>

          <input
            type="text"
            required={opt.Required && !isEdit}
            value={val}
            onChange={(e) => setValues((prev) => ({ ...prev, [opt.Name]: e.target.value }))}
            placeholder={
              isCombine
                ? 'e.g. gdrive=gdrive: onedrive=onedrive:'
                : 'e.g. remoteA:folder remoteB:folder'
            }
            className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-xs font-mono text-slate-100 focus:outline-none focus:border-sky-500"
          />
          {opt.Help && (
            <p className="text-[11px] text-slate-400 mt-1 line-clamp-2">
              {opt.Help.split('\n')[0]}
            </p>
          )}
        </div>
      );
    }

    if (opt.Type === 'bool') {
      const checked = val ? val === 'true' : opt.DefaultStr === 'true';
      return (
        <label
          key={opt.Name}
          className="flex items-start gap-2.5 p-2.5 rounded-lg bg-slate-950 border border-slate-800 cursor-pointer"
        >
          <input
            type="checkbox"
            checked={checked}
            onChange={(e) =>
              setValues((prev) => ({ ...prev, [opt.Name]: e.target.checked ? 'true' : 'false' }))
            }
            className="mt-0.5 w-4 h-4 rounded accent-sky-500"
          />
          <div className="min-w-0">
            <div className="text-xs font-mono font-medium text-slate-200">{opt.Name}</div>
            {opt.Help && (
              <p className="text-[11px] text-slate-400 mt-0.5 line-clamp-2">
                {opt.Help.split('\n')[0]}
              </p>
            )}
          </div>
        </label>
      );
    }

    return (
      <div key={opt.Name}>
        <div className="flex items-center justify-between gap-2 mb-1">
          <label className="text-xs font-mono font-medium text-slate-200">
            {opt.Name} {opt.Required && <span className="text-rose-400">*</span>}
          </label>
          {isEdit && isSecret && val === '••••••••' && (
            <span className="text-[10px] text-emerald-400">
              Stored securely (leave unchanged to keep)
            </span>
          )}
        </div>
        {relevantExamples.length > 0 ? (
          <div className="space-y-1.5">
            <select
              value={relevantExamples.some((ex) => ex.Value === val) ? val : '__custom__'}
              onChange={(e) => {
                if (e.target.value !== '__custom__') {
                  setValues((prev) => ({ ...prev, [opt.Name]: e.target.value }));
                }
              }}
              className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-xs text-slate-100 focus:outline-none focus:border-sky-500"
            >
              <option value="__custom__">
                -- Custom / Default ({opt.DefaultStr || 'none'}) --
              </option>
              {relevantExamples.map((ex) => (
                <option key={ex.Value} value={ex.Value}>
                  {ex.Value} {ex.Help ? `— ${ex.Help.slice(0, 60)}` : ''}
                </option>
              ))}
            </select>
            <input
              type={opt.IsPassword ? 'password' : 'text'}
              value={val}
              onChange={(e) => setValues((prev) => ({ ...prev, [opt.Name]: e.target.value }))}
              placeholder={opt.DefaultStr || `Enter ${opt.Name}`}
              className="w-full px-3 py-1.5 rounded-lg bg-slate-950 border border-slate-800 text-xs font-mono text-slate-100 focus:outline-none focus:border-sky-500"
            />
          </div>
        ) : (
          <input
            type={opt.IsPassword ? 'password' : 'text'}
            required={opt.Required && !isEdit}
            value={val}
            onChange={(e) => setValues((prev) => ({ ...prev, [opt.Name]: e.target.value }))}
            placeholder={opt.DefaultStr || `Enter ${opt.Name}`}
            className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-xs font-mono text-slate-100 focus:outline-none focus:border-sky-500"
          />
        )}
        {opt.Help && (
          <p className="text-[11px] text-slate-400 mt-1 line-clamp-2">{opt.Help.split('\n')[0]}</p>
        )}
      </div>
    );
  };

  return (
    <Modal
      title={isEdit ? `Edit Remote: ${existing?.name}` : 'Add Storage Remote'}
      subtitle={
        activeProvider
          ? `${activeProvider.Description} (${activeProvider.Name})`
          : 'Choose any of rclone’s supported backends or crypt overlay'
      }
      onClose={onClose}
      maxWidth="max-w-2xl"
      footer={
        <>
          <button
            type="button"
            onClick={onClose}
            className="px-4 py-2 rounded-lg text-sm font-medium text-slate-300 hover:bg-slate-800"
          >
            Cancel
          </button>
          {selectedType && (
            <button
              type="submit"
              form="remote-form"
              disabled={saving}
              className="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-semibold bg-sky-500 hover:bg-sky-400 text-slate-950 disabled:opacity-50"
            >
              {saving && <Loader2 className="w-4 h-4 animate-spin" />}
              {isEdit ? 'Save Remote' : 'Create Remote'}
            </button>
          )}
        </>
      }
    >
      {!selectedType ? (
        <div className="space-y-4">
          <div>
            <div className="text-xs font-semibold uppercase tracking-wider text-slate-400 mb-2">
              Popular Providers
            </div>
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
              {POPULAR_PROVIDERS.map((p) => (
                <button
                  key={p.name}
                  type="button"
                  onClick={() => setSelectedType(p.name)}
                  className="p-2.5 rounded-xl bg-slate-950 border border-slate-800 hover:border-sky-500/60 text-left transition-colors"
                >
                  <div className="text-xs font-semibold text-slate-100">{p.label}</div>
                  <div className="text-[11px] font-mono text-sky-400 mt-0.5">{p.name}</div>
                </button>
              ))}
            </div>
          </div>

          <div>
            <div className="flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wider text-emerald-400 mb-2">
              <Layers className="w-3.5 h-3.5" />
              Wrapper / Overlay Types (Wrap Existing Remotes)
            </div>
            <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
              {WRAPPER_PROVIDERS.map((p) => (
                <button
                  key={p.name}
                  type="button"
                  onClick={() => setSelectedType(p.name)}
                  className="p-2.5 rounded-xl bg-slate-950 border border-emerald-500/20 hover:border-emerald-500/60 text-left transition-colors"
                >
                  <div className="text-xs font-semibold text-slate-100">{p.label}</div>
                  <div className="text-[11px] font-mono text-emerald-400 mt-0.5">{p.name}</div>
                </button>
              ))}
            </div>
          </div>

          <div>
            <div className="relative mb-2">
              <Search className="w-4 h-4 text-slate-500 absolute left-3 top-2.5" />
              <input
                value={providerQuery}
                onChange={(e) => setProviderQuery(e.target.value)}
                placeholder="Search all 70+ rclone providers..."
                className="w-full pl-9 pr-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm text-slate-100 focus:outline-none focus:border-sky-500"
              />
            </div>
            <div className="max-h-60 overflow-y-auto border border-slate-800 rounded-xl divide-y divide-slate-800/70 bg-slate-950/60">
              {filteredProviders.map((p) => (
                <button
                  key={p.Name}
                  type="button"
                  onClick={() => setSelectedType(p.Name)}
                  className="w-full flex items-center justify-between px-3.5 py-2.5 text-left hover:bg-slate-800/60 transition-colors"
                >
                  <div className="min-w-0 pr-3">
                    <div className="text-sm font-medium text-slate-100 truncate">
                      {p.Description}
                    </div>
                    <div className="text-xs font-mono text-slate-400">{p.Name}</div>
                  </div>
                  {p.is_oauth && (
                    <span className="px-2 py-0.5 rounded text-[10px] font-medium bg-sky-500/15 text-sky-300 border border-sky-500/30 shrink-0">
                      OAuth
                    </span>
                  )}
                </button>
              ))}
            </div>
          </div>
        </div>
      ) : (
        <form id="remote-form" onSubmit={handleSave} className="space-y-4">
          {error && (
            <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-xs text-rose-300">
              {error}
            </div>
          )}

          <div className="flex items-center justify-between gap-2">
            <div className="flex-1">
              <label className="block text-xs font-medium text-slate-300 mb-1">Remote Name *</label>
              <input
                required
                disabled={isEdit}
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="e.g. r2-backups or my-crypt"
                className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100 disabled:opacity-60 focus:outline-none focus:border-sky-500"
              />
            </div>
            {!isEdit && (
              <button
                type="button"
                onClick={() => {
                  setSelectedType('');
                  setValues({});
                }}
                className="mt-5 px-3 py-2 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200"
              >
                Change Type
              </button>
            )}
          </div>

          {activeProvider?.is_oauth && (
            <div className="p-3.5 rounded-xl bg-sky-950/30 border border-sky-500/30 space-y-3">
              <div className="flex items-center justify-between gap-2">
                <div className="flex items-center gap-2 text-xs font-semibold text-sky-300">
                  <KeyRound className="w-4 h-4" />
                  Headless OAuth Credentials &amp; Token Paste
                </div>
                <button
                  type="button"
                  onClick={handleCopyAuthCmd}
                  className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md text-[11px] bg-sky-500/20 hover:bg-sky-500/30 text-sky-200"
                >
                  {copiedCmd ? <Check className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
                  {copiedCmd ? 'Copied!' : `rclone authorize "${selectedType}"`}
                </button>
              </div>
              <p className="text-[11px] text-slate-300 leading-relaxed">
                Run{' '}
                <code className="text-sky-300">rclone authorize &quot;{selectedType}&quot;</code> on
                a machine with a browser, then paste the resulting JSON token below. Optional custom
                OAuth client credentials are supported.
              </p>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
                <div>
                  <label className="block text-[11px] font-mono text-slate-300 mb-1">
                    client_id (optional)
                  </label>
                  <input
                    value={values.client_id ?? ''}
                    onChange={(e) => setValues((p) => ({ ...p, client_id: e.target.value }))}
                    placeholder="Custom OAuth Client ID"
                    className="w-full px-2.5 py-1.5 rounded-lg bg-slate-950 border border-slate-800 text-xs font-mono text-slate-100"
                  />
                </div>
                <div>
                  <label className="block text-[11px] font-mono text-slate-300 mb-1">
                    client_secret (optional)
                  </label>
                  <input
                    type="password"
                    value={values.client_secret ?? ''}
                    onChange={(e) => setValues((p) => ({ ...p, client_secret: e.target.value }))}
                    placeholder="Custom OAuth Client Secret"
                    className="w-full px-2.5 py-1.5 rounded-lg bg-slate-950 border border-slate-800 text-xs font-mono text-slate-100"
                  />
                </div>
              </div>

              <div>
                <label className="block text-[11px] font-mono text-slate-300 mb-1">
                  token (JSON blob)
                </label>
                <textarea
                  rows={3}
                  value={values.token ?? ''}
                  onChange={(e) => setValues((p) => ({ ...p, token: e.target.value }))}
                  placeholder='{"access_token":"...","token_type":"Bearer","refresh_token":"...","expiry":"..."}'
                  className="w-full px-2.5 py-1.5 rounded-lg bg-slate-950 border border-slate-800 text-xs font-mono text-slate-100"
                />
              </div>
            </div>
          )}

          <div className="space-y-3">{basicOptions.map(renderOptionInput)}</div>

          {advancedOptions.length > 0 && (
            <div className="pt-2 border-t border-slate-800">
              <button
                type="button"
                onClick={() => setShowAdvanced((v) => !v)}
                className="inline-flex items-center gap-1.5 text-xs font-medium text-slate-300 hover:text-sky-400"
              >
                {showAdvanced ? (
                  <ChevronDown className="w-4 h-4" />
                ) : (
                  <ChevronRight className="w-4 h-4" />
                )}
                Advanced Provider Options ({advancedOptions.length})
              </button>
              {showAdvanced && (
                <div className="mt-3 space-y-3">{advancedOptions.map(renderOptionInput)}</div>
              )}
            </div>
          )}
        </form>
      )}
      {pickerOpt && (
        <PathPickerModal
          remotes={selectableRemotes}
          initialValue={pickerOpt.mode === 'replace' ? (values[pickerOpt.optName] ?? '') : ''}
          onSelect={(picked) => {
            if (pickerOpt.mode === 'replace') {
              setValues((prev) => ({ ...prev, [pickerOpt.optName]: picked }));
            } else {
              appendUpstreamToken(pickerOpt.optName, picked, pickerOpt.mode === 'append-combine');
            }
          }}
          onClose={() => setPickerOpt(null)}
        />
      )}
    </Modal>
  );
}
