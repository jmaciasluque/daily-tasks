import React, { useMemo, useState } from 'react';
import { useTaskData } from './hooks/useTaskData';
import { TaskRow, TaskEditor, SettingsModal } from './components';
import { orderedTasks } from './services/data';
import { getTheme, isLightColor } from './theme/themes';
import type { Task, TaskStatus, Settings } from './types';
import { appVersion } from './config/env';

export default function App() {
  const {
    data,
    settings,
    statusMsg,
    syncing,
    syncFromRemote,
    addTask,
    editTask,
    deleteTask,
    toggleTaskStatus,
    skipTask,
    cycleTheme,
    updateSettings,
  } = useTaskData();

  const [activeStatus, setActiveStatus] = useState<TaskStatus>('todo');
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [isEditorOpen, setIsEditorOpen] = useState(false);
  const [editingTask, setEditingTask] = useState<Task | null>(null);
  const [pendingSettings, setPendingSettings] = useState<Settings>(settings);

  const theme = useMemo(() => getTheme(data.theme_index), [data.theme_index]);
  const list = orderedTasks(data, activeStatus);
  const isLight = isLightColor(theme.bg);

  const todoCount = data.tasks.filter(t => t.status === 'todo').length;
  const doneCount = data.tasks.filter(t => t.status === 'done').length;
  const skippedCount = data.tasks.filter(t => t.status === 'skipped').length;

  const openAdd = () => { setEditingTask(null); setIsEditorOpen(true); };
  const openEdit = (task: Task) => { setEditingTask(task); setIsEditorOpen(true); };

  const handleSaveTask = (title: string, duration: number, deadline?: string) => {
    if (editingTask) editTask(editingTask.id, title, duration, deadline);
    else addTask(title, duration, deadline);
    setIsEditorOpen(false);
  };

  const handleDeleteTask = (task: Task) => {
    if (window.confirm(`Delete "${task.title}"?`)) deleteTask(task.id);
  };

  const openSettings = () => { setPendingSettings(settings); setIsSettingsOpen(true); };

  const handleSaveSettings = async () => {
    await updateSettings(pendingSettings);
    setIsSettingsOpen(false);
  };

  const tabBtn = (status: TaskStatus, label: string): React.CSSProperties => ({
    flex: 1,
    border: `1px solid ${theme.border}`,
    borderRadius: 12,
    padding: '10px 0',
    background: activeStatus === status ? theme.focusBg : theme.panelBg,
    color: theme.text,
    fontWeight: 600,
    cursor: 'pointer',
    fontSize: 14,
  });

  const smallBtn: React.CSSProperties = {
    border: `1px solid ${theme.border}`,
    borderRadius: 10,
    padding: '6px 10px',
    background: 'transparent',
    color: theme.text,
    cursor: 'pointer',
    fontSize: 14,
  };

  return (
    <div style={{ minHeight: '100vh', backgroundColor: theme.bg, color: theme.text }}>
      <div style={{ maxWidth: 600, margin: '0 auto', padding: '0 18px', minHeight: '100vh', display: 'flex', flexDirection: 'column' }}>

        {/* Header */}
        <div style={{ borderBottom: `1px solid ${theme.border}`, padding: '16px 0', display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 16 }}>
          <div>
            <div style={{ fontSize: 24, fontWeight: 700, color: theme.text }}>Daily Tasks</div>
            <div style={{ fontSize: 13, color: theme.muted, marginTop: 4 }}>Theme: {theme.name}</div>
          </div>
          <div style={{ display: 'flex', gap: 8 }}>
            <button onClick={cycleTheme} style={smallBtn}>Theme</button>
            <button onClick={openSettings} style={smallBtn}>Settings</button>
          </div>
        </div>

        {/* Tab switcher */}
        <div style={{ display: 'flex', gap: 8, paddingTop: 8 }}>
          <button onClick={() => setActiveStatus('todo')} style={tabBtn('todo', '')}>To Do ({todoCount})</button>
          <button onClick={() => setActiveStatus('done')} style={tabBtn('done', '')}>Done ({doneCount})</button>
          <button onClick={() => setActiveStatus('skipped')} style={{ ...tabBtn('skipped', ''), color: theme.muted }}>Skipped ({skippedCount})</button>
        </div>

        {/* Task list */}
        <div style={{
          flex: 1, margin: '18px 0',
          backgroundColor: theme.panelBg, border: `1px solid ${theme.border}`,
          borderRadius: 16, padding: 12, overflowY: 'auto',
          display: 'flex', flexDirection: 'column', gap: 10,
        }}>
          {list.length === 0 ? (
            <div style={{ textAlign: 'center', padding: '24px 0', color: theme.muted }}>No tasks.</div>
          ) : (
            list.map(task => (
              <TaskRow
                key={task.id}
                task={task}
                theme={theme}
                onToggle={() => toggleTaskStatus(task)}
                onSkip={task.status === 'todo' ? () => skipTask(task.id) : undefined}
                onEdit={() => openEdit(task)}
                onDelete={() => handleDeleteTask(task)}
              />
            ))
          )}
        </div>

        {/* Footer */}
        <div style={{ display: 'flex', gap: 12, paddingBottom: 8 }}>
          <button onClick={openAdd} style={{ borderRadius: 12, padding: '12px 16px', background: theme.accent, color: isLight ? theme.text : '#111111', fontWeight: 700, border: 'none', cursor: 'pointer', fontSize: 14 }}>
            Add Task
          </button>
          <button onClick={() => syncFromRemote()} style={{ borderRadius: 12, padding: '12px 16px', border: `1px solid ${theme.border}`, background: 'transparent', color: theme.text, cursor: 'pointer', fontSize: 14 }}>
            {syncing ? 'Syncing...' : 'Sync'}
          </button>
        </div>

        {/* Status */}
        {statusMsg ? <div style={{ textAlign: 'center', color: theme.muted, paddingBottom: 4, fontSize: 13 }}>{statusMsg}</div> : null}
        <div style={{ textAlign: 'center', color: theme.muted, paddingBottom: 12, fontSize: 13 }}>Version {appVersion}</div>
      </div>

      <TaskEditor
        visible={isEditorOpen}
        task={editingTask}
        theme={theme}
        onSave={handleSaveTask}
        onClose={() => setIsEditorOpen(false)}
      />
      <SettingsModal
        visible={isSettingsOpen}
        settings={pendingSettings}
        theme={theme}
        onUpdate={setPendingSettings}
        onSave={handleSaveSettings}
        onClose={() => setIsSettingsOpen(false)}
      />
    </div>
  );
}
