import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { AppConfig, Data, History, Settings, Task, TaskStatus } from '../types';
import { emptyData, normalizeData, resetIfNeeded, nextOrder } from '../services/data';
import { applyDailyResetWithHistory, emptyHistory, normalizeHistory, recordDataChange } from '../services/history';
import {
  loadAppConfig,
  saveAppConfig,
  loadCachedData,
  saveCachedData,
  loadCachedHistory,
  saveCachedHistory,
  nextcloudSettingsFromConfig,
} from '../services/storage';
import { isSettingsComplete, pushRemoteState, syncWithRemoteState } from '../services/webdav';
import { rescheduleAllNotifications } from '../services/notifications';
import { appVariant } from '../config/env';

export function useTaskData() {
  const [data, setData] = useState<Data>(emptyData());
  const [history, setHistory] = useState<History>(emptyHistory());
  const [config, setConfigState] = useState<AppConfig>({});
  const [statusMsg, setStatusMsg] = useState('');
  const [syncing, setSyncing] = useState(false);
  const [initialized, setInitialized] = useState(false);
  const dataRef = useRef(data);
  const historyRef = useRef(history);

  const settings = useMemo(() => nextcloudSettingsFromConfig(config), [config]);
  const nextcloudConfigured = config.backend === 'nextcloud' && isSettingsComplete(settings);

  useEffect(() => {
    dataRef.current = data;
  }, [data]);

  useEffect(() => {
    historyRef.current = history;
  }, [history]);

  useEffect(() => {
    (async () => {
      const loadedConfig = await loadAppConfig();
      setConfigState(loadedConfig);

      const cached = await loadCachedData();
      const cachedHistory = await loadCachedHistory();
      const normalizedHistory = normalizeHistory(cachedHistory);
      const resetResult = applyDailyResetWithHistory(cached, normalizedHistory);
      dataRef.current = resetResult.data;
      historyRef.current = resetResult.history;
      setData(resetResult.data);
      setHistory(resetResult.history);
      setInitialized(true);

      if (appVariant !== 'production') {
        setStatusMsg('Testing build: syncing with .daily-tasks-test.json');
      }
    })();
  }, [appVariant]);

  useEffect(() => {
    if (initialized) {
      saveCachedData(data);
      saveCachedHistory(history);
      rescheduleAllNotifications(data.tasks);
    }
  }, [data, history, initialized]);

  const syncFromRemote = useCallback(async (overrideSettings?: Settings) => {
    const activeSettings = overrideSettings ?? settings;
    if (config.backend !== 'nextcloud' || !isSettingsComplete(activeSettings)) {
      setStatusMsg(config.backend === 'local' ? 'Using local-only backend.' : 'Connect Nextcloud first.');
      return;
    }

    setSyncing(true);
    try {
      const latestLocal = await loadCachedData();
      const latestHistory = await loadCachedHistory();
      const localReset = applyDailyResetWithHistory(latestLocal, latestHistory);
      const result = await syncWithRemoteState(activeSettings, localReset.data, localReset.history);
      if (result.action !== 'error') {
        const synced = normalizeData(result.data);
        const syncedReset = applyDailyResetWithHistory(synced, result.history, Date.now());
        dataRef.current = syncedReset.data;
        historyRef.current = syncedReset.history;
        setHistory(syncedReset.history);
        setData(syncedReset.data);
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

  const pushToRemote = useCallback(async (dataToSave: Data, historyToSave: History) => {
    if (!nextcloudConfigured) {
      return;
    }

    try {
      await pushRemoteState(settings, dataToSave, historyToSave);
      setStatusMsg('Saved to Nextcloud.');
    } catch (err) {
      setStatusMsg(`Save error: ${(err as Error).message}`);
    }
  }, [nextcloudConfigured, settings]);

  const updateData = useCallback((updater: (prev: Data) => Data) => {
    const before = dataRef.current;
    const next = resetIfNeeded(normalizeData(updater(before)));
    next.last_modified = Date.now();
    const nextHistory = recordDataChange(historyRef.current, before, next, next.last_modified);
    dataRef.current = next;
    historyRef.current = nextHistory;
    setHistory(nextHistory);
    setData(next);
    pushToRemote(next, nextHistory);
  }, [pushToRemote]);

  const reloadFromCache = useCallback(async () => {
    const cached = await loadCachedData();
    const cachedHistory = await loadCachedHistory();
    const resetResult = applyDailyResetWithHistory(cached, cachedHistory);
    dataRef.current = resetResult.data;
    historyRef.current = resetResult.history;
    setHistory(resetResult.history);
    setData(resetResult.data);
  }, []);

  const addTask = useCallback((title: string, duration: number, deadline?: string, visibility?: number[]) => {
    updateData((prev) => {
      const id = prev.next_id || 1;
      const order = nextOrder(prev, 'todo');
      return {
        ...prev,
        next_id: id + 1,
        tasks: [
          ...prev.tasks,
          { id, title, duration, status: 'todo' as const, order, deadline, visibility },
        ],
      };
    });
  }, [updateData]);

  const editTask = useCallback((id: number, title: string, duration: number, deadline?: string, visibility?: number[]) => {
    updateData((prev) => ({
      ...prev,
      tasks: prev.tasks.map((task) =>
        task.id === id ? { ...task, title, duration, deadline, visibility } : task
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
    history,
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
