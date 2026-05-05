import React, { useEffect, useMemo, useState } from 'react';
import { useTaskData } from './hooks/useTaskData';
import { TaskRow, TaskEditor, SettingsModal } from './components';
import { orderedTasks } from './services/data';
import { fetchServerStats, pollNextcloudSetup, startNextcloudSetup } from './services/api';
import { getTheme, isLightColor } from './theme/themes';
import type { StatsSummary, Task, TaskStatus } from './types';
import { appVersion } from './config/env';

type Screen = 'tasks' | 'stats';
type StatsPeriod = 'today' | '7d' | '30d' | '90d' | '365d' | 'custom';

function formatDateInput(date: Date): string {
  return date.toISOString().slice(0, 10);
}

function rangeForPeriod(period: Exclude<StatsPeriod, 'custom'>): { from: string; to: string } {
  const end = new Date();
  if (period === 'today') {
    const today = formatDateInput(end);
    return { from: today, to: today };
  }
  const days = period === '7d' ? 7 : period === '90d' ? 90 : period === '365d' ? 365 : 30;
  const start = new Date(end);
  start.setDate(end.getDate() - (days - 1));
  return { from: formatDateInput(start), to: formatDateInput(end) };
}

function formatShortDate(value: string): string {
  const date = new Date(`${value}T00:00:00`);
  return date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

function formatPercent(value: number): string {
  return `${Math.round(value * 100)}%`;
}

function formatDuration(minutes: number): string {
  if (minutes < 60) {
    return `${minutes}m`;
  }
  const hours = Math.floor(minutes / 60);
  const mins = minutes % 60;
  return mins === 0 ? `${hours}h` : `${hours}h ${mins}m`;
}

export default function App() {
  const {
    data,
    serverState,
    statusMsg,
    syncing,
    refreshing,
    refreshServerState,
    reloadFromDisk,
    syncFromRemote,
    addTask,
    editTask,
    deleteTask,
    toggleTaskStatus,
    skipTask,
    cycleTheme,
    chooseLocalBackend,
  } = useTaskData();

  const [screen, setScreen] = useState<Screen>('tasks');
  const [activeStatus, setActiveStatus] = useState<TaskStatus>('todo');
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [isEditorOpen, setIsEditorOpen] = useState(false);
  const [editingTask, setEditingTask] = useState<Task | null>(null);
  const [statsPeriod, setStatsPeriod] = useState<StatsPeriod>('today');
  const [statsRange, setStatsRange] = useState(() => rangeForPeriod('today'));
  const [stats, setStats] = useState<StatsSummary | null>(null);
  const [statsLoading, setStatsLoading] = useState(false);
  const [statsError, setStatsError] = useState('');

  const theme = useMemo(() => getTheme(data.theme_index), [data.theme_index]);
  const list = orderedTasks(data, activeStatus);
  const isLight = isLightColor(theme.bg);

  const todoCount = orderedTasks(data, 'todo').length;
  const doneCount = orderedTasks(data, 'done').length;
  const skippedCount = orderedTasks(data, 'skipped').length;

  useEffect(() => {
    if (screen !== 'stats') {
      return;
    }

    let cancelled = false;
    setStatsLoading(true);
    setStatsError('');

    void fetchServerStats(statsRange.from, statsRange.to)
      .then((payload) => {
        if (!cancelled) {
          setStats(payload);
        }
      })
      .catch((err: Error) => {
        if (!cancelled) {
          setStatsError(err.message);
        }
      })
      .finally(() => {
        if (!cancelled) {
          setStatsLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [screen, statsRange.from, statsRange.to]);

  const openAdd = () => { setEditingTask(null); setIsEditorOpen(true); };
  const openEdit = (task: Task) => { setEditingTask(task); setIsEditorOpen(true); };

  const handleSaveTask = (title: string, duration: number, deadline?: string, visibility?: number[]) => {
    if (editingTask) {
      editTask(editingTask.id, title, duration, deadline, visibility);
    } else {
      addTask(title, duration, deadline, visibility);
    }
    setIsEditorOpen(false);
  };

  const handleDeleteTask = (task: Task) => {
    if (window.confirm(`Delete "${task.title}"?`)) {
      deleteTask(task.id);
    }
  };

  const setPresetPeriod = (period: Exclude<StatsPeriod, 'custom'>) => {
    setStatsPeriod(period);
    setStatsRange(rangeForPeriod(period));
  };

  const updateCustomRange = (field: 'from' | 'to', value: string) => {
    setStatsPeriod('custom');
    setStatsRange((prev) => ({ ...prev, [field]: value }));
  };

  const smallBtn: React.CSSProperties = {
    border: `1px solid ${theme.border}`,
    borderRadius: 10,
    padding: '6px 10px',
    background: 'transparent',
    color: theme.text,
    cursor: 'pointer',
    fontSize: 14,
  };

  const tabBtn = (active: boolean): React.CSSProperties => ({
    flex: 1,
    border: `1px solid ${theme.border}`,
    borderRadius: 14,
    padding: '10px 0',
    background: active ? theme.focusBg : theme.panelBg,
    color: theme.text,
    fontWeight: 700,
    cursor: 'pointer',
    fontSize: 14,
  });

  const statsTotals = stats ? stats.done_count + stats.skipped_count + stats.todo_count : 0;
  const pieStops = statsTotals === 0 ? [0, 0, 0] : [
    (stats!.done_count / statsTotals) * 360,
    ((stats!.done_count + stats!.skipped_count) / statsTotals) * 360,
    360,
  ];
  const chartColors = {
    done: theme.accent,
    skipped: theme.focusBorder,
    todo: theme.muted,
  };
  const canSync = serverState?.backend === 'nextcloud' && serverState.sync_configured;

  const maxDailyTasks = useMemo(() => {
    if (!stats || stats.daily.length === 0) {
      return 1;
    }
    return Math.max(...stats.daily.map((day) => day.task_count), 1);
  }, [stats]);

  const renderTasksScreen = () => (
    <>
      <div style={{ display: 'flex', gap: 8, paddingTop: 8 }}>
        <button onClick={() => setActiveStatus('todo')} style={tabBtn(activeStatus === 'todo')}>To Do ({todoCount})</button>
        <button onClick={() => setActiveStatus('done')} style={tabBtn(activeStatus === 'done')}>Done ({doneCount})</button>
        <button onClick={() => setActiveStatus('skipped')} style={{ ...tabBtn(activeStatus === 'skipped'), color: theme.muted }}>Skipped ({skippedCount})</button>
      </div>

      <div style={{
        flex: 1,
        margin: '18px 0',
        backgroundColor: theme.panelBg,
        border: `1px solid ${theme.border}`,
        borderRadius: 20,
        padding: 12,
        overflowY: 'auto',
        display: 'flex',
        flexDirection: 'column',
        gap: 10,
      }}>
        {list.length === 0 ? (
          <div style={{ textAlign: 'center', padding: '24px 0', color: theme.muted }}>No tasks.</div>
        ) : (
          list.map((task) => (
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

      <div style={{ display: 'flex', gap: 12, paddingBottom: 8 }}>
        <button onClick={openAdd} style={{ borderRadius: 12, padding: '12px 16px', background: theme.accent, color: isLight ? theme.text : '#111111', fontWeight: 700, border: 'none', cursor: 'pointer', fontSize: 14 }}>
          Add Task
        </button>
        <button onClick={() => reloadFromDisk()} style={{ borderRadius: 12, padding: '12px 16px', border: `1px solid ${theme.border}`, background: 'transparent', color: theme.text, cursor: 'pointer', fontSize: 14 }}>
          {refreshing ? 'Refreshing...' : 'Refresh'}
        </button>
        <button
          onClick={() => syncFromRemote()}
          disabled={!canSync || syncing}
          style={{
            borderRadius: 12,
            padding: '12px 16px',
            border: `1px solid ${theme.border}`,
            background: 'transparent',
            color: canSync ? theme.text : theme.muted,
            cursor: canSync && !syncing ? 'pointer' : 'default',
            fontSize: 14,
            opacity: canSync ? 1 : 0.65,
          }}
        >
          {syncing ? 'Syncing...' : canSync ? 'Sync' : 'Local Only'}
        </button>
      </div>
    </>
  );

  const renderStatsScreen = () => (
    <div style={{
      flex: 1,
      margin: '18px 0',
      display: 'flex',
      flexDirection: 'column',
      gap: 14,
      overflowY: 'auto',
      paddingBottom: 8,
    }}>
      <div style={{
        background: `linear-gradient(135deg, ${theme.panelBg} 0%, ${theme.focusBg} 100%)`,
        border: `1px solid ${theme.border}`,
        borderRadius: 24,
        padding: 18,
        display: 'grid',
        gap: 14,
      }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 12, flexWrap: 'wrap' }}>
          <div>
            <div style={{ fontSize: 24, fontWeight: 800 }}>Stats</div>
            <div style={{ color: theme.muted, marginTop: 4 }}>
              {stats?.from && stats?.to ? `${stats.from} to ${stats.to}` : `${statsRange.from} to ${statsRange.to}`}
            </div>
          </div>
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            {(['today', '7d', '30d', '90d', '365d'] as const).map((period) => (
              <button
                key={period}
                onClick={() => setPresetPeriod(period)}
                style={{
                  ...smallBtn,
                  background: statsPeriod === period ? theme.focusBg : 'transparent',
                  fontWeight: statsPeriod === period ? 700 : 500,
                }}
              >
                {period}
              </button>
            ))}
          </div>
        </div>

        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))', gap: 10 }}>
          <input
            type="date"
            value={statsRange.from}
            onChange={(event) => updateCustomRange('from', event.target.value)}
            style={{ borderRadius: 12, border: `1px solid ${theme.border}`, background: theme.panelBg, color: theme.text, padding: '10px 12px', font: 'inherit' }}
          />
          <input
            type="date"
            value={statsRange.to}
            onChange={(event) => updateCustomRange('to', event.target.value)}
            style={{ borderRadius: 12, border: `1px solid ${theme.border}`, background: theme.panelBg, color: theme.text, padding: '10px 12px', font: 'inherit' }}
          />
        </div>
      </div>

      {statsLoading ? (
        <div style={{ color: theme.muted, textAlign: 'center', padding: '30px 0' }}>Loading stats...</div>
      ) : null}

      {!statsLoading && statsError ? (
        <div style={{ color: theme.text, background: theme.panelBg, border: `1px solid ${theme.border}`, borderRadius: 20, padding: 18 }}>
          {statsError}
        </div>
      ) : null}

      {!statsLoading && !statsError && stats ? (
        <>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(140px, 1fr))', gap: 12 }}>
            {[
              { label: 'Recorded Days', value: stats.recorded_days },
              { label: 'Completion Rate', value: formatPercent(stats.completion_rate) },
              { label: 'Done Time', value: formatDuration(stats.done_duration) },
              { label: 'Skipped Time', value: formatDuration(stats.skipped_duration) },
            ].map((card) => (
              <div key={card.label} style={{ background: theme.panelBg, border: `1px solid ${theme.border}`, borderRadius: 18, padding: 16 }}>
                <div style={{ color: theme.muted, fontSize: 12, textTransform: 'uppercase', letterSpacing: '0.08em' }}>{card.label}</div>
                <div style={{ fontSize: 28, fontWeight: 800, marginTop: 8 }}>{card.value}</div>
              </div>
            ))}
          </div>

          <div style={{ display: 'grid', gridTemplateColumns: 'minmax(0, 240px) minmax(0, 1fr)', gap: 12 }}>
            <div style={{ background: theme.panelBg, border: `1px solid ${theme.border}`, borderRadius: 22, padding: 18 }}>
              <div style={{ fontSize: 15, fontWeight: 700 }}>Status mix</div>
              <div style={{ color: theme.muted, fontSize: 13, marginTop: 4 }}>All recorded task snapshots in this range</div>
              <div style={{ display: 'flex', justifyContent: 'center', padding: '18px 0' }}>
                <div style={{
                  width: 170,
                  height: 170,
                  borderRadius: '50%',
                  background: statsTotals === 0
                    ? theme.border
                    : `conic-gradient(${chartColors.done} 0deg ${pieStops[0]}deg, ${chartColors.skipped} ${pieStops[0]}deg ${pieStops[1]}deg, ${chartColors.todo} ${pieStops[1]}deg ${pieStops[2]}deg)`,
                  position: 'relative',
                }}>
                  <div style={{
                    position: 'absolute',
                    inset: 26,
                    borderRadius: '50%',
                    background: theme.panelBg,
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    flexDirection: 'column',
                    textAlign: 'center',
                  }}>
                    <div style={{ fontSize: 28, fontWeight: 800 }}>{stats.task_count}</div>
                    <div style={{ color: theme.muted, fontSize: 12 }}>snapshots</div>
                  </div>
                </div>
              </div>
              <div style={{ display: 'grid', gap: 8 }}>
                {[
                  ['Done', stats.done_count, chartColors.done],
                  ['Skipped', stats.skipped_count, chartColors.skipped],
                  ['Todo', stats.todo_count, chartColors.todo],
                ].map(([label, value, color]) => (
                  <div key={label as string} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <span style={{ width: 10, height: 10, borderRadius: '50%', background: color as string, display: 'inline-block' }} />
                      <span>{label as string}</span>
                    </div>
                    <strong>{value as number}</strong>
                  </div>
                ))}
              </div>
            </div>

            <div style={{ background: theme.panelBg, border: `1px solid ${theme.border}`, borderRadius: 22, padding: 18 }}>
              <div style={{ fontSize: 15, fontWeight: 700 }}>Daily histogram</div>
              <div style={{ color: theme.muted, fontSize: 13, marginTop: 4 }}>Done, skipped, and remaining tasks captured each day</div>
              <div style={{ display: 'flex', alignItems: 'flex-end', gap: 8, minHeight: 220, paddingTop: 18, overflowX: 'auto' }}>
                {stats.daily.map((day) => (
                  <div key={day.date} style={{ minWidth: 28, display: 'grid', gap: 8, justifyItems: 'center' }}>
                    <div style={{ width: 24, height: 170, display: 'flex', flexDirection: 'column-reverse', borderRadius: 999, overflow: 'hidden', background: theme.bg, border: `1px solid ${theme.border}` }}>
                      <div style={{ height: `${(day.done_count / maxDailyTasks) * 100}%`, background: chartColors.done }} />
                      <div style={{ height: `${(day.skipped_count / maxDailyTasks) * 100}%`, background: chartColors.skipped }} />
                      <div style={{ height: `${(day.todo_count / maxDailyTasks) * 100}%`, background: chartColors.todo }} />
                    </div>
                    <div style={{ fontSize: 11, color: theme.muted, writingMode: 'vertical-rl', transform: 'rotate(180deg)' }}>{formatShortDate(day.date)}</div>
                  </div>
                ))}
              </div>
            </div>
          </div>

          <div style={{ background: theme.panelBg, border: `1px solid ${theme.border}`, borderRadius: 22, padding: 18 }}>
            <div style={{ fontSize: 15, fontWeight: 700 }}>Task frequency</div>
            <div style={{ color: theme.muted, fontSize: 13, marginTop: 4 }}>Which tasks recur most often and how consistently they get done</div>
            <div style={{ display: 'grid', gap: 10, marginTop: 14 }}>
              {stats.tasks.slice(0, 8).map((task) => (
                <div key={task.task_id} style={{ border: `1px solid ${theme.border}`, borderRadius: 18, padding: 14, background: theme.bg }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, alignItems: 'baseline' }}>
                    <div style={{ fontWeight: 700 }}>{task.title}</div>
                    <div style={{ color: theme.muted, fontSize: 13 }}>{task.recorded_days} days</div>
                  </div>
                  <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, minmax(0, 1fr))', gap: 10, marginTop: 12 }}>
                    <div>
                      <div style={{ color: theme.muted, fontSize: 12 }}>Done</div>
                      <div style={{ fontSize: 20, fontWeight: 800 }}>{task.done_days}</div>
                    </div>
                    <div>
                      <div style={{ color: theme.muted, fontSize: 12 }}>Skipped</div>
                      <div style={{ fontSize: 20, fontWeight: 800 }}>{task.skipped_days}</div>
                    </div>
                    <div>
                      <div style={{ color: theme.muted, fontSize: 12 }}>Completion</div>
                      <div style={{ fontSize: 20, fontWeight: 800 }}>{formatPercent(task.completion_rate)}</div>
                    </div>
                  </div>
                </div>
              ))}
              {stats.tasks.length === 0 ? (
                <div style={{ color: theme.muted }}>No history yet. Use the app for a day or import older history to populate this view.</div>
              ) : null}
            </div>
          </div>
        </>
      ) : null}
    </div>
  );

  return (
    <div style={{ minHeight: '100vh', background: `radial-gradient(circle at top, ${theme.focusBg} 0%, ${theme.bg} 48%)`, color: theme.text }}>
      <div style={{ maxWidth: 980, margin: '0 auto', padding: '0 18px', minHeight: '100vh', display: 'flex', flexDirection: 'column' }}>
        <div style={{ borderBottom: `1px solid ${theme.border}`, padding: '16px 0', display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', gap: 16 }}>
          <div>
            <div style={{ fontSize: 28, fontWeight: 800, color: theme.text }}>Daily Tasks</div>
            <div style={{ fontSize: 13, color: theme.muted, marginTop: 4 }}>Theme: {theme.name}</div>
          </div>
          <div style={{ display: 'flex', gap: 8, flexWrap: 'wrap' }}>
            <button onClick={() => setScreen('tasks')} style={{ ...smallBtn, background: screen === 'tasks' ? theme.focusBg : 'transparent' }}>Tasks</button>
            <button onClick={() => setScreen('stats')} style={{ ...smallBtn, background: screen === 'stats' ? theme.focusBg : 'transparent' }}>Stats</button>
            <button onClick={cycleTheme} style={smallBtn}>Theme</button>
            <button onClick={() => setIsSettingsOpen(true)} style={smallBtn}>Config</button>
          </div>
        </div>

        {screen === 'tasks' ? renderTasksScreen() : renderStatsScreen()}

        {statusMsg ? <div style={{ textAlign: 'center', color: theme.muted, paddingBottom: 4, fontSize: 13 }}>{statusMsg}</div> : null}
        <div style={{ textAlign: 'center', color: theme.muted, paddingBottom: 12, fontSize: 13 }}>Version {serverState?.version || appVersion}</div>
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
        serverState={serverState}
        theme={theme}
        onUseLocal={chooseLocalBackend}
        onStartNextcloud={startNextcloudSetup}
        onPollNextcloud={pollNextcloudSetup}
        onConfigured={() => refreshServerState('Connected to Nextcloud.')}
        onClose={() => setIsSettingsOpen(false)}
      />
    </div>
  );
}
