import * as Notifications from 'expo-notifications';
import AsyncStorage from '@react-native-async-storage/async-storage';
import type { Data } from '../types';
import {
  loadAppConfig,
  loadCachedData,
  loadCachedHistory,
  nextcloudSettingsFromConfig,
  saveCachedData,
  saveCachedHistory,
} from './storage';
import { recordDataChange } from './history';
import { isSettingsComplete, pushRemoteState } from './webdav';

const NOTIFICATION_IDS_KEY = 'dailyTasksNotificationIds';

function isActionableResponse(
  response: Notifications.NotificationResponse,
): response is Notifications.NotificationResponse & {
  actionIdentifier: 'skip' | 'done';
} {
  return response.actionIdentifier === 'skip' || response.actionIdentifier === 'done';
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
  if (!task || task.status !== 'todo') {
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

export async function syncNotificationActionUpdate(data: Data, history: import('../types').History): Promise<void> {
  const config = await loadAppConfig();
  const settings = nextcloudSettingsFromConfig(config);
  if (config.backend !== 'nextcloud' || !isSettingsComplete(settings)) {
    return;
  }

  try {
    await pushRemoteState(settings, data, history);
  } catch {
    // Keep the local change even if remote sync fails.
  }
}

async function loadNotificationIds(): Promise<Record<number, string>> {
  try {
    const stored = await AsyncStorage.getItem(NOTIFICATION_IDS_KEY);
    return stored ? JSON.parse(stored) : {};
  } catch {
    return {};
  }
}

async function saveNotificationIds(ids: Record<number, string>): Promise<void> {
  try {
    await AsyncStorage.setItem(NOTIFICATION_IDS_KEY, JSON.stringify(ids));
  } catch {}
}

async function cancelTaskNotification(taskId: number): Promise<void> {
  const ids = await loadNotificationIds();
  const notifId = ids[taskId];

  if (!notifId) {
    return;
  }

  try {
    await Notifications.cancelScheduledNotificationAsync(notifId);
  } catch {}

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
    return false;
  }

  const updatedHistory = recordDataChange(cachedHistory, cached, updatedData, updatedData.last_modified || Date.now());
  await saveCachedData(updatedData);
  await saveCachedHistory(updatedHistory);
  await dismissPresentedNotification(response);
  const taskId = response.notification.request.content.data?.taskId as number | undefined;
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
