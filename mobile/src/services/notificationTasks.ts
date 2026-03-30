import * as Notifications from 'expo-notifications';
import * as TaskManager from 'expo-task-manager';
import { handleNotificationTaskPayload } from './notificationActionHandler';

export const BACKGROUND_NOTIFICATION_TASK = 'daily-tasks-notification-task';

if (!TaskManager.isTaskDefined(BACKGROUND_NOTIFICATION_TASK)) {
  // Define the task at module scope so Android can load it for notification actions.
  TaskManager.defineTask<Notifications.NotificationTaskPayload>(
    BACKGROUND_NOTIFICATION_TASK,
    async ({ data, error }) => {
      if (error || !data) {
        return;
      }

      await handleNotificationTaskPayload(data);
    },
  );
}

export async function registerNotificationTaskAsync(): Promise<void> {
  const isAvailable = await TaskManager.isAvailableAsync().catch(() => false);
  if (!isAvailable) {
    return;
  }

  const isRegistered = await TaskManager
    .isTaskRegisteredAsync(BACKGROUND_NOTIFICATION_TASK)
    .catch(() => false);

  if (!isRegistered) {
    await Notifications.registerTaskAsync(BACKGROUND_NOTIFICATION_TASK);
  }
}
