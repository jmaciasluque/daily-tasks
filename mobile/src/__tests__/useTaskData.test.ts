// Tell react-test-renderer we are in the act() testing environment so that
// state updates inside async callbacks don't trigger "not configured" warnings.
(global as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

jest.mock('../config/env', () => ({
  appVariant: 'testing',
  storagePrefix: 'dailyTasks',
  defaultRemotePath: '',
}));
jest.mock('../services/data', () => ({
  emptyData: jest.fn().mockReturnValue({
    last_reset: '2026-01-01', next_id: 1, tasks: [], theme_index: 0, last_modified: 0,
  }),
  normalizeData: jest.fn().mockImplementation((d: unknown) => d),
  resetIfNeeded: jest.fn().mockImplementation((d: unknown) => d),
  nextOrder: jest.fn().mockReturnValue(1),
}));
jest.mock('../services/history', () => ({
  emptyHistory: jest.fn().mockReturnValue({ version: 1, days: [], events: [] }),
  normalizeHistory: jest.fn().mockImplementation((h: unknown) => h),
  applyDailyResetWithHistory: jest.fn().mockImplementation((data: unknown, history: unknown) => ({ data, history })),
  recordDataChange: jest.fn().mockImplementation((_h: unknown, _b: unknown, _n: unknown, _t: unknown) => ({ version: 1, days: [], events: [] })),
}));
jest.mock('../services/storage', () => ({
  loadAppConfig: jest.fn().mockResolvedValue({}),
  saveAppConfig: jest.fn().mockResolvedValue(undefined),
  loadCachedData: jest.fn().mockResolvedValue({
    last_reset: '2026-01-01', next_id: 1, tasks: [], theme_index: 0, last_modified: 0,
  }),
  saveCachedData: jest.fn().mockResolvedValue(undefined),
  loadCachedHistory: jest.fn().mockResolvedValue({ version: 1, days: [], events: [] }),
  saveCachedHistory: jest.fn().mockResolvedValue(undefined),
  backendFromConfig: jest.fn().mockReturnValue(null),
  nextcloudSettingsFromConfig: jest.fn().mockReturnValue({ baseUrl: '', username: '', password: '', remotePath: '' }),
}));
jest.mock('../services/backend_webdav', () => ({
  isSettingsComplete: jest.fn().mockReturnValue(false),
  WebDAVBackend: jest.fn().mockImplementation(function(this: { fetch: jest.Mock; push: jest.Mock }) {
    this.fetch = jest.fn();
    this.push = jest.fn();
  }),
}));
jest.mock('../services/syncQueue', () => ({
  remoteStateSyncQueue: { enqueue: jest.fn().mockResolvedValue(undefined) },
}));
jest.mock('../services/notifications', () => ({
  rescheduleAllNotifications: jest.fn().mockResolvedValue(undefined),
}));

import React from 'react';
import { act } from 'react';
import { create } from 'react-test-renderer';
import { useTaskData } from '../hooks/useTaskData';
import { remoteStateSyncQueue } from '../services/syncQueue';
import { saveAppConfig } from '../services/storage';
import type { Task } from '../types';

async function mountHook() {
  let latest!: ReturnType<typeof useTaskData>;
  function TestComponent() {
    latest = useTaskData();
    return null;
  }
  await act(async () => {
    create(React.createElement(TestComponent));
    await new Promise<void>(resolve => setTimeout(resolve, 0));
  });
  return { get current() { return latest; } };
}

describe('useTaskData', () => {
  let originalError: typeof console.error;

  beforeAll(() => {
    originalError = console.error;
    // react-test-renderer is deprecated in React 19 but required by this task.
    console.error = (...args: unknown[]) => {
      if (typeof args[0] === 'string' && args[0].includes('react-test-renderer is deprecated')) {
        return;
      }
      originalError(...args);
    };
  });

  afterAll(() => {
    console.error = originalError;
  });

  beforeEach(() => {
    jest.clearAllMocks();
    // Restore default mock return values after clearAllMocks
    const storage = require('../services/storage');
    storage.loadAppConfig.mockResolvedValue({});
    storage.loadCachedData.mockResolvedValue({
      last_reset: '2026-01-01', next_id: 1, tasks: [], theme_index: 0, last_modified: 0,
    });
    storage.loadCachedHistory.mockResolvedValue({ version: 1, days: [], events: [] });
    storage.saveCachedData.mockResolvedValue(undefined);
    storage.saveCachedHistory.mockResolvedValue(undefined);
    storage.saveAppConfig.mockResolvedValue(undefined);
    storage.backendFromConfig.mockReturnValue(null);
    storage.nextcloudSettingsFromConfig.mockReturnValue({ baseUrl: '', username: '', password: '', remotePath: '' });

    const history = require('../services/history');
    history.emptyHistory.mockReturnValue({ version: 1, days: [], events: [] });
    history.normalizeHistory.mockImplementation((h: unknown) => h);
    history.applyDailyResetWithHistory.mockImplementation((data: unknown, hist: unknown) => ({ data, history: hist }));
    history.recordDataChange.mockReturnValue({ version: 1, days: [], events: [] });

    const data = require('../services/data');
    data.emptyData.mockReturnValue({ last_reset: '2026-01-01', next_id: 1, tasks: [], theme_index: 0, last_modified: 0 });
    data.normalizeData.mockImplementation((d: unknown) => d);
    data.resetIfNeeded.mockImplementation((d: unknown) => d);
    data.nextOrder.mockReturnValue(1);

    const webdav = require('../services/backend_webdav');
    webdav.isSettingsComplete.mockReturnValue(false);

    (remoteStateSyncQueue.enqueue as jest.Mock).mockResolvedValue(undefined);
  });

  describe('initialization', () => {
    it('loads data from cache on mount', async () => {
      const hook = await mountHook();

      expect(hook.current.data).toMatchObject({
        last_reset: '2026-01-01',
        next_id: 1,
        tasks: [],
        theme_index: 0,
      });
    });

    it('sets statusMsg to Testing build message when appVariant is testing', async () => {
      const hook = await mountHook();

      expect(hook.current.statusMsg).toContain('Testing build');
    });

    it('calls loadAppConfig on mount', async () => {
      const storage = require('../services/storage');
      await mountHook();

      expect(storage.loadAppConfig).toHaveBeenCalled();
    });
  });

  describe('addTask', () => {
    it('adds a task with the correct id and title', async () => {
      const hook = await mountHook();

      await act(async () => {
        hook.current.addTask('My task', 30);
      });

      expect(hook.current.data.tasks).toHaveLength(1);
      expect(hook.current.data.tasks[0]).toMatchObject({
        id: 1,
        title: 'My task',
        duration: 30,
        status: 'todo',
      });
    });

    it('increments next_id after adding a task', async () => {
      const hook = await mountHook();

      await act(async () => {
        hook.current.addTask('Task A', 15);
      });

      expect(hook.current.data.next_id).toBe(2);
    });
  });

  describe('editTask', () => {
    it('updates title, duration, and deadline on the matching task', async () => {
      const hook = await mountHook();

      await act(async () => {
        hook.current.addTask('Original', 10);
      });

      await act(async () => {
        hook.current.editTask(1, 'Updated', 20, '09:00');
      });

      expect(hook.current.data.tasks[0]).toMatchObject({
        id: 1,
        title: 'Updated',
        duration: 20,
        deadline: '09:00',
      });
    });
  });

  describe('deleteTask', () => {
    it('removes the task by id', async () => {
      const hook = await mountHook();

      await act(async () => {
        hook.current.addTask('To delete', 5);
      });

      expect(hook.current.data.tasks).toHaveLength(1);

      await act(async () => {
        hook.current.deleteTask(1);
      });

      expect(hook.current.data.tasks).toHaveLength(0);
    });
  });

  describe('toggleTaskStatus', () => {
    it('toggles todo → done', async () => {
      const hook = await mountHook();

      await act(async () => {
        hook.current.addTask('Toggle me', 10);
      });

      const task = hook.current.data.tasks[0] as Task;
      expect(task.status).toBe('todo');

      await act(async () => {
        hook.current.toggleTaskStatus(task);
      });

      expect(hook.current.data.tasks[0].status).toBe('done');
    });

    it('toggles done → todo', async () => {
      const hook = await mountHook();

      await act(async () => {
        hook.current.addTask('Toggle back', 10);
      });

      const task = hook.current.data.tasks[0] as Task;

      await act(async () => {
        hook.current.toggleTaskStatus(task);
      });

      const doneTask = hook.current.data.tasks[0] as Task;
      expect(doneTask.status).toBe('done');

      await act(async () => {
        hook.current.toggleTaskStatus(doneTask);
      });

      expect(hook.current.data.tasks[0].status).toBe('todo');
    });
  });

  describe('skipTask', () => {
    it('skips a todo task', async () => {
      const hook = await mountHook();

      await act(async () => {
        hook.current.addTask('Skip me', 10);
      });

      await act(async () => {
        hook.current.skipTask(1);
      });

      expect(hook.current.data.tasks[0].status).toBe('skipped');
    });

    it('does not skip a task that is already done', async () => {
      const hook = await mountHook();

      await act(async () => {
        hook.current.addTask('Already done', 10);
      });

      const task = hook.current.data.tasks[0] as Task;
      await act(async () => {
        hook.current.toggleTaskStatus(task);
      });

      expect(hook.current.data.tasks[0].status).toBe('done');

      await act(async () => {
        hook.current.skipTask(1);
      });

      // Should remain 'done', not become 'skipped'
      expect(hook.current.data.tasks[0].status).toBe('done');
    });
  });

  describe('reorderTasks', () => {
    it('updates order numbers on todo tasks', async () => {
      const hook = await mountHook();

      // Add two tasks
      await act(async () => {
        hook.current.addTask('Task A', 10);
      });
      await act(async () => {
        hook.current.addTask('Task B', 10);
      });

      const tasks = hook.current.data.tasks as Task[];
      const reordered = [tasks[1], tasks[0]]; // reversed

      await act(async () => {
        hook.current.reorderTasks(reordered);
      });

      const finalTasks = hook.current.data.tasks;
      const taskA = finalTasks.find(t => t.title === 'Task A')!;
      const taskB = finalTasks.find(t => t.title === 'Task B')!;
      expect(taskB.order).toBe(1);
      expect(taskA.order).toBe(2);
    });
  });

  describe('cycleTheme', () => {
    it('increments theme_index', async () => {
      const hook = await mountHook();

      const before = hook.current.data.theme_index;

      await act(async () => {
        hook.current.cycleTheme();
      });

      expect(hook.current.data.theme_index).toBe((before + 1) % 25);
    });

    it('wraps at 25', async () => {
      const storage = require('../services/storage');
      storage.loadCachedData.mockResolvedValue({
        last_reset: '2026-01-01', next_id: 1, tasks: [], theme_index: 24, last_modified: 0,
      });

      const hook = await mountHook();

      expect(hook.current.data.theme_index).toBe(24);

      await act(async () => {
        hook.current.cycleTheme();
      });

      expect(hook.current.data.theme_index).toBe(0);
    });
  });

  describe('chooseLocalBackend', () => {
    it('sets config.backend to local and calls saveAppConfig', async () => {
      const hook = await mountHook();

      await act(async () => {
        await hook.current.chooseLocalBackend();
      });

      expect(hook.current.config.backend).toBe('local');
      expect(saveAppConfig).toHaveBeenCalledWith({ backend: 'local' });
    });
  });

  describe('saveNextcloudSettings', () => {
    it('saves config and does not trigger sync when settings are incomplete', async () => {
      const hook = await mountHook();

      await act(async () => {
        await hook.current.saveNextcloudSettings({
          baseUrl: 'https://cloud.example.com',
          username: 'user',
          password: 'pass',
          remotePath: '/path',
        });
      });

      expect(saveAppConfig).toHaveBeenCalled();
      expect(remoteStateSyncQueue.enqueue).not.toHaveBeenCalled();
    });
  });

  describe('syncFromRemote', () => {
    it('sets statusMsg to Connect Nextcloud first when no backend is configured', async () => {
      const hook = await mountHook();

      await act(async () => {
        await hook.current.syncFromRemote();
      });

      expect(hook.current.statusMsg).toBe('Connect Nextcloud first.');
    });

    it('enqueues a sync when called with override settings', async () => {
      const hook = await mountHook();

      await act(async () => {
        await hook.current.syncFromRemote({
          baseUrl: 'https://cloud.example.com',
          username: 'u',
          password: 'p',
          remotePath: '/r',
        });
      });

      expect(remoteStateSyncQueue.enqueue).toHaveBeenCalled();
    });
  });
});
