import { decode as base64Decode } from 'base-64';
import { Code, ConnectError, createClient, type Client, type Interceptor, type Transport } from '@connectrpc/connect';
import { createConnectTransport } from '@connectrpc/connect-web';
import { AccountService } from '../gen/dailytasks/v1/account_pb';
import { SyncService, type GetStateResponse } from '../gen/dailytasks/v1/sync_pb';
import {
  Backend,
  Blob,
  EtagMismatchError,
  IF_NONE_MATCH_ANY,
  KeyData,
  KeyHistory,
} from './backend';
import { utf8Decode, utf8Encode } from './utf8';

export type HostedProvider = 'google';

export type HostedSettings = {
  apiUrl: string;
  token?: string;
  email?: string;
};

export class HostedAuthError extends Error {
  constructor(message = 'Your session expired. Sign in again.') {
    super(message);
    this.name = 'HostedAuthError';
  }
}

// Platform-neutral: shared by the mobile app and the hosted web UI. Callers
// identify themselves once with setHostedClientName().
let clientName = 'daily-tasks';

export function setHostedClientName(name: string): void {
  clientName = name;
}

// A cold Fly machine takes ~1 s to start; leave generous headroom on mobile
// networks.
const requestTimeoutMs = 20000;

export function normalizeApiUrl(apiUrl: string): string {
  return apiUrl.trim().replace(/\/+$/, '');
}

/** Builds the Connect transport for the hosted API (JSON over fetch). */
export function createHostedTransport(apiUrl: string, getToken: () => string | undefined): Transport {
  const auth: Interceptor = (next) => async (req) => {
    const token = getToken();
    if (token) {
      req.header.set('Authorization', `Bearer ${token}`);
    }
    req.header.set('X-Client', clientName);
    return next(req);
  };
  return createConnectTransport({
    baseUrl: normalizeApiUrl(apiUrl),
    // JSON keeps payloads debuggable and avoids binary fetch bodies on RN.
    useBinaryFormat: false,
    interceptors: [auth],
    defaultTimeoutMs: requestTimeoutMs,
  });
}

function toBackendError(err: unknown): Error {
  const ce = ConnectError.from(err);
  if (ce.code === Code.Unauthenticated || ce.code === Code.PermissionDenied) {
    return new HostedAuthError();
  }
  if (ce.code === Code.FailedPrecondition) {
    return new EtagMismatchError();
  }
  if (ce.code === Code.Unavailable || ce.code === Code.DeadlineExceeded) {
    return new Error('Sync server unreachable. Your changes are saved on this device.');
  }
  return new Error(`Sync failed: ${ce.rawMessage || ce.message}`);
}

/**
 * HostedBackend syncs with the Daily Tasks service (dailytasks.v1 over
 * Connect). The service stores data and history atomically under one etag;
 * this adapter exposes them as the two keys the sync layer works with.
 */
export class HostedBackend implements Backend {
  readonly settings: HostedSettings;
  private readonly sync: Client<typeof SyncService>;
  private readonly account: Client<typeof AccountService>;

  constructor(settings: HostedSettings, transport?: Transport) {
    this.settings = {
      ...settings,
      apiUrl: normalizeApiUrl(settings.apiUrl),
      token: settings.token?.trim(),
    };
    const t = transport ?? createHostedTransport(this.settings.apiUrl, () => this.settings.token);
    this.sync = createClient(SyncService, t);
    this.account = createClient(AccountService, t);
  }

  private requireToken(): void {
    if (!this.settings.token) {
      throw new HostedAuthError('Not signed in. Sign in again.');
    }
  }

  private async getState(): Promise<GetStateResponse> {
    this.requireToken();
    try {
      return await this.sync.getState({});
    } catch (err) {
      throw toBackendError(err);
    }
  }

  async fetch(key: string): Promise<Blob> {
    if (key !== KeyData && key !== KeyHistory) {
      throw new Error(`hosted backend: unknown key ${key}`);
    }
    const state = await this.getState();
    const doc = key === KeyData ? state.data : state.history;
    if (!state.exists || doc.length === 0) {
      return { bytes: null, etag: state.etag };
    }
    return { bytes: utf8Decode(doc), etag: state.etag };
  }

  async push(key: string, body: string, ifMatch?: string): Promise<void> {
    if (key !== KeyData && key !== KeyHistory) {
      throw new Error(`hosted backend: unknown key ${key}`);
    }
    const state = await this.getState();
    const empty = utf8Encode('{}');
    let data = state.data.length > 0 ? state.data : empty;
    let history = state.history.length > 0 ? state.history : empty;
    if (key === KeyData) {
      data = utf8Encode(body);
    } else {
      history = utf8Encode(body);
    }
    const precondition = !ifMatch
      ? { case: undefined }
      : ifMatch === IF_NONE_MATCH_ANY
        ? { case: 'createOnly' as const, value: true }
        : { case: 'ifMatch' as const, value: ifMatch };
    try {
      await this.sync.putState({ data, history, precondition });
    } catch (err) {
      throw toBackendError(err);
    }
  }

  async getAccount(): Promise<{ email: string; provider: string; refreshRecommended: boolean }> {
    this.requireToken();
    try {
      const res = await this.account.getAccount({});
      return { email: res.email, provider: res.provider, refreshRecommended: res.refreshRecommended };
    } catch (err) {
      throw toBackendError(err);
    }
  }

  /** Returns a renewed session token. */
  async refreshSession(): Promise<string> {
    this.requireToken();
    try {
      return (await this.account.refreshSession({})).token;
    } catch (err) {
      throw toBackendError(err);
    }
  }

  async revokeAllSessions(): Promise<void> {
    this.requireToken();
    try {
      await this.account.revokeAllSessions({});
    } catch (err) {
      throw toBackendError(err);
    }
  }

  /** Permanently deletes the account and all synced data. */
  async deleteAccount(): Promise<void> {
    this.requireToken();
    try {
      await this.account.deleteAccount({});
    } catch (err) {
      throw toBackendError(err);
    }
  }
}

// Session tokens last 60 days; renew once a week so active users are never
// signed out.
export const tokenRefreshAgeMs = 7 * 24 * 60 * 60 * 1000;

/** Reads the unverified "iat" claim of a JWT, in milliseconds. */
export function tokenIssuedAtMs(token: string): number | null {
  const parts = token.split('.');
  if (parts.length !== 3) {
    return null;
  }
  try {
    const padded = parts[1].replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(parts[1].length / 4) * 4, '=');
    const claims = JSON.parse(base64Decode(padded)) as { iat?: number };
    return typeof claims.iat === 'number' ? claims.iat * 1000 : null;
  } catch {
    return null;
  }
}

/**
 * Returns a fresh token when the current one is older than a week, or null
 * when no renewal is needed or possible (the old token keeps working).
 */
export async function refreshTokenIfStale(backend: HostedBackend, now = Date.now()): Promise<string | null> {
  const token = backend.settings.token;
  if (!token) {
    return null;
  }
  const issued = tokenIssuedAtMs(token);
  if (issued === null || now - issued < tokenRefreshAgeMs) {
    return null;
  }
  try {
    return await backend.refreshSession();
  } catch {
    return null;
  }
}
