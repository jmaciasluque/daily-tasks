jest.mock('../config/env', () => ({
  defaultRemotePath: '/remote.php/dav/files/<username>/.daily-tasks.json',
}));

import { syncWithRemote } from '../services/webdav';
import type { Data, Settings } from '../types';

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
