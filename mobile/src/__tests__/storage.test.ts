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
import { loadAppConfig, saveAppConfig } from '../services/storage';

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
});
