jest.mock('../config/env', () => ({ hostedApiUrl: 'unused', appVersion: 'test' }));

import { parseDevLoginUrl, performDevLogin } from '../services/devLogin';

describe('dev login', () => {
  it('parses the deep link', () => {
    expect(parseDevLoginUrl('daily-tasks-dev://dev-login?api=http://10.0.2.2:8080/&secret=s&sub=me'))
      .toEqual({ api: 'http://10.0.2.2:8080', secret: 's', sub: 'me' });
    expect(parseDevLoginUrl('daily-tasks-dev://dev-login?api=http://x&secret=s')?.sub).toBe('e2e');
  });

  it('ignores other links and incomplete ones', () => {
    expect(parseDevLoginUrl('daily-tasks-dev://auth?token=abc')).toBeNull();
    expect(parseDevLoginUrl('daily-tasks-dev://dev-login?api=http://x')).toBeNull();
    expect(parseDevLoginUrl('not a url')).toBeNull();
  });

  it('exchanges the secret for a session on the given server', async () => {
    const fetchMock = jest.fn().mockResolvedValue({ ok: true, json: async () => ({ token: 't', email: 'me@example.test' }) });
    await expect(performDevLogin({ api: 'http://h', secret: 's', sub: 'me' }, fetchMock as unknown as typeof fetch))
      .resolves.toEqual({ apiUrl: 'http://h', token: 't', email: 'me@example.test' });
    expect(fetchMock.mock.calls[0][0]).toBe('http://h/auth/test?secret=s&sub=me&email=me%40example.test');
  });

  it('surfaces server refusals', async () => {
    const fetchMock = jest.fn().mockResolvedValue({ ok: false, status: 403 });
    await expect(performDevLogin({ api: 'http://h', secret: 'bad', sub: 'me' }, fetchMock as unknown as typeof fetch))
      .rejects.toThrow('HTTP 403');
  });
});
