import AsyncStorage from '@react-native-async-storage/async-storage';
import type { Data, Settings } from '../types';
import { defaultSettings } from './webdav';
import { emptyData, normalizeData } from './data';
import { storagePrefix } from '../config/env';

const STORAGE_SETTINGS = `${storagePrefix}Settings`;
const STORAGE_CACHE = `${storagePrefix}Cache`;

export async function loadSettings(): Promise<Settings> {
  try {
    const savedSettings = await AsyncStorage.getItem(STORAGE_SETTINGS);
    if (savedSettings) {
      return { ...defaultSettings, ...JSON.parse(savedSettings) };
    }
  } catch {
    // Ignore errors, return defaults
  }
  return defaultSettings;
}

export async function saveSettings(settings: Settings): Promise<void> {
  await AsyncStorage.setItem(STORAGE_SETTINGS, JSON.stringify(settings));
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
