import * as Notifications from 'expo-notifications';
import AsyncStorage from '@react-native-async-storage/async-storage';
import type { Data } from '../types';
import {
  backendFromConfig,
  loadAppConfig,
  loadCachedData,
  loadCachedHistory,
  saveCachedData,
  saveCachedHistory,
} from './storage';
import { recordDataChange } from './history';
import { syncWithRemoteState } from './sync';

const NOTIFICATION_IDS_KEY = 'dailyTasksNotificationIds';
type NotificationIdMap = Record<number, string[]>;

function normalizeNotificationIdMap(input: unknown): NotificationIdMap {
  if (!input || typeof input !== 'object') {
    return {};
  }

  return Object.fromEntries(
    Object.entries(input).map(([taskId, ids]) => {
      if (Array.isArray(ids)) {
        return [taskId, ids.filter((id): id is string => typeof id === 'string')];
      }
      return [taskId, typeof ids === 'string' ? [ids] : []];
    }),
  );
}

function isActionableResponse(
  response: Notifications.NotificationResponse,
): response is Notifications.NotificationResponse & {
  actionIdentifier: 'skip' | 'done';
} {
  return response.actionIdentifier === 'skip' || response.actionIdentifier === 'done';
}

function isVisibleToday(task: Data['tasks'][number]): boolean {
  if (!task.visibility || task.visibility.length === 0) {
    return true;
  }
  return task.visibility.includes(new Date().getDay());
}

function applyNotificationAction(
  cached: Data,
  response: Notifications.NotificationResponse,
): Data | null {
  const taskId = response.notification.request.content.data?.taskId as number | undefined;

  if (!taskId || !isActionableResponse(response)) {
    return null;
  }

  const task = cached.tasks.find(t => t.id === taskId);
  if (!task || task.status !== 'todo' || !isVisibleToday(task)) {
    return null;
  }

  const newStatus = response.actionIdentifier === 'done' ? 'done' : 'skipped';
  let maxOrder = 0;
  cached.tasks.forEach(t => {
    if (t.status === newStatus) {
      maxOrder = Math.max(maxOrder, t.order);
    }
  });

  return {
    ...cached,
    tasks: cached.tasks.map(t =>
      t.id === taskId ? { ...t, status: newStatus, order: maxOrder + 1 } : t
    ),
    last_modified: Date.now(),
  };
}

function taskIdFromResponse(response: Notifications.NotificationResponse): number | undefined {
  return response.notification.request.content.data?.taskId as number | undefined;
}

function isStaleActionableNotification(cached: Data, response: Notifications.NotificationResponse): boolean {
  const taskId = taskIdFromResponse(response);
  if (!taskId || !isActionableResponse(response)) {
    return false;
  }

  const task = cached.tasks.find(t => t.id === taskId);
  return !!task && (task.status !== 'todo' || !isVisibleToday(task));
}

export async function syncNotificationActionUpdate(data: Data, history: import('../types').History): Promise<void> {
  const config = await loadAppConfig();
  const backend = backendFromConfig(config);
  if (!backend) {
    return;
  }

  try {
    // Use the etag-aware sync path so background notification updates can't
    // blind-overwrite a concurrent foreground write. Local change remains in
    // AsyncStorage even if the remote write loses its race.
    await syncWithRemoteState(backend, data, history);
  } catch {
    // Keep the local change even if remote sync fails.
  }
}

async function loadNotificationIds(): Promise<NotificationIdMap> {
  try {
    const stored = await AsyncStorage.getItem(NOTIFICATION_IDS_KEY);
    return normalizeNotificationIdMap(stored ? JSON.parse(stored) : {});
  } catch {
    return {};
  }
}

async function saveNotificationIds(ids: NotificationIdMap): Promise<void> {
  try {
    await AsyncStorage.setItem(NOTIFICATION_IDS_KEY, JSON.stringify(ids));
  } catch {}
}

async function cancelTaskNotification(taskId: number): Promise<void> {
  const ids = await loadNotificationIds();
  const notifIds = ids[taskId] ?? [];

  if (notifIds.length === 0) {
    return;
  }

  await Promise.all(notifIds.map(async (notifId) => {
    try {
      await Notifications.cancelScheduledNotificationAsync(notifId);
    } catch {}
  }));

  const updated = { ...ids };
  delete updated[taskId];
  await saveNotificationIds(updated);
}

async function dismissPresentedNotification(
  response: Notifications.NotificationResponse,
): Promise<void> {
  const notificationId = response.notification.request.identifier;

  if (!notificationId) {
    return;
  }

  try {
    await Notifications.dismissNotificationAsync(notificationId);
  } catch {}
}

export async function handleNotificationAction(
  response: Notifications.NotificationResponse,
): Promise<boolean> {
  const cached = await loadCachedData();
  const cachedHistory = await loadCachedHistory();
  const updatedData = applyNotificationAction(cached, response);

  if (!updatedData) {
    if (isStaleActionableNotification(cached, response)) {
      await dismissPresentedNotification(response);
      const taskId = taskIdFromResponse(response);
      if (taskId) {
        await cancelTaskNotification(taskId);
      }
    }
    return false;
  }

  const updatedHistory = recordDataChange(cachedHistory, cached, updatedData, updatedData.last_modified || Date.now());
  await saveCachedData(updatedData);
  await saveCachedHistory(updatedHistory);
  await dismissPresentedNotification(response);
  const taskId = taskIdFromResponse(response);
  if (taskId) {
    await cancelTaskNotification(taskId);
  }
  await syncNotificationActionUpdate(updatedData, updatedHistory);
  return true;
}

export function isNotificationResponsePayload(
  payload: Notifications.NotificationTaskPayload,
): payload is Notifications.NotificationResponse {
  return typeof payload === 'object' && payload !== null && 'actionIdentifier' in payload;
}

export async function handleNotificationTaskPayload(
  payload: Notifications.NotificationTaskPayload,
): Promise<boolean> {
  if (!isNotificationResponsePayload(payload)) {
    return false;
  }

  return handleNotificationAction(payload);
}
