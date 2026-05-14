import * as Notifications from 'expo-notifications';
import { Platform } from 'react-native';
import AsyncStorage from '@react-native-async-storage/async-storage';
import type { Task } from '../types';
import { handleNotificationAction as applyNotificationAction } from './notificationActionHandler';
import { registerNotificationTaskAsync } from './notificationTasks';

const NOTIFICATION_IDS_KEY = 'dailyTasksNotificationIds';
const TASK_CATEGORY_ID = 'task-deadline';
const NOTIFICATION_CHANNEL_ID = 'default';
type StoredNotificationIds = Record<number, string | string[]>;

// Configure how notifications are displayed when the app is in the foreground
Notifications.setNotificationHandler({
  handleNotification: async () => ({
    shouldShowAlert: true,
    shouldShowBanner: true,
    shouldShowList: true,
    shouldPlaySound: true,
    shouldSetBadge: false,
  }),
});

async function ensureNotificationChannelAsync(): Promise<void> {
  if (Platform.OS !== 'android') {
    return;
  }

  await Notifications.setNotificationChannelAsync(NOTIFICATION_CHANNEL_ID, {
    name: 'Daily Tasks',
    importance: Notifications.AndroidImportance.HIGH,
    sound: 'default',
    vibrationPattern: [0, 250, 250, 250],
  });
}

/**
 * Request notification permissions and set up action categories.
 * Returns true if permissions were granted.
 */
export async function setupNotifications(): Promise<boolean> {
  try {
    await ensureNotificationChannelAsync();

    // Set up "Mark Done" and "Skip for Today" action buttons on notifications
    await Notifications.setNotificationCategoryAsync(TASK_CATEGORY_ID, [
      {
        identifier: 'done',
        buttonTitle: 'Mark Done',
        options: { opensAppToForeground: Platform.OS !== 'android' },
      },
      {
        identifier: 'skip',
        buttonTitle: 'Skip for Today',
        options: { opensAppToForeground: Platform.OS !== 'android' },
      },
    ]);

    try {
      await registerNotificationTaskAsync();
    } catch {
      // Background notification actions are best-effort in unsupported runtimes.
    }

    const { status: existing } = await Notifications.getPermissionsAsync();
    if (existing === 'granted') return true;

    const { status } = await Notifications.requestPermissionsAsync();
    return status === 'granted';
  } catch {
    return false;
  }
}

async function loadNotificationIds(): Promise<StoredNotificationIds> {
  try {
    const stored = await AsyncStorage.getItem(NOTIFICATION_IDS_KEY);
    return stored ? JSON.parse(stored) : {};
  } catch {
    return {};
  }
}

async function saveNotificationIds(ids: StoredNotificationIds): Promise<void> {
  try {
    await AsyncStorage.setItem(NOTIFICATION_IDS_KEY, JSON.stringify(ids));
  } catch {}
}

export async function cancelTaskNotification(taskId: number): Promise<void> {
  const ids = await loadNotificationIds();
  const notifIds = ids[taskId];
  if (notifIds) {
    try {
      for (const notifId of Array.isArray(notifIds) ? notifIds : [notifIds]) {
        await Notifications.cancelScheduledNotificationAsync(notifId);
      }
    } catch {}
    const updated = { ...ids };
    delete updated[taskId];
    await saveNotificationIds(updated);
  }
}

function visibleWeekdays(visibility?: number[]): number[] {
  if (!visibility || visibility.length === 0) {
    return [];
  }

  const validDays = visibility.filter(
    (day): day is number => Number.isInteger(day) && day >= 0 && day <= 6,
  );
  return Array.from(new Set(validDays)).sort((a, b) => a - b);
}

function notificationTriggersForTask(
  task: Task,
  hour: number,
  minute: number,
): Notifications.NotificationTriggerInput[] {
  if (!task.visibility || task.visibility.length === 0) {
    return [{
      type: Notifications.SchedulableTriggerInputTypes.DAILY,
      hour,
      minute,
      channelId: NOTIFICATION_CHANNEL_ID,
    }];
  }

  const weekdays = visibleWeekdays(task.visibility);
  return weekdays.map(day => ({
    type: Notifications.SchedulableTriggerInputTypes.WEEKLY,
    weekday: day + 1, // Expo uses 1=Sunday; task visibility uses 0=Sunday.
    hour,
    minute,
    channelId: NOTIFICATION_CHANNEL_ID,
  }));
}

/**
 * Cancel all scheduled notifications and reschedule only for todo tasks with deadlines.
 * Call this whenever the task list changes.
 */
export async function rescheduleAllNotifications(tasks: Task[]): Promise<void> {
  try {
    await ensureNotificationChannelAsync();
    await Notifications.cancelAllScheduledNotificationsAsync();
    await saveNotificationIds({});

    const todoWithDeadlines = tasks.filter(t => t.status === 'todo' && t.deadline);
    const newIds: StoredNotificationIds = {};

    for (const task of todoWithDeadlines) {
      const parts = task.deadline!.split(':');
      const hour = parseInt(parts[0], 10);
      const minute = parseInt(parts[1], 10);
      if (isNaN(hour) || isNaN(minute)) continue;

      const notifIds: string[] = [];
      for (const trigger of notificationTriggersForTask(task, hour, minute)) {
        const notifId = await Notifications.scheduleNotificationAsync({
          content: {
            title: task.title,
            body: 'Deadline reached — mark it done or skip for today.',
            data: { taskId: task.id },
            categoryIdentifier: TASK_CATEGORY_ID,
          },
          trigger,
        });
        notifIds.push(notifId);
      }
      if (notifIds.length === 1) {
        newIds[task.id] = notifIds[0];
      } else if (notifIds.length > 1) {
        newIds[task.id] = notifIds;
      }
    }

    await saveNotificationIds(newIds);
  } catch {
    // Silently ignore if notifications aren't available (e.g. web, permissions denied)
  }
}

export async function handleNotificationAction(
  response: Notifications.NotificationResponse,
): Promise<boolean> {
  return applyNotificationAction(response);
}

export async function scheduleTestNotification(task: Task): Promise<void> {
  await ensureNotificationChannelAsync();

  await Notifications.scheduleNotificationAsync({
    content: {
      title: `${task.title} (test)`,
      body: 'Testing notification actions: Mark Done / Skip for Today.',
      data: { taskId: task.id },
      categoryIdentifier: TASK_CATEGORY_ID,
      sound: 'default',
    },
    trigger: {
      type: Notifications.SchedulableTriggerInputTypes.TIME_INTERVAL,
      seconds: 3,
      repeats: false,
      channelId: NOTIFICATION_CHANNEL_ID,
    },
  });
}
