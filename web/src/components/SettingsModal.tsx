import React from 'react';
import type { ServerState } from '../types';
import type { Theme } from '../theme/themes';

type Props = {
  visible: boolean;
  serverState: ServerState | null;
  theme: Theme;
  onClose: () => void;
};

export function SettingsModal({ visible, serverState, theme, onClose }: Props) {
  if (!visible) return null;

  return (
    <div style={{
      position: 'fixed', top: 0, left: 0, right: 0, bottom: 0,
      backgroundColor: 'rgba(0,0,0,0.45)',
      display: 'flex', justifyContent: 'center', alignItems: 'center',
      padding: 18, zIndex: 1000,
    }}>
      <div style={{
        backgroundColor: theme.panelBg, border: `1px solid ${theme.border}`,
        borderRadius: 16, padding: 18, width: '100%', maxWidth: 480,
        display: 'flex', flexDirection: 'column', gap: 12,
      }}>
        <div style={{ fontSize: 18, fontWeight: 700, color: theme.text }}>Local Server</div>
        <div style={{ fontSize: 13, color: theme.muted, lineHeight: 1.5 }}>
          The web app talks only to the local `daily-tasks web` server. Your Nextcloud
          credentials stay on the server side and are never stored in the browser.
        </div>
        <div style={{ border: `1px solid ${theme.border}`, borderRadius: 12, padding: 14, background: theme.bg, display: 'flex', flexDirection: 'column', gap: 8 }}>
          <div style={{ fontSize: 13, color: theme.muted }}>Local data file</div>
          <div style={{ fontSize: 14, color: theme.text, wordBreak: 'break-all' }}>{serverState?.data_path || 'Loading...'}</div>
        </div>
        <div style={{ border: `1px solid ${theme.border}`, borderRadius: 12, padding: 14, background: theme.bg, display: 'flex', flexDirection: 'column', gap: 8 }}>
          <div style={{ fontSize: 13, color: theme.muted }}>Nextcloud sync</div>
          <div style={{ fontSize: 14, color: theme.text }}>
            {serverState?.sync_configured ? 'Configured via DAILY_TASKS_WEBDAV_*' : 'Not configured'}
          </div>
          <div style={{ fontSize: 12, color: theme.muted, lineHeight: 1.45 }}>
            To enable sync, start the server with `DAILY_TASKS_WEBDAV_URL`,
            `DAILY_TASKS_WEBDAV_USER`, and `DAILY_TASKS_WEBDAV_PASS` set.
          </div>
        </div>
        <div style={{ fontSize: 12, color: theme.muted, lineHeight: 1.45 }}>
          Version {serverState?.version || '...'}
        </div>
        <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: 8 }}>
          <button onClick={onClose} style={{ border: `1px solid ${theme.border}`, borderRadius: 12, padding: '12px 16px', background: 'transparent', color: theme.text, cursor: 'pointer' }}>
            Close
          </button>
        </div>
      </div>
    </div>
  );
}
