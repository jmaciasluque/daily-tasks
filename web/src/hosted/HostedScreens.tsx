import React, { useState } from 'react';
import type { Theme } from '../theme/themes';
import { appVersion } from '../config/env';

const card = (theme: Theme): React.CSSProperties => ({
  background: theme.panelBg,
  border: `1px solid ${theme.border}`,
  borderRadius: 16,
  padding: 24,
  maxWidth: 420,
  width: '100%',
  boxSizing: 'border-box',
});

const button = (theme: Theme, primary: boolean, danger = false): React.CSSProperties => ({
  display: 'block',
  width: '100%',
  textAlign: 'center',
  borderRadius: 12,
  padding: '12px 16px',
  border: primary ? 'none' : `1px solid ${danger ? '#c62828' : theme.border}`,
  background: primary ? theme.accent : 'transparent',
  color: primary ? '#fff' : danger ? '#e53935' : theme.text,
  fontSize: 15,
  fontWeight: 600,
  cursor: 'pointer',
  textDecoration: 'none',
  boxSizing: 'border-box',
});

export function SignInScreen({ theme, signInUrl, message }: { theme: Theme; signInUrl: string; message: string }) {
  return (
    <div style={{ minHeight: '100vh', background: theme.bg, display: 'grid', placeItems: 'center', padding: 16, boxSizing: 'border-box' }}>
      <div style={card(theme)}>
        <h1 style={{ color: theme.text, fontSize: 26, margin: '0 0 8px' }}>Daily Tasks</h1>
        <p style={{ color: theme.muted, margin: '0 0 20px', lineHeight: 1.5 }}>
          Sign in with the same Google account you use in the app to see and tick off today&apos;s tasks.
        </p>
        <a href={signInUrl} style={button(theme, true)} data-testid="sign-in-google">Sign in with Google</a>
        {message ? <p style={{ color: theme.muted, fontSize: 14, marginTop: 16 }}>{message}</p> : null}
        <p style={{ color: theme.muted, fontSize: 13, marginTop: 24 }}>
          <a href="/privacy" style={{ color: theme.muted }}>Privacy</a>{' · '}
          <a href="/terms" style={{ color: theme.muted }}>Terms</a>{' · '}
          v{appVersion}
        </p>
      </div>
    </div>
  );
}

type SettingsProps = {
  visible: boolean;
  theme: Theme;
  email?: string;
  onSignOut: () => void;
  onDeleteAccount: () => Promise<void>;
  onClose: () => void;
};

export function HostedSettingsModal({ visible, theme, email, onSignOut, onDeleteAccount, onClose }: SettingsProps) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  if (!visible) return null;

  const handleDelete = async () => {
    if (!window.confirm(`Permanently delete ${email || 'this account'} and all synced tasks? This cannot be undone.`)) {
      return;
    }
    setBusy(true);
    setError('');
    try {
      await onDeleteAccount();
      onClose();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  };

  return (
    <div style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.5)', display: 'grid', placeItems: 'center', padding: 16 }} onClick={onClose}>
      <div style={card(theme)} onClick={(e) => e.stopPropagation()}>
        <h2 style={{ color: theme.text, margin: '0 0 4px' }}>Account</h2>
        <p style={{ color: theme.muted, margin: '0 0 20px' }}>Signed in as {email || 'your Google account'}</p>
        <div style={{ display: 'grid', gap: 10 }}>
          <button style={button(theme, false)} onClick={onSignOut} disabled={busy}>Sign out</button>
          <button style={button(theme, false, true)} onClick={handleDelete} disabled={busy}>Delete account</button>
          <button style={button(theme, true)} onClick={onClose}>Close</button>
        </div>
        {error ? <p style={{ color: '#e53935', fontSize: 14 }}>{error}</p> : null}
        <p style={{ color: theme.muted, fontSize: 13, marginTop: 20 }}>
          <a href="/privacy" style={{ color: theme.muted }}>Privacy</a>{' · '}
          <a href="/terms" style={{ color: theme.muted }}>Terms</a>{' · '}
          v{appVersion}
        </p>
      </div>
    </div>
  );
}
