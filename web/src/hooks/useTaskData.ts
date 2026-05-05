import { useCallback, useEffect, useState } from 'react';
import type { Data, ServerState, Task, TaskStatus } from '../types';
import { emptyData, normalizeData, resetIfNeeded, nextOrder } from '../services/data';
import { fetchServerState, saveServerData, setupLocalBackend, syncServerData } from '../services/api';

export function useTaskData() {
  const [data, setData] = useState<Data>(emptyData());
  const [serverState, setServerState] = useState<ServerState | null>(null);
  const [statusMsg, setStatusMsg] = useState('');
  const [syncing, setSyncing] = useState(false);
  const [refreshing, setRefreshing] = useState(false);

  const loadState = useCallback(async (successMessage?: string) => {
    try {
      const state = await fetchServerState();
      setData(resetIfNeeded(normalizeData(state.data)));
      setServerState(state);
      setStatusMsg(
        successMessage || (
          state.backend === 'nextcloud'
            ? 'Connected to the local Daily Tasks server.'
            : 'Using local-only backend.'
        ),
      );
    } catch (err) {
      setStatusMsg(`Load error: ${(err as Error).message}`);
    }
  }, []);

  useEffect(() => {
    void loadState();
  }, [loadState]);

  const persistData = useCallback(async (next: Data) => {
    try {
      const state = await saveServerData(next);
      setData(resetIfNeeded(normalizeData(state.data)));
      setServerState(state);
      setStatusMsg(state.message || 'Saved locally.');
    } catch (err) {
      setStatusMsg(`Save error: ${(err as Error).message}`);
    }
  }, []);

  const updateData = useCallback((updater: (prev: Data) => Data) => {
    setData((prev) => {
      const next = resetIfNeeded(normalizeData(updater(prev)));
      void persistData(next);
      return next;
    });
  }, [persistData]);

  const syncFromRemote = useCallback(async () => {
    setSyncing(true);
    try {
      const state = await syncServerData();
      setData(resetIfNeeded(normalizeData(state.data)));
      setServerState(state);
      setStatusMsg(state.message || 'Sync complete.');
    } catch (err) {
      setStatusMsg(`Sync error: ${(err as Error).message}`);
    } finally {
      setSyncing(false);
    }
  }, []);

  const reloadFromDisk = useCallback(async () => {
    setRefreshing(true);
    try {
      await loadState('Reloaded local data.');
    } finally {
      setRefreshing(false);
    }
  }, [loadState]);

  const chooseLocalBackend = useCallback(async () => {
    await setupLocalBackend();
    await loadState('Using local-only backend.');
  }, [loadState]);

  const addTask = useCallback((title: string, duration: number, deadline?: string, visibility?: number[]) => {
    updateData((prev) => {
      const id = prev.next_id || 1;
      const order = nextOrder(prev, 'todo');
      return {
        ...prev,
        next_id: id + 1,
        tasks: [...prev.tasks, { id, title, duration, status: 'todo' as const, order, deadline, visibility }],
      };
    });
  }, [updateData]);

  const editTask = useCallback((id: number, title: string, duration: number, deadline?: string, visibility?: number[]) => {
    updateData((prev) => ({
      ...prev,
      tasks: prev.tasks.map((task) => task.id === id ? { ...task, title, duration, deadline, visibility } : task),
    }));
  }, [updateData]);

  const deleteTask = useCallback((id: number) => {
    updateData((prev) => ({ ...prev, tasks: prev.tasks.filter((t) => t.id !== id) }));
  }, [updateData]);

  const toggleTaskStatus = useCallback((task: Task) => {
    updateData((prev) => {
      const newStatus: TaskStatus = task.status === 'todo' ? 'done' : 'todo';
      const order = nextOrder(prev, newStatus);
      return {
        ...prev,
        tasks: prev.tasks.map((t) => t.id === task.id ? { ...t, status: newStatus, order } : t),
      };
    });
  }, [updateData]);

  const skipTask = useCallback((id: number) => {
    updateData((prev) => {
      const task = prev.tasks.find(t => t.id === id);
      if (!task || task.status !== 'todo') return prev;
      const order = nextOrder(prev, 'skipped');
      return {
        ...prev,
        tasks: prev.tasks.map((t) => t.id === id ? { ...t, status: 'skipped' as const, order } : t),
      };
    });
  }, [updateData]);

  const cycleTheme = useCallback(() => {
    updateData((prev) => ({ ...prev, theme_index: (prev.theme_index + 1) % 25 }));
  }, [updateData]);

  return {
    data,
    serverState,
    statusMsg,
    syncing,
    refreshing,
    refreshServerState: loadState,
    reloadFromDisk,
    syncFromRemote,
    addTask,
    editTask,
    deleteTask,
    toggleTaskStatus,
    skipTask,
    cycleTheme,
    chooseLocalBackend,
  };
}
