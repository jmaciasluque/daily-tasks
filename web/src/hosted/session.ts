// Session handling for the hosted web UI. The OAuth flow redirects back to
// /app/?token=…&email=…; we keep the token in localStorage (this origin only)
// and strip it from the address bar immediately.
export type Session = { token: string; email?: string };

const KEY = 'daily-tasks.session';

export function apiOrigin(): string {
  return window.location.origin;
}

export function loadSession(): Session | null {
  try {
    const raw = window.localStorage.getItem(KEY);
    return raw ? (JSON.parse(raw) as Session) : null;
  } catch {
    return null;
  }
}

export function saveSession(session: Session | null): void {
  try {
    if (session) {
      window.localStorage.setItem(KEY, JSON.stringify(session));
    } else {
      window.localStorage.removeItem(KEY);
    }
  } catch {
    // Storage unavailable (private mode): the session lasts for this tab only.
  }
}

/** Consumes ?token=&email= left by the OAuth redirect, if present. */
export function consumeLoginRedirect(): Session | null {
  const url = new URL(window.location.href);
  const token = url.searchParams.get('token');
  if (!token) {
    return null;
  }
  const session: Session = { token, email: url.searchParams.get('email') || undefined };
  url.searchParams.delete('token');
  url.searchParams.delete('email');
  window.history.replaceState(null, '', url.pathname + (url.search ? url.search : '') + url.hash);
  saveSession(session);
  return session;
}

export function signInUrl(): string {
  const back = `${apiOrigin()}/app/`;
  return `${apiOrigin()}/auth/google?redirect_uri=${encodeURIComponent(back)}`;
}
