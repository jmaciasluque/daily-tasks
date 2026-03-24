import { useCallback, useEffect, useState } from 'react';
import type { Data, Settings, Task, TaskStatus } from '../types';
import { emptyData, normalizeData, resetIfNeeded, nextOrder, orderedTasks } from '../services/data';
import { loadSettings, saveSettings, loadCachedData, saveCachedData } from '../services/storage';
import { isSettingsComplete, pushRemoteData, syncWithRemote, defaultSettings } from '../services/webdav';
import { rescheduleAllNotifications } from '../services/notifications';
import { appVariant } from '../config/env';

export function useTaskData() {
  const [data, setData] = useState<Data>(emptyData());
  const [settings, setSettingsState] = useState<Settings>(defaultSettings);
  const [statusMsg, setStatusMsg] = useState('');
  const [syncing, setSyncing] = useState(false);
  const [initialized, setInitialized] = useState(false);

  // Load settings and cached data on mount
  useEffect(() => {
    (async () => {
      const loadedSettings = await loadSettings();
      setSettingsState(loadedSettings);

      const cached = await loadCachedData();
      setData(resetIfNeeded(cached));
      setInitialized(true);

      if (appVariant !== 'production') {
        setStatusMsg('Testing build: syncing with .daily-tasks-test.json');
      }
    })();
  }, [appVariant]);

  // Save data to cache and reschedule notifications whenever data changes
  useEffect(() => {
    if (initialized) {
      saveCachedData(data);
      rescheduleAllNotifications(data.tasks);
    }
  }, [data, initialized]);

  // Sync when settings are complete and initialized
  useEffect(() => {
    if (initialized && isSettingsComplete(settings)) {
      syncFromRemote();
    }
  }, [initialized, settings.baseUrl, settings.username, settings.password, settings.remotePath]);

  const syncFromRemote = useCallback(async (overrideSettings?: Settings) => {
    const activeSettings = overrideSettings ?? settings;
    if (!isSettingsComplete(activeSettings)) {
      setStatusMsg('Configure WebDAV settings first.');
      return;
    }

    setSyncing(true);
    try {
      const result = await syncWithRemote(activeSettings, data);
      if (result.action !== 'error') {
        const normalized = resetIfNeeded(normalizeData(result.data));
        setData(normalized);
      }
      setStatusMsg(result.message);
    } catch (err) {
      setStatusMsg(`Sync error: ${(err as Error).message}`);
    } finally {
      setSyncing(false);
    }
  }, [settings, data]);

  const pushToRemote = useCallback(async (dataToSave: Data) => {
    if (!isSettingsComplete(settings)) {
      return;
    }
    try {
      await pushRemoteData(settings, dataToSave);
      setStatusMsg('Saved to Nextcloud.');
    } catch (err) {
      setStatusMsg(`Save error: ${(err as Error).message}`);
    }
  }, [settings]);

  const updateData = useCallback((updater: (prev: Data) => Data) => {
    setData((prev) => {
      const next = resetIfNeeded(normalizeData(updater(prev)));
      next.last_modified = Date.now();
      pushToRemote(next);
      return next;
    });
  }, [pushToRemote]);

  /**
   * Reload task data from the local cache.
   * Call this after a notification action has updated storage in the background.
   */
  const reloadFromCache = useCallback(async () => {
    const cached = await loadCachedData();
    setData(resetIfNeeded(cached));
  }, []);

  const addTask = useCallback((title: string, duration: number, deadline?: string) => {
    updateData((prev) => {
      const id = prev.next_id || 1;
      const order = nextOrder(prev, 'todo');
      return {
        ...prev,
        next_id: id + 1,
        tasks: [
          ...prev.tasks,
          { id, title, duration, status: 'todo' as const, order, deadline },
        ],
      };
    });
  }, [updateData]);

  const editTask = useCallback((id: number, title: string, duration: number, deadline?: string) => {
    updateData((prev) => ({
      ...prev,
      tasks: prev.tasks.map((task) =>
        task.id === id ? { ...task, title, duration, deadline } : task
      ),
    }));
  }, [updateData]);

  const deleteTask = useCallback((id: number) => {
    updateData((prev) => ({
      ...prev,
      tasks: prev.tasks.filter((t) => t.id !== id),
    }));
  }, [updateData]);

  const toggleTaskStatus = useCallback((task: Task) => {
    updateData((prev) => {
      // todo → done, done → todo, skipped → todo
      const newStatus: TaskStatus = task.status === 'todo' ? 'done' : 'todo';
      const order = nextOrder(prev, newStatus);
      return {
        ...prev,
        tasks: prev.tasks.map((t) =>
          t.id === task.id ? { ...t, status: newStatus, order } : t
        ),
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
        tasks: prev.tasks.map((t) =>
          t.id === id ? { ...t, status: 'skipped' as const, order } : t
        ),
      };
    });
  }, [updateData]);

  const moveTask = useCallback((task: Task, delta: number) => {
    updateData((prev) => {
      const list = orderedTasks(prev, task.status);
      const idx = list.findIndex((t) => t.id === task.id);
      const swapIdx = idx + delta;
      if (idx < 0 || swapIdx < 0 || swapIdx >= list.length) {
        return prev;
      }
      const current = list[idx];
      const target = list[swapIdx];
      return {
        ...prev,
        tasks: prev.tasks.map((t) => {
          if (t.id === current.id) return { ...t, order: target.order };
          if (t.id === target.id) return { ...t, order: current.order };
          return t;
        }),
      };
    });
  }, [updateData]);

  const moveTaskToTop = useCallback((task: Task) => {
    updateData((prev) => {
      const list = orderedTasks(prev, task.status);
      if (list.length === 0) {
        return prev;
      }

      const orderedIds = [
        task.id,
        ...list.filter((t) => t.id !== task.id).map((t) => t.id),
      ];

      return {
        ...prev,
        tasks: prev.tasks.map((t) => {
          if (t.status !== task.status) {
            return t;
          }
          const idx = orderedIds.indexOf(t.id);
          if (idx === -1) {
            return t;
          }
          return { ...t, order: idx + 1 };
        }),
      };
    });
  }, [updateData]);

  const cycleTheme = useCallback(() => {
    updateData((prev) => ({
      ...prev,
      theme_index: (prev.theme_index + 1) % 25,
    }));
  }, [updateData]);

  const updateSettings = useCallback(async (newSettings: Settings) => {
    setSettingsState(newSettings);
    await saveSettings(newSettings);
    // Force a sync right after updating settings to pull remote before any local push
    if (isSettingsComplete(newSettings)) {
      await syncFromRemote(newSettings);
    }
  }, [syncFromRemote]);

  return {
    data,
    settings,
    statusMsg,
    syncing,
    syncFromRemote,
    reloadFromCache,
    addTask,
    editTask,
    deleteTask,
    toggleTaskStatus,
    skipTask,
    moveTask,
    moveTaskToTop,
    cycleTheme,
    updateSettings,
  };
}
