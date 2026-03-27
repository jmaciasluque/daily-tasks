import React from 'react';
import type { Task } from '../types';
import type { Theme } from '../theme/themes';

type Props = {
  task: Task;
  theme: Theme;
  onToggle: () => void;
  onSkip?: () => void;
  onEdit: () => void;
  onDelete: () => void;
};

export function TaskRow({ task, theme, onToggle, onSkip, onEdit, onDelete }: Props) {
  const btn: React.CSSProperties = {
    border: `1px solid ${theme.border}`,
    borderRadius: 10,
    padding: '6px 10px',
    background: 'transparent',
    cursor: 'pointer',
    fontSize: 14,
  };

  return (
    <div style={{ paddingBottom: 12, borderBottom: `1px solid ${theme.border}` }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start' }}>
        <div style={{ flex: 1 }}>
          <div style={{ fontSize: 16, fontWeight: 600, color: theme.text }}>{task.title}</div>
          <div style={{ fontSize: 13, color: theme.muted, marginTop: 2 }}>
            {task.duration}m{task.deadline ? ` · ⏰ ${task.deadline}` : ''}
          </div>
        </div>
      </div>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginTop: 8 }}>
        <button onClick={onToggle} style={{ ...btn, color: theme.text }}>
          {task.status === 'todo' ? '✅' : '↩'}
        </button>
        {task.status === 'todo' && onSkip && (
          <button onClick={onSkip} style={{ ...btn, color: theme.muted }}>⏭</button>
        )}
        {task.status === 'skipped' && (
          <span style={{ ...btn, color: theme.muted, opacity: 0.4, cursor: 'default' }}>⏭</span>
        )}
        <button onClick={onEdit} style={{ ...btn, color: theme.text }}>📝</button>
        <button onClick={onDelete} style={{ ...btn, color: theme.text }}>🗑️</button>
      </div>
    </div>
  );
}
