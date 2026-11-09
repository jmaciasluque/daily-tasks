import type { Data, History } from '../types';
import { normalizeData } from './data';
import { emptyHistory, ensureHistorySnapshot, historyContentEqual, mergeHistories, normalizeHistory } from './history';
import {
  Backend,
  EtagMismatchError,
  IF_NONE_MATCH_ANY,
  KeyData,
  KeyHistory,
} from './backend';

export type FetchedData = { data: Data | null; etag: string };
export type FetchedHistory = { history: History | null; etag: string };

// fetchRemoteData fetches the data blob via the supplied backend and
// returns the parsed body alongside its concurrency token. Pass the
// token back to pushRemoteData as ifMatch to detect concurrent writers.
export async function fetchRemoteData(backend: Backend): Promise<FetchedData> {
  const blob = await backend.fetch(KeyData);
  if (blob.bytes === null) {
    return { data: null, etag: blob.etag };
  }
  return { data: JSON.parse(blob.bytes) as Data, etag: blob.etag };
}

// pushRemoteData encodes and writes data via the backend. ifMatch follows
// the Backend.push contract (undefined unconditional, IF_NONE_MATCH_ANY
// create-only, otherwise an etag from a prior fetch). Throws
// EtagMismatchError on 412.
export async function pushRemoteData(backend: Backend, data: Data, ifMatch?: string): Promise<void> {
  const dataWithTimestamp = {
    ...data,
    last_modified: data.last_modified || Date.now(),
  };
  await backend.push(KeyData, JSON.stringify(dataWithTimestamp, null, 2), ifMatch);
}

export async function fetchRemoteHistory(backend: Backend): Promise<FetchedHistory> {
  const blob = await backend.fetch(KeyHistory);
  if (blob.bytes === null) {
    return { history: null, etag: blob.etag };
  }
  return { history: normalizeHistory(JSON.parse(blob.bytes)), etag: blob.etag };
}

export async function pushRemoteHistory(backend: Backend, history: History, ifMatch?: string): Promise<void> {
  const normalized = normalizeHistory(history);
  const payload = {
    ...normalized,
    updated_at: normalized.updated_at || Date.now(),
  };
  await backend.push(KeyHistory, JSON.stringify(payload, null, 2), ifMatch);
}

// pushRemoteState writes both data and history unconditionally — used by
// the dedicated "Save to Nextcloud" flow that is meant to overwrite. Sync
// flows go through syncWithRemote / syncWithRemoteState instead, which
// fetch first and use If-Match to avoid clobbering concurrent writers.
export async function pushRemoteState(backend: Backend, data: Data, history: History): Promise<void> {
  await pushRemoteData(backend, data);
  await pushRemoteHistory(backend, ensureHistorySnapshot(history, data));
}

export async function syncRemoteHistory(backend: Backend, localHistory: History, currentData: Data): Promise<History> {
  try {
    return await mergeAndPushHistory(backend, localHistory, currentData);
  } catch (err) {
    if (err instanceof EtagMismatchError) {
      // Remote etag moved between our fetch and PUT — retry once with the
      // latest server state folded in.
      return mergeAndPushHistory(backend, localHistory, currentData);
    }
    throw err;
  }
}

async function mergeAndPushHistory(
  backend: Backend,
  localHistory: History,
  currentData: Data,
): Promise<History> {
  const local = normalizeHistory(localHistory);
  const { history: remote, etag } = await fetchRemoteHistory(backend);
  const merged = ensureHistorySnapshot(
    mergeHistories(local, remote ?? emptyHistory()),
    currentData,
  );

  if (!remote) {
    if (merged.days.length > 0 || merged.events.length > 0) {
      await pushRemoteHistory(backend, merged, IF_NONE_MATCH_ANY);
    }
    return merged;
  }

  if (!historyContentEqual(merged, remote)) {
    await pushRemoteHistory(backend, merged, etag);
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

const syncEtagRetryLimit = 4;

async function syncOnce(backend: Backend, localData: Data): Promise<SyncResult> {
  const { data: remoteRaw, etag } = await fetchRemoteData(backend);

  if (!remoteRaw) {
    await pushRemoteData(backend, localData, IF_NONE_MATCH_ANY);
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
    await pushRemoteData(backend, local, etag);
    return { data: local, action: 'pushed', message: 'Pushed local changes' };
  }

  return { data: local, action: 'in_sync', message: 'Already in sync' };
}

export async function syncWithRemote(backend: Backend, localData: Data): Promise<SyncResult> {
  for (let attempt = 0; attempt < syncEtagRetryLimit; attempt += 1) {
    try {
      return await syncOnce(backend, localData);
    } catch (err) {
      if (!(err instanceof EtagMismatchError)) {
        return {
          data: localData,
          action: 'error',
          message: `Sync error: ${(err as Error).message}`,
        };
      }
    }
  }

  return {
    data: localData,
    action: 'error',
    message: 'Sync conflict: remote changed while saving. Please retry sync.',
  };
}

export async function syncWithRemoteState(
  backend: Backend,
  localData: Data,
  localHistory: History,
): Promise<SyncStateResult> {
  const result = await syncWithRemote(backend, localData);
  if (result.action === 'error') {
    return { ...result, history: normalizeHistory(localHistory) };
  }

  const normalizedData = normalizeData(result.data);

  // The history merge can lose its etag race even when the data sync above
  // succeeded. Surface that as an info-level warning rather than failing the
  // whole sync — the user-visible task data should still be applied to the
  // UI. The next sync attempt will reconcile the history file.
  try {
    const history = await syncRemoteHistory(backend, localHistory, normalizedData);
    return {
      ...result,
      data: normalizedData,
      history,
    };
  } catch (err) {
    return {
      ...result,
      data: normalizedData,
      history: normalizeHistory(localHistory),
      message: `${result.message}; history merge deferred (${(err as Error).message})`,
    };
  }
}
