import AsyncStorage from '@react-native-async-storage/async-storage';
import type { AppConfig, Data, History, Settings } from '../types';
import { defaultSettings, WebDAVBackend } from './backend_webdav';
import type { Backend } from './backend';
import { emptyData, normalizeData } from './data';
import { emptyHistory, normalizeHistory } from './history';
import { storagePrefix } from '../config/env';

const STORAGE_SETTINGS = `${storagePrefix}Settings`;
const STORAGE_BACKEND_CONFIG = `${storagePrefix}BackendConfig`;
const STORAGE_CACHE = `${storagePrefix}Cache`;
const STORAGE_HISTORY = `${storagePrefix}History`;

type PersistedNextcloudConfig = {
  server_url?: string;
  login_name?: string;
  app_password?: string;
  remote_path?: string;
};

type PersistedAppConfig = {
  backend?: AppConfig['backend'];
  nextcloud?: PersistedNextcloudConfig;
};

function normalizeAppConfig(config: AppConfig): AppConfig {
  if (config.backend === 'local') {
    return { backend: 'local' };
  }

  if (config.backend === 'nextcloud') {
    return {
      backend: 'nextcloud',
      nextcloud: {
        ...defaultSettings,
        ...(config.nextcloud ?? {}),
      },
    };
  }

  return {};
}

function fromPersistedConfig(config: PersistedAppConfig): AppConfig {
  if (config.backend === 'local') {
    return { backend: 'local' };
  }

  if (config.backend === 'nextcloud') {
    return normalizeAppConfig({
      backend: 'nextcloud',
      nextcloud: {
        baseUrl: config.nextcloud?.server_url ?? '',
        username: config.nextcloud?.login_name ?? '',
        password: config.nextcloud?.app_password ?? '',
        remotePath: config.nextcloud?.remote_path ?? '',
      },
    });
  }

  return {};
}

function toPersistedConfig(config: AppConfig): PersistedAppConfig {
  if (config.backend === 'local') {
    return { backend: 'local' };
  }

  if (config.backend === 'nextcloud' && config.nextcloud) {
    return {
      backend: 'nextcloud',
      nextcloud: {
        server_url: config.nextcloud.baseUrl,
        login_name: config.nextcloud.username,
        app_password: config.nextcloud.password,
        remote_path: config.nextcloud.remotePath,
      },
    };
  }

  return {};
}

export async function loadAppConfig(): Promise<AppConfig> {
  try {
    const savedConfig = await AsyncStorage.getItem(STORAGE_BACKEND_CONFIG);
    if (savedConfig) {
      return fromPersistedConfig(JSON.parse(savedConfig));
    }

    const savedSettings = await AsyncStorage.getItem(STORAGE_SETTINGS);
    if (savedSettings) {
      const legacySettings = { ...defaultSettings, ...JSON.parse(savedSettings) };
      if (legacySettings.baseUrl && legacySettings.username && legacySettings.password && legacySettings.remotePath) {
        const migrated = normalizeAppConfig({
          backend: 'nextcloud',
          nextcloud: legacySettings,
        });
        await saveAppConfig(migrated);
        await AsyncStorage.removeItem(STORAGE_SETTINGS).catch(() => {});
        return migrated;
      }
    }
  } catch {
    // Ignore errors, return empty config
  }
  return {};
}

export async function saveAppConfig(config: AppConfig): Promise<void> {
  await AsyncStorage.setItem(STORAGE_BACKEND_CONFIG, JSON.stringify(toPersistedConfig(normalizeAppConfig(config))));
}

export function nextcloudSettingsFromConfig(config: AppConfig): Settings {
  return { ...defaultSettings, ...(config.nextcloud ?? {}) };
}

// backendFromConfig returns a runtime Backend for the configured remote
// store, or null when the user has not finished setup. Today the only
// remote backend is WebDAV (Nextcloud); future backends slot in here.
export function backendFromConfig(config: AppConfig): Backend | null {
  if (config.backend !== 'nextcloud') {
    return null;
  }
  const settings = nextcloudSettingsFromConfig(config);
  if (!settings.baseUrl || !settings.username || !settings.password || !settings.remotePath) {
    return null;
  }
  return new WebDAVBackend(settings);
}

export async function loadCachedData(): Promise<Data> {
  try {
    const cached = await AsyncStorage.getItem(STORAGE_CACHE);
    if (cached) {
      return normalizeData(JSON.parse(cached));
    }
  } catch {
    // Ignore errors, return empty data
  }
  return emptyData();
}

export async function saveCachedData(data: Data): Promise<void> {
  try {
    await AsyncStorage.setItem(STORAGE_CACHE, JSON.stringify(data));
  } catch {
    // Ignore errors silently
  }
}

export async function loadCachedHistory(): Promise<History> {
  try {
    const cached = await AsyncStorage.getItem(STORAGE_HISTORY);
    if (cached) {
      return normalizeHistory(JSON.parse(cached));
    }
  } catch {
    // Ignore errors, return empty history
  }
  return emptyHistory();
}

export async function saveCachedHistory(history: History): Promise<void> {
  try {
    await AsyncStorage.setItem(STORAGE_HISTORY, JSON.stringify(history));
  } catch {
    // Ignore errors silently
  }
}
