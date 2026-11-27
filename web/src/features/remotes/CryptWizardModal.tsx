import {
  Activity,
  Check,
  CheckCircle2,
  Copy,
  Eye,
  EyeOff,
  FolderSearch,
  KeyRound,
  Loader2,
  Lock,
  RefreshCw,
  XCircle,
} from 'lucide-react';
import { useState } from 'react';
import { api } from '../../api/client';
import { Modal } from '../../components/ui/Modal';
import type { RemoteInfo, RemoteTestResult } from '../../types/api';
import { PathPickerModal } from '../jobs/PathPickerModal';

function generateSecret(length = 32): string {
  const alphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#$%^&*-_';
  const bytes = new Uint8Array(length);
  crypto.getRandomValues(bytes);
  let out = '';
  for (let i = 0; i < length; i++) {
    out += alphabet[bytes[i] % alphabet.length];
  }
  return out;
}

export function CryptWizardModal({
  remotes,
  onCreated,
  onClose,
}: {
  remotes: RemoteInfo[];
  onCreated: (name: string) => void;
  onClose: () => void;
}) {
  const nonCryptRemotes = remotes.filter((r) => r.type !== 'crypt');
  const [name, setName] = useState('');
  const [sourceType, setSourceType] = useState<'remote' | 'local'>(
    nonCryptRemotes.length > 0 ? 'remote' : 'local'
  );
  const [baseRemote, setBaseRemote] = useState(nonCryptRemotes[0]?.name ?? '');
  const [subpath, setSubpath] = useState('encrypted');
  const [localPath, setLocalPath] = useState('/data/encrypted');
  const [filenameEncryption, setFilenameEncryption] = useState<'standard' | 'obfuscate' | 'off'>(
    'standard'
  );
  const [dirNameEncryption, setDirNameEncryption] = useState(true);
  const [password, setPassword] = useState(() => generateSecret(32));
  const [password2, setPassword2] = useState(() => generateSecret(32));
  const [showSecrets, setShowSecrets] = useState(false);
  const [copiedCreds, setCopiedCreds] = useState(false);
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<RemoteTestResult | null>(null);
  const [error, setError] = useState('');
  const [showPicker, setShowPicker] = useState(false);

  const handlePickedTarget = (picked: string) => {
    const clean = picked.trim();
    if (!clean) return;
    if (clean.startsWith('/') || !clean.includes(':')) {
      setSourceType('local');
      setLocalPath(clean);
      return;
    }
    const idx = clean.indexOf(':');
    const rem = clean.slice(0, idx);
    const sub = clean.slice(idx + 1).replace(/^\/+|\/+$/g, '');
    setSourceType('remote');
    setBaseRemote(rem);
    setSubpath(sub);
  };

  const targetRemotePath =
    sourceType === 'local'
      ? localPath.trim()
      : `${baseRemote}:${subpath.trim().replace(/^\/+|\/+$/g, '')}`;

  const handleCopyCreds = () => {
    const text = [
      `Crypt Remote: ${name.trim() || '(unnamed)'}`,
      `Target: ${targetRemotePath}`,
      `Filename Encryption: ${filenameEncryption}`,
      `Directory Encryption: ${dirNameEncryption ? 'true' : 'false'}`,
      `Password: ${password}`,
      `Salt (password2): ${password2 || '(none)'}`,
    ].join('\n');
    void navigator.clipboard.writeText(text);
    setCopiedCreds(true);
    window.setTimeout(() => setCopiedCreds(false), 1800);
  };

  const handleTestConnection = async () => {
    if (!targetRemotePath || (sourceType === 'remote' && !baseRemote)) {
      setError('Target storage location is required.');
      return;
    }
    if (!password) {
      setError('Encryption password is required.');
      return;
    }
    setTesting(true);
    setError('');
    setTestResult(null);
    try {
      const params: Record<string, string> = {
        remote: targetRemotePath,
        filename_encryption: filenameEncryption,
        directory_name_encryption: dirNameEncryption ? 'true' : 'false',
        password,
      };
      if (password2.trim()) {
        params.password2 = password2.trim();
      }
      const res = await api.testDraftRemote(name.trim() || 'crypt-draft', 'crypt', params);
      setTestResult(res);
    } catch (err) {
      setTestResult({
        remote: name.trim() || 'crypt',
        ok: false,
        latency_ms: 0,
        error: err instanceof Error ? err.message : 'Connection test failed',
      });
    } finally {
      setTesting(false);
    }
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    const cleanName = name.trim();
    if (!cleanName) {
      setError('Encrypted remote name is required.');
      return;
    }
    if (!targetRemotePath || (sourceType === 'remote' && !baseRemote)) {
      setError('Target storage location is required.');
      return;
    }
    if (!password) {
      setError('Encryption password is required.');
      return;
    }

    setSaving(true);
    setError('');
    try {
      const params: Record<string, string> = {
        remote: targetRemotePath,
        filename_encryption: filenameEncryption,
        directory_name_encryption: dirNameEncryption ? 'true' : 'false',
        password,
      };
      if (password2.trim()) {
        params.password2 = password2.trim();
      }
      await api.createRemote(cleanName, 'crypt', params);
      onCreated(cleanName);
      onClose();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create encrypted remote');
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      title="Encrypted Remote Wizard (crypt)"
      subtitle="Wrap any cloud or local folder in client-side AES/NaCl encryption before upload"
      onClose={() => !saving && onClose()}
      maxWidth="max-w-xl"
      footer={
        <>
          <button
            type="button"
            disabled={saving}
            onClick={onClose}
            className="px-4 py-2 rounded-lg text-sm font-medium text-slate-300 hover:bg-slate-800 disabled:opacity-50"
          >
            Cancel
          </button>
          <button
            type="button"
            disabled={saving || testing}
            onClick={() => void handleTestConnection()}
            className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-lg text-sm font-medium bg-slate-800 hover:bg-slate-700 text-emerald-300 disabled:opacity-50"
          >
            {testing ? (
              <Loader2 className="w-4 h-4 animate-spin" />
            ) : (
              <Activity className="w-4 h-4" />
            )}
            Test Connection
          </button>
          <button
            type="submit"
            form="crypt-wizard-form"
            disabled={saving || testing}
            className="inline-flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-semibold bg-emerald-500 hover:bg-emerald-400 text-slate-950 disabled:opacity-50"
          >
            {saving ? <Loader2 className="w-4 h-4 animate-spin" /> : <Lock className="w-4 h-4" />}
            Create Encrypted Remote
          </button>
        </>
      }
    >
      <form id="crypt-wizard-form" onSubmit={handleSubmit} className="space-y-4">
        {error && (
          <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-xs text-rose-300">
            {error}
          </div>
        )}

        {testResult && (
          <div
            className={`p-3 rounded-xl border text-xs flex items-start gap-2 ${
              testResult.ok
                ? 'bg-emerald-500/10 border-emerald-500/30 text-emerald-300'
                : 'bg-rose-500/10 border-rose-500/30 text-rose-300'
            }`}
          >
            {testResult.ok ? (
              <CheckCircle2 className="w-4 h-4 shrink-0 mt-0.5" />
            ) : (
              <XCircle className="w-4 h-4 shrink-0 mt-0.5" />
            )}
            <span className="break-words">
              {testResult.ok
                ? `${testResult.message} (${testResult.latency_ms}ms)`
                : testResult.error}
            </span>
          </div>
        )}

        <div>
          <label
            htmlFor="crypt-remote-name"
            className="block text-xs font-medium text-slate-300 mb-1"
          >
            New Crypt Remote Name *
          </label>
          <input
            id="crypt-remote-name"
            required
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g. gdrive-crypt or r2-vault"
            className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100 focus:outline-none focus:border-emerald-500"
          />
        </div>

        <div className="p-3.5 rounded-xl bg-slate-950 border border-slate-800 space-y-3">
          <div className="flex items-center justify-between gap-2">
            <span className="text-xs font-semibold text-slate-200">
              Underlying Storage Location
            </span>
            <div className="flex flex-wrap gap-1.5 text-xs">
              <button
                type="button"
                onClick={() => setShowPicker(true)}
                className="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200"
              >
                <FolderSearch className="w-3.5 h-3.5 text-sky-400" />
                Browse
              </button>
              <button
                type="button"
                disabled={nonCryptRemotes.length === 0}
                onClick={() => setSourceType('remote')}
                className={`px-2.5 py-1 rounded-lg border disabled:opacity-40 ${
                  sourceType === 'remote'
                    ? 'bg-sky-500/15 border-sky-500/50 text-sky-300'
                    : 'bg-slate-900 border-slate-800 text-slate-400'
                }`}
              >
                Remote
              </button>
              <button
                type="button"
                onClick={() => setSourceType('local')}
                className={`px-2.5 py-1 rounded-lg border ${
                  sourceType === 'local'
                    ? 'bg-sky-500/15 border-sky-500/50 text-sky-300'
                    : 'bg-slate-900 border-slate-800 text-slate-400'
                }`}
              >
                Local Folder
              </button>
            </div>
          </div>

          {sourceType === 'remote' ? (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
              <div>
                <label className="block text-[11px] text-slate-400 mb-1">Base Remote</label>
                <select
                  value={baseRemote}
                  onChange={(e) => setBaseRemote(e.target.value)}
                  className="w-full px-3 py-2 rounded-lg bg-slate-900 border border-slate-800 text-xs text-slate-100"
                >
                  {nonCryptRemotes.map((r) => (
                    <option key={r.name} value={r.name}>
                      {r.name} ({r.type})
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-[11px] text-slate-400 mb-1">Subfolder Path</label>
                <input
                  value={subpath}
                  onChange={(e) => setSubpath(e.target.value)}
                  placeholder="encrypted"
                  className="w-full px-3 py-2 rounded-lg bg-slate-900 border border-slate-800 text-xs font-mono text-slate-100"
                />
              </div>
            </div>
          ) : (
            <div>
              <label className="block text-[11px] text-slate-400 mb-1">Local Folder Path</label>
              <input
                value={localPath}
                onChange={(e) => setLocalPath(e.target.value)}
                placeholder="/data/encrypted"
                className="w-full px-3 py-2 rounded-lg bg-slate-900 border border-slate-800 text-xs font-mono text-slate-100"
              />
            </div>
          )}

          <div className="text-[11px] font-mono text-slate-400">
            Target wrapped remote: <span className="text-emerald-300">{targetRemotePath}</span>
          </div>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Filename Encryption
            </label>
            <select
              value={filenameEncryption}
              onChange={(e) =>
                setFilenameEncryption(e.target.value as 'standard' | 'obfuscate' | 'off')
              }
              className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-xs text-slate-100"
            >
              <option value="standard">standard (recommended, randomized names)</option>
              <option value="obfuscate">obfuscate (shorter obfuscated names)</option>
              <option value="off">off (encrypt contents only, keep names)</option>
            </select>
          </div>
          <label className="flex items-center gap-2.5 pt-5 cursor-pointer">
            <input
              type="checkbox"
              checked={dirNameEncryption}
              onChange={(e) => setDirNameEncryption(e.target.checked)}
              className="w-4 h-4 rounded accent-emerald-500"
            />
            <span className="text-xs text-slate-200">Encrypt directory names too</span>
          </label>
        </div>

        <div className="p-3.5 rounded-xl bg-amber-500/10 border border-amber-500/30 space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <span className="inline-flex items-center gap-1.5 text-xs font-semibold text-amber-200">
              <KeyRound className="w-4 h-4 text-amber-400" />
              Encryption Keys (Save These in a Password Manager!)
            </span>
            <div className="flex items-center gap-1.5">
              <button
                type="button"
                onClick={() => setShowSecrets((v) => !v)}
                className="inline-flex items-center gap-1 px-2 py-1 rounded bg-slate-900 hover:bg-slate-800 text-[11px] text-slate-300"
              >
                {showSecrets ? <EyeOff className="w-3 h-3" /> : <Eye className="w-3 h-3" />}
                {showSecrets ? 'Hide' : 'Show'}
              </button>
              <button
                type="button"
                onClick={handleCopyCreds}
                className="inline-flex items-center gap-1 px-2 py-1 rounded bg-slate-900 hover:bg-slate-800 text-[11px] text-emerald-300"
              >
                {copiedCreds ? <Check className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
                {copiedCreds ? 'Copied' : 'Copy Keys'}
              </button>
            </div>
          </div>

          <p className="text-[11px] text-amber-200/80">
            Without this Password and Salt, encrypted files cannot be recovered later. rclone will
            obscure them inside <code>rclone.conf</code>.
          </p>

          <div className="space-y-2.5">
            <div>
              <label className="block text-[11px] text-slate-300 mb-1">Password *</label>
              <div className="flex gap-2">
                <input
                  required
                  type={showSecrets ? 'text' : 'password'}
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  className="flex-1 px-3 py-1.5 rounded-lg bg-slate-950 border border-slate-800 text-xs font-mono text-slate-100"
                />
                <button
                  type="button"
                  onClick={() => setPassword(generateSecret(32))}
                  className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg bg-slate-900 hover:bg-slate-800 text-xs text-slate-300"
                  title="Generate new random password"
                >
                  <RefreshCw className="w-3.5 h-3.5" />
                  Generate
                </button>
              </div>
            </div>

            <div>
              <label className="block text-[11px] text-slate-300 mb-1">
                Salt (<code>password2</code>, recommended)
              </label>
              <div className="flex gap-2">
                <input
                  type={showSecrets ? 'text' : 'password'}
                  value={password2}
                  onChange={(e) => setPassword2(e.target.value)}
                  className="flex-1 px-3 py-1.5 rounded-lg bg-slate-950 border border-slate-800 text-xs font-mono text-slate-100"
                />
                <button
                  type="button"
                  onClick={() => setPassword2(generateSecret(32))}
                  className="inline-flex items-center gap-1 px-2.5 py-1.5 rounded-lg bg-slate-900 hover:bg-slate-800 text-xs text-slate-300"
                  title="Generate new random salt"
                >
                  <RefreshCw className="w-3.5 h-3.5" />
                  Generate
                </button>
              </div>
            </div>
          </div>
        </div>
      </form>

      {showPicker && (
        <PathPickerModal
          remotes={nonCryptRemotes}
          initialValue={targetRemotePath}
          onSelect={handlePickedTarget}
          onClose={() => setShowPicker(false)}
        />
      )}
    </Modal>
  );
}
