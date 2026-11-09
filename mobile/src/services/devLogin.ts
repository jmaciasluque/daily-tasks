// Developer sign-in for dev/test builds only (never production):
//
//   adb shell am start -a android.intent.action.VIEW \
//     -d "daily-tasks-dev://dev-login?api=http://10.0.2.2:8080&secret=local&sub=e2e"
//
// It calls the server's /auth/test endpoint, which exists only on servers
// started with TEST_LOGIN_SECRET (local or staging), and signs the app in
// against that server. This makes the emulator scriptable without a Google
// account.
import { normalizeApiUrl, type HostedSettings } from './backend_hosted';

export type DevLoginRequest = { api: string; secret: string; sub: string };

export function parseDevLoginUrl(url: string): DevLoginRequest | null {
  let parsed: URL;
  try {
    parsed = new URL(url);
  } catch {
    return null;
  }
  // URL parses "scheme://dev-login?..." with "dev-login" as the host.
  if (parsed.hostname !== 'dev-login' && !parsed.pathname.replace(/^\/+/, '').startsWith('dev-login')) {
    return null;
  }
  const api = parsed.searchParams.get('api');
  const secret = parsed.searchParams.get('secret');
  if (!api || !secret) {
    return null;
  }
  return { api: normalizeApiUrl(api), secret, sub: parsed.searchParams.get('sub') || 'e2e' };
}

export async function performDevLogin(req: DevLoginRequest, fetchImpl: typeof fetch = fetch): Promise<HostedSettings> {
  const params = new URLSearchParams({ secret: req.secret, sub: req.sub, email: `${req.sub}@example.test` });
  const res = await fetchImpl(`${req.api}/auth/test?${params.toString()}`);
  if (!res.ok) {
    throw new Error(`dev login failed: HTTP ${res.status}`);
  }
  const body = await res.json() as { token?: string; email?: string };
  if (!body.token) {
    throw new Error('dev login returned no token');
  }
  return { apiUrl: req.api, token: body.token, email: body.email || undefined };
}
