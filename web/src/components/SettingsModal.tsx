import React from 'react';
import type { Settings } from '../types';
import type { Theme } from '../theme/themes';

type Props = {
  visible: boolean;
  settings: Settings;
  theme: Theme;
  onUpdate: (settings: Settings) => void;
  onSave: () => void;
  onClose: () => void;
};

export function SettingsModal({ visible, settings, theme, onUpdate, onSave, onClose }: Props) {
  if (!visible) return null;

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
        <div style={{ fontSize: 18, fontWeight: 700, color: theme.text }}>Nextcloud WebDAV</div>
        <input
          placeholder="Base URL (https://cloud.example.com)"
          value={settings.baseUrl}
          onChange={e => onUpdate({ ...settings, baseUrl: e.target.value })}
          style={input}
        />
        <input
          placeholder="Username"
          value={settings.username}
          onChange={e => onUpdate({ ...settings, username: e.target.value })}
          style={input}
        />
        <input
          placeholder="App password"
          value={settings.password}
          onChange={e => onUpdate({ ...settings, password: e.target.value })}
          type="password"
          style={input}
        />
        <input
          placeholder="Remote path (/remote.php/dav/files/user/.daily-tasks.json)"
          value={settings.remotePath}
          onChange={e => onUpdate({ ...settings, remotePath: e.target.value })}
          style={input}
        />
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 12, marginTop: 8 }}>
          <button onClick={onClose} style={{ border: `1px solid ${theme.border}`, borderRadius: 12, padding: '12px 16px', background: 'transparent', color: theme.text, cursor: 'pointer' }}>
            Close
          </button>
          <button onClick={onSave} style={{ border: 'none', borderRadius: 12, padding: '12px 16px', background: theme.accent, color: '#111111', fontWeight: 700, cursor: 'pointer' }}>
            Save
          </button>
        </div>
      </div>
    </div>
  );
}
