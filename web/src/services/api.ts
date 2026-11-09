import type { Data, NextcloudSetupPoll, NextcloudSetupStart, ServerState, StatsSummary } from '../types';
import { normalizeData } from './data';

type ErrorPayload = {
  error?: string;
  message?: string;
};

async function requestJSON<T>(input: RequestInfo, init?: RequestInit): Promise<T> {
  const response = await fetch(input, init);
  const text = await response.text();
  let payload: T | ErrorPayload | null = null;
  if (text) {
    try {
      payload = JSON.parse(text) as T | ErrorPayload;
    } catch {
      if (!response.ok) {
        throw new Error(text.trim() || `Request failed: ${response.status}`);
      }
      throw new Error('Server returned invalid JSON');
    }
  }

  if (!response.ok) {
    const errorMessage =
      (payload as ErrorPayload | null)?.error ||
      (payload as ErrorPayload | null)?.message ||
      `Request failed: ${response.status}`;
    throw new Error(errorMessage);
  }

  return payload as T;
}

function normalizeState(state: ServerState): ServerState {
  return {
    ...state,
    data: normalizeData(state.data),
  };
}

export async function fetchServerState(): Promise<ServerState> {
  return normalizeState(await requestJSON<ServerState>('/api/state'));
}

export async function saveServerData(data: Data): Promise<ServerState> {
  return normalizeState(await requestJSON<ServerState>('/api/data', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(data),
  }));
}

export async function syncServerData(): Promise<ServerState> {
  return normalizeState(await requestJSON<ServerState>('/api/sync', {
    method: 'POST',
  }));
}

export async function fetchServerStats(from: string, to: string): Promise<StatsSummary> {
  const params = new URLSearchParams({ from, to });
  const payload = await requestJSON<{ stats: StatsSummary }>(`/api/stats?${params.toString()}`);
  return payload.stats;
}

export async function setupLocalBackend(): Promise<void> {
  await requestJSON<{ status: string }>('/api/setup/local', {
    method: 'POST',
  });
}

export async function startNextcloudSetup(serverUrl: string): Promise<NextcloudSetupStart> {
  return requestJSON<NextcloudSetupStart>('/api/setup/nextcloud/start', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ server_url: serverUrl }),
  });
}

export async function pollNextcloudSetup(sessionId: string): Promise<NextcloudSetupPoll> {
  const params = new URLSearchParams({ session: sessionId });
  return requestJSON<NextcloudSetupPoll>(`/api/setup/nextcloud/poll?${params.toString()}`);
}
