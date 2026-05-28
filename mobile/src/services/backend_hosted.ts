import { encode as base64Encode, decode as base64Decode } from 'base-64';
import * as AuthSession from 'expo-auth-session';
import * as WebBrowser from 'expo-web-browser';
import { hostedApiUrl } from '../config/env';
import {
  Backend,
  Blob,
  EtagMismatchError,
  IF_NONE_MATCH_ANY,
  KeyData,
  KeyHistory,
} from './backend';

WebBrowser.maybeCompleteAuthSession();

export type HostedProvider = 'google' | 'facebook';

export type HostedSettings = {
  apiUrl: string;
  token?: string;
  email?: string;
};

export type HostedLoginResult = {
  token: string;
  email?: string;
};

type HostedSyncPayload = {
  data?: string;
  history?: string;
  updated_at?: string;
};

export class HostedAuthError extends Error {
  constructor(message = 'Hosted login expired. Sign in again.') {
    super(message);
    this.name = 'HostedAuthError';
  }
}

export const defaultHostedSettings: HostedSettings = {
  apiUrl: hostedApiUrl,
};

function normalizeApiUrl(apiUrl: string): string {
  return apiUrl.trim().replace(/\/+$/, '');
}

function syncUrl(apiUrl: string): string {
  return `${normalizeApiUrl(apiUrl)}/api/v1/sync`;
}

function authHeader(token: string): Record<string, string> {
  return { Authorization: `Bearer ${token}` };
}

function decodeBlob(encoded: string | undefined, key: string): string | null {
  if (!encoded) {
    return null;
  }
  try {
    return base64Decode(encoded);
  } catch (err) {
    throw new Error(`hosted backend: invalid ${key} payload: ${(err as Error).message}`);
  }
}

function encodeBlob(body: string): string {
  return base64Encode(body);
}

function etagFromResponse(res: Response): string {
  return res.headers.get('ETag') ?? '';
}

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

export class HostedBackend implements Backend {
  readonly settings: HostedSettings;

  constructor(settings: HostedSettings) {
    this.settings = {
      ...settings,
      apiUrl: normalizeApiUrl(settings.apiUrl || hostedApiUrl),
      token: settings.token?.trim(),
    };
  }

  private requireToken(): string {
    if (!this.settings.token) {
      throw new HostedAuthError('Hosted token missing. Sign in again.');
    }
    return this.settings.token;
  }

  private async fetchPayload(): Promise<{ payload: HostedSyncPayload; etag: string }> {
    const res = await globalThis.fetch(syncUrl(this.settings.apiUrl), {
      headers: {
        ...authHeader(this.requireToken()),
        Accept: 'application/json',
      },
    });
    if (res.status === 401 || res.status === 403) {
      throw new HostedAuthError();
    }
    if (!res.ok) {
      throw new Error(`Hosted fetch failed: ${res.status}`);
    }
    return { payload: await res.json() as HostedSyncPayload, etag: etagFromResponse(res) };
  }

  async fetch(key: string): Promise<Blob> {
    const { payload, etag } = await this.fetchPayload();
    switch (key) {
      case KeyData:
        return { bytes: decodeBlob(payload.data, key), etag };
      case KeyHistory:
        return { bytes: decodeBlob(payload.history, key), etag };
      default:
        throw new Error(`hosted backend: unknown key ${key}`);
    }
  }

  async push(key: string, body: string, ifMatch?: string): Promise<void> {
    const { payload } = await this.fetchPayload();
    switch (key) {
      case KeyData:
        payload.data = encodeBlob(body);
        break;
      case KeyHistory:
        payload.history = encodeBlob(body);
        break;
      default:
        throw new Error(`hosted backend: unknown key ${key}`);
    }

    const headers: Record<string, string> = {
      ...authHeader(this.requireToken()),
      'Content-Type': 'application/json',
    };
    applyIfMatch(headers, ifMatch);
    const res = await globalThis.fetch(syncUrl(this.settings.apiUrl), {
      method: 'PUT',
      headers,
      body: JSON.stringify({
        data: payload.data ?? encodeBlob('{}'),
        history: payload.history ?? encodeBlob('{}'),
      }),
    });
    if (res.status === 401 || res.status === 403) {
      throw new HostedAuthError();
    }
    if (res.status === 412) {
      throw new EtagMismatchError();
    }
    if (!res.ok) {
      throw new Error(`Hosted save failed: ${res.status}`);
    }
  }
}

export function buildHostedAuthUrl(provider: HostedProvider, apiUrl: string, redirectUri: string): string {
  const params = new URLSearchParams();
  params.set('redirect_uri', redirectUri);
  return `${normalizeApiUrl(apiUrl)}/auth/${provider}?${params.toString()}`;
}

export function parseHostedLoginCallback(url: string): HostedLoginResult {
  const parsed = new URL(url);
  const token = parsed.searchParams.get('token');
  if (!token) {
    throw new Error('Hosted login did not return a token');
  }
  return {
    token,
    email: parsed.searchParams.get('email') || undefined,
  };
}

export async function startHostedLogin(provider: HostedProvider, apiUrl = hostedApiUrl): Promise<HostedLoginResult> {
  const redirectUri = AuthSession.makeRedirectUri({ scheme: 'daily-tasks', path: 'auth' });
  const authUrl = buildHostedAuthUrl(provider, apiUrl, redirectUri);
  const result = await WebBrowser.openAuthSessionAsync(authUrl, redirectUri);
  if (result.type !== 'success' || !result.url) {
    throw new Error('Hosted login was cancelled');
  }
  return parseHostedLoginCallback(result.url);
}
