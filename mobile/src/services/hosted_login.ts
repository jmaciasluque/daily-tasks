import * as AuthSession from 'expo-auth-session';
import * as WebBrowser from 'expo-web-browser';
import { appScheme, hostedApiUrl } from '../config/env';
import { normalizeApiUrl, type HostedProvider } from './backend_hosted';

WebBrowser.maybeCompleteAuthSession();

export type HostedLoginResult = {
  token: string;
  email?: string;
};

export function buildHostedAuthUrl(provider: HostedProvider, apiUrl: string, redirectUri: string): string {
  const params = new URLSearchParams();
  params.set('redirect_uri', redirectUri);
  return `${normalizeApiUrl(apiUrl)}/auth/${provider}?${params.toString()}`;
}

export function parseHostedLoginCallback(url: string): HostedLoginResult {
  const parsed = new URL(url);
  const token = parsed.searchParams.get('token');
  if (!token) {
    throw new Error('Sign-in did not return a session');
  }
  return {
    token,
    email: parsed.searchParams.get('email') || undefined,
  };
}

export async function startHostedLogin(provider: HostedProvider, apiUrl = hostedApiUrl): Promise<HostedLoginResult> {
  const redirectUri = AuthSession.makeRedirectUri({ scheme: appScheme, path: 'auth' });
  const authUrl = buildHostedAuthUrl(provider, apiUrl, redirectUri);
  const result = await WebBrowser.openAuthSessionAsync(authUrl, redirectUri);
  if (result.type !== 'success' || !result.url) {
    throw new Error('Sign-in was cancelled');
  }
  return parseHostedLoginCallback(result.url);
}
