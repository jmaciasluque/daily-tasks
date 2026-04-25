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

// Sentinel passed to pushRemoteData / pushRemoteHistory as ifMatch when the
// caller wants the PUT to succeed only if the resource does not yet exist
// (translated to an If-None-Match: * request header).
export const IF_NONE_MATCH_ANY = '*';

// Thrown by pushRemoteData / pushRemoteHistory when the server returns
// 412 Precondition Failed — i.e. the resource changed since the etag we
// passed in If-Match was issued. Callers can catch this to refetch and merge.
export class EtagMismatchError extends Error {
  constructor(message = 'Remote etag changed since last fetch') {
    super(message);
    this.name = 'EtagMismatchError';
  }
}

export type FetchedData = { data: Data | null; etag: string };
export type FetchedHistory = { history: History | null; etag: string };

function applyIfMatch(headers: Record<string, string>, ifMatch?: string): void {
  if (!ifMatch) {
    return;
  }
  if (ifMatch === IF_NONE_MATCH_ANY) {
    headers['If-None-Match'] = '*';
  } else {
    headers['If-Match'] = ifMatch;
  }
}

export async function fetchRemoteData(settings: Settings): Promise<FetchedData> {
  const url = buildWebdavUrl(settings);
  const res = await fetch(url, {
    headers: {
      Authorization: basicAuthHeader(settings),
      Accept: 'application/json',
    },
  });
  if (res.status === 404) {
    return { data: null, etag: '' };
  }
  if (!res.ok) {
    throw new Error(`Fetch failed: ${res.status}`);
  }
  const data = (await res.json()) as Data;
  return { data, etag: res.headers.get('ETag') ?? '' };
}

export async function fetchRemoteHistory(settings: Settings): Promise<FetchedHistory> {
  const url = buildHistoryWebdavUrl(settings);
  const res = await fetch(url, {
    headers: {
      Authorization: basicAuthHeader(settings),
      Accept: 'application/json',
    },
  });
  if (res.status === 404) {
    return { history: null, etag: '' };
  }
  if (!res.ok) {
    throw new Error(`History fetch failed: ${res.status}`);
  }
  return { history: normalizeHistory(await res.json()), etag: res.headers.get('ETag') ?? '' };
}

// pushRemoteData PUTs the data to the remote WebDAV server. Pass `ifMatch`
// to make the request conditional: an opaque ETag to require the resource
// still match it (If-Match), or IF_NONE_MATCH_ANY to require the resource
// not yet exist (If-None-Match: *). Throws EtagMismatchError on 412.
export async function pushRemoteData(settings: Settings, data: Data, ifMatch?: string): Promise<void> {
  const url = buildWebdavUrl(settings);
  const dataWithTimestamp = {
    ...data,
    last_modified: data.last_modified || Date.now(),
  };
  const headers: Record<string, string> = {
    Authorization: basicAuthHeader(settings),
    'Content-Type': 'application/json',
  };
  applyIfMatch(headers, ifMatch);
  const res = await fetch(url, {
    method: 'PUT',
    headers,
    body: JSON.stringify(dataWithTimestamp, null, 2),
  });
  if (res.status === 412) {
    throw new EtagMismatchError();
  }
  if (!res.ok) {
    throw new Error(`Save failed: ${res.status}`);
  }
}

export async function pushRemoteHistory(settings: Settings, history: History, ifMatch?: string): Promise<void> {
  const url = buildHistoryWebdavUrl(settings);
  const normalized = normalizeHistory(history);
  const payload = {
    ...normalized,
    updated_at: normalized.updated_at || Date.now(),
  };
  const headers: Record<string, string> = {
    Authorization: basicAuthHeader(settings),
    'Content-Type': 'application/json',
  };
  applyIfMatch(headers, ifMatch);
  const res = await fetch(url, {
    method: 'PUT',
    headers,
    body: JSON.stringify(payload, null, 2),
  });
  if (res.status === 412) {
    throw new EtagMismatchError();
  }
  if (!res.ok) {
    throw new Error(`History save failed: ${res.status}`);
  }
}

// pushRemoteState writes both data and history unconditionally — used by the
// dedicated "Save to Nextcloud" flow that is meant to overwrite. Sync flows
// go through syncWithRemote / syncWithRemoteState instead, which fetch first
// and use If-Match to avoid clobbering concurrent writers.
export async function pushRemoteState(settings: Settings, data: Data, history: History): Promise<void> {
  await pushRemoteData(settings, data);
  await pushRemoteHistory(settings, ensureHistorySnapshot(history, data));
}

export async function syncRemoteHistory(settings: Settings, localHistory: History, currentData: Data): Promise<History> {
  try {
    return await mergeAndPushHistory(settings, localHistory, currentData);
  } catch (err) {
    if (err instanceof EtagMismatchError) {
      // Remote etag moved between our fetch and PUT — retry once with the
      // latest server state folded in.
      return mergeAndPushHistory(settings, localHistory, currentData);
    }
    throw err;
  }
}

async function mergeAndPushHistory(
  settings: Settings,
  localHistory: History,
  currentData: Data,
): Promise<History> {
  const local = normalizeHistory(localHistory);
  const { history: remote, etag } = await fetchRemoteHistory(settings);
  const merged = ensureHistorySnapshot(
    mergeHistories(local, remote ?? emptyHistory()),
    currentData,
  );

  if (!remote) {
    if (merged.days.length > 0 || merged.events.length > 0) {
      await pushRemoteHistory(settings, merged, IF_NONE_MATCH_ANY);
    }
    return merged;
  }

  if (!historyContentEqual(merged, remote)) {
    await pushRemoteHistory(settings, merged, etag);
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

async function syncOnce(settings: Settings, localData: Data): Promise<SyncResult> {
  const { data: remoteRaw, etag } = await fetchRemoteData(settings);

  if (!remoteRaw) {
    await pushRemoteData(settings, localData, IF_NONE_MATCH_ANY);
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
    return { data: remote, action: 'pulled', message: 'Pulled newer remote data' };
  } else if (localTimestamp > remoteTimestamp) {
    await pushRemoteData(settings, local, etag);
    return { data: local, action: 'pushed', message: 'Pushed local changes' };
  }

  return { data: local, action: 'in_sync', message: 'Already in sync' };
}

export async function syncWithRemote(settings: Settings, localData: Data): Promise<SyncResult> {
  try {
    return await syncOnce(settings, localData);
  } catch (err) {
    if (err instanceof EtagMismatchError) {
      try {
        return await syncOnce(settings, localData);
      } catch (retryErr) {
        return {
          data: localData,
          action: 'error',
          message: `Sync error: ${(retryErr as Error).message}`,
        };
      }
    }
    return {
      data: localData,
      action: 'error',
      message: `Sync error: ${(err as Error).message}`,
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
