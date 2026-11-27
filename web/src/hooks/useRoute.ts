import { useCallback, useEffect, useState } from 'react';

export type Tab = 'dashboard' | 'jobs' | 'remotes' | 'browse' | 'history' | 'settings';

export interface RouteState {
  tab: Tab;
  browseRemote?: string;
}

const PATH_TO_TAB: Record<string, Tab> = {
  '/': 'dashboard',
  '/dashboard': 'dashboard',
  '/jobs': 'jobs',
  '/remotes': 'remotes',
  '/browse': 'browse',
  '/history': 'history',
  '/settings': 'settings',
};

const TAB_TO_PATH: Record<Tab, string> = {
  dashboard: '/',
  jobs: '/jobs',
  remotes: '/remotes',
  browse: '/browse',
  history: '/history',
  settings: '/settings',
};

function parseLocation(): RouteState {
  const tab = PATH_TO_TAB[window.location.pathname] ?? 'dashboard';
  const params = new URLSearchParams(window.location.search);
  const browseRemote = params.get('remote') || undefined;
  return { tab, browseRemote };
}

export function useRoute() {
  const [route, setRoute] = useState<RouteState>(parseLocation);

  useEffect(() => {
    const onPopState = () => setRoute(parseLocation());
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener('popstate', onPopState);
  }, []);

  const navigate = useCallback((tab: Tab, opts?: { remote?: string }) => {
    const base = TAB_TO_PATH[tab] ?? '/';
    const url =
      tab === 'browse' && opts?.remote ? `${base}?remote=${encodeURIComponent(opts.remote)}` : base;
    if (window.location.pathname + window.location.search !== url) {
      window.history.pushState(null, '', url);
    }
    setRoute({ tab, browseRemote: opts?.remote });
  }, []);

  return { tab: route.tab, browseRemote: route.browseRemote, navigate };
}
