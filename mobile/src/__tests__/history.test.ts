import {
  aggregateStats,
  applyDailyResetWithHistory,
  buildStatsSummary,
  emptyHistory,
  recordDataChange,
} from '../services/history';
import type { Data } from '../types';

describe('recordDataChange', () => {
  it('records snapshots and task events', () => {
    const before: Data = {
      last_reset: '2026-04-07',
      next_id: 3,
      tasks: [
        { id: 1, title: 'Workout', duration: 30, status: 'todo', order: 1 },
      ],
      theme_index: 0,
    };
    const after: Data = {
      ...before,
      tasks: [
        { id: 1, title: 'Workout', duration: 30, status: 'done', order: 1 },
        { id: 2, title: 'Read', duration: 20, status: 'todo', order: 2 },
      ],
    };

    const history = recordDataChange(emptyHistory(), before, after, 1712476800000);

    expect(history.days).toHaveLength(1);
    expect(history.days[0].tasks).toHaveLength(2);
    expect(history.days[0].tasks[0].status).toBe('done');
    expect(history.events).toHaveLength(2);
    expect(history.events.map((event) => event.type)).toEqual(['status_changed', 'task_added']);
  });
});

describe('applyDailyResetWithHistory', () => {
  it('preserves the previous day snapshot before resetting tasks to todo', () => {
    jest.useFakeTimers().setSystemTime(new Date('2026-04-08T09:00:00Z'));

    const data: Data = {
      last_reset: '2026-04-07',
      next_id: 3,
      tasks: [
        { id: 1, title: 'Workout', duration: 30, status: 'done', order: 1 },
        { id: 2, title: 'Read', duration: 20, status: 'skipped', order: 1 },
      ],
      theme_index: 0,
    };

    const { data: resetData, history, changed } = applyDailyResetWithHistory(data, emptyHistory(), new Date('2026-04-08T09:00:00Z').getTime());

    expect(changed).toBe(true);
    expect(resetData.last_reset).toBe('2026-04-08');
    expect(resetData.tasks.every((task) => task.status === 'todo')).toBe(true);
    expect(history.days).toHaveLength(2);
    expect(history.days[0].date).toBe('2026-04-07');
    expect(history.days[0].tasks.map((task) => task.status)).toEqual(['done', 'skipped']);
    expect(history.days[1].date).toBe('2026-04-08');
    expect(history.days[1].tasks.map((task) => task.status)).toEqual(['todo', 'todo']);

    jest.useRealTimers();
  });
});

describe('aggregateStats', () => {
  it('builds summary totals and task rankings from recorded days', () => {
    const history = {
      version: 1,
      days: [
        {
          date: '2026-04-07',
          tasks: [
            { id: 1, title: 'Workout', duration: 30, status: 'done' as const },
            { id: 2, title: 'Read', duration: 20, status: 'skipped' as const },
          ],
        },
        {
          date: '2026-04-08',
          tasks: [
            { id: 1, title: 'Workout', duration: 30, status: 'done' as const },
            { id: 2, title: 'Read', duration: 20, status: 'todo' as const },
          ],
        },
      ],
      events: [],
    };

    const stats = aggregateStats(history, '2026-04-07', '2026-04-08');

    expect(stats.recorded_days).toBe(2);
    expect(stats.done_count).toBe(2);
    expect(stats.todo_count).toBe(1);
    expect(stats.skipped_count).toBe(1);
    expect(stats.done_duration).toBe(60);
    expect(stats.tasks[0].title).toBe('Workout');
    expect(stats.tasks[0].done_days).toBe(2);
  });
});

describe('buildStatsSummary', () => {
  it('includes the current day even when history is empty', () => {
    const current: Data = {
      last_reset: '2026-04-15',
      next_id: 3,
      tasks: [
        { id: 1, title: 'Workout', duration: 30, status: 'done', order: 1 },
        { id: 2, title: 'Read', duration: 20, status: 'todo', order: 2 },
      ],
      theme_index: 0,
    };

    const stats = buildStatsSummary(emptyHistory(), current, '2026-04-15', '2026-04-15');

    expect(stats.recorded_days).toBe(1);
    expect(stats.task_count).toBe(2);
    expect(stats.completion_rate).toBe(0.5);
    expect(stats.daily[0].date).toBe('2026-04-15');
  });
});
