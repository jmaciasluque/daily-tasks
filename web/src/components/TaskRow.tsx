import React from 'react';
import type { Task } from '../types';
import type { Theme } from '../theme/themes';

type Props = {
  task: Task;
  theme: Theme;
  hiddenToday?: boolean;
  onToggle?: () => void;
  onSkip?: () => void;
  onEdit: () => void;
  onDelete: () => void;
};

function formatVisibility(visibility?: number[]): string {
  if (!visibility || visibility.length === 0) return 'Every day';
  const names = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
  return visibility.map((day) => names[day] ?? String(day)).join(', ');
}

export function TaskRow({ task, theme, hiddenToday, onToggle, onSkip, onEdit, onDelete }: Props) {
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
          {hiddenToday && (
            <div style={{
              display: 'inline-flex',
              border: `1px solid ${theme.border}`,
              borderRadius: 8,
              padding: '4px 8px',
              color: theme.muted,
              fontSize: 12,
              marginTop: 8,
            }}>
              Hidden today · {formatVisibility(task.visibility)}
            </div>
          )}
        </div>
      </div>
      <div style={{ display: 'flex', flexWrap: 'wrap', gap: 6, marginTop: 8 }}>
        {onToggle ? (
          <button onClick={onToggle} style={{ ...btn, color: theme.text }}>
            {task.status === 'todo' ? '✅' : '↩'}
          </button>
        ) : (
          <span style={{ ...btn, color: theme.muted, opacity: 0.4, cursor: 'default' }}>
            {task.status === 'todo' ? '✅' : '↩'}
          </span>
        )}
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
