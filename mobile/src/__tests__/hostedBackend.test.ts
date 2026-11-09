jest.mock('../config/env', () => ({
  hostedApiUrl: 'https://api.example.com',
  appScheme: 'daily-tasks',
  appVersion: 'test',
}));

jest.mock('expo-auth-session', () => ({
  makeRedirectUri: jest.fn(() => 'daily-tasks://auth'),
}));

jest.mock('expo-web-browser', () => ({
  maybeCompleteAuthSession: jest.fn(),
  openAuthSessionAsync: jest.fn(),
}));

import { encode as base64Encode } from 'base-64';
import { Code, ConnectError, createRouterTransport } from '@connectrpc/connect';
import { AccountService } from '../gen/dailytasks/v1/account_pb';
import { SyncService } from '../gen/dailytasks/v1/sync_pb';
import {
  HostedBackend,
  HostedAuthError,
  refreshTokenIfStale,
  tokenIssuedAtMs,
} from '../services/backend_hosted';
import { buildHostedAuthUrl, parseHostedLoginCallback } from '../services/hosted_login';
import { EtagMismatchError, IF_NONE_MATCH_ANY, KeyData, KeyHistory } from '../services/backend';
import { utf8Decode, utf8Encode } from '../services/utf8';

// In-memory dailytasks.v1 server with the real service's precondition rules.
function fakeServer(validToken = 'jwt-token') {
  const state = {
    exists: false,
    data: new Uint8Array() as Uint8Array<ArrayBufferLike>,
    history: new Uint8Array() as Uint8Array<ArrayBufferLike>,
    version: 0,
    puts: [] as { data: string; history: string; precondition?: string }[],
    authHeaders: [] as (string | null)[],
    deleted: false,
    refreshed: false,
  };
  const etag = () => (state.exists ? `"sync-${state.version}"` : '"empty"');
  const authorize = (header: Headers) => {
    const auth = header.get('Authorization');
    state.authHeaders.push(auth);
    if (state.deleted || auth !== `Bearer ${validToken}`) {
      throw new ConnectError('unauthenticated', Code.Unauthenticated);
    }
  };
  const transport = createRouterTransport(({ service }) => {
    service(SyncService, {
      getState(_req, ctx) {
        authorize(ctx.requestHeader);
        return { exists: state.exists, data: state.data, history: state.history, etag: etag() };
      },
      putState(req, ctx) {
        authorize(ctx.requestHeader);
        const p = req.precondition;
        if (p.case === 'createOnly' && state.exists) {
          throw new ConnectError('exists', Code.FailedPrecondition);
        }
        if (p.case === 'ifMatch' && p.value !== etag()) {
          throw new ConnectError('stale', Code.FailedPrecondition);
        }
        state.puts.push({ data: utf8Decode(req.data), history: utf8Decode(req.history), precondition: p.case });
        state.exists = true;
        state.data = req.data;
        state.history = req.history;
        state.version += 1;
        return { etag: etag() };
      },
    });
    service(AccountService, {
      getAccount(_req, ctx) {
        authorize(ctx.requestHeader);
        return { id: 'u1', email: 'me@example.com', provider: 'google', refreshRecommended: false };
      },
      refreshSession(_req, ctx) {
        authorize(ctx.requestHeader);
        state.refreshed = true;
        return { token: 'renewed-token' };
      },
      revokeAllSessions(_req, ctx) {
        authorize(ctx.requestHeader);
        return {};
      },
      deleteAccount(_req, ctx) {
        authorize(ctx.requestHeader);
        state.deleted = true;
        return {};
      },
    });
  });
  return { state, transport };
}

function backendFor(token: string | undefined, transport: ReturnType<typeof fakeServer>['transport']) {
  const settings = { apiUrl: 'https://api.example.com', token };
  // Production builds the transport with an auth interceptor; for the router
  // transport we add the header through a wrapping transport.
  const authed = {
    unary: (method: never, signal: never, timeoutMs: never, header: HeadersInit | undefined, input: never, contextValues: never) => {
      const h = new Headers(header);
      if (token) {
        h.set('Authorization', `Bearer ${token}`);
      }
      return transport.unary(method, signal, timeoutMs, h, input, contextValues);
    },
    stream: transport.stream,
  };
  return new HostedBackend(settings, authed as unknown as typeof transport);
}

describe('HostedBackend (Connect)', () => {
  it('reports a missing remote as null bytes with the empty etag', async () => {
    const { transport } = fakeServer();
    const backend = backendFor('jwt-token', transport);
    await expect(backend.fetch(KeyData)).resolves.toEqual({ bytes: null, etag: '"empty"' });
  });

  it('creates, then updates one document while preserving the other', async () => {
    const { state, transport } = fakeServer();
    const backend = backendFor('jwt-token', transport);

    await backend.push(KeyData, '{"tasks":[]}', IF_NONE_MATCH_ANY);
    expect(state.puts[0]).toEqual({ data: '{"tasks":[]}', history: '{}', precondition: 'createOnly' });

    const history = await backend.fetch(KeyHistory);
    expect(history).toEqual({ bytes: '{}', etag: '"sync-1"' });
    await backend.push(KeyHistory, '{"version":1,"days":[]}', history.etag);
    expect(state.puts[1]).toEqual({ data: '{"tasks":[]}', history: '{"version":1,"days":[]}', precondition: 'ifMatch' });

    await expect(backend.fetch(KeyData)).resolves.toEqual({ bytes: '{"tasks":[]}', etag: '"sync-2"' });
  });

  it('maps failed preconditions to EtagMismatchError', async () => {
    const { transport } = fakeServer();
    const backend = backendFor('jwt-token', transport);
    await backend.push(KeyData, '{}', IF_NONE_MATCH_ANY);
    await expect(backend.push(KeyData, '{}', IF_NONE_MATCH_ANY)).rejects.toBeInstanceOf(EtagMismatchError);
    await expect(backend.push(KeyData, '{}', '"sync-0"')).rejects.toBeInstanceOf(EtagMismatchError);
  });

  it('round-trips non-ASCII task titles', async () => {
    const { transport } = fakeServer();
    const backend = backendFor('jwt-token', transport);
    const doc = '{"tasks":[{"title":"Café ☕ – 読書 📚"}]}';
    await backend.push(KeyData, doc);
    await expect(backend.fetch(KeyData)).resolves.toMatchObject({ bytes: doc });
  });

  it('throws HostedAuthError for missing or rejected tokens', async () => {
    const { transport } = fakeServer();
    await expect(backendFor(undefined, transport).fetch(KeyData)).rejects.toBeInstanceOf(HostedAuthError);
    await expect(backendFor('expired', transport).fetch(KeyData)).rejects.toBeInstanceOf(HostedAuthError);
  });

  it('deletes the account so the token stops working', async () => {
    const { state, transport } = fakeServer();
    const backend = backendFor('jwt-token', transport);
    await expect(backend.getAccount()).resolves.toMatchObject({ email: 'me@example.com' });
    await backend.deleteAccount();
    expect(state.deleted).toBe(true);
    await expect(backend.fetch(KeyData)).rejects.toBeInstanceOf(HostedAuthError);
  });
});

function jwtIssuedAt(ms: number): string {
  const payload = base64Encode(JSON.stringify({ uid: 'u', iat: Math.floor(ms / 1000) }))
    .replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  return `eyJhbGciOiJIUzI1NiJ9.${payload}.sig`;
}

describe('session renewal', () => {
  it('reads iat from a JWT', () => {
    expect(tokenIssuedAtMs(jwtIssuedAt(1_700_000_000_000))).toBe(1_700_000_000_000);
    expect(tokenIssuedAtMs('not-a-jwt')).toBeNull();
  });

  it('refreshes only tokens older than a week', async () => {
    const now = Date.now();
    const fresh = jwtIssuedAt(now - 60 * 60 * 1000);
    let server = fakeServer(fresh);
    await expect(refreshTokenIfStale(backendFor(fresh, server.transport), now)).resolves.toBeNull();
    expect(server.state.refreshed).toBe(false);

    const old = jwtIssuedAt(now - 8 * 24 * 60 * 60 * 1000);
    server = fakeServer(old);
    await expect(refreshTokenIfStale(backendFor(old, server.transport), now)).resolves.toBe('renewed-token');
  });
});

describe('utf8 helpers', () => {
  it('match TextEncoder/TextDecoder', () => {
    for (const s of ['', 'ascii', 'Café ☕', '読書 📚', '\u{1F600}x']) {
      expect(Array.from(utf8Encode(s))).toEqual(Array.from(new TextEncoder().encode(s)));
      expect(utf8Decode(new TextEncoder().encode(s))).toBe(s);
    }
    expect(utf8Decode(Uint8Array.from([0xe2, 0x82]))).toBe('�');
  });
});

describe('hosted auth helpers', () => {
  it('builds provider auth URLs with redirect_uri', () => {
    expect(buildHostedAuthUrl('google', 'https://api.example.com/', 'daily-tasks://auth'))
      .toBe('https://api.example.com/auth/google?redirect_uri=daily-tasks%3A%2F%2Fauth');
  });

  it('parses token and email from the OAuth callback URL', () => {
    expect(parseHostedLoginCallback('daily-tasks://auth?token=abc&email=me%40example.com'))
      .toEqual({ token: 'abc', email: 'me@example.com' });
  });

  it('rejects callback URLs without tokens', () => {
    expect(() => parseHostedLoginCallback('daily-tasks://auth?email=x')).toThrow('Sign-in did not return a session');
  });
});
