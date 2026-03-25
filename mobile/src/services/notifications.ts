import * as Notifications from 'expo-notifications';
import AsyncStorage from '@react-native-async-storage/async-storage';
import type { Task } from '../types';
import { loadCachedData, saveCachedData } from './storage';

const NOTIFICATION_IDS_KEY = 'dailyTasksNotificationIds';
const TASK_CATEGORY_ID = 'task-deadline';

// Configure how notifications are displayed when the app is in the foreground
Notifications.setNotificationHandler({
  handleNotification: async () => ({
    shouldShowAlert: true,
    shouldPlaySound: true,
    shouldSetBadge: false,
  }),
});

/**
 * Request notification permissions and set up action categories.
 * Returns true if permissions were granted.
 */
export async function setupNotifications(): Promise<boolean> {
  try {
    // Set up "Mark Done" and "Skip for Today" action buttons on notifications
    await Notifications.setNotificationCategoryAsync(TASK_CATEGORY_ID, [
      {
        identifier: 'done',
        buttonTitle: 'Mark Done',
        options: { opensAppToForeground: true },
      },
      {
        identifier: 'skip',
        buttonTitle: 'Skip for Today',
        options: { opensAppToForeground: true },
      },
    ]);

    const { status: existing } = await Notifications.getPermissionsAsync();
    if (existing === 'granted') return true;

    const { status } = await Notifications.requestPermissionsAsync();
    return status === 'granted';
  } catch {
    return false;
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

export async function cancelTaskNotification(taskId: number): Promise<void> {
  const ids = await loadNotificationIds();
  const notifId = ids[taskId];
  if (notifId) {
    try {
      await Notifications.cancelScheduledNotificationAsync(notifId);
    } catch {}
    const updated = { ...ids };
    delete updated[taskId];
    await saveNotificationIds(updated);
  }
}

/**
 * Cancel all scheduled notifications and reschedule only for todo tasks with deadlines.
 * Call this whenever the task list changes.
 */
export async function rescheduleAllNotifications(tasks: Task[]): Promise<void> {
  try {
    await Notifications.cancelAllScheduledNotificationsAsync();
    await saveNotificationIds({});

    const todoWithDeadlines = tasks.filter(t => t.status === 'todo' && t.deadline);
    const newIds: Record<number, string> = {};

    for (const task of todoWithDeadlines) {
      const parts = task.deadline!.split(':');
      const hour = parseInt(parts[0], 10);
      const minute = parseInt(parts[1], 10);
      if (isNaN(hour) || isNaN(minute)) continue;

      const notifId = await Notifications.scheduleNotificationAsync({
        content: {
          title: task.title,
          body: 'Deadline reached — mark it done or skip for today.',
          data: { taskId: task.id },
          categoryIdentifier: TASK_CATEGORY_ID,
        },
        trigger: {
          type: Notifications.SchedulableTriggerInputTypes.DAILY,
          hour,
          minute,
        },
      });
      newIds[task.id] = notifId;
    }

    await saveNotificationIds(newIds);
  } catch {
    // Silently ignore if notifications aren't available (e.g. web, permissions denied)
  }
}

/**
 * Handle a notification action (Skip / Mark Done) directly via storage.
 * This works even when the app is not fully active.
 * Returns true if the data was changed and the caller should reload state.
 */
export async function handleNotificationAction(
  response: Notifications.NotificationResponse,
): Promise<boolean> {
  const { actionIdentifier, notification } = response;
  const taskId = notification.request.content.data?.taskId as number | undefined;

  if (!taskId) return false;
  if (actionIdentifier !== 'skip' && actionIdentifier !== 'done') return false;

  const cached = await loadCachedData();
  const task = cached.tasks.find(t => t.id === taskId);
  if (!task || task.status !== 'todo') return false;

  const newStatus = actionIdentifier === 'done' ? 'done' : 'skipped';
  let maxOrder = 0;
  cached.tasks.forEach(t => {
    if (t.status === newStatus) maxOrder = Math.max(maxOrder, t.order);
  });

  const updatedData = {
    ...cached,
    tasks: cached.tasks.map(t =>
      t.id === taskId ? { ...t, status: newStatus as 'done' | 'skipped', order: maxOrder + 1 } : t
    ),
    last_modified: Date.now(),
  };

  await saveCachedData(updatedData);
  await cancelTaskNotification(taskId);
  return true;
}
