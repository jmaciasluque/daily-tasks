import React, { useEffect, useState } from 'react';
import type { NextcloudSetupPoll, NextcloudSetupStart, ServerState } from '../types';
import type { Theme } from '../theme/themes';

type Props = {
  visible: boolean;
  serverState: ServerState | null;
  theme: Theme;
  onUseLocal: () => Promise<void>;
  onStartNextcloud: (serverUrl: string) => Promise<NextcloudSetupStart>;
  onPollNextcloud: (sessionId: string) => Promise<NextcloudSetupPoll>;
  onConfigured: () => Promise<void>;
  onClose: () => void;
};

export function SettingsModal({
  visible,
  serverState,
  theme,
  onUseLocal,
  onStartNextcloud,
  onPollNextcloud,
  onConfigured,
  onClose,
}: Props) {
  const [serverUrl, setServerUrl] = useState('');
  const [loginSession, setLoginSession] = useState<NextcloudSetupStart | null>(null);
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState('');

  useEffect(() => {
    if (!visible) {
      return;
    }
    setServerUrl(serverState?.nextcloud?.server_url ?? '');
    setLoginSession(null);
    setBusy(false);
    setMessage('');
  }, [serverState?.nextcloud?.server_url, visible]);

  if (!visible) return null;

  const backendLabel = serverState?.backend === 'nextcloud' ? 'Nextcloud' : 'Local only';
  const nextcloud = serverState?.nextcloud;

  const handleUseLocal = async () => {
    setBusy(true);
    setMessage('');
    try {
      await onUseLocal();
      setLoginSession(null);
      setMessage('Using local-only backend.');
    } catch (err) {
      setMessage((err as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const handleStartNextcloud = async () => {
    const trimmed = serverUrl.trim();
    if (!trimmed) {
      setMessage('Enter a Nextcloud server URL.');
      return;
    }

    setBusy(true);
    setMessage('');
    try {
      const session = await onStartNextcloud(trimmed);
      setLoginSession(session);
      window.open(session.login_url, '_blank', 'noopener,noreferrer');
      setMessage('Nextcloud login started.');
    } catch (err) {
      setMessage((err as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const handleFinishNextcloud = async () => {
    if (!loginSession) {
      return;
    }

    setBusy(true);
    setMessage('');
    try {
      const result = await onPollNextcloud(loginSession.session_id);
      if (result.status === 'pending') {
        setMessage('Nextcloud login is still pending.');
        return;
      }
      await onConfigured();
      setLoginSession(null);
      setMessage('Connected to Nextcloud.');
    } catch (err) {
      setMessage((err as Error).message);
    } finally {
      setBusy(false);
    }
  };

  const card: React.CSSProperties = {
    border: `1px solid ${theme.border}`,
    borderRadius: 12,
    padding: 14,
    background: theme.bg,
    display: 'flex',
    flexDirection: 'column',
    gap: 8,
  };

  const label: React.CSSProperties = {
    fontSize: 12,
    color: theme.muted,
    textTransform: 'uppercase',
    letterSpacing: 0,
  };

  const input: React.CSSProperties = {
    border: `1px solid ${theme.border}`,
    borderRadius: 12,
    padding: '10px 12px',
    background: theme.bg,
    color: theme.text,
    fontSize: 14,
    width: '100%',
    outline: 'none',
  };

  const secondaryButton: React.CSSProperties = {
    border: `1px solid ${theme.border}`,
    borderRadius: 12,
    padding: '12px 16px',
    background: 'transparent',
    color: theme.text,
    cursor: busy ? 'default' : 'pointer',
    opacity: busy ? 0.65 : 1,
  };

  const primaryButton: React.CSSProperties = {
    border: 'none',
    borderRadius: 12,
    padding: '12px 16px',
    background: theme.accent,
    color: '#111111',
    fontWeight: 700,
    cursor: busy ? 'default' : 'pointer',
    opacity: busy ? 0.7 : 1,
  };

  return (
    <div style={{
      position: 'fixed', top: 0, left: 0, right: 0, bottom: 0,
      backgroundColor: 'rgba(0,0,0,0.45)',
      display: 'flex', justifyContent: 'center', alignItems: 'center',
      padding: 18, zIndex: 1000,
    }}>
      <div style={{
        backgroundColor: theme.panelBg, border: `1px solid ${theme.border}`,
        borderRadius: 16, padding: 18, width: '100%', maxWidth: 560,
        display: 'flex', flexDirection: 'column', gap: 12,
        maxHeight: '90vh', overflowY: 'auto',
      }}>
        <div style={{ fontSize: 18, fontWeight: 700, color: theme.text }}>Config</div>

        <div style={card}>
          <div style={label}>Storage Backend</div>
          <div style={{ color: theme.text, fontSize: 15, fontWeight: 700 }}>{backendLabel}</div>
          {nextcloud ? (
            <>
              <div style={{ color: theme.muted, fontSize: 13, wordBreak: 'break-all' }}>{nextcloud.server_url}</div>
              <div style={{ color: theme.muted, fontSize: 13 }}>Login {nextcloud.login_name}</div>
              <div style={{ color: theme.muted, fontSize: 13, wordBreak: 'break-all' }}>{nextcloud.remote_path}</div>
            </>
          ) : null}
        </div>

        <div style={card}>
          <div style={label}>Local Server</div>
          <div style={{ color: theme.muted, fontSize: 13 }}>Data file</div>
          <div style={{ color: theme.text, fontSize: 14, wordBreak: 'break-all' }}>{serverState?.data_path || 'Loading...'}</div>
          <div style={{ color: theme.muted, fontSize: 13 }}>Config file</div>
          <div style={{ color: theme.text, fontSize: 14, wordBreak: 'break-all' }}>{serverState?.config_path || 'Loading...'}</div>
        </div>

        <div style={card}>
          <div style={label}>Nextcloud</div>
          <input
            placeholder="https://cloud.example.com"
            value={serverUrl}
            onChange={(event) => setServerUrl(event.target.value)}
            type="url"
            style={input}
          />
          <div style={{ display: 'flex', gap: 10, flexWrap: 'wrap' }}>
            <button onClick={handleStartNextcloud} disabled={busy} style={primaryButton}>
              {serverState?.backend === 'nextcloud' ? 'Reconnect Nextcloud' : 'Connect Nextcloud'}
            </button>
            {loginSession ? (
              <>
                <button
                  onClick={() => window.open(loginSession.login_url, '_blank', 'noopener,noreferrer')}
                  disabled={busy}
                  style={secondaryButton}
                >
                  Open Login
                </button>
                <button onClick={handleFinishNextcloud} disabled={busy} style={secondaryButton}>
                  Finish Connection
                </button>
              </>
            ) : null}
          </div>
        </div>

        {message ? (
          <div style={{ color: theme.muted, fontSize: 13, lineHeight: 1.45 }}>{message}</div>
        ) : null}

        <div style={{ color: theme.muted, fontSize: 12, lineHeight: 1.45 }}>
          Version {serverState?.version || '...'}
        </div>

        <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, marginTop: 8, flexWrap: 'wrap' }}>
          <button onClick={handleUseLocal} disabled={busy} style={secondaryButton}>
            Use Local Only
          </button>
          <button onClick={onClose} style={secondaryButton}>
            Close
          </button>
        </div>
      </div>
    </div>
  );
}
