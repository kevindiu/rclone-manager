import {
  Bell,
  Download,
  Eye,
  EyeOff,
  KeyRound,
  Loader2,
  Lock,
  LogOut,
  Monitor,
  Save,
  Send,
  Server,
  ShieldCheck,
  SlidersHorizontal,
  Upload,
} from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';
import { api } from '../../api/client';
import { ConfirmModal } from '../../components/ui/ConfirmModal';
import type { AuthUser, GlobalSettings, SessionInfo, SystemInfo } from '../../types/api';
import { COMMON_TIMEZONES, formatDateTime } from '../../utils/format';

type SettingsTab = 'general' | 'notifications' | 'security' | 'backup';

export function SettingsPage({
  settings,
  systemInfo,
  currentUser,
  onSaved,
  onToast,
}: {
  settings: GlobalSettings;
  systemInfo: SystemInfo | null;
  currentUser: AuthUser | null;
  onSaved: () => void;
  onToast: (msg: string, isError?: boolean) => void;
}) {
  const [activeTab, setActiveTab] = useState<SettingsTab>('general');
  const [form, setForm] = useState<GlobalSettings>(settings);
  const [saving, setSaving] = useState(false);
  const [testingWebhook, setTestingWebhook] = useState(false);
  const [importMode, setImportMode] = useState<'merge' | 'replace'>('merge');
  const [importing, setImporting] = useState(false);

  const [exportPassphrase, setExportPassphrase] = useState('');
  const [showExportPassphrase, setShowExportPassphrase] = useState(false);
  const [exportingFull, setExportingFull] = useState(false);
  const [restorePassphrase, setRestorePassphrase] = useState('');
  const [showRestorePassphrase, setShowRestorePassphrase] = useState(false);
  const [restoreMode, setRestoreMode] = useState<'merge' | 'replace'>('merge');
  const [restoringFull, setRestoringFull] = useState(false);

  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmNewPassword, setConfirmNewPassword] = useState('');
  const [showPasswords, setShowPasswords] = useState(false);
  const [showTelegramToken, setShowTelegramToken] = useState(false);
  const [changingPassword, setChangingPassword] = useState(false);

  const [sessions, setSessions] = useState<SessionInfo[]>([]);
  const [loadingSessions, setLoadingSessions] = useState(false);
  const [confirmRevokeOthers, setConfirmRevokeOthers] = useState(false);
  const [revokingOthers, setRevokingOthers] = useState(false);

  const hasUnsavedChanges =
    form.transfers !== settings.transfers ||
    form.checkers !== settings.checkers ||
    form.bwlimit !== settings.bwlimit ||
    form.retries !== settings.retries ||
    form.low_level_retries !== settings.low_level_retries ||
    form.timeout !== settings.timeout ||
    form.contimeout !== settings.contimeout ||
    form.log_retention_days !== settings.log_retention_days ||
    form.timezone !== settings.timezone ||
    form.max_concurrent_jobs !== settings.max_concurrent_jobs ||
    (form.webhook_url ?? '') !== (settings.webhook_url ?? '') ||
    (form.webhook_on || 'failure') !== (settings.webhook_on || 'failure') ||
    (form.webhook_type || 'generic') !== (settings.webhook_type || 'generic') ||
    (form.telegram_bot_token ?? '') !== (settings.telegram_bot_token ?? '') ||
    (form.telegram_chat_id ?? '') !== (settings.telegram_chat_id ?? '');

  useEffect(() => {
    setForm(settings);
  }, [settings]);

  const loadSessions = useCallback(async () => {
    setLoadingSessions(true);
    try {
      const list = await api.authSessions();
      setSessions(list ?? []);
    } catch {
      // ignore session load errors
    } finally {
      setLoadingSessions(false);
    }
  }, []);

  useEffect(() => {
    void loadSessions();
  }, [loadSessions]);

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

  const handleTestWebhook = async () => {
    setTestingWebhook(true);
    try {
      await api.testWebhook({
        webhook_url: form.webhook_url ?? '',
        webhook_type: form.webhook_type || 'generic',
        telegram_bot_token: form.telegram_bot_token ?? '',
        telegram_chat_id: form.telegram_chat_id ?? '',
      });
      onToast('Test notification sent');
    } catch (err) {
      onToast(err instanceof Error ? err.message : 'Notification test failed', true);
    } finally {
      setTestingWebhook(false);
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

  const handleExportFullBackup = async (e: React.FormEvent) => {
    e.preventDefault();
    if (exportPassphrase.length < 8) {
      onToast('Backup passphrase must be at least 8 characters', true);
      return;
    }
    setExportingFull(true);
    try {
      const { blob, filename } = await api.exportFullBackup(exportPassphrase);
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
      setExportPassphrase('');
      onToast('Encrypted full backup downloaded');
    } catch (err) {
      onToast(err instanceof Error ? err.message : 'Failed to export encrypted backup', true);
    } finally {
      setExportingFull(false);
    }
  };

  const handleRestoreFullBackupFile = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (!restorePassphrase) {
      onToast('Enter the backup passphrase before selecting a file', true);
      e.target.value = '';
      return;
    }
    setRestoringFull(true);
    try {
      const content = await file.text();
      const res = await api.importFullBackup(content, restorePassphrase, restoreMode);
      setRestorePassphrase('');
      onToast(
        `Restored ${res.remotes_imported} remote(s) and ${res.jobs_imported} job(s) (${res.mode} mode)`
      );
      onSaved();
    } catch (err) {
      onToast(err instanceof Error ? err.message : 'Encrypted backup restore failed', true);
    } finally {
      setRestoringFull(false);
      e.target.value = '';
    }
  };

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!currentPassword) {
      onToast('Current password is required', true);
      return;
    }
    if (newPassword.length < 8) {
      onToast('New password must be at least 8 characters', true);
      return;
    }
    if (newPassword !== confirmNewPassword) {
      onToast('New passwords do not match', true);
      return;
    }
    setChangingPassword(true);
    try {
      await api.authChangePassword(currentPassword, newPassword);
      setCurrentPassword('');
      setNewPassword('');
      setConfirmNewPassword('');
      void loadSessions();
      onToast('Password updated. Other active sessions were signed out.');
    } catch (err) {
      onToast(err instanceof Error ? err.message : 'Failed to update password', true);
    } finally {
      setChangingPassword(false);
    }
  };

  const handleRevokeOtherSessions = async () => {
    setRevokingOthers(true);
    try {
      await api.authRevokeOtherSessions();
      setConfirmRevokeOthers(false);
      await loadSessions();
      onToast('Signed out all other devices');
    } catch (err) {
      onToast(err instanceof Error ? err.message : 'Failed to revoke other sessions', true);
    } finally {
      setRevokingOthers(false);
    }
  };

  const hasGeneralChanges =
    form.transfers !== settings.transfers ||
    form.checkers !== settings.checkers ||
    form.bwlimit !== settings.bwlimit ||
    form.retries !== settings.retries ||
    form.low_level_retries !== settings.low_level_retries ||
    form.timeout !== settings.timeout ||
    form.contimeout !== settings.contimeout ||
    form.log_retention_days !== settings.log_retention_days ||
    form.timezone !== settings.timezone ||
    form.max_concurrent_jobs !== settings.max_concurrent_jobs;

  const hasNotificationChanges =
    (form.webhook_url ?? '') !== (settings.webhook_url ?? '') ||
    (form.webhook_on || 'failure') !== (settings.webhook_on || 'failure') ||
    (form.webhook_type || 'generic') !== (settings.webhook_type || 'generic') ||
    (form.telegram_bot_token ?? '') !== (settings.telegram_bot_token ?? '') ||
    (form.telegram_chat_id ?? '') !== (settings.telegram_chat_id ?? '');

  const tabs: Array<{
    id: SettingsTab;
    label: string;
    icon: typeof SlidersHorizontal;
    dirty?: boolean;
  }> = [
    { id: 'general', label: 'General', icon: SlidersHorizontal, dirty: hasGeneralChanges },
    { id: 'notifications', label: 'Notifications', icon: Bell, dirty: hasNotificationChanges },
    { id: 'security', label: 'Security', icon: KeyRound },
    { id: 'backup', label: 'Backup & Info', icon: Server },
  ];

  return (
    <div className="space-y-5 max-w-4xl">
      <div
        role="tablist"
        aria-label="Settings sections"
        className="flex flex-wrap gap-1.5 p-1 rounded-xl bg-slate-900/90 border border-slate-800 w-fit"
      >
        {tabs.map((tab) => {
          const Icon = tab.icon;
          const active = activeTab === tab.id;
          return (
            <button
              key={tab.id}
              type="button"
              role="tab"
              aria-selected={active}
              onClick={() => setActiveTab(tab.id)}
              className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium transition-colors ${
                active
                  ? 'bg-sky-500/20 text-sky-300 border border-sky-500/40'
                  : 'text-slate-400 hover:text-slate-200 border border-transparent'
              }`}
            >
              <Icon className="w-3.5 h-3.5" />
              <span>{tab.label}</span>
              {tab.dirty && (
                <span className="w-1.5 h-1.5 rounded-full bg-amber-400" title="Unsaved changes" />
              )}
            </button>
          );
        })}
      </div>

      {(activeTab === 'general' || activeTab === 'notifications') && (
        <form
          onSubmit={handleSubmit}
          className="p-5 rounded-2xl bg-slate-900/90 border border-slate-800 space-y-5"
        >
          {activeTab === 'general' && (
            <>
              <div>
                <h2 className="text-base font-semibold text-slate-100">Global Default Settings</h2>
                <p className="text-xs text-slate-400">
                  Applied to all jobs unless overridden per job. Seeded from environment variables
                  on first start.
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
                    onChange={(e) =>
                      setForm({ ...form, max_concurrent_jobs: Number(e.target.value) })
                    }
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
                    onChange={(e) =>
                      setForm({ ...form, low_level_retries: Number(e.target.value) })
                    }
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
                    onChange={(e) =>
                      setForm({ ...form, log_retention_days: Number(e.target.value) })
                    }
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
            </>
          )}

          {activeTab === 'notifications' && (
            <div className="space-y-3">
              <div>
                <h2 className="text-base font-semibold text-slate-100 flex items-center gap-2">
                  <Bell className="w-4 h-4 text-sky-400" />
                  Notifications
                </h2>
                <p className="text-xs text-slate-400">
                  Choose a notification provider: Webhook (Discord / Slack / Generic), Telegram Bot,
                  or Ntfy.
                </p>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-3 gap-2.5">
                {(
                  [
                    {
                      id: 'generic',
                      title: 'Webhook',
                      desc: 'Discord, Slack, or HTTP JSON',
                    },
                    {
                      id: 'telegram',
                      title: 'Telegram Bot',
                      desc: 'Bot Token + Chat ID (no URL needed)',
                    },
                    {
                      id: 'ntfy',
                      title: 'Ntfy',
                      desc: 'Self-hosted or ntfy.sh topic',
                    },
                  ] as const
                ).map((opt) => {
                  const active = (form.webhook_type || 'generic') === opt.id;
                  return (
                    <button
                      key={opt.id}
                      type="button"
                      onClick={() => setForm({ ...form, webhook_type: opt.id })}
                      className={`p-3 rounded-xl border text-left transition-colors ${
                        active
                          ? 'bg-sky-500/15 border-sky-500/60 text-slate-100'
                          : 'bg-slate-950 border-slate-800 text-slate-300 hover:border-slate-700'
                      }`}
                    >
                      <div className="text-xs font-semibold">{opt.title}</div>
                      <div className="text-[11px] text-slate-400 mt-0.5">{opt.desc}</div>
                    </button>
                  );
                })}
              </div>

              {(form.webhook_type || 'generic') === 'telegram' ? (
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 items-start">
                  <div>
                    <div className="flex items-center justify-between gap-2 mb-1">
                      <label
                        htmlFor="settings-telegram-bot-token"
                        className="text-xs font-medium text-slate-300"
                      >
                        Telegram Bot Token *
                      </label>
                      <button
                        type="button"
                        onClick={() => setShowTelegramToken((v) => !v)}
                        className="inline-flex items-center gap-1 text-[11px] text-slate-400 hover:text-slate-200"
                      >
                        {showTelegramToken ? (
                          <EyeOff className="w-3 h-3" />
                        ) : (
                          <Eye className="w-3 h-3" />
                        )}
                        {showTelegramToken ? 'Hide' : 'Show'}
                      </button>
                    </div>
                    <input
                      id="settings-telegram-bot-token"
                      type={showTelegramToken ? 'text' : 'password'}
                      value={form.telegram_bot_token ?? ''}
                      onChange={(e) => setForm({ ...form, telegram_bot_token: e.target.value })}
                      placeholder="123456789:ABCdefGHIjklMNO..."
                      className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100"
                    />
                    {form.telegram_bot_token === '••••••••' && (
                      <p className="text-[11px] text-emerald-400 mt-1">
                        Stored securely (leave unchanged to keep)
                      </p>
                    )}
                  </div>

                  <div>
                    <label
                      htmlFor="settings-telegram-chat-id"
                      className="block text-xs font-medium text-slate-300 mb-1"
                    >
                      Telegram Chat ID *
                    </label>
                    <input
                      id="settings-telegram-chat-id"
                      value={form.telegram_chat_id ?? ''}
                      onChange={(e) => setForm({ ...form, telegram_chat_id: e.target.value })}
                      placeholder="e.g. -1001234567890 or 123456789"
                      className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100"
                    />
                  </div>

                  <div>
                    <label
                      htmlFor="settings-webhook-on"
                      className="block text-xs font-medium text-slate-300 mb-1"
                    >
                      Notify On
                    </label>
                    <select
                      id="settings-webhook-on"
                      value={form.webhook_on || 'failure'}
                      onChange={(e) =>
                        setForm({
                          ...form,
                          webhook_on: e.target.value as 'failure' | 'always' | 'off',
                        })
                      }
                      className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-sm text-slate-100"
                    >
                      <option value="failure">On failure only</option>
                      <option value="always">Always (all runs)</option>
                      <option value="off">Disabled</option>
                    </select>
                  </div>
                </div>
              ) : (
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 items-end">
                  <div className="sm:col-span-2">
                    <label
                      htmlFor="settings-webhook-url"
                      className="block text-xs font-medium text-slate-300 mb-1"
                    >
                      {(form.webhook_type || 'generic') === 'ntfy'
                        ? 'Ntfy Topic URL'
                        : 'Webhook URL'}
                    </label>
                    <input
                      id="settings-webhook-url"
                      type="url"
                      value={form.webhook_url ?? ''}
                      onChange={(e) => setForm({ ...form, webhook_url: e.target.value })}
                      placeholder={
                        (form.webhook_type || 'generic') === 'ntfy'
                          ? 'https://ntfy.sh/my-backup-topic'
                          : 'https://discord.com/api/webhooks/...'
                      }
                      className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-sm font-mono text-slate-100"
                    />
                  </div>
                  <div>
                    <label
                      htmlFor="settings-webhook-on"
                      className="block text-xs font-medium text-slate-300 mb-1"
                    >
                      Notify On
                    </label>
                    <select
                      id="settings-webhook-on"
                      value={form.webhook_on || 'failure'}
                      onChange={(e) =>
                        setForm({
                          ...form,
                          webhook_on: e.target.value as 'failure' | 'always' | 'off',
                        })
                      }
                      className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-sm text-slate-100"
                    >
                      <option value="failure">On failure only</option>
                      <option value="always">Always (all runs)</option>
                      <option value="off">Disabled</option>
                    </select>
                  </div>
                </div>
              )}

              <div>
                <button
                  type="button"
                  disabled={
                    testingWebhook ||
                    ((form.webhook_type || 'generic') === 'telegram'
                      ? (!(form.telegram_bot_token ?? '').trim() &&
                          !(form.webhook_url ?? '').trim()) ||
                        !(form.telegram_chat_id ?? '').trim()
                      : !(form.webhook_url ?? '').trim())
                  }
                  onClick={() => void handleTestWebhook()}
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 text-slate-200 disabled:opacity-50"
                >
                  {testingWebhook ? (
                    <Loader2 className="w-3.5 h-3.5 animate-spin" />
                  ) : (
                    <Send className="w-3.5 h-3.5 text-sky-400" />
                  )}
                  Send Test
                </button>
              </div>
            </div>
          )}

          <div className="flex items-center justify-end gap-3 pt-2 border-t border-slate-800/70">
            {hasUnsavedChanges && <span className="text-xs text-amber-300">Unsaved changes</span>}
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
      )}

      {activeTab === 'backup' && (
        <>
          <div className="p-5 rounded-2xl bg-slate-900/90 border border-slate-800 space-y-5">
            <div>
              <h3 className="text-base font-semibold text-slate-100 flex items-center gap-2">
                <ShieldCheck className="w-4 h-4 text-emerald-400" />
                Full Encrypted Backup (Remotes + Settings + Jobs)
              </h3>
              <p className="text-xs text-slate-400 mt-0.5">
                Creates a password-encrypted <code>.rcmbak</code> archive containing{' '}
                <code>rclone.conf</code> (all remotes &amp; credentials), Global Settings, and all
                Jobs. Encrypted server-side with PBKDF2-SHA256 &amp; AES-256-GCM.
              </p>
            </div>

            <form onSubmit={handleExportFullBackup} className="flex flex-wrap items-center gap-2.5">
              <div className="relative flex-1 min-w-[220px]">
                <Lock className="w-3.5 h-3.5 text-slate-500 absolute left-3 top-2.5" />
                <input
                  type={showExportPassphrase ? 'text' : 'password'}
                  value={exportPassphrase}
                  onChange={(e) => setExportPassphrase(e.target.value)}
                  placeholder="Backup passphrase (min 8 chars)"
                  autoComplete="new-password"
                  className="w-full pl-9 pr-9 py-2 rounded-xl bg-slate-950 border border-slate-800 text-xs sm:text-sm text-slate-100 focus:outline-none focus:border-sky-500"
                />
                <button
                  type="button"
                  onClick={() => setShowExportPassphrase((v) => !v)}
                  className="absolute right-2.5 top-2 text-slate-400 hover:text-slate-200"
                  aria-label="Toggle export passphrase visibility"
                >
                  {showExportPassphrase ? (
                    <EyeOff className="w-4 h-4" />
                  ) : (
                    <Eye className="w-4 h-4" />
                  )}
                </button>
              </div>
              <button
                type="submit"
                disabled={exportingFull}
                className="inline-flex items-center gap-1.5 px-3.5 py-2 rounded-xl text-xs font-semibold bg-sky-500 hover:bg-sky-400 text-slate-950 disabled:opacity-50"
              >
                {exportingFull ? (
                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                ) : (
                  <Download className="w-3.5 h-3.5" />
                )}
                Download Encrypted Backup (.rcmbak)
              </button>
            </form>

            <div className="pt-4 border-t border-slate-800 space-y-3">
              <div className="text-xs font-medium text-slate-300">
                Restore Full Encrypted Backup
              </div>
              <div className="flex flex-wrap items-center gap-3">
                <div className="relative flex-1 min-w-[220px]">
                  <Lock className="w-3.5 h-3.5 text-slate-500 absolute left-3 top-2.5" />
                  <input
                    type={showRestorePassphrase ? 'text' : 'password'}
                    value={restorePassphrase}
                    onChange={(e) => setRestorePassphrase(e.target.value)}
                    placeholder="Passphrase to decrypt backup"
                    autoComplete="off"
                    className="w-full pl-9 pr-9 py-2 rounded-xl bg-slate-950 border border-slate-800 text-xs sm:text-sm text-slate-100 focus:outline-none focus:border-sky-500"
                  />
                  <button
                    type="button"
                    onClick={() => setShowRestorePassphrase((v) => !v)}
                    className="absolute right-2.5 top-2 text-slate-400 hover:text-slate-200"
                    aria-label="Toggle restore passphrase visibility"
                  >
                    {showRestorePassphrase ? (
                      <EyeOff className="w-4 h-4" />
                    ) : (
                      <Eye className="w-4 h-4" />
                    )}
                  </button>
                </div>

                <div className="flex items-center gap-3 text-xs text-slate-300">
                  <label className="inline-flex items-center gap-1.5 cursor-pointer">
                    <input
                      type="radio"
                      checked={restoreMode === 'merge'}
                      onChange={() => setRestoreMode('merge')}
                      className="accent-sky-500"
                    />
                    Merge
                  </label>
                  <label className="inline-flex items-center gap-1.5 cursor-pointer">
                    <input
                      type="radio"
                      checked={restoreMode === 'replace'}
                      onChange={() => setRestoreMode('replace')}
                      className="accent-rose-500"
                    />
                    Replace all
                  </label>
                </div>

                <div
                  className={`relative overflow-hidden inline-flex items-center gap-2 px-3.5 py-2 rounded-xl text-xs font-medium bg-emerald-500/15 hover:bg-emerald-500/25 text-emerald-300 border border-emerald-500/30 ${
                    restoringFull ? 'opacity-50 pointer-events-none' : 'cursor-pointer'
                  }`}
                >
                  {restoringFull ? (
                    <Loader2 className="w-3.5 h-3.5 animate-spin" />
                  ) : (
                    <Upload className="w-3.5 h-3.5" />
                  )}
                  <span>Restore .rcmbak File</span>
                  <input
                    type="file"
                    aria-label="Restore encrypted backup file"
                    disabled={restoringFull}
                    onChange={(e) => void handleRestoreFullBackupFile(e)}
                    className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
                  />
                </div>
              </div>
            </div>
          </div>

          <div className="p-5 rounded-2xl bg-slate-900/90 border border-slate-800 space-y-4">
            <div>
              <h3 className="text-base font-semibold text-slate-100">
                Settings &amp; Jobs Only (JSON / YAML)
              </h3>
              <p className="text-xs text-slate-400">
                Export or restore global settings and job definitions in plain JSON or YAML
                (excludes <code>rclone.conf</code> remotes).
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

              <div
                className={`relative overflow-hidden inline-flex items-center gap-2 px-3.5 py-2 rounded-xl text-xs font-medium bg-sky-500/15 hover:bg-sky-500/25 text-sky-300 border border-sky-500/30 ${
                  importing ? 'opacity-50 pointer-events-none' : 'cursor-pointer'
                }`}
              >
                {importing ? (
                  <Loader2 className="w-3.5 h-3.5 animate-spin" />
                ) : (
                  <Upload className="w-3.5 h-3.5" />
                )}
                <span>Import JSON / YAML File</span>
                <input
                  type="file"
                  aria-label="Import JSON or YAML configuration file"
                  disabled={importing}
                  onChange={(e) => void handleImportFile(e)}
                  className="absolute inset-0 w-full h-full opacity-0 cursor-pointer"
                />
              </div>
            </div>
          </div>
        </>
      )}

      {activeTab === 'security' && (
        <form
          onSubmit={handleChangePassword}
          className="p-5 rounded-2xl bg-slate-900/90 border border-slate-800 space-y-4"
        >
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div>
              <h3 className="text-base font-semibold text-slate-100 flex items-center gap-2">
                <KeyRound className="w-4 h-4 text-sky-400" />
                Account Security
              </h3>
              <p className="text-xs text-slate-400">
                Update your administrator password. Updating it revokes other signed-in sessions.
              </p>
            </div>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => setShowPasswords((v) => !v)}
                className="inline-flex items-center gap-1 px-2.5 py-1 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs text-slate-300"
              >
                {showPasswords ? (
                  <EyeOff className="w-3.5 h-3.5" />
                ) : (
                  <Eye className="w-3.5 h-3.5" />
                )}
                {showPasswords ? 'Hide' : 'Show'}
              </button>
              {currentUser && (
                <span className="px-2.5 py-1 rounded-lg bg-slate-950 border border-slate-800 text-xs font-mono text-slate-300">
                  {currentUser.username}
                </span>
              )}
            </div>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
            <div>
              <label
                htmlFor="settings-current-password"
                className="block text-xs font-medium text-slate-300 mb-1"
              >
                Current Password
              </label>
              <input
                id="settings-current-password"
                type={showPasswords ? 'text' : 'password'}
                autoComplete="current-password"
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
                className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-sm text-slate-100"
              />
            </div>

            <div>
              <label
                htmlFor="settings-new-password"
                className="block text-xs font-medium text-slate-300 mb-1"
              >
                New Password (min 8 chars)
              </label>
              <input
                id="settings-new-password"
                type={showPasswords ? 'text' : 'password'}
                autoComplete="new-password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-sm text-slate-100"
              />
            </div>

            <div>
              <label
                htmlFor="settings-confirm-password"
                className="block text-xs font-medium text-slate-300 mb-1"
              >
                Confirm New Password
              </label>
              <input
                id="settings-confirm-password"
                type={showPasswords ? 'text' : 'password'}
                autoComplete="new-password"
                value={confirmNewPassword}
                onChange={(e) => setConfirmNewPassword(e.target.value)}
                className="w-full px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-sm text-slate-100"
              />
            </div>
          </div>

          <div className="flex justify-end">
            <button
              type="submit"
              disabled={changingPassword}
              className="inline-flex items-center gap-2 px-4 py-2 rounded-xl text-sm font-semibold bg-slate-800 hover:bg-slate-700 text-slate-100 disabled:opacity-50"
            >
              {changingPassword ? (
                <Loader2 className="w-4 h-4 animate-spin" />
              ) : (
                <KeyRound className="w-4 h-4 text-sky-400" />
              )}
              Update Password
            </button>
          </div>

          <div className="pt-4 border-t border-slate-800 space-y-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div>
                <h4 className="text-sm font-semibold text-slate-200 flex items-center gap-2">
                  <Monitor className="w-4 h-4 text-emerald-400" />
                  Active Login Sessions ({sessions.length})
                </h4>
                <p className="text-xs text-slate-400">
                  Sessions currently authenticated with your administrator account.
                </p>
              </div>
              <button
                type="button"
                disabled={!sessions.some((s) => !s.current)}
                onClick={() => setConfirmRevokeOthers(true)}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-rose-500/15 hover:bg-rose-500/25 text-rose-300 border border-rose-500/30 disabled:opacity-40"
              >
                <LogOut className="w-3.5 h-3.5" />
                Sign Out Other Devices
              </button>
            </div>

            {loadingSessions && sessions.length === 0 ? (
              <div className="text-xs text-slate-500">Loading active sessions...</div>
            ) : sessions.length === 0 ? (
              <div className="text-xs text-slate-500">No active sessions found.</div>
            ) : (
              <div className="divide-y divide-slate-800/80 rounded-xl bg-slate-950 border border-slate-800 overflow-hidden">
                {sessions.map((s) => (
                  <div
                    key={s.id}
                    className="px-3.5 py-2.5 flex flex-wrap items-center justify-between gap-2 text-xs"
                  >
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-slate-200">Session {s.id}</span>
                      {s.current && (
                        <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-emerald-500/15 text-emerald-300 border border-emerald-500/30">
                          Current Device
                        </span>
                      )}
                    </div>
                    <div className="text-[11px] text-slate-400 tabular-nums">
                      Signed in: {formatDateTime(s.created_at)} · Expires:{' '}
                      {formatDateTime(s.expires_at)}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </form>
      )}

      {confirmRevokeOthers && (
        <ConfirmModal
          title="Sign Out Other Devices"
          message="Revoke all other active login sessions for your account? This browser will remain signed in."
          confirmLabel="Sign Out Others"
          busy={revokingOthers}
          onCancel={() => setConfirmRevokeOthers(false)}
          onConfirm={() => void handleRevokeOtherSessions()}
        />
      )}

      {activeTab === 'backup' && systemInfo && (
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
