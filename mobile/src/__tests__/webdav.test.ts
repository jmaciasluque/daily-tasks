jest.mock('../config/env', () => ({
  defaultRemotePath: '/remote.php/dav/files/<username>/.daily-tasks.json',
}));

import { pollNextcloudLogin, pushRemoteHistory, startNextcloudLogin, syncWithRemote, syncWithRemoteState } from '../services/webdav';
import type { Data, History, Settings } from '../types';

const settings: Settings = {
  baseUrl: 'https://cloud.example.com',
  username: 'user',
  password: 'pass',
  remotePath: '/remote.php/dav/files/user/.daily-tasks.json',
};

describe('syncWithRemote', () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    Object.defineProperty(global, 'fetch', {
      configurable: true,
      writable: true,
      value: originalFetch,
    });
    jest.restoreAllMocks();
  });

  it('pulls newer remote data even when the remote timestamp was written in seconds', async () => {
    const localData: Data = {
      last_reset: '2026-03-27',
      next_id: 2,
      tasks: [{ id: 1, title: 'Local stale task', duration: 5, status: 'todo', order: 1 }],
      theme_index: 0,
      last_modified: 1700000000000,
    };

    const remoteData = {
      last_reset: '2026-03-27',
      next_id: 2,
      tasks: [{ id: 1, title: 'CLI newer task', duration: 5, status: 'todo', order: 1 }],
      theme_index: 0,
      last_modified: 1700001000,
    };

    const fetchMock = jest.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => remoteData,
    } as any);

    Object.defineProperty(global, 'fetch', {
      configurable: true,
      writable: true,
      value: fetchMock,
    });

    const result = await syncWithRemote(settings, localData);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(result.action).toBe('pulled');
    expect(result.data.tasks[0].title).toBe('CLI newer task');
    expect(result.data.last_modified).toBe(1700001000000);
  });
});

describe('syncWithRemoteState', () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    Object.defineProperty(global, 'fetch', {
      configurable: true,
      writable: true,
      value: originalFetch,
    });
    jest.restoreAllMocks();
  });

  it('merges a remote shared history file alongside pulled data', async () => {
    const localData: Data = {
      last_reset: '2026-03-27',
      next_id: 2,
      tasks: [{ id: 1, title: 'Local stale task', duration: 5, status: 'todo', order: 1 }],
      theme_index: 0,
      last_modified: 1700000000000,
    };
    const localHistory: History = {
      version: 1,
      days: [],
      events: [],
    };

    const fetchMock = jest.fn()
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          last_reset: '2026-03-27',
          next_id: 2,
          tasks: [{ id: 1, title: 'CLI newer task', duration: 5, status: 'todo', order: 1 }],
          theme_index: 0,
          last_modified: 1700001000,
        }),
      } as any)
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          version: 1,
          days: [
            {
              date: '2026-03-26',
              tasks: [{ id: 1, title: 'CLI newer task', duration: 5, status: 'done' }],
            },
          ],
          events: [],
        }),
      } as any)
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
      } as any);

    Object.defineProperty(global, 'fetch', {
      configurable: true,
      writable: true,
      value: fetchMock,
    });

    const result = await syncWithRemoteState(settings, localData, localHistory);

    expect(result.action).toBe('pulled');
    expect(result.data.tasks[0].title).toBe('CLI newer task');
    expect(result.history.days).toHaveLength(2);
    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(fetchMock.mock.calls[1][0]).toContain('.daily-tasks.history.json');
  });
});

describe('pushRemoteHistory', () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    Object.defineProperty(global, 'fetch', {
      configurable: true,
      writable: true,
      value: originalFetch,
    });
    jest.restoreAllMocks();
  });

  it('preserves a caller-supplied updated_at instead of restamping it', async () => {
    const callerUpdatedAt = 1700000000000;
    let receivedBody: any;

    const fetchMock = jest.fn().mockImplementation(async (_url: string, init: any) => {
      receivedBody = JSON.parse(init.body as string);
      return { ok: true, status: 201 } as any;
    });

    Object.defineProperty(global, 'fetch', {
      configurable: true,
      writable: true,
      value: fetchMock,
    });

    const history: History = {
      version: 1,
      days: [],
      events: [],
      updated_at: callerUpdatedAt,
    };

    await pushRemoteHistory(settings, history);

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(receivedBody.updated_at).toBe(callerUpdatedAt);
  });

  it('stamps updated_at when the caller left it unset', async () => {
    let receivedBody: any;

    const fetchMock = jest.fn().mockImplementation(async (_url: string, init: any) => {
      receivedBody = JSON.parse(init.body as string);
      return { ok: true, status: 201 } as any;
    });

    Object.defineProperty(global, 'fetch', {
      configurable: true,
      writable: true,
      value: fetchMock,
    });

    const before = Date.now();
    await pushRemoteHistory(settings, { version: 1, days: [], events: [] });
    const after = Date.now();

    expect(receivedBody.updated_at).toBeGreaterThanOrEqual(before);
    expect(receivedBody.updated_at).toBeLessThanOrEqual(after);
  });
});

describe('Nextcloud login flow', () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    Object.defineProperty(global, 'fetch', {
      configurable: true,
      writable: true,
      value: originalFetch,
    });
    jest.restoreAllMocks();
  });

  it('starts login flow v2 and returns the browser URL plus poll metadata', async () => {
    const fetchMock = jest.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        poll: {
          token: 'poll-token',
          endpoint: 'https://cloud.example.com/login/v2/poll',
        },
        login: 'https://cloud.example.com/login/v2/flow/token',
      }),
    } as any);

    Object.defineProperty(global, 'fetch', {
      configurable: true,
      writable: true,
      value: fetchMock,
    });

    const session = await startNextcloudLogin('https://cloud.example.com/');

    expect(fetchMock).toHaveBeenCalledWith('https://cloud.example.com/index.php/login/v2', {
      method: 'POST',
    });
    expect(session.serverUrl).toBe('https://cloud.example.com');
    expect(session.pollToken).toBe('poll-token');
  });

  it('returns null while the login flow is still pending', async () => {
    const fetchMock = jest.fn().mockResolvedValue({
      ok: false,
      status: 404,
    } as any);

    Object.defineProperty(global, 'fetch', {
      configurable: true,
      writable: true,
      value: fetchMock,
    });

    const result = await pollNextcloudLogin({
      serverUrl: 'https://cloud.example.com',
      loginUrl: 'https://cloud.example.com/login/v2/flow/token',
      pollEndpoint: 'https://cloud.example.com/login/v2/poll',
      pollToken: 'poll-token',
    });

    expect(result).toBeNull();
  });

  it('returns WebDAV settings once the login flow completes', async () => {
    const fetchMock = jest.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        server: 'https://cloud.example.com/',
        loginName: 'user',
        appPassword: 'app-pass',
      }),
    } as any);

    Object.defineProperty(global, 'fetch', {
      configurable: true,
      writable: true,
      value: fetchMock,
    });

    const result = await pollNextcloudLogin({
      serverUrl: 'https://cloud.example.com',
      loginUrl: 'https://cloud.example.com/login/v2/flow/token',
      pollEndpoint: 'https://cloud.example.com/login/v2/poll',
      pollToken: 'poll-token',
    });

    expect(result).toEqual({
      baseUrl: 'https://cloud.example.com',
      username: 'user',
      password: 'app-pass',
      remotePath: '/remote.php/dav/files/user/.daily-tasks.json',
    });
  });
});
