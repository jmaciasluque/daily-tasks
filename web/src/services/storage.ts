import type { Data, Settings } from '../types';
import { defaultSettings } from './webdav';
import { emptyData, normalizeData } from './data';

const STORAGE_SETTINGS = 'dailyTasksSettings';
const STORAGE_CACHE = 'dailyTasksCache';

export async function loadSettings(): Promise<Settings> {
  try {
    const saved = localStorage.getItem(STORAGE_SETTINGS);
    if (saved) return { ...defaultSettings, ...JSON.parse(saved) };
  } catch { /* ignore */ }
  return defaultSettings;
}

export async function saveSettings(settings: Settings): Promise<void> {
  localStorage.setItem(STORAGE_SETTINGS, JSON.stringify(settings));
}

export async function loadCachedData(): Promise<Data> {
  try {
    const cached = localStorage.getItem(STORAGE_CACHE);
    if (cached) return normalizeData(JSON.parse(cached));
  } catch { /* ignore */ }
  return emptyData();
}

export async function saveCachedData(data: Data): Promise<void> {
  try {
    localStorage.setItem(STORAGE_CACHE, JSON.stringify(data));
  } catch { /* ignore */ }
}
