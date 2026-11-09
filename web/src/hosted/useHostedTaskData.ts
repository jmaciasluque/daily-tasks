import { useCallback, useEffect, useRef, useState } from 'react';
import { HostedAuthError, HostedBackend, refreshTokenIfStale, setHostedClientName } from '@mobile/services/backend_hosted';
import { syncWithRemoteState } from '@mobile/services/sync';
import {
  applyDailyResetWithHistory,
  buildStatsSummary,
  emptyHistory,
  normalizeHistory,
  recordDataChange,
} from '@mobile/services/history';
import type { Data as SharedData, History } from '@mobile/types';
import { emptyData, nextOrder, normalizeData, resetIfNeeded } from '../services/data';
import type { Data, ServerState, StatsSummary, Task, TaskStatus } from '../types';
import { appVersion } from '../config/env';
import type { TaskDataApi } from '../hooks/types';
import { apiOrigin, consumeLoginRedirect, loadSession, saveSession, signInUrl, type Session } from './session';

setHostedClientName(`daily-tasks-web/${appVersion}`);

const CACHE_KEY = 'daily-tasks.cache';

type Cache = { data: SharedData; history: History };

function loadCache(): Cache {
  try {
    const raw = window.localStorage.getItem(CACHE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw) as Cache;
      return { data: normalizeData(parsed.data) as SharedData, history: normalizeHistory(parsed.history) };
    }
  } catch {
    // ignore corrupt cache
  }
  return { data: emptyData() as SharedData, history: emptyHistory() };
}

function saveCache(cache: Cache | null): void {
  try {
    if (cache) {
      window.localStorage.setItem(CACHE_KEY, JSON.stringify(cache));
    } else {
      window.localStorage.removeItem(CACHE_KEY);
    }
  } catch {
    // storage unavailable
  }
}

export function useHostedTaskData(): TaskDataApi {
  const [session, setSession] = useState<Session | null>(() => consumeLoginRedirect() ?? loadSession());
  const initial = useRef(loadCache());
  const [data, setData] = useState<Data>(() => applyDailyResetWithHistory(initial.current.data, initial.current.history).data);
  const dataRef = useRef<SharedData>(initial.current.data);
  const historyRef = useRef<History>(initial.current.history);
  const [statusMsg, setStatusMsg] = useState('');
  const [syncing, setSyncing] = useState(false);
  const queue = useRef<Promise<unknown>>(Promise.resolve());

  const backend = useCallback(() => (session ? new HostedBackend({ apiUrl: apiOrigin(), token: session.token }) : null), [session]);

  const apply = useCallback((nextData: SharedData, nextHistory: History) => {
    const reset = applyDailyResetWithHistory(nextData, nextHistory);
    dataRef.current = reset.data;
    historyRef.current = reset.history;
    setData(reset.data as Data);
    saveCache({ data: reset.data, history: reset.history });
  }, []);

  const signOut = useCallback(() => {
    saveSession(null);
    saveCache(null);
    dataRef.current = emptyData() as SharedData;
    historyRef.current = emptyHistory();
    setData(emptyData());
    setSession(null);
  }, []);

  // Serialises every network round trip so edits never race each other.
  const enqueue = useCallback(<T,>(job: () => Promise<T>): Promise<T> => {
    const run = queue.current.then(job, job);
    queue.current = run.catch(() => undefined);
    return run;
  }, []);

  const sync = useCallback(() => enqueue(async () => {
    const b = backend();
    if (!b) {
      return;
    }
    setSyncing(true);
    try {
      const result = await syncWithRemoteState(b, dataRef.current, historyRef.current);
      if (result.action === 'error') {
        setStatusMsg(result.message);
        return;
      }
      apply(result.data, result.history);
      setStatusMsg(result.action === 'in_sync' ? 'Up to date.' : result.message);
    } finally {
      setSyncing(false);
    }
  }), [apply, backend, enqueue]);

  // On sign-in / page load: validate the session, renew it if old, then sync.
  useEffect(() => {
    const b = backend();
    if (!b || !session) {
      return;
    }
    let cancelled = false;
    (async () => {
      try {
        const account = await b.getAccount();
        if (cancelled) return;
        const renewed = await refreshTokenIfStale(b);
        if (renewed) {
          const next = { token: renewed, email: account.email || session.email };
          saveSession(next);
          setSession(next);
          return; // the effect re-runs with the new token
        }
        if (account.email && account.email !== session.email) {
          const next = { ...session, email: account.email };
          saveSession(next);
          setSession(next);
          return;
        }
        await sync();
      } catch (err) {
        if (err instanceof HostedAuthError) {
          signOut();
          setStatusMsg('Your session expired. Sign in again.');
        } else {
          setStatusMsg((err as Error).message);
        }
      }
    })();
    return () => { cancelled = true; };
  }, [backend, session, signOut, sync]);

  // Pick up changes from other devices when the tab regains focus.
  useEffect(() => {
    const onVisible = () => {
      if (document.visibilityState === 'visible') {
        void sync();
      }
    };
    document.addEventListener('visibilitychange', onVisible);
    return () => document.removeEventListener('visibilitychange', onVisible);
  }, [sync]);

  const updateData = useCallback((updater: (prev: Data) => Data) => {
    const before = dataRef.current;
    const next = resetIfNeeded(normalizeData(updater(before as Data))) as SharedData;
    next.last_modified = Date.now();
    const nextHistory = recordDataChange(historyRef.current, before, next, next.last_modified);
    apply(next, nextHistory);
    void sync();
  }, [apply, sync]);

  const addTask = useCallback((title: string, duration: number, deadline?: string, visibility?: number[]) => {
    updateData((prev) => {
      const id = prev.next_id || 1;
      return {
        ...prev,
        next_id: id + 1,
        tasks: [...prev.tasks, { id, title, duration, status: 'todo' as const, order: nextOrder(prev, 'todo'), deadline, visibility }],
      };
    });
  }, [updateData]);

  const editTask = useCallback((id: number, title: string, duration: number, deadline?: string, visibility?: number[]) => {
    updateData((prev) => ({
      ...prev,
      tasks: prev.tasks.map((t) => (t.id === id ? { ...t, title, duration, deadline, visibility } : t)),
    }));
  }, [updateData]);

  const deleteTask = useCallback((id: number) => {
    updateData((prev) => ({ ...prev, tasks: prev.tasks.filter((t) => t.id !== id) }));
  }, [updateData]);

  const toggleTaskStatus = useCallback((task: Task) => {
    updateData((prev) => {
      const status: TaskStatus = task.status === 'todo' ? 'done' : 'todo';
      const order = nextOrder(prev, status);
      return { ...prev, tasks: prev.tasks.map((t) => (t.id === task.id ? { ...t, status, order } : t)) };
    });
  }, [updateData]);

  const skipTask = useCallback((id: number) => {
    updateData((prev) => {
      const task = prev.tasks.find((t) => t.id === id);
      if (!task || task.status !== 'todo') return prev;
      const order = nextOrder(prev, 'skipped');
      return { ...prev, tasks: prev.tasks.map((t) => (t.id === id ? { ...t, status: 'skipped' as const, order } : t)) };
    });
  }, [updateData]);

  const cycleTheme = useCallback(() => {
    updateData((prev) => ({ ...prev, theme_index: (prev.theme_index + 1) % 25 }));
  }, [updateData]);

  const loadStats = useCallback(async (from: string, to: string): Promise<StatsSummary> => (
    buildStatsSummary(historyRef.current, dataRef.current, from, to) as StatsSummary
  ), []);

  const deleteAccount = useCallback(async () => {
    const b = backend();
    if (!b) return;
    await b.deleteAccount();
    signOut();
    setStatusMsg('Your account and synced data were deleted.');
  }, [backend, signOut]);

  const serverState: ServerState = {
    data,
    data_path: '',
    sync_configured: !!session,
    version: appVersion,
  };

  return {
    data,
    serverState,
    statusMsg,
    syncing,
    refreshing: false,
    refreshServerState: async () => { await sync(); },
    reloadFromDisk: async () => { await sync(); },
    syncFromRemote: async () => { await sync(); },
    addTask,
    editTask,
    deleteTask,
    toggleTaskStatus,
    skipTask,
    cycleTheme,
    chooseLocalBackend: async () => signOut(),
    loadStats,
    hosted: {
      signedIn: !!session,
      email: session?.email,
      signInUrl: signInUrl(),
      signOut,
      deleteAccount,
    },
  };
}
