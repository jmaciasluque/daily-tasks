import AsyncStorage from '@react-native-async-storage/async-storage';
import * as SecureStore from 'expo-secure-store';
import type { AppConfig, Data, History, Settings } from '../types';
import { defaultSettings, WebDAVBackend } from './backend_webdav';
import { defaultHostedSettings, HostedBackend } from './backend_hosted';
import type { Backend } from './backend';
import { emptyData, normalizeData } from './data';
import { emptyHistory, normalizeHistory } from './history';
import { storagePrefix } from '../config/env';

const STORAGE_SETTINGS = `${storagePrefix}Settings`;
const STORAGE_BACKEND_CONFIG = `${storagePrefix}BackendConfig`;
const STORAGE_HOSTED_TOKEN = `${storagePrefix}HostedToken`;
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
  hosted?: {
    api_url?: string;
    email?: string;
  };
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

  if (config.backend === 'hosted') {
    return {
      backend: 'hosted',
      hosted: {
        ...defaultHostedSettings,
        ...(config.hosted ?? {}),
      },
    };
  }

  return {};
}

async function fromPersistedConfig(config: PersistedAppConfig): Promise<AppConfig> {
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

  if (config.backend === 'hosted') {
    const token = await SecureStore.getItemAsync(STORAGE_HOSTED_TOKEN).catch(() => null);
    return normalizeAppConfig({
      backend: 'hosted',
      hosted: {
        apiUrl: config.hosted?.api_url ?? defaultHostedSettings.apiUrl,
        email: config.hosted?.email,
        token: token ?? undefined,
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

  if (config.backend === 'hosted' && config.hosted) {
    return {
      backend: 'hosted',
      hosted: {
        api_url: config.hosted.apiUrl,
        email: config.hosted.email,
      },
    };
  }

  return {};
}

export async function loadAppConfig(): Promise<AppConfig> {
  try {
    const savedConfig = await AsyncStorage.getItem(STORAGE_BACKEND_CONFIG);
    if (savedConfig) {
      return await fromPersistedConfig(JSON.parse(savedConfig));
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
  const normalized = normalizeAppConfig(config);
  if (normalized.backend === 'hosted' && normalized.hosted?.token) {
    await SecureStore.setItemAsync(STORAGE_HOSTED_TOKEN, normalized.hosted.token);
  } else if (normalized.backend !== 'hosted') {
    await SecureStore.deleteItemAsync(STORAGE_HOSTED_TOKEN).catch(() => {});
  }
  await AsyncStorage.setItem(STORAGE_BACKEND_CONFIG, JSON.stringify(toPersistedConfig(normalized)));
}

export function nextcloudSettingsFromConfig(config: AppConfig): Settings {
  return { ...defaultSettings, ...(config.nextcloud ?? {}) };
}

// backendFromConfig returns a runtime Backend for the configured remote
// store, or null when the user has not finished setup.
export function backendFromConfig(config: AppConfig): Backend | null {
  if (config.backend === 'nextcloud') {
    const settings = nextcloudSettingsFromConfig(config);
    if (!settings.baseUrl || !settings.username || !settings.password || !settings.remotePath) {
      return null;
    }
    return new WebDAVBackend(settings);
  }

  if (config.backend === 'hosted') {
    const hosted = { ...defaultHostedSettings, ...(config.hosted ?? {}) };
    if (!hosted.apiUrl || !hosted.token) {
      return null;
    }
    return new HostedBackend(hosted);
  }

  return null;
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
