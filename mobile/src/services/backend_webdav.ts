import { encode as base64Encode } from 'base-64';
import type { Settings } from '../types';
import { defaultRemotePath } from '../config/env';
import {
  Backend,
  Blob,
  EtagMismatchError,
  IF_NONE_MATCH_ANY,
  KeyData,
  KeyHistory,
} from './backend';

// LoginFlowSession is the in-flight state of a Nextcloud Login Flow v2
// authorization. The user opens loginUrl in a browser; meanwhile the app
// polls pollEndpoint with pollToken until the server hands back an app
// password.
export type LoginFlowSession = {
  serverUrl: string;
  loginUrl: string;
  pollEndpoint: string;
  pollToken: string;
};

// defaultSettings is the seed value for a fresh Nextcloud config.
export const defaultSettings: Settings = {
  baseUrl: '',
  username: '',
  password: '',
  remotePath: defaultRemotePath,
};

export function isSettingsComplete(settings: Settings): boolean {
  return !!(settings.baseUrl && settings.username && settings.password && settings.remotePath);
}

function normalizeServerUrl(serverUrl: string): string {
  return serverUrl.trim().replace(/\/+$/, '');
}

function basicAuthHeader(user: string, pass: string): string {
  const token = `${user}:${pass}`;
  const encoded = globalThis.btoa ? globalThis.btoa(token) : base64Encode(token);
  return `Basic ${encoded}`;
}

function normalizeEtag(etag: string | null): string {
  return (etag ?? '').replace(/-gzip(?="?$)/, '');
}

function buildDataURL(settings: Settings): string {
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

function buildHistoryURL(settings: Settings): string {
  const base = settings.baseUrl.replace(/\/+$/, '');
  const historyPath = buildHistoryRemotePath(settings.remotePath);
  const normalizedPath = historyPath.startsWith('/') ? historyPath : `/${historyPath}`;
  return `${base}${normalizedPath}`;
}

function buildRemotePathForLogin(loginName: string): string {
  return `/remote.php/dav/files/${encodeURIComponent(loginName)}/.daily-tasks.json`;
}

// WebDAVBackend implements Backend against a WebDAV server (Nextcloud,
// generic WebDAV, ownCloud, etc.). It maps the well-known keys onto a
// pair of URLs derived from a single configured data path: the history
// URL is the data URL with `.history` inserted before its extension.
export class WebDAVBackend implements Backend {
  readonly settings: Settings;

  constructor(settings: Settings) {
    this.settings = settings;
  }

  private urlFor(key: string): string {
    switch (key) {
      case KeyData:
        return buildDataURL(this.settings);
      case KeyHistory:
        return buildHistoryURL(this.settings);
      default:
        throw new Error(`webdav backend: unknown key ${key}`);
    }
  }

  async fetch(key: string): Promise<Blob> {
    const url = this.urlFor(key);
    const res = await globalThis.fetch(url, {
      headers: {
        Authorization: basicAuthHeader(this.settings.username, this.settings.password),
        Accept: 'application/json',
      },
    });
    if (res.status === 404) {
      return { bytes: null, etag: '' };
    }
    if (!res.ok) {
      throw new Error(`Fetch failed: ${res.status}`);
    }
    return { bytes: await res.text(), etag: normalizeEtag(res.headers.get('ETag')) };
  }

  async push(key: string, body: string, ifMatch?: string): Promise<void> {
    const url = this.urlFor(key);
    const headers: Record<string, string> = {
      Authorization: basicAuthHeader(this.settings.username, this.settings.password),
      'Content-Type': 'application/json',
    };
    applyIfMatch(headers, ifMatch);
    const res = await globalThis.fetch(url, {
      method: 'PUT',
      headers,
      body,
    });
    if (res.status === 412) {
      throw new EtagMismatchError();
    }
    if (!res.ok) {
      throw new Error(`Save failed: ${res.status}`);
    }
  }
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

// startNextcloudLogin kicks off Nextcloud Login Flow v2. Open the returned
// loginUrl in a browser; meanwhile poll pollNextcloudLogin until it returns
// a Settings.
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

// pollNextcloudLogin checks whether the user has completed the browser
// step. Returns null while the flow is still pending; returns a Settings
// once Nextcloud hands back the per-client app password.
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
    remotePath: buildRemotePathForLogin(payload.loginName.trim()),
  };
}
