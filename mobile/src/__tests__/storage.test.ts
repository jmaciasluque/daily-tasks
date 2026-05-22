jest.mock('../config/env', () => ({
  storagePrefix: 'dailyTasks',
  defaultRemotePath: '/remote.php/dav/files/<username>/.daily-tasks.json',
}));

jest.mock('@react-native-async-storage/async-storage', () => ({
  __esModule: true,
  default: {
    getItem: jest.fn().mockResolvedValue(null),
    setItem: jest.fn().mockResolvedValue(undefined),
    removeItem: jest.fn().mockResolvedValue(undefined),
  },
}));

import AsyncStorage from '@react-native-async-storage/async-storage';
import {
  loadAppConfig,
  saveAppConfig,
  nextcloudSettingsFromConfig,
  backendFromConfig,
  loadCachedData,
  saveCachedData,
  loadCachedHistory,
  saveCachedHistory,
} from '../services/storage';
import { WebDAVBackend } from '../services/backend_webdav';

describe('backend config storage', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('loads the shared persisted backend schema into mobile settings', async () => {
    (AsyncStorage.getItem as jest.Mock).mockResolvedValueOnce(JSON.stringify({
      backend: 'nextcloud',
      nextcloud: {
        server_url: 'https://cloud.example.com',
        login_name: 'user',
        app_password: 'app-pass',
        remote_path: '/remote.php/dav/files/user/.daily-tasks.json',
      },
    }));

    const config = await loadAppConfig();

    expect(config).toEqual({
      backend: 'nextcloud',
      nextcloud: {
        baseUrl: 'https://cloud.example.com',
        username: 'user',
        password: 'app-pass',
        remotePath: '/remote.php/dav/files/user/.daily-tasks.json',
      },
    });
  });

  it('migrates legacy manual settings into the new backend config', async () => {
    (AsyncStorage.getItem as jest.Mock)
      .mockResolvedValueOnce(null)
      .mockResolvedValueOnce(JSON.stringify({
        baseUrl: 'https://cloud.example.com',
        username: 'user',
        password: 'app-pass',
        remotePath: '/remote.php/dav/files/user/.daily-tasks.json',
      }));

    const config = await loadAppConfig();

    expect(config.backend).toBe('nextcloud');
    expect(AsyncStorage.setItem).toHaveBeenCalledWith(
      'dailyTasksBackendConfig',
      JSON.stringify({
        backend: 'nextcloud',
        nextcloud: {
          server_url: 'https://cloud.example.com',
          login_name: 'user',
          app_password: 'app-pass',
          remote_path: '/remote.php/dav/files/user/.daily-tasks.json',
        },
      }),
    );
  });

  it('writes the shared snake_case schema when saving config', async () => {
    await saveAppConfig({
      backend: 'nextcloud',
      nextcloud: {
        baseUrl: 'https://cloud.example.com',
        username: 'user',
        password: 'app-pass',
        remotePath: '/remote.php/dav/files/user/.daily-tasks.json',
      },
    });

    expect(AsyncStorage.setItem).toHaveBeenCalledWith(
      'dailyTasksBackendConfig',
      JSON.stringify({
        backend: 'nextcloud',
        nextcloud: {
          server_url: 'https://cloud.example.com',
          login_name: 'user',
          app_password: 'app-pass',
          remote_path: '/remote.php/dav/files/user/.daily-tasks.json',
        },
      }),
    );
  });

  it('loads backend: local config', async () => {
    (AsyncStorage.getItem as jest.Mock).mockResolvedValueOnce(
      JSON.stringify({ backend: 'local' }),
    );

    const config = await loadAppConfig();

    expect(config).toEqual({ backend: 'local' });
  });

  it('returns empty config when no saved config and no legacy settings', async () => {
    (AsyncStorage.getItem as jest.Mock)
      .mockResolvedValueOnce(null)
      .mockResolvedValueOnce(null);

    const config = await loadAppConfig();

    expect(config).toEqual({});
  });

  it('returns empty config when AsyncStorage.getItem throws', async () => {
    (AsyncStorage.getItem as jest.Mock).mockRejectedValueOnce(new Error('storage error'));

    const config = await loadAppConfig();

    expect(config).toEqual({});
  });

  it('saves backend: local config', async () => {
    await saveAppConfig({ backend: 'local' });

    expect(AsyncStorage.setItem).toHaveBeenCalledWith(
      'dailyTasksBackendConfig',
      JSON.stringify({ backend: 'local' }),
    );
  });

  it('saves empty config', async () => {
    await saveAppConfig({});

    expect(AsyncStorage.setItem).toHaveBeenCalledWith(
      'dailyTasksBackendConfig',
      JSON.stringify({}),
    );
  });
});

describe('nextcloudSettingsFromConfig', () => {
  it('returns defaults when config has no nextcloud field', () => {
    const settings = nextcloudSettingsFromConfig({});

    expect(settings).toEqual({
      baseUrl: '',
      username: '',
      password: '',
      remotePath: '/remote.php/dav/files/<username>/.daily-tasks.json',
    });
  });

  it('merges provided nextcloud fields over defaults', () => {
    const settings = nextcloudSettingsFromConfig({
      backend: 'nextcloud',
      nextcloud: {
        baseUrl: 'https://cloud.example.com',
        username: 'user',
        password: 'pass',
        remotePath: '/remote.php/dav/files/user/.daily-tasks.json',
      },
    });

    expect(settings).toEqual({
      baseUrl: 'https://cloud.example.com',
      username: 'user',
      password: 'pass',
      remotePath: '/remote.php/dav/files/user/.daily-tasks.json',
    });
  });
});

describe('backendFromConfig', () => {
  it('returns null for non-nextcloud backend', () => {
    expect(backendFromConfig({})).toBeNull();
    expect(backendFromConfig({ backend: 'local' })).toBeNull();
  });

  it('returns null when nextcloud credentials are incomplete', () => {
    expect(
      backendFromConfig({
        backend: 'nextcloud',
        nextcloud: { baseUrl: '', username: 'user', password: 'pass', remotePath: '/path' },
      }),
    ).toBeNull();
  });

  it('returns a WebDAVBackend instance when credentials are complete', () => {
    const result = backendFromConfig({
      backend: 'nextcloud',
      nextcloud: {
        baseUrl: 'https://cloud.example.com',
        username: 'user',
        password: 'pass',
        remotePath: '/remote.php/dav/files/user/.daily-tasks.json',
      },
    });

    expect(result).toBeInstanceOf(WebDAVBackend);
  });
});

describe('loadCachedData', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('returns normalised data from cache on hit', async () => {
    const stored = {
      last_reset: '2026-01-01',
      next_id: 5,
      tasks: [],
      theme_index: 2,
      last_modified: 1700000000000,
    };
    (AsyncStorage.getItem as jest.Mock).mockResolvedValueOnce(JSON.stringify(stored));

    const data = await loadCachedData();

    expect(data.next_id).toBe(5);
    expect(data.theme_index).toBe(2);
    expect(data.tasks).toEqual([]);
  });

  it('returns emptyData on cache miss (null)', async () => {
    (AsyncStorage.getItem as jest.Mock).mockResolvedValueOnce(null);

    const data = await loadCachedData();

    expect(data.next_id).toBe(1);
    expect(data.tasks).toEqual([]);
  });

  it('returns emptyData when AsyncStorage throws', async () => {
    (AsyncStorage.getItem as jest.Mock).mockRejectedValueOnce(new Error('disk full'));

    const data = await loadCachedData();

    expect(data.tasks).toEqual([]);
  });
});

describe('saveCachedData', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('stores serialised data under the cache key', async () => {
    const data = {
      last_reset: '2026-01-01',
      next_id: 1,
      tasks: [],
      theme_index: 0,
      last_modified: 0,
    };

    await saveCachedData(data);

    expect(AsyncStorage.setItem).toHaveBeenCalledWith(
      'dailyTasksCache',
      JSON.stringify(data),
    );
  });

  it('swallows errors silently', async () => {
    (AsyncStorage.setItem as jest.Mock).mockRejectedValueOnce(new Error('quota'));

    await expect(saveCachedData({ last_reset: '2026-01-01', next_id: 1, tasks: [], theme_index: 0 })).resolves.toBeUndefined();
  });
});

describe('loadCachedHistory', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('returns normalised history from cache on hit', async () => {
    const stored = { version: 1, days: [], events: [] };
    (AsyncStorage.getItem as jest.Mock).mockResolvedValueOnce(JSON.stringify(stored));

    const history = await loadCachedHistory();

    expect(history.version).toBe(1);
    expect(history.days).toEqual([]);
    expect(history.events).toEqual([]);
  });

  it('returns emptyHistory on cache miss (null)', async () => {
    (AsyncStorage.getItem as jest.Mock).mockResolvedValueOnce(null);

    const history = await loadCachedHistory();

    expect(history.version).toBe(1);
    expect(history.days).toEqual([]);
  });

  it('returns emptyHistory when AsyncStorage throws', async () => {
    (AsyncStorage.getItem as jest.Mock).mockRejectedValueOnce(new Error('io error'));

    const history = await loadCachedHistory();

    expect(history.days).toEqual([]);
  });
});

describe('saveCachedHistory', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('stores serialised history under the history key', async () => {
    const history = { version: 1, days: [], events: [] };

    await saveCachedHistory(history);

    expect(AsyncStorage.setItem).toHaveBeenCalledWith(
      'dailyTasksHistory',
      JSON.stringify(history),
    );
  });

  it('swallows errors silently', async () => {
    (AsyncStorage.setItem as jest.Mock).mockRejectedValueOnce(new Error('quota'));

    await expect(saveCachedHistory({ version: 1, days: [], events: [] })).resolves.toBeUndefined();
  });
});
