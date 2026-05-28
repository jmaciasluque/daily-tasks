jest.mock('../config/env', () => ({
  hostedApiUrl: 'https://api.example.com',
}));

jest.mock('expo-auth-session', () => ({
  makeRedirectUri: jest.fn(() => 'daily-tasks://auth'),
}));

jest.mock('expo-web-browser', () => ({
  maybeCompleteAuthSession: jest.fn(),
  openAuthSessionAsync: jest.fn(),
}));

import { decode as base64Decode } from 'base-64';
import {
  HostedBackend,
  HostedAuthError,
  buildHostedAuthUrl,
  parseHostedLoginCallback,
} from '../services/backend_hosted';
import { EtagMismatchError, IF_NONE_MATCH_ANY, KeyData, KeyHistory } from '../services/backend';

const fetchMock = jest.fn();

describe('HostedBackend', () => {
  beforeEach(() => {
    fetchMock.mockReset();
    globalThis.fetch = fetchMock as unknown as typeof fetch;
  });

  it('fetches data and history blobs through the hosted sync endpoint', async () => {
    fetchMock.mockResolvedValueOnce({
      ok: true,
      status: 200,
      headers: { get: (name: string) => name === 'ETag' ? '"sync-1"' : null },
      json: async () => ({
        data: 'eyJ0YXNrcyI6W119',
        history: 'eyJldmVudHMiOltdfQ==',
      }),
    });

    const backend = new HostedBackend({ apiUrl: 'https://api.example.com/', token: 'jwt-token' });

    await expect(backend.fetch(KeyData)).resolves.toEqual({ bytes: '{"tasks":[]}', etag: '"sync-1"' });
    expect(fetchMock).toHaveBeenCalledWith('https://api.example.com/api/v1/sync', {
      headers: {
        Authorization: 'Bearer jwt-token',
        Accept: 'application/json',
      },
    });
  });

  it('preserves the other hosted blob when pushing one key', async () => {
    fetchMock
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        headers: { get: (name: string) => name === 'ETag' ? '"sync-1"' : null },
        json: async () => ({
          data: 'eyJ0YXNrcyI6W119',
          history: 'eyJldmVudHMiOltdfQ==',
        }),
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        headers: { get: () => '"sync-2"' },
        text: async () => '{"status":"ok"}',
      });

    const backend = new HostedBackend({ apiUrl: 'https://api.example.com', token: 'jwt-token' });
    await backend.push(KeyData, '{"tasks":[{"id":1}]}', '"sync-1"');

    const put = fetchMock.mock.calls[1];
    expect(put[0]).toBe('https://api.example.com/api/v1/sync');
    expect(put[1].method).toBe('PUT');
    expect(put[1].headers).toMatchObject({
      Authorization: 'Bearer jwt-token',
      'Content-Type': 'application/json',
      'If-Match': '"sync-1"',
    });
    const body = JSON.parse(put[1].body);
    expect(base64Decode(body.data)).toBe('{"tasks":[{"id":1}]}');
    expect(base64Decode(body.history)).toBe('{"events":[]}');
  });

  it('maps create-only pushes to If-None-Match star and 412 to EtagMismatchError', async () => {
    fetchMock
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        headers: { get: (name: string) => name === 'ETag' ? '"empty"' : null },
        json: async () => ({ data: 'e30=', history: 'e30=' }),
      })
      .mockResolvedValueOnce({
        ok: false,
        status: 412,
        headers: { get: () => null },
        text: async () => 'precondition failed',
      });

    const backend = new HostedBackend({ apiUrl: 'https://api.example.com', token: 'jwt-token' });
    await expect(backend.push(KeyHistory, '{"events":[]}', IF_NONE_MATCH_ANY)).rejects.toBeInstanceOf(EtagMismatchError);
    expect(fetchMock.mock.calls[1][1].headers['If-None-Match']).toBe('*');
  });

  it('throws HostedAuthError for expired tokens', async () => {
    fetchMock.mockResolvedValueOnce({
      ok: false,
      status: 401,
      headers: { get: () => null },
      text: async () => 'unauthorized',
    });

    const backend = new HostedBackend({ apiUrl: 'https://api.example.com', token: 'expired' });
    await expect(backend.fetch(KeyData)).rejects.toBeInstanceOf(HostedAuthError);
  });
});

describe('hosted auth helpers', () => {
  it('builds provider auth URLs with redirect_uri', () => {
    expect(buildHostedAuthUrl('google', 'https://daily-tasks.fly.dev/', 'daily-tasks://auth')).toBe(
      'https://daily-tasks.fly.dev/auth/google?redirect_uri=daily-tasks%3A%2F%2Fauth',
    );
  });

  it('parses token and email from hosted OAuth callback URLs', () => {
    expect(parseHostedLoginCallback('daily-tasks://auth?token=jwt&email=user%40example.com')).toEqual({
      token: 'jwt',
      email: 'user@example.com',
    });
  });

  it('rejects callback URLs without tokens', () => {
    expect(() => parseHostedLoginCallback('daily-tasks://auth?error=access_denied')).toThrow('Hosted login did not return a token');
  });
});
