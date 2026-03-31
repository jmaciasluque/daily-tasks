import { useCallback, useEffect, useMemo, useState } from 'react';
import type { AppConfig, Data, Settings, Task, TaskStatus } from '../types';
import { emptyData, normalizeData, resetIfNeeded, nextOrder } from '../services/data';
import { loadAppConfig, saveAppConfig, loadCachedData, saveCachedData, nextcloudSettingsFromConfig } from '../services/storage';
import { isSettingsComplete, pushRemoteData, syncWithRemote } from '../services/webdav';
import { rescheduleAllNotifications } from '../services/notifications';
import { appVariant } from '../config/env';

export function useTaskData() {
  const [data, setData] = useState<Data>(emptyData());
  const [config, setConfigState] = useState<AppConfig>({});
  const [statusMsg, setStatusMsg] = useState('');
  const [syncing, setSyncing] = useState(false);
  const [initialized, setInitialized] = useState(false);

  const settings = useMemo(() => nextcloudSettingsFromConfig(config), [config]);
  const nextcloudConfigured = config.backend === 'nextcloud' && isSettingsComplete(settings);

  useEffect(() => {
    (async () => {
      const loadedConfig = await loadAppConfig();
      setConfigState(loadedConfig);

      const cached = await loadCachedData();
      setData(resetIfNeeded(cached));
      setInitialized(true);

      if (appVariant !== 'production') {
        setStatusMsg('Testing build: syncing with .daily-tasks-test.json');
      }
    })();
  }, [appVariant]);

  useEffect(() => {
    if (initialized) {
      saveCachedData(data);
      rescheduleAllNotifications(data.tasks);
    }
  }, [data, initialized]);

  const syncFromRemote = useCallback(async (overrideSettings?: Settings) => {
    const activeSettings = overrideSettings ?? settings;
    if (config.backend !== 'nextcloud' || !isSettingsComplete(activeSettings)) {
      setStatusMsg(config.backend === 'local' ? 'Using local-only backend.' : 'Connect Nextcloud first.');
      return;
    }

    setSyncing(true);
    try {
      const latestLocal = await loadCachedData();
      const result = await syncWithRemote(activeSettings, latestLocal);
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
  }, [config.backend, settings]);

  useEffect(() => {
    if (initialized && nextcloudConfigured) {
      syncFromRemote();
    }
  }, [initialized, nextcloudConfigured, settings.baseUrl, settings.username, settings.password, settings.remotePath, syncFromRemote]);

  const pushToRemote = useCallback(async (dataToSave: Data) => {
    if (!nextcloudConfigured) {
      return;
    }

    try {
      await pushRemoteData(settings, dataToSave);
      setStatusMsg('Saved to Nextcloud.');
    } catch (err) {
      setStatusMsg(`Save error: ${(err as Error).message}`);
    }
  }, [nextcloudConfigured, settings]);

  const updateData = useCallback((updater: (prev: Data) => Data) => {
    setData((prev) => {
      const next = resetIfNeeded(normalizeData(updater(prev)));
      next.last_modified = Date.now();
      pushToRemote(next);
      return next;
    });
  }, [pushToRemote]);

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

  const reorderTasks = useCallback((reorderedTodos: Task[]) => {
    updateData((prev) => ({
      ...prev,
      tasks: prev.tasks.map((t) => {
        if (t.status !== 'todo') return t;
        const idx = reorderedTodos.findIndex((rt) => rt.id === t.id);
        return idx === -1 ? t : { ...t, order: idx + 1 };
      }),
    }));
  }, [updateData]);

  const cycleTheme = useCallback(() => {
    updateData((prev) => ({
      ...prev,
      theme_index: (prev.theme_index + 1) % 25,
    }));
  }, [updateData]);

  const chooseLocalBackend = useCallback(async () => {
    const nextConfig: AppConfig = { backend: 'local' };
    setConfigState(nextConfig);
    await saveAppConfig(nextConfig);
    setStatusMsg('Using local-only backend.');
  }, []);

  const saveNextcloudSettings = useCallback(async (nextcloud: Settings) => {
    const nextConfig: AppConfig = {
      backend: 'nextcloud',
      nextcloud,
    };
    setConfigState(nextConfig);
    await saveAppConfig(nextConfig);
    if (isSettingsComplete(nextcloud)) {
      await syncFromRemote(nextcloud);
    }
  }, [syncFromRemote]);

  return {
    data,
    config,
    settings,
    backendConfigured: !!config.backend,
    nextcloudConfigured,
    statusMsg,
    syncing,
    syncFromRemote,
    reloadFromCache,
    addTask,
    editTask,
    deleteTask,
    toggleTaskStatus,
    skipTask,
    reorderTasks,
    cycleTheme,
    chooseLocalBackend,
    saveNextcloudSettings,
  };
}
