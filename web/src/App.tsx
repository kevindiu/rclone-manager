import {
  Activity,
  Cloud,
  FolderOpen,
  History,
  LayoutDashboard,
  ListChecks,
  Loader2,
  LogOut,
  Settings,
} from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';
import { api, setUnauthorizedHandler } from './api/client';
import { LoginPage } from './features/auth/LoginPage';
import { BrowsePage } from './features/browse/BrowsePage';
import { DashboardPage } from './features/dashboard/DashboardPage';
import { HistoryPage } from './features/history/HistoryPage';
import { LogViewerModal } from './features/history/LogViewerModal';
import { JobsPage } from './features/jobs/JobsPage';
import { RemotesPage } from './features/remotes/RemotesPage';
import { SettingsPage } from './features/settings/SettingsPage';
import { type Tab, useRoute } from './hooks/useRoute';
import { useSSE } from './hooks/useSSE';
import { useToast } from './hooks/useToast';
import type {
  AuthUser,
  GlobalSettings,
  JobRun,
  JobWithStatus,
  ProviderInfo,
  RemoteInfo,
  SystemInfo,
} from './types/api';

const DEFAULT_SETTINGS: GlobalSettings = {
  transfers: 4,
  checkers: 8,
  bwlimit: '',
  retries: 3,
  low_level_retries: 10,
  timeout: '5m',
  contimeout: '1m',
  log_retention_days: 30,
  timezone: 'UTC',
  max_concurrent_jobs: 2,
  webhook_url: '',
  webhook_on: 'failure',
};

export function App() {
  const { tab, browseRemote, navigate } = useRoute();
  const { toast, showToast } = useToast();

  const [authLoading, setAuthLoading] = useState(true);
  const [setupRequired, setSetupRequired] = useState(false);
  const [currentUser, setCurrentUser] = useState<AuthUser | null>(null);
  const authenticated = currentUser !== null;

  const [systemInfo, setSystemInfo] = useState<SystemInfo | null>(null);
  const [settings, setSettings] = useState<GlobalSettings>(DEFAULT_SETTINGS);
  const [jobs, setJobs] = useState<JobWithStatus[]>([]);
  const [remotes, setRemotes] = useState<RemoteInfo[]>([]);
  const [providers, setProviders] = useState<ProviderInfo[]>([]);
  const [recentRuns, setRecentRuns] = useState<JobRun[]>([]);
  const [historySignal, setHistorySignal] = useState(0);
  const [logRun, setLogRun] = useState<JobRun | null>(null);

  const clearSessionState = useCallback(() => {
    setCurrentUser(null);
    setSystemInfo(null);
    setJobs([]);
    setRemotes([]);
    setRecentRuns([]);
    setLogRun(null);
  }, []);

  useEffect(() => {
    setUnauthorizedHandler(() => {
      clearSessionState();
    });
    return () => setUnauthorizedHandler(null);
  }, [clearSessionState]);

  useEffect(() => {
    let active = true;
    api
      .authStatus()
      .then((status) => {
        if (!active) return;
        setSetupRequired(status.setup_required);
        setCurrentUser(status.authenticated && status.user ? status.user : null);
      })
      .catch(() => {
        if (!active) return;
        setCurrentUser(null);
      })
      .finally(() => {
        if (active) setAuthLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);

  const refreshCore = useCallback(async () => {
    if (!authenticated) return;
    try {
      const [sys, sett, jobList, remList, runRes] = await Promise.all([
        api.getSystemInfo(),
        api.getSettings(),
        api.listJobs(),
        api.listRemotes(),
        api.listRuns({ limit: 10 }),
      ]);
      setSystemInfo(sys);
      setSettings(sett);
      setJobs(jobList);
      setRemotes(remList);
      setRecentRuns(runRes.runs ?? []);
    } catch {
      // handled silently on background refresh (401 clears session via unauthorizedHandler)
    }
  }, [authenticated]);

  useEffect(() => {
    if (!authenticated) return;
    void refreshCore();
    api
      .listProviders()
      .then(setProviders)
      .catch(() => {});
  }, [authenticated, refreshCore]);

  const handleSSEError = useCallback(() => {
    api
      .authStatus()
      .then((st) => {
        if (!st.authenticated) {
          setSetupRequired(st.setup_required);
          clearSessionState();
        }
      })
      .catch(() => {});
  }, [clearSessionState]);

  const { activeRuns, connected: sseConnected } = useSSE({
    enabled: authenticated,
    onRunStarted: () => void refreshCore(),
    onRunCompleted: () => {
      setHistorySignal((n) => n + 1);
      void refreshCore();
    },
    onJobUpdated: () => void refreshCore(),
    onError: handleSSEError,
  });

  const handleLogout = async () => {
    try {
      await api.authLogout();
    } catch {
      // clear local state even if network failed
    }
    setSetupRequired(false);
    clearSessionState();
    showToast('Signed out');
  };

  if (authLoading) {
    return (
      <div className="min-h-dvh bg-slate-950 text-slate-300 flex items-center justify-center gap-2 text-sm">
        <Loader2 className="w-5 h-5 animate-spin text-sky-400" />
        <span>Checking authentication...</span>
      </div>
    );
  }

  if (!authenticated) {
    return (
      <LoginPage
        setupRequired={setupRequired}
        onAuthenticated={(user) => {
          setSetupRequired(false);
          setCurrentUser(user);
        }}
      />
    );
  }

  const navItems: { key: Tab; label: string; icon: typeof LayoutDashboard }[] = [
    { key: 'dashboard', label: 'Dashboard', icon: LayoutDashboard },
    { key: 'jobs', label: 'Jobs', icon: ListChecks },
    { key: 'remotes', label: 'Remotes', icon: Cloud },
    { key: 'browse', label: 'Browse', icon: FolderOpen },
    { key: 'history', label: 'History', icon: History },
    { key: 'settings', label: 'Settings', icon: Settings },
  ];

  return (
    <div className="min-h-dvh bg-slate-950 text-slate-100 flex flex-col md:flex-row">
      {/* Desktop Sidebar */}
      <aside className="hidden md:flex md:w-60 md:flex-col md:fixed md:inset-y-0 bg-slate-900/80 border-r border-slate-800">
        <div className="p-5 border-b border-slate-800 flex items-center gap-3">
          <img src="/icon.svg" alt="logo" className="w-8 h-8 rounded-lg" />
          <div className="min-w-0">
            <div className="font-bold text-sm tracking-tight text-slate-100 truncate">
              rclone-manager
            </div>
            <div className="text-[11px] font-mono text-slate-400 truncate">
              {systemInfo?.rclone_version || 'loading...'}
            </div>
          </div>
        </div>

        <nav className="flex-1 p-3 space-y-1">
          {navItems.map((item) => {
            const Icon = item.icon;
            const active = tab === item.key;
            return (
              <button
                key={item.key}
                type="button"
                onClick={() => navigate(item.key)}
                className={`w-full flex items-center gap-3 px-3.5 py-2.5 rounded-xl text-sm font-medium transition-colors ${
                  active
                    ? 'bg-sky-500/15 text-sky-300 border border-sky-500/30'
                    : 'text-slate-400 hover:text-slate-100 hover:bg-slate-800/60'
                }`}
              >
                <Icon className="w-4 h-4 shrink-0" />
                {item.label}
                {item.key === 'dashboard' && activeRuns.length > 0 && (
                  <span className="ml-auto px-1.5 py-0.5 rounded-full text-[10px] font-bold bg-sky-500 text-slate-950">
                    {activeRuns.length}
                  </span>
                )}
              </button>
            );
          })}
        </nav>

        <div className="p-4 border-t border-slate-800 space-y-2.5 text-xs text-slate-400">
          <div className="flex items-center justify-between">
            <span className="inline-flex items-center gap-1.5">
              <span
                className={`w-2 h-2 rounded-full ${
                  sseConnected ? 'bg-emerald-400' : 'bg-amber-400 animate-pulse'
                }`}
              />
              {sseConnected ? 'Live SSE' : 'Reconnecting'}
            </span>
            <span className="font-mono text-[11px]">{settings.timezone}</span>
          </div>
          <div className="flex items-center justify-between pt-2 border-t border-slate-800/80">
            <span className="truncate font-medium text-slate-300" title={currentUser.username}>
              {currentUser.username}
            </span>
            <button
              type="button"
              onClick={() => void handleLogout()}
              className="inline-flex items-center gap-1 text-slate-400 hover:text-rose-300 transition-colors"
              title="Sign out"
            >
              <LogOut className="w-3.5 h-3.5" />
              Sign out
            </button>
          </div>
        </div>
      </aside>

      {/* Mobile Top Header */}
      <header className="md:hidden sticky top-0 z-30 pt-safe bg-slate-900/95 backdrop-blur-md border-b border-slate-800 px-4 py-3 flex items-center justify-between">
        <div className="flex items-center gap-2.5">
          <img src="/icon.svg" alt="logo" className="w-7 h-7 rounded-lg" />
          <span className="font-bold text-sm text-slate-100">rclone-manager</span>
        </div>
        <div className="flex items-center gap-2.5 text-xs">
          {activeRuns.length > 0 && (
            <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full bg-sky-500/15 text-sky-300 border border-sky-500/30">
              <Activity className="w-3 h-3 animate-pulse" />
              {activeRuns.length}
            </span>
          )}
          <span
            className={`w-2 h-2 rounded-full ${
              sseConnected ? 'bg-emerald-400' : 'bg-amber-400 animate-pulse'
            }`}
            title={sseConnected ? 'Live SSE connected' : 'Reconnecting'}
          />
          <button
            type="button"
            onClick={() => void handleLogout()}
            aria-label="Sign out"
            title={`Sign out (${currentUser.username})`}
            className="p-1.5 rounded-lg text-slate-400 hover:text-rose-300 hover:bg-slate-800"
          >
            <LogOut className="w-4 h-4" />
          </button>
        </div>
      </header>

      {/* Main Content */}
      <main className="flex-1 md:pl-60 pb-24 md:pb-8">
        <div className="max-w-7xl mx-auto p-4 sm:p-6 lg:p-8">
          <div className="mb-5 hidden md:flex items-center justify-between">
            <h1 className="text-xl font-bold text-slate-100 capitalize">{tab}</h1>
          </div>

          {tab === 'dashboard' && (
            <DashboardPage
              activeRuns={activeRuns}
              jobs={jobs}
              remotes={remotes}
              recentRuns={recentRuns}
              systemInfo={systemInfo}
              onNavigate={navigate}
              onViewLogs={setLogRun}
              onRefreshAll={() => void refreshCore()}
            />
          )}
          {tab === 'jobs' && (
            <JobsPage
              jobs={jobs}
              settings={settings}
              remotes={remotes}
              onReload={() => void refreshCore()}
              onToast={showToast}
            />
          )}
          {tab === 'remotes' && (
            <RemotesPage
              remotes={remotes}
              providers={providers}
              onReload={() => void refreshCore()}
              onBrowseRemote={(name) => navigate('browse', { remote: name })}
              onToast={showToast}
            />
          )}
          {tab === 'browse' && (
            <BrowsePage
              remotes={remotes}
              jobs={jobs}
              activeRuns={activeRuns}
              initialRemote={browseRemote}
              onToast={showToast}
            />
          )}
          {tab === 'history' && (
            <HistoryPage
              jobs={jobs}
              refreshSignal={historySignal}
              onViewLogs={setLogRun}
              onToast={showToast}
            />
          )}
          {tab === 'settings' && (
            <SettingsPage
              settings={settings}
              systemInfo={systemInfo}
              currentUser={currentUser}
              onSaved={() => void refreshCore()}
              onToast={showToast}
            />
          )}
        </div>
      </main>

      {/* Mobile Bottom Nav */}
      <nav className="md:hidden fixed bottom-0 inset-x-0 z-30 bg-slate-900/95 backdrop-blur-md border-t border-slate-800 pb-safe">
        <div className="grid grid-cols-6">
          {navItems.map((item) => {
            const Icon = item.icon;
            const active = tab === item.key;
            return (
              <button
                key={item.key}
                type="button"
                onClick={() => navigate(item.key)}
                className={`py-2 flex flex-col items-center justify-center gap-0.5 text-[10px] font-medium ${
                  active ? 'text-sky-400' : 'text-slate-400'
                }`}
              >
                <Icon className="w-4 h-4" />
                <span>{item.label}</span>
              </button>
            );
          })}
        </div>
      </nav>

      {logRun && <LogViewerModal run={logRun} onClose={() => setLogRun(null)} />}

      {toast && (
        <div
          className={`fixed bottom-20 md:bottom-6 right-4 sm:right-6 z-50 px-4 py-2.5 rounded-xl shadow-xl border text-xs sm:text-sm font-medium ${
            toast.isError
              ? 'bg-rose-950/95 border-rose-500/40 text-rose-200'
              : 'bg-slate-900/95 border-emerald-500/40 text-emerald-200'
          }`}
        >
          {toast.msg}
        </div>
      )}
    </div>
  );
}

export default App;
