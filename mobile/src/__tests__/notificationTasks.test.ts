jest.mock('expo-notifications', () => ({
  registerTaskAsync: jest.fn().mockResolvedValue(undefined),
}));
jest.mock('expo-task-manager', () => ({
  defineTask: jest.fn(),
  isTaskDefined: jest.fn().mockReturnValue(false),
  isTaskRegisteredAsync: jest.fn().mockResolvedValue(false),
  isAvailableAsync: jest.fn().mockResolvedValue(true),
}));
jest.mock('../services/notificationActionHandler', () => ({
  handleNotificationTaskPayload: jest.fn().mockResolvedValue(undefined),
}));

import * as Notifications from 'expo-notifications';
import * as TaskManager from 'expo-task-manager';
import { handleNotificationTaskPayload } from '../services/notificationActionHandler';
import {
  BACKGROUND_NOTIFICATION_TASK,
  registerNotificationTaskAsync,
} from '../services/notificationTasks';

describe('background task callback (defineTask)', () => {
  // The module registers the callback at import time; retrieve it once.
  const taskCallback = (TaskManager.defineTask as jest.Mock).mock.calls[0][1] as (
    args: { data: unknown; error: Error | null },
  ) => Promise<void>;

  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('returns early when error is set', async () => {
    await taskCallback({ data: null, error: new Error('boom') });

    expect(handleNotificationTaskPayload).not.toHaveBeenCalled();
  });

  it('returns early when data is null and no error', async () => {
    await taskCallback({ data: null, error: null });

    expect(handleNotificationTaskPayload).not.toHaveBeenCalled();
  });

  it('calls handleNotificationTaskPayload when data is present', async () => {
    const payload = { actionIdentifier: 'done' };

    await taskCallback({ data: payload, error: null });

    expect(handleNotificationTaskPayload).toHaveBeenCalledWith(payload);
  });
});

describe('registerNotificationTaskAsync', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    // Restore defaults after clearAllMocks
    (TaskManager.isAvailableAsync as jest.Mock).mockResolvedValue(true);
    (TaskManager.isTaskRegisteredAsync as jest.Mock).mockResolvedValue(false);
    (Notifications.registerTaskAsync as jest.Mock).mockResolvedValue(undefined);
  });

  it('does nothing when task manager is not available', async () => {
    (TaskManager.isAvailableAsync as jest.Mock).mockResolvedValue(false);

    await registerNotificationTaskAsync();

    expect(Notifications.registerTaskAsync).not.toHaveBeenCalled();
  });

  it('skips registration when task is already registered', async () => {
    (TaskManager.isTaskRegisteredAsync as jest.Mock).mockResolvedValue(true);

    await registerNotificationTaskAsync();

    expect(Notifications.registerTaskAsync).not.toHaveBeenCalled();
  });

  it('registers the task when available and not yet registered', async () => {
    await registerNotificationTaskAsync();

    expect(Notifications.registerTaskAsync).toHaveBeenCalledWith(
      BACKGROUND_NOTIFICATION_TASK,
    );
  });
});
