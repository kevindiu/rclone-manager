import { Download, Loader2, Save, Server, Upload } from 'lucide-react';
import { useEffect, useState } from 'react';
import { api } from '../../api/client';
import type { GlobalSettings, SystemInfo } from '../../types/api';
import { COMMON_TIMEZONES } from '../../utils/format';

export function SettingsPage({
  settings,
  systemInfo,
  onSaved,
  onToast,
}: {
  settings: GlobalSettings;
  systemInfo: SystemInfo | null;
  onSaved: () => void;
  onToast: (msg: string, isError?: boolean) => void;
}) {
  const [form, setForm] = useState<GlobalSettings>(settings);
  const [saving, setSaving] = useState(false);
  const [importMode, setImportMode] = useState<'merge' | 'replace'>('merge');
  const [importing, setImporting] = useState(false);

  useEffect(() => {
    setForm(settings);
  }, [settings]);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      await api.updateSettings(form);
      onToast('Global settings saved');
      onSaved();
    } catch (err) {
      onToast(err instanceof Error ? err.message : 'Failed to save settings', true);
    } finally {
      setSaving(false);
    }
  };

  const handleImportFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setImporting(true);
    try {
      const content = await file.text();
      const isYaml = file.name.endsWith('.yaml') || file.name.endsWith('.yml');
      const res = await api.importConfig(
        content,
        importMode,
        isYaml ? 'application/yaml' : 'application/json'
      );
      onToast(`Imported ${res.jobs_imported} job(s) (${res.mode} mode)`);
      onSaved();
    } catch (err) {
      onToast(err instanceof Error ? err.message : 'Import failed', true);
    } finally {
      setImporting(false);
      e.target.value = '';
    }
  };

  return (
    <div className="space-y-6 max-w-4xl">
      <form
        onSubmit={handleSubmit}
        className="p-5 rounded-2xl bg-slate-900/90 border border-slate-800 space-y-5"
      >
        <div>
          <h2 className="text-base font-semibold text-slate-100">Global Default Settings</h2>
          <p className="text-xs text-slate-400">
            Applied to all jobs unless overridden per job. Seeded from environment variables on
            first start.
          </p>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Transfers (<code>--transfers</code>)
            </label>
            <input
              type="number"
              min={1}
              value={form.transfers}
              onChange={(e) => setForm({ ...form, transfers: Number(e.target.value) })}
              className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-sm text-slate-100"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Checkers (<code>--checkers</code>)
            </label>
            <input
              type="number"
              min={1}
              value={form.checkers}
              onChange={(e) => setForm({ ...form, checkers: Number(e.target.value) })}
              className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-sm text-slate-100"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Max Concurrent Jobs
            </label>
            <input
              type="number"
              min={1}
              value={form.max_concurrent_jobs}
              onChange={(e) => setForm({ ...form, max_concurrent_jobs: Number(e.target.value) })}
              className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-sm text-slate-100"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Retries (<code>--retries</code>)
            </label>
            <input
              type="number"
              min={0}
              value={form.retries}
              onChange={(e) => setForm({ ...form, retries: Number(e.target.value) })}
              className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-sm text-slate-100"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Low-Level Retries
            </label>
            <input
              type="number"
              min={0}
              value={form.low_level_retries}
              onChange={(e) => setForm({ ...form, low_level_retries: Number(e.target.value) })}
              className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-sm text-slate-100"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Log Retention (Days, 0 = forever)
            </label>
            <input
              type="number"
              min={0}
              value={form.log_retention_days}
              onChange={(e) => setForm({ ...form, log_retention_days: Number(e.target.value) })}
              className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-sm text-slate-100"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              IO Timeout (<code>--timeout</code>)
            </label>
            <input
              value={form.timeout}
              onChange={(e) => setForm({ ...form, timeout: e.target.value })}
              placeholder="5m"
              className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Connect Timeout (<code>--contimeout</code>)
            </label>
            <input
              value={form.contimeout}
              onChange={(e) => setForm({ ...form, contimeout: e.target.value })}
              placeholder="1m"
              className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">Timezone</label>
            <input
              list="common-tz-list"
              value={form.timezone}
              onChange={(e) => setForm({ ...form, timezone: e.target.value })}
              placeholder="UTC"
              className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100"
            />
            <datalist id="common-tz-list">
              {COMMON_TIMEZONES.map((tz) => (
                <option key={tz} value={tz} />
              ))}
            </datalist>
          </div>
        </div>

        <div>
          <label className="block text-xs font-medium text-slate-300 mb-1">
            Bandwidth Limit / Schedule (<code>--bwlimit</code>)
          </label>
          <input
            value={form.bwlimit}
            onChange={(e) => setForm({ ...form, bwlimit: e.target.value })}
            placeholder='e.g. "10M" or "08:00,5M 23:00,off" (blank = unlimited)'
            className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100"
          />
        </div>

        <div className="flex justify-end">
          <button
            type="submit"
            disabled={saving}
            className="inline-flex items-center gap-2 px-4 py-2 rounded-xl text-sm font-semibold bg-sky-500 hover:bg-sky-400 text-slate-950 disabled:opacity-50"
          >
            {saving ? <Loader2 className="w-4 h-4 animate-spin" /> : <Save className="w-4 h-4" />}
            Save Global Settings
          </button>
        </div>
      </form>

      <div className="p-5 rounded-2xl bg-slate-900/90 border border-slate-800 space-y-4">
        <div>
          <h3 className="text-base font-semibold text-slate-100">
            Backup &amp; Migration (Export / Import)
          </h3>
          <p className="text-xs text-slate-400">
            Export or restore global settings and job definitions in JSON or YAML format.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2.5">
          <a
            href="/api/config/export?format=json"
            className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-xl text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200"
          >
            <Download className="w-3.5 h-3.5 text-sky-400" />
            Export JSON
          </a>
          <a
            href="/api/config/export?format=yaml"
            className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-xl text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200"
          >
            <Download className="w-3.5 h-3.5 text-emerald-400" />
            Export YAML
          </a>
        </div>

        <div className="pt-3 border-t border-slate-800 flex flex-wrap items-center gap-3">
          <div className="flex items-center gap-3 text-xs text-slate-300">
            <label className="inline-flex items-center gap-1.5 cursor-pointer">
              <input
                type="radio"
                checked={importMode === 'merge'}
                onChange={() => setImportMode('merge')}
                className="accent-sky-500"
              />
              Merge jobs
            </label>
            <label className="inline-flex items-center gap-1.5 cursor-pointer">
              <input
                type="radio"
                checked={importMode === 'replace'}
                onChange={() => setImportMode('replace')}
                className="accent-rose-500"
              />
              Replace all jobs
            </label>
          </div>

          <label className="inline-flex items-center gap-2 px-3.5 py-2 rounded-xl text-xs font-medium bg-sky-500/15 hover:bg-sky-500/25 text-sky-300 border border-sky-500/30 cursor-pointer">
            {importing ? (
              <Loader2 className="w-3.5 h-3.5 animate-spin" />
            ) : (
              <Upload className="w-3.5 h-3.5" />
            )}
            Import JSON / YAML File
            <input
              type="file"
              accept=".json,.yaml,.yml"
              onChange={handleImportFile}
              className="hidden"
            />
          </label>
        </div>
      </div>

      {systemInfo && (
        <div className="p-5 rounded-2xl bg-slate-900/90 border border-slate-800 space-y-3">
          <h3 className="text-sm font-semibold text-slate-200 flex items-center gap-2">
            <Server className="w-4 h-4 text-sky-400" />
            System Information
          </h3>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5 text-xs">
            <div className="p-2.5 rounded-xl bg-slate-950 border border-slate-800 flex justify-between">
              <span className="text-slate-400">Bundled rclone</span>
              <span className="font-mono text-emerald-400">{systemInfo.rclone_version}</span>
            </div>
            <div className="p-2.5 rounded-xl bg-slate-950 border border-slate-800 flex justify-between">
              <span className="text-slate-400">App Version</span>
              <span className="font-mono text-slate-200">{systemInfo.app_version}</span>
            </div>
            <div className="p-2.5 rounded-xl bg-slate-950 border border-slate-800 flex justify-between gap-2">
              <span className="text-slate-400 shrink-0">rclone.conf</span>
              <span className="font-mono text-slate-200 truncate">{systemInfo.rclone_config}</span>
            </div>
            <div className="p-2.5 rounded-xl bg-slate-950 border border-slate-800 flex justify-between gap-2">
              <span className="text-slate-400 shrink-0">Data Dir</span>
              <span className="font-mono text-slate-200 truncate">{systemInfo.data_dir}</span>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
