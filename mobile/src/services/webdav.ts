import { encode as base64Encode } from 'base-64';
import type { Data, Settings } from '../types';
import { normalizeData } from './data';
import { defaultRemotePath } from '../config/env';

export const defaultSettings: Settings = {
  baseUrl: '',
  username: '',
  password: '',
  remotePath: defaultRemotePath,
};

export function isSettingsComplete(settings: Settings): boolean {
  return !!(settings.baseUrl && settings.username && settings.password && settings.remotePath);
}

function basicAuthHeader(settings: Settings): string {
  const token = `${settings.username}:${settings.password}`;
  const encoded = globalThis.btoa ? globalThis.btoa(token) : base64Encode(token);
  return `Basic ${encoded}`;
}

function buildWebdavUrl(settings: Settings): string {
  const base = settings.baseUrl.replace(/\/+$/, '');
  const path = settings.remotePath.startsWith('/') ? settings.remotePath : `/${settings.remotePath}`;
  return `${base}${path}`;
}

export async function fetchRemoteData(settings: Settings): Promise<Data | null> {
  const url = buildWebdavUrl(settings);
  const res = await fetch(url, {
    headers: {
      Authorization: basicAuthHeader(settings),
      Accept: 'application/json',
    },
  });
  if (res.status === 404) {
    return null;
  }
  if (!res.ok) {
    throw new Error(`Fetch failed: ${res.status}`);
  }
  return res.json();
}

export async function pushRemoteData(settings: Settings, data: Data): Promise<void> {
  const url = buildWebdavUrl(settings);
  const dataWithTimestamp = {
    ...data,
    last_modified: Date.now(),
  };
  const res = await fetch(url, {
    method: 'PUT',
    headers: {
      Authorization: basicAuthHeader(settings),
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(dataWithTimestamp, null, 2),
  });
  if (!res.ok) {
    throw new Error(`Save failed: ${res.status}`);
  }
}

export type SyncResult = {
  data: Data;
  action: 'pulled' | 'pushed' | 'error' | 'in_sync';
  message: string;
};

export async function syncWithRemote(settings: Settings, localData: Data): Promise<SyncResult> {
  try {
    const remoteRaw = await fetchRemoteData(settings);

    if (!remoteRaw) {
      // Remote doesn't exist, push local
      await pushRemoteData(settings, localData);
      return { data: localData, action: 'pushed', message: 'Created remote file' };
    }

    const remote = normalizeData(remoteRaw);
    const local = normalizeData(localData);

    const localTimestamp = local.last_modified || 0;
    const remoteTimestamp = remote.last_modified || 0;

    if (remoteTimestamp > localTimestamp) {
      // Remote is newer
      return { data: remote, action: 'pulled', message: 'Pulled newer remote data' };
    } else if (localTimestamp > remoteTimestamp) {
      // Local is newer
      await pushRemoteData(settings, local);
      return { data: local, action: 'pushed', message: 'Pushed local changes' };
    }

    return { data: local, action: 'in_sync', message: 'Already in sync' };
  } catch (err) {
    return { 
      data: localData, 
      action: 'error', 
      message: `Sync error: ${(err as Error).message}` 
    };
  }
}
