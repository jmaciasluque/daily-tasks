import React, { useState, useEffect } from 'react';
import type { Task } from '../types';
import type { Theme } from '../theme/themes';

type Props = {
  visible: boolean;
  task: Task | null;
  theme: Theme;
  onSave: (title: string, duration: number, deadline?: string) => void;
  onClose: () => void;
};

export function TaskEditor({ visible, task, theme, onSave, onClose }: Props) {
  const [title, setTitle] = useState('');
  const [duration, setDuration] = useState('5');
  const [deadline, setDeadline] = useState('');
  const [error, setError] = useState('');

  useEffect(() => {
    if (task) {
      setTitle(task.title);
      setDuration(String(task.duration));
      setDeadline(task.deadline ?? '');
    } else {
      setTitle('');
      setDuration('5');
      setDeadline('');
    }
    setError('');
  }, [task, visible]);

  if (!visible) return null;

  const handleSave = () => {
    const trimmedTitle = title.trim();
    const parsedDuration = Number.parseInt(duration, 10);

    if (!trimmedTitle) { setError('Title cannot be empty.'); return; }
    if (!Number.isFinite(parsedDuration) || parsedDuration <= 0) { setError('Duration must be a positive integer.'); return; }

    const trimmedDeadline = deadline.trim();
    if (trimmedDeadline) {
      if (!/^\d{2}:\d{2}$/.test(trimmedDeadline)) { setError('Deadline must be in HH:MM format (e.g. 09:30).'); return; }
      const [h, m] = trimmedDeadline.split(':').map(Number);
      if (h < 0 || h > 23 || m < 0 || m > 59) { setError('Deadline time is out of range.'); return; }
    }

    onSave(trimmedTitle, parsedDuration, trimmedDeadline || undefined);
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
        <div style={{ fontSize: 18, fontWeight: 700, color: theme.text }}>
          {task ? 'Edit Task' : 'Add Task'}
        </div>
        <input placeholder="Title" value={title} onChange={e => setTitle(e.target.value)} style={input} />
        <input placeholder="Duration (minutes)" value={duration} onChange={e => setDuration(e.target.value)} type="number" min="1" style={input} />
        <input placeholder="Deadline (HH:MM, optional)" value={deadline} onChange={e => setDeadline(e.target.value)} style={input} />
        <div style={{ fontSize: 12, color: theme.muted, marginTop: -4 }}>
          Set a deadline to receive a daily notification reminder.
        </div>
        {error && <div style={{ color: '#f87171', fontSize: 13 }}>{error}</div>}
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 12, marginTop: 8 }}>
          <button onClick={onClose} style={{ border: `1px solid ${theme.border}`, borderRadius: 12, padding: '12px 16px', background: 'transparent', color: theme.text, cursor: 'pointer' }}>
            Cancel
          </button>
          <button onClick={handleSave} style={{ border: 'none', borderRadius: 12, padding: '12px 16px', background: theme.accent, color: '#111111', fontWeight: 700, cursor: 'pointer' }}>
            Save
          </button>
        </div>
      </div>
    </div>
  );
}
