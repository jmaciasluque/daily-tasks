import { encode as base64Encode } from 'base-64';
import type { Data, History, Settings } from '../types';
import { normalizeData } from './data';
import { emptyHistory, ensureHistorySnapshot, historyContentEqual, mergeHistories, normalizeHistory } from './history';
import { defaultRemotePath } from '../config/env';

export type LoginFlowSession = {
  serverUrl: string;
  loginUrl: string;
  pollEndpoint: string;
  pollToken: string;
};

export const defaultSettings: Settings = {
  baseUrl: '',
  username: '',
  password: '',
  remotePath: defaultRemotePath,
};

function normalizeServerUrl(serverUrl: string): string {
  return serverUrl.trim().replace(/\/+$/, '');
}

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

function buildHistoryRemotePath(remotePath: string): string {
  const slash = remotePath.lastIndexOf('/');
  const dir = slash >= 0 ? remotePath.slice(0, slash + 1) : '';
  const name = slash >= 0 ? remotePath.slice(slash + 1) : remotePath;
  const dot = name.lastIndexOf('.');
  if (dot <= 0) {
    return `${dir}${name}.history.json`;
  }
  return `${dir}${name.slice(0, dot)}.history${name.slice(dot)}`;
}

function buildHistoryWebdavUrl(settings: Settings): string {
  const base = settings.baseUrl.replace(/\/+$/, '');
  const historyPath = buildHistoryRemotePath(settings.remotePath);
  const normalizedPath = historyPath.startsWith('/') ? historyPath : `/${historyPath}`;
  return `${base}${normalizedPath}`;
}

function buildRemotePath(loginName: string): string {
  return `/remote.php/dav/files/${encodeURIComponent(loginName)}/.daily-tasks.json`;
}

export async function startNextcloudLogin(serverUrl: string): Promise<LoginFlowSession> {
  const normalizedServerUrl = normalizeServerUrl(serverUrl);
  if (!normalizedServerUrl) {
    throw new Error('Server URL is required');
  }

  const res = await fetch(`${normalizedServerUrl}/index.php/login/v2`, {
    method: 'POST',
  });
  if (!res.ok) {
    throw new Error(`Login flow start failed: ${res.status}`);
  }

  const payload = await res.json() as {
    poll?: { token?: string; endpoint?: string };
    login?: string;
  };

  if (!payload.login || !payload.poll?.token || !payload.poll.endpoint) {
    throw new Error('Login flow response was incomplete');
  }

  return {
    serverUrl: normalizedServerUrl,
    loginUrl: payload.login,
    pollEndpoint: payload.poll.endpoint,
    pollToken: payload.poll.token,
  };
}

export async function pollNextcloudLogin(session: LoginFlowSession): Promise<Settings | null> {
  const params = new URLSearchParams();
  params.set('token', session.pollToken);

  const res = await fetch(session.pollEndpoint, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/x-www-form-urlencoded',
    },
    body: params.toString(),
  });

  if (res.status === 404) {
    return null;
  }
  if (!res.ok) {
    throw new Error(`Login flow poll failed: ${res.status}`);
  }

  const payload = await res.json() as {
    server?: string;
    loginName?: string;
    appPassword?: string;
  };

  if (!payload.server || !payload.loginName || !payload.appPassword) {
    throw new Error('Login flow poll response was incomplete');
  }

  return {
    baseUrl: normalizeServerUrl(payload.server),
    username: payload.loginName.trim(),
    password: payload.appPassword.trim(),
    remotePath: buildRemotePath(payload.loginName.trim()),
  };
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

export async function fetchRemoteHistory(settings: Settings): Promise<History | null> {
  const url = buildHistoryWebdavUrl(settings);
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
    throw new Error(`History fetch failed: ${res.status}`);
  }
  return normalizeHistory(await res.json());
}

export async function pushRemoteData(settings: Settings, data: Data): Promise<void> {
  const url = buildWebdavUrl(settings);
  const dataWithTimestamp = {
    ...data,
    last_modified: data.last_modified || Date.now(),
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

export async function pushRemoteHistory(settings: Settings, history: History): Promise<void> {
  const url = buildHistoryWebdavUrl(settings);
  const payload = {
    ...normalizeHistory(history),
    updated_at: Date.now(),
  };
  const res = await fetch(url, {
    method: 'PUT',
    headers: {
      Authorization: basicAuthHeader(settings),
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload, null, 2),
  });
  if (!res.ok) {
    throw new Error(`History save failed: ${res.status}`);
  }
}

export async function pushRemoteState(settings: Settings, data: Data, history: History): Promise<void> {
  await pushRemoteData(settings, data);
  await pushRemoteHistory(settings, ensureHistorySnapshot(history, data));
}

export async function syncRemoteHistory(settings: Settings, localHistory: History, currentData: Data): Promise<History> {
  const local = normalizeHistory(localHistory);
  const remote = await fetchRemoteHistory(settings);
  const merged = ensureHistorySnapshot(
    mergeHistories(local, remote ?? emptyHistory()),
    currentData,
  );

  if (!remote) {
    if (merged.days.length > 0 || merged.events.length > 0) {
      await pushRemoteHistory(settings, merged);
    }
    return merged;
  }

  if (!historyContentEqual(merged, remote)) {
    await pushRemoteHistory(settings, merged);
  }

  return merged;
}

export type SyncResult = {
  data: Data;
  action: 'pulled' | 'pushed' | 'error' | 'in_sync';
  message: string;
};

export type SyncStateResult = SyncResult & {
  history: History;
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

    // Never overwrite remote tasks with an empty local state (e.g. fresh install or daily reset with no tasks)
    if (local.tasks.length === 0 && remote.tasks.length > 0) {
      return { data: remote, action: 'pulled', message: 'Pulled remote data' };
    }

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

export async function syncWithRemoteState(
  settings: Settings,
  localData: Data,
  localHistory: History,
): Promise<SyncStateResult> {
  const result = await syncWithRemote(settings, localData);
  if (result.action === 'error') {
    return { ...result, history: normalizeHistory(localHistory) };
  }

  const normalizedData = normalizeData(result.data);
  const history = await syncRemoteHistory(settings, localHistory, normalizedData);
  return {
    ...result,
    data: normalizedData,
    history,
  };
}
