import { useCallback, useEffect, useState } from 'react';
import type { Data, Settings, Task } from '../types';
import { emptyData, normalizeData, resetIfNeeded, nextOrder, orderedTasks } from '../services/data';
import { loadSettings, saveSettings, loadCachedData, saveCachedData } from '../services/storage';
import { isSettingsComplete, pushRemoteData, syncWithRemote, defaultSettings } from '../services/webdav';

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
    })();
  }, []);

  // Save data to cache whenever it changes
  useEffect(() => {
    if (initialized) {
      saveCachedData(data);
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

  const addTask = useCallback((title: string, duration: number) => {
    updateData((prev) => {
      const id = prev.next_id || 1;
      const order = nextOrder(prev, 'todo');
      return {
        ...prev,
        next_id: id + 1,
        tasks: [
          ...prev.tasks,
          { id, title, duration, status: 'todo' as const, order },
        ],
      };
    });
  }, [updateData]);

  const editTask = useCallback((id: number, title: string, duration: number) => {
    updateData((prev) => ({
      ...prev,
      tasks: prev.tasks.map((task) =>
        task.id === id ? { ...task, title, duration } : task
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
      const newStatus = task.status === 'todo' ? 'done' : 'todo';
      const order = nextOrder(prev, newStatus);
      return {
        ...prev,
        tasks: prev.tasks.map((t) =>
          t.id === task.id ? { ...t, status: newStatus, order } : t
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
    addTask,
    editTask,
    deleteTask,
    toggleTaskStatus,
    moveTask,
    cycleTheme,
    updateSettings,
  };
}
