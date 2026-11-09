jest.mock('react-native', () => ({
  Platform: { OS: 'android' },
}));

// Mock expo-notifications before importing the module under test.
// setNotificationHandler is called at module load time so the mock must be
// in place before the import is resolved — jest.mock() is hoisted so this works.
jest.mock('expo-notifications', () => ({
  setNotificationHandler: jest.fn(),
  setNotificationChannelAsync: jest.fn().mockResolvedValue(undefined),
  setNotificationCategoryAsync: jest.fn().mockResolvedValue(undefined),
  registerTaskAsync: jest.fn().mockResolvedValue(undefined),
  getPermissionsAsync: jest.fn().mockResolvedValue({ status: 'granted' }),
  requestPermissionsAsync: jest.fn().mockResolvedValue({ status: 'granted' }),
  cancelAllScheduledNotificationsAsync: jest.fn().mockResolvedValue(undefined),
  scheduleNotificationAsync: jest.fn().mockResolvedValue('mock-notif-id'),
  cancelScheduledNotificationAsync: jest.fn().mockResolvedValue(undefined),
  dismissNotificationAsync: jest.fn().mockResolvedValue(undefined),
  AndroidImportance: { HIGH: 4 },
  SchedulableTriggerInputTypes: { DAILY: 'daily', TIME_INTERVAL: 'timeInterval' },
}));

jest.mock('expo-task-manager', () => ({
  defineTask: jest.fn(),
  isTaskDefined: jest.fn().mockReturnValue(false),
  isTaskRegisteredAsync: jest.fn().mockResolvedValue(false),
  isAvailableAsync: jest.fn().mockResolvedValue(true),
}));

jest.mock('@react-native-async-storage/async-storage', () => ({
  __esModule: true,
  default: {
    getItem: jest.fn().mockResolvedValue(null),
    setItem: jest.fn().mockResolvedValue(undefined),
  },
}));

// Mock storage service so tests control what "cached" data looks like
jest.mock('../services/storage', () => ({
  loadCachedData: jest.fn(),
  loadCachedHistory: jest.fn().mockResolvedValue({ version: 1, days: [], events: [] }),
  loadAppConfig: jest.fn().mockResolvedValue({}),
  nextcloudSettingsFromConfig: jest.fn((config: {
    nextcloud?: {
      baseUrl?: string;
      username?: string;
      password?: string;
      remotePath?: string;
    };
  }) => ({
    baseUrl: '',
    username: '',
    password: '',
    remotePath: '',
    ...(config.nextcloud ?? {}),
  })),
  // backendFromConfig returns a stub Backend object when the test config
  // looks complete; null otherwise. The actual Backend is irrelevant
  // because syncWithRemoteState is mocked at the module level.
  backendFromConfig: jest.fn((config: { backend?: string; nextcloud?: { baseUrl?: string; username?: string; password?: string; remotePath?: string } }) => {
    if (config.backend !== 'nextcloud') return null;
    const nc = config.nextcloud ?? {};
    if (!nc.baseUrl || !nc.username || !nc.password || !nc.remotePath) return null;
    return { fetch: jest.fn(), push: jest.fn() };
  }),
  saveCachedData: jest.fn().mockResolvedValue(undefined),
  saveCachedHistory: jest.fn().mockResolvedValue(undefined),
}));

jest.mock('../services/syncQueue', () => ({
  remoteStateSyncQueue: {
    enqueue: jest.fn().mockResolvedValue(undefined),
  },
}));

import * as Notifications from 'expo-notifications';
import * as TaskManager from 'expo-task-manager';
import AsyncStorage from '@react-native-async-storage/async-storage';
import { loadAppConfig, loadCachedData, saveCachedData } from '../services/storage';
import { remoteStateSyncQueue } from '../services/syncQueue';
import {
  setupNotifications,
  rescheduleAllNotifications,
  handleNotificationAction,
  scheduleTestNotification,
} from '../services/notifications';
import {
  handleNotificationTaskPayload,
} from '../services/notificationActionHandler';
import { BACKGROUND_NOTIFICATION_TASK } from '../services/notificationTasks';
import type { Data, Task } from '../types';

const mockLoadCachedData = loadCachedData as jest.MockedFunction<typeof loadCachedData>;
const mockLoadAppConfig = loadAppConfig as jest.MockedFunction<typeof loadAppConfig>;
const mockSaveCachedData = saveCachedData as jest.MockedFunction<typeof saveCachedData>;
const mockRemoteStateSyncQueue = remoteStateSyncQueue.enqueue as jest.Mock;
const mockSchedule = Notifications.scheduleNotificationAsync as jest.Mock;
const mockSetChannel = Notifications.setNotificationChannelAsync as jest.Mock;
const mockCancelAll = Notifications.cancelAllScheduledNotificationsAsync as jest.Mock;
const mockDismiss = Notifications.dismissNotificationAsync as jest.Mock;
const mockGetPerms = Notifications.getPermissionsAsync as jest.Mock;
const mockRequestPerms = Notifications.requestPermissionsAsync as jest.Mock;

function makeData(tasks: Task[]): Data {
  return { last_reset: '2026-01-01', next_id: 10, tasks, theme_index: 0, last_modified: 0 };
}

function makeResponse(
  actionIdentifier: string,
  taskId?: number,
  identifier = 'presented-notif-id',
): Notifications.NotificationResponse {
  return {
    actionIdentifier,
    notification: {
      request: {
        identifier,
        content: {
          data: taskId !== undefined ? { taskId } : {},
        },
      },
    },
  } as unknown as Notifications.NotificationResponse;
}

it('defines the background notification task at module load', () => {
  expect(TaskManager.defineTask).toHaveBeenCalledWith(
    BACKGROUND_NOTIFICATION_TASK,
    expect.any(Function),
  );
});

// ─── handleNotificationAction ────────────────────────────────────────────────

describe('handleNotificationAction', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    (AsyncStorage.getItem as jest.Mock).mockResolvedValue(null);
    mockLoadAppConfig.mockResolvedValue({});
  });

  it('returns false when taskId is absent from notification data', async () => {
    const result = await handleNotificationAction(makeResponse('skip'));
    expect(result).toBe(false);
    expect(mockSaveCachedData).not.toHaveBeenCalled();
  });

  it('returns false for default tap (not a recognised action)', async () => {
    // iOS default action identifier when user taps the notification body
    const result = await handleNotificationAction(
      makeResponse('com.apple.UNNotificationDefaultActionIdentifier', 1),
    );
    expect(result).toBe(false);
    expect(mockSaveCachedData).not.toHaveBeenCalled();
  });

  it('returns false when the task does not exist in cached data', async () => {
    mockLoadCachedData.mockResolvedValue(makeData([]));
    const result = await handleNotificationAction(makeResponse('skip', 99));
    expect(result).toBe(false);
    expect(mockSaveCachedData).not.toHaveBeenCalled();
  });

  it('returns false when the task is already done (not actionable)', async () => {
    mockLoadCachedData.mockResolvedValue(makeData([
      { id: 1, title: 'A', duration: 5, status: 'done', order: 1 },
    ]));
    const result = await handleNotificationAction(makeResponse('skip', 1));
    expect(result).toBe(false);
    expect(mockSaveCachedData).not.toHaveBeenCalled();
  });

  it('returns false when the task is already skipped (not actionable)', async () => {
    mockLoadCachedData.mockResolvedValue(makeData([
      { id: 1, title: 'A', duration: 5, status: 'skipped', order: 1 },
    ]));
    const result = await handleNotificationAction(makeResponse('skip', 1));
    expect(result).toBe(false);
    expect(mockSaveCachedData).not.toHaveBeenCalled();
  });

  it('marks a todo task as skipped on Skip action and returns true', async () => {
    mockLoadCachedData.mockResolvedValue(makeData([
      { id: 1, title: 'Morning run', duration: 30, status: 'todo', order: 1 },
    ]));

    const result = await handleNotificationAction(makeResponse('skip', 1));

    expect(result).toBe(true);
    const saved: Data = mockSaveCachedData.mock.calls[0][0];
    const task = saved.tasks.find(t => t.id === 1)!;
    expect(task.status).toBe('skipped');
  });

  it('dismisses the presented notification after a successful action', async () => {
    mockLoadCachedData.mockResolvedValue(makeData([
      { id: 1, title: 'Morning run', duration: 30, status: 'todo', order: 1 },
    ]));

    await handleNotificationAction(makeResponse('skip', 1, 'delivered-notif-id'));

    expect(mockDismiss).toHaveBeenCalledWith('delivered-notif-id');
  });

  it('marks a todo task as done on Done action and returns true', async () => {
    mockLoadCachedData.mockResolvedValue(makeData([
      { id: 1, title: 'Morning run', duration: 30, status: 'todo', order: 1 },
    ]));

    const result = await handleNotificationAction(makeResponse('done', 1));

    expect(result).toBe(true);
    const saved: Data = mockSaveCachedData.mock.calls[0][0];
    const task = saved.tasks.find(t => t.id === 1)!;
    expect(task.status).toBe('done');
  });

  it('assigns order as max-existing-status-order + 1 when skipping', async () => {
    // Two tasks already in 'skipped' at orders 1 and 3
    mockLoadCachedData.mockResolvedValue(makeData([
      { id: 1, title: 'A', duration: 5, status: 'todo', order: 1 },
      { id: 2, title: 'B', duration: 5, status: 'skipped', order: 1 },
      { id: 3, title: 'C', duration: 5, status: 'skipped', order: 3 },
    ]));

    await handleNotificationAction(makeResponse('skip', 1));

    const saved: Data = mockSaveCachedData.mock.calls[0][0];
    const task = saved.tasks.find(t => t.id === 1)!;
    expect(task.order).toBe(4); // max(1, 3) + 1
  });

  it('assigns order 1 when no existing tasks share the target status', async () => {
    mockLoadCachedData.mockResolvedValue(makeData([
      { id: 1, title: 'A', duration: 5, status: 'todo', order: 1 },
    ]));

    await handleNotificationAction(makeResponse('skip', 1));

    const saved: Data = mockSaveCachedData.mock.calls[0][0];
    const task = saved.tasks.find(t => t.id === 1)!;
    expect(task.order).toBe(1); // no existing skipped tasks → 0 + 1
  });

  it('does not change other tasks when one task is acted upon', async () => {
    mockLoadCachedData.mockResolvedValue(makeData([
      { id: 1, title: 'A', duration: 5, status: 'todo', order: 1 },
      { id: 2, title: 'B', duration: 5, status: 'todo', order: 2 },
    ]));

    await handleNotificationAction(makeResponse('skip', 1));

    const saved: Data = mockSaveCachedData.mock.calls[0][0];
    const other = saved.tasks.find(t => t.id === 2)!;
    expect(other.status).toBe('todo');
    expect(other.order).toBe(2);
  });

  it('updates last_modified when saving', async () => {
    const before = Date.now();
    mockLoadCachedData.mockResolvedValue(makeData([
      { id: 1, title: 'A', duration: 5, status: 'todo', order: 1 },
    ]));

    await handleNotificationAction(makeResponse('done', 1));

    const saved: Data = mockSaveCachedData.mock.calls[0][0];
    expect(saved.last_modified).toBeGreaterThanOrEqual(before);
  });

  it('pushes the action update to WebDAV when settings are complete', async () => {
    mockLoadCachedData.mockResolvedValue(makeData([
      { id: 1, title: 'A', duration: 5, status: 'todo', order: 1 },
    ]));
    mockLoadAppConfig.mockResolvedValue({
      backend: 'nextcloud',
      nextcloud: {
        baseUrl: 'https://cloud.example.com',
        username: 'user',
        password: 'pass',
        remotePath: '/remote.php/dav/files/user/.daily-tasks.json',
      },
    });

    await handleNotificationAction(makeResponse('done', 1));

    expect(mockRemoteStateSyncQueue).toHaveBeenCalledTimes(1);
    const pushed = mockRemoteStateSyncQueue.mock.calls[0][0].data as Data;
    expect(pushed.tasks.find(t => t.id === 1)?.status).toBe('done');
  });

  it('keeps the local change when WebDAV push fails', async () => {
    mockLoadCachedData.mockResolvedValue(makeData([
      { id: 1, title: 'A', duration: 5, status: 'todo', order: 1 },
    ]));
    mockLoadAppConfig.mockResolvedValue({
      backend: 'nextcloud',
      nextcloud: {
        baseUrl: 'https://cloud.example.com',
        username: 'user',
        password: 'pass',
        remotePath: '/remote.php/dav/files/user/.daily-tasks.json',
      },
    });
    mockRemoteStateSyncQueue.mockRejectedValueOnce(new Error('network down'));

    const result = await handleNotificationAction(makeResponse('skip', 1));

    expect(result).toBe(true);
    expect(mockSaveCachedData).toHaveBeenCalledTimes(1);
  });

  it('cancels the stored scheduled reminder for the acted-on task', async () => {
    mockLoadCachedData.mockResolvedValue(makeData([
      { id: 1, title: 'A', duration: 5, status: 'todo', order: 1 },
    ]));
    (AsyncStorage.getItem as jest.Mock).mockResolvedValue(JSON.stringify({ 1: 'scheduled-notif-id' }));

    await handleNotificationAction(makeResponse('done', 1));

    expect(Notifications.cancelScheduledNotificationAsync).toHaveBeenCalledWith('scheduled-notif-id');
  });
});

describe('handleNotificationTaskPayload', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    (AsyncStorage.getItem as jest.Mock).mockResolvedValue(null);
  });

  it('handles task payloads that contain a notification response', async () => {
    mockLoadCachedData.mockResolvedValue(makeData([
      { id: 1, title: 'A', duration: 5, status: 'todo', order: 1 },
    ]));

    const changed = await handleNotificationTaskPayload(makeResponse('done', 1));

    expect(changed).toBe(true);
    expect(mockSaveCachedData).toHaveBeenCalledTimes(1);
  });

  it('ignores background payloads that are not notification responses', async () => {
    const changed = await handleNotificationTaskPayload({
      notification: null,
      data: { dataString: '{"taskId":1}' },
    });

    expect(changed).toBe(false);
    expect(mockSaveCachedData).not.toHaveBeenCalled();
  });
});

// ─── rescheduleAllNotifications ──────────────────────────────────────────────

describe('rescheduleAllNotifications', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    (AsyncStorage.getItem as jest.Mock).mockResolvedValue(null);
  });

  it('cancels all existing notifications before scheduling new ones', async () => {
    await rescheduleAllNotifications([]);
    expect(mockCancelAll).toHaveBeenCalledTimes(1);
  });

  it('creates the Android notification channel before rescheduling', async () => {
    await rescheduleAllNotifications([]);
    expect(mockSetChannel).toHaveBeenCalledWith('default', expect.objectContaining({
      name: 'Daily Tasks',
      importance: 4,
    }));
  });

  it('does not schedule any notifications when the task list is empty', async () => {
    await rescheduleAllNotifications([]);
    expect(mockSchedule).not.toHaveBeenCalled();
  });

  it('schedules one notification per todo task that has a deadline', async () => {
    await rescheduleAllNotifications([
      { id: 1, title: 'Morning run', duration: 30, status: 'todo', order: 1, deadline: '07:00' },
      { id: 2, title: 'Read',        duration: 20, status: 'todo', order: 2 }, // no deadline
    ]);

    expect(mockSchedule).toHaveBeenCalledTimes(1);
    const call = mockSchedule.mock.calls[0][0];
    expect(call.content.data.taskId).toBe(1);
  });

  it('sets the correct hour and minute from the deadline string', async () => {
    await rescheduleAllNotifications([
      { id: 1, title: 'Lunch', duration: 30, status: 'todo', order: 1, deadline: '13:45' },
    ]);

    const trigger = mockSchedule.mock.calls[0][0].trigger;
    expect(trigger.hour).toBe(13);
    expect(trigger.minute).toBe(45);
    expect(trigger.channelId).toBe('default');
  });

  it('schedules notifications for multiple todo tasks with deadlines', async () => {
    await rescheduleAllNotifications([
      { id: 1, title: 'Task A', duration: 10, status: 'todo', order: 1, deadline: '09:00' },
      { id: 2, title: 'Task B', duration: 10, status: 'todo', order: 2, deadline: '18:00' },
    ]);

    expect(mockSchedule).toHaveBeenCalledTimes(2);
  });

  it('does not schedule for done tasks even if they have a deadline', async () => {
    await rescheduleAllNotifications([
      { id: 1, title: 'Done task', duration: 10, status: 'done', order: 1, deadline: '08:00' },
    ]);

    expect(mockSchedule).not.toHaveBeenCalled();
  });

  it('does not schedule for skipped tasks even if they have a deadline', async () => {
    await rescheduleAllNotifications([
      { id: 1, title: 'Skipped task', duration: 10, status: 'skipped', order: 1, deadline: '08:00' },
    ]);

    expect(mockSchedule).not.toHaveBeenCalled();
  });

  it('saves new notification IDs to storage after scheduling', async () => {
    mockSchedule.mockResolvedValue('new-notif-id');

    await rescheduleAllNotifications([
      { id: 1, title: 'Task A', duration: 10, status: 'todo', order: 1, deadline: '09:00' },
    ]);

    // rescheduleAllNotifications calls setItem twice: once to clear ({}) then once
    // to persist the new IDs. We want the last call with this key.
    const setItemCalls = (AsyncStorage.setItem as jest.Mock).mock.calls;
    const matchingCalls = setItemCalls.filter(([key]: [string]) => key === 'dailyTasksNotificationIds');
    expect(matchingCalls.length).toBeGreaterThanOrEqual(2);
    const lastCall = matchingCalls[matchingCalls.length - 1];
    const stored = JSON.parse(lastCall[1]);
    // JSON keys are strings, but JS coerces numeric property access to string
    expect(stored['1']).toBe('new-notif-id');
  });
});

describe('scheduleTestNotification', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('schedules a one-off notification on the Android channel for the given task', async () => {
    await scheduleTestNotification({
      id: 7,
      title: 'Notif test',
      duration: 5,
      status: 'todo',
      order: 1,
    });

    expect(mockSetChannel).toHaveBeenCalledTimes(1);
    expect(mockSchedule).toHaveBeenCalledTimes(1);
    const request = mockSchedule.mock.calls[0][0];
    expect(request.content.data.taskId).toBe(7);
    expect(request.content.categoryIdentifier).toBe('task-deadline');
    expect(request.trigger.type).toBe('timeInterval');
    expect(request.trigger.seconds).toBe(3);
    expect(request.trigger.channelId).toBe('default');
  });
});

// ─── setupNotifications ──────────────────────────────────────────────────────

describe('setupNotifications', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('returns true when permissions are already granted', async () => {
    mockGetPerms.mockResolvedValue({ status: 'granted' });
    const result = await setupNotifications();
    expect(result).toBe(true);
    expect(mockRequestPerms).not.toHaveBeenCalled();
  });

  it('requests permissions when not yet determined', async () => {
    mockGetPerms.mockResolvedValue({ status: 'undetermined' });
    mockRequestPerms.mockResolvedValue({ status: 'granted' });
    const result = await setupNotifications();
    expect(result).toBe(true);
    expect(mockRequestPerms).toHaveBeenCalledTimes(1);
  });

  it('returns false when the user denies permissions', async () => {
    mockGetPerms.mockResolvedValue({ status: 'undetermined' });
    mockRequestPerms.mockResolvedValue({ status: 'denied' });
    const result = await setupNotifications();
    expect(result).toBe(false);
  });

  it('registers the Skip and Mark Done notification categories', async () => {
    mockGetPerms.mockResolvedValue({ status: 'granted' });
    await setupNotifications();
    const mockSetCategory = Notifications.setNotificationCategoryAsync as jest.Mock;
    expect(mockSetCategory).toHaveBeenCalledTimes(1);
    const [, actions] = mockSetCategory.mock.calls[0];
    const identifiers = actions.map((a: { identifier: string }) => a.identifier);
    expect(identifiers).toContain('skip');
    expect(identifiers).toContain('done');
  });

  it('keeps Android notification actions in the background', async () => {
    mockGetPerms.mockResolvedValue({ status: 'granted' });

    await setupNotifications();

    const [, actions] = (Notifications.setNotificationCategoryAsync as jest.Mock).mock.calls[0];
    expect(actions[0].options.opensAppToForeground).toBe(false);
    expect(actions[1].options.opensAppToForeground).toBe(false);
  });

  it('registers the background notification task', async () => {
    mockGetPerms.mockResolvedValue({ status: 'granted' });

    await setupNotifications();

    expect(Notifications.registerTaskAsync).toHaveBeenCalledWith(BACKGROUND_NOTIFICATION_TASK);
  });

  it('creates the Android notification channel during setup', async () => {
    mockGetPerms.mockResolvedValue({ status: 'granted' });

    await setupNotifications();

    expect(mockSetChannel).toHaveBeenCalledWith('default', expect.objectContaining({
      name: 'Daily Tasks',
      importance: 4,
    }));
  });
});
