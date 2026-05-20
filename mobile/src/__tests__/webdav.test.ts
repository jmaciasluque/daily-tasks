jest.mock('../config/env', () => ({
  defaultRemotePath: '/remote.php/dav/files/<username>/.daily-tasks.json',
}));

import { EtagMismatchError, IF_NONE_MATCH_ANY } from '../services/backend';
import { WebDAVBackend, pollNextcloudLogin, startNextcloudLogin } from '../services/backend_webdav';
import {
  fetchRemoteData,
  pushRemoteData,
  pushRemoteHistory,
  syncWithRemote,
  syncWithRemoteState,
} from '../services/sync';
import type { Data, History, Settings } from '../types';

const settings: Settings = {
  baseUrl: 'https://cloud.example.com',
  username: 'user',
  password: 'pass',
  remotePath: '/remote.php/dav/files/user/.daily-tasks.json',
};

const backend = new WebDAVBackend(settings);

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
      headers: { get: () => '"remote-etag"' },
      text: async () => JSON.stringify(remoteData),
    } as any);

    Object.defineProperty(global, 'fetch', {
      configurable: true,
      writable: true,
      value: fetchMock,
    });

    const result = await syncWithRemote(backend, localData);

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
        headers: { get: () => '"data-etag"' },
        text: async () => JSON.stringify({
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
        headers: { get: () => '"history-etag"' },
        text: async () => JSON.stringify({
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

    const result = await syncWithRemoteState(backend, localData, localHistory);

    expect(result.action).toBe('pulled');
    expect(result.data.tasks[0].title).toBe('CLI newer task');
    expect(result.history.days).toHaveLength(2);
    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(fetchMock.mock.calls[1][0]).toContain('.daily-tasks.history.json');
  });

  it('still applies pulled data when the history sync 412s twice', async () => {
    const localData: Data = {
      last_reset: '2026-03-27',
      next_id: 2,
      tasks: [{ id: 1, title: 'Local stale task', duration: 5, status: 'todo', order: 1 }],
      theme_index: 0,
      last_modified: 1700000000000,
    };
    const localHistory: History = { version: 1, days: [], events: [] };

    const fetchMock = jest.fn()
      // 1) data GET — returns newer remote, triggers a pull (no PUT for data)
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        headers: { get: () => '"data-etag"' },
        text: async () => JSON.stringify({
          last_reset: '2026-03-27',
          next_id: 2,
          tasks: [{ id: 1, title: 'CLI newer task', duration: 5, status: 'todo', order: 1 }],
          theme_index: 5,
          last_modified: 1700001000,
        }),
      } as any)
      // 2) history GET (first attempt)
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        headers: { get: () => '"history-etag-1"' },
        text: async () => JSON.stringify({ version: 1, days: [], events: [] }),
      } as any)
      // 3) history PUT — first 412
      .mockResolvedValueOnce({ ok: false, status: 412 } as any)
      // 4) history GET (retry)
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        headers: { get: () => '"history-etag-2"' },
        text: async () => JSON.stringify({ version: 1, days: [], events: [] }),
      } as any)
      // 5) history PUT — second 412 (gives up after this)
      .mockResolvedValueOnce({ ok: false, status: 412 } as any);

    Object.defineProperty(global, 'fetch', {
      configurable: true,
      writable: true,
      value: fetchMock,
    });

    const result = await syncWithRemoteState(backend, localData, localHistory);

    // Data was pulled successfully — UI should be able to apply this.
    expect(result.action).toBe('pulled');
    expect(result.data.tasks[0].title).toBe('CLI newer task');
    expect(result.data.theme_index).toBe(5);
    // History merge gave up but the action is still 'pulled', not 'error',
    // so the UI's `if (result.action !== 'error')` branch applies the data.
    expect(result.message).toContain('history merge deferred');
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

    await pushRemoteHistory(backend, history);

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
    await pushRemoteHistory(backend, { version: 1, days: [], events: [] });
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

describe('etag-aware WebDAV', () => {
  const originalFetch = global.fetch;

  afterEach(() => {
    Object.defineProperty(global, 'fetch', {
      configurable: true,
      writable: true,
      value: originalFetch,
    });
    jest.restoreAllMocks();
  });

  it('fetchRemoteData captures ETag from response headers', async () => {
    const fetchMock = jest.fn().mockResolvedValue({
      ok: true,
      status: 200,
      headers: { get: (name: string) => (name === 'ETag' ? '"abc123"' : null) },
      text: async () => JSON.stringify({
        last_reset: '2026-03-27',
        next_id: 1,
        tasks: [],
        theme_index: 0,
        last_modified: 1700000000000,
      }),
    } as any);

    Object.defineProperty(global, 'fetch', {
      configurable: true,
      writable: true,
      value: fetchMock,
    });

    const result = await fetchRemoteData(backend);
    expect(result.etag).toBe('"abc123"');
    expect(result.data).not.toBeNull();
  });

  it('pushRemoteData sends If-Match when given a concrete etag', async () => {
    let capturedHeaders: Record<string, string> | undefined;
    const fetchMock = jest.fn().mockImplementation(async (_url: string, init: any) => {
      capturedHeaders = init.headers;
      return { ok: true, status: 204 } as any;
    });

    Object.defineProperty(global, 'fetch', {
      configurable: true,
      writable: true,
      value: fetchMock,
    });

    const data: Data = {
      last_reset: '2026-03-27',
      next_id: 1,
      tasks: [],
      theme_index: 0,
      last_modified: 1700000000000,
    };

    await pushRemoteData(backend, data, '"abc123"');

    expect(capturedHeaders?.['If-Match']).toBe('"abc123"');
    expect(capturedHeaders?.['If-None-Match']).toBeUndefined();
  });

  it('pushRemoteData sends If-None-Match: * for IF_NONE_MATCH_ANY', async () => {
    let capturedHeaders: Record<string, string> | undefined;
    const fetchMock = jest.fn().mockImplementation(async (_url: string, init: any) => {
      capturedHeaders = init.headers;
      return { ok: true, status: 201 } as any;
    });

    Object.defineProperty(global, 'fetch', {
      configurable: true,
      writable: true,
      value: fetchMock,
    });

    await pushRemoteData(
      backend,
      { last_reset: '2026-03-27', next_id: 1, tasks: [], theme_index: 0, last_modified: 1700000000000 },
      IF_NONE_MATCH_ANY,
    );

    expect(capturedHeaders?.['If-None-Match']).toBe('*');
    expect(capturedHeaders?.['If-Match']).toBeUndefined();
  });

  it('pushRemoteData throws EtagMismatchError on 412', async () => {
    const fetchMock = jest.fn().mockResolvedValue({ ok: false, status: 412 } as any);
    Object.defineProperty(global, 'fetch', {
      configurable: true,
      writable: true,
      value: fetchMock,
    });

    await expect(
      pushRemoteData(
        backend,
        { last_reset: '2026-03-27', next_id: 1, tasks: [], theme_index: 0, last_modified: 1700000000000 },
        '"stale"',
      ),
    ).rejects.toBeInstanceOf(EtagMismatchError);
  });

  it('syncWithRemote retries once after 412 and pulls newer remote', async () => {
    const localData: Data = {
      last_reset: '2026-03-27',
      next_id: 2,
      tasks: [{ id: 1, title: 'Local', duration: 5, status: 'todo', order: 1 }],
      theme_index: 0,
      last_modified: 1700001000000,
    };

    const fetchMock = jest.fn()
      // First GET — returns older remote
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        headers: { get: () => '"etag-1"' },
        text: async () => JSON.stringify({
          last_reset: '2026-03-27',
          next_id: 2,
          tasks: [{ id: 1, title: 'Stale remote', duration: 5, status: 'todo', order: 1 }],
          theme_index: 0,
          last_modified: 1700000000000,
        }),
      } as any)
      // First PUT — 412 because something changed server-side
      .mockResolvedValueOnce({ ok: false, status: 412 } as any)
      // Retry GET — returns now-newer remote
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        headers: { get: () => '"etag-2"' },
        text: async () => JSON.stringify({
          last_reset: '2026-03-27',
          next_id: 2,
          tasks: [{ id: 1, title: 'Newer remote', duration: 5, status: 'todo', order: 1 }],
          theme_index: 0,
          last_modified: 1700002000000,
        }),
      } as any);

    Object.defineProperty(global, 'fetch', {
      configurable: true,
      writable: true,
      value: fetchMock,
    });

    const result = await syncWithRemote(backend, localData);
    expect(result.action).toBe('pulled');
    expect(result.data.tasks[0].title).toBe('Newer remote');
    expect(fetchMock).toHaveBeenCalledTimes(3);
  });

  it('syncWithRemote retries repeated etag races before surfacing an error', async () => {
    const localData: Data = {
      last_reset: '2026-03-27',
      next_id: 2,
      tasks: [{ id: 1, title: 'Local', duration: 5, status: 'todo', order: 1 }],
      theme_index: 0,
      last_modified: 1700001000000,
    };

    const fetchMock = jest.fn()
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        headers: { get: () => '"etag-1"' },
        text: async () => JSON.stringify({
          last_reset: '2026-03-27',
          next_id: 2,
          tasks: [{ id: 1, title: 'Remote', duration: 5, status: 'todo', order: 1 }],
          theme_index: 0,
          last_modified: 1700000000000,
        }),
      } as any)
      .mockResolvedValueOnce({ ok: false, status: 412 } as any)
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        headers: { get: () => '"etag-2"' },
        text: async () => JSON.stringify({
          last_reset: '2026-03-27',
          next_id: 2,
          tasks: [{ id: 1, title: 'Remote again', duration: 5, status: 'todo', order: 1 }],
          theme_index: 0,
          last_modified: 1700000000000,
        }),
      } as any)
      .mockResolvedValueOnce({ ok: false, status: 412 } as any)
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        headers: { get: () => '"etag-3"' },
        text: async () => JSON.stringify({
          last_reset: '2026-03-27',
          next_id: 2,
          tasks: [{ id: 1, title: 'Remote final', duration: 5, status: 'todo', order: 1 }],
          theme_index: 0,
          last_modified: 1700000000000,
        }),
      } as any)
      .mockResolvedValueOnce({ ok: true, status: 204 } as any);

    Object.defineProperty(global, 'fetch', {
      configurable: true,
      writable: true,
      value: fetchMock,
    });

    const result = await syncWithRemote(backend, localData);

    expect(result.action).toBe('pushed');
    expect(result.message).toBe('Pushed local changes');
    expect(fetchMock).toHaveBeenCalledTimes(6);
  });

  it('syncWithRemote hides raw etag wording when retry limit is exhausted', async () => {
    const localData: Data = {
      last_reset: '2026-03-27',
      next_id: 2,
      tasks: [{ id: 1, title: 'Local', duration: 5, status: 'todo', order: 1 }],
      theme_index: 0,
      last_modified: 1700001000000,
    };

    const fetchMock = jest.fn().mockImplementation(async (_url: string, init?: RequestInit) => {
      if (init?.method === 'PUT') {
        return { ok: false, status: 412 } as any;
      }
      return {
        ok: true,
        status: 200,
        headers: { get: () => '"moving-etag"' },
        text: async () => JSON.stringify({
          last_reset: '2026-03-27',
          next_id: 2,
          tasks: [{ id: 1, title: 'Remote', duration: 5, status: 'todo', order: 1 }],
          theme_index: 0,
          last_modified: 1700000000000,
        }),
      } as any;
    });

    Object.defineProperty(global, 'fetch', {
      configurable: true,
      writable: true,
      value: fetchMock,
    });

    const result = await syncWithRemote(backend, localData);

    expect(result.action).toBe('error');
    expect(result.message).toBe('Sync conflict: remote changed while saving. Please retry sync.');
    expect(result.message).not.toContain('Remote etag changed since last fetch');
  });
});
