import { cloneData, normalizeData, resetIfNeeded } from './data';
import type {
  Data,
  DailyStats,
  History,
  HistoryDay,
  HistoryEvent,
  StatsSummary,
  Task,
  TaskFrequencyStats,
  TaskSnapshot,
} from '../types';

const HISTORY_VERSION = 1;

type TaskAccumulator = TaskFrequencyStats;

export function emptyHistory(): History {
  return {
    version: HISTORY_VERSION,
    days: [],
    events: [],
  };
}

export function normalizeHistory(input?: Partial<History> | null): History {
  const history: History = {
    version: input?.version || HISTORY_VERSION,
    updated_at: input?.updated_at,
    days: (input?.days ?? []).map((day) => ({
      date: day.date || '',
      updated_at: day.updated_at,
      tasks: (day.tasks ?? []).map((task) => ({
        id: task.id,
        title: task.title,
        duration: task.duration,
        status: task.status,
        deadline: task.deadline,
      })),
    })),
    events: (input?.events ?? []).map((event) => ({
      timestamp: event.timestamp,
      date: event.date || '',
      type: event.type,
      task_id: event.task_id,
      title: event.title,
      from_status: event.from_status,
      to_status: event.to_status,
      duration: event.duration,
      deadline: event.deadline,
    })),
  };

  sortHistory(history);
  return history;
}

export function ensureHistorySnapshot(historyInput: History, current: Data, now = Date.now()): History {
  const history = normalizeHistory(historyInput);
  const changed = upsertHistoryDay(history, normalizeData(cloneData(current)), now);
  if (changed) {
    history.updated_at = now;
  }
  sortHistory(history);
  return history;
}

export function historyContentEqual(aInput: History, bInput: History): boolean {
  const a = normalizeHistory(aInput);
  const b = normalizeHistory(bInput);

  if (a.version !== b.version || a.days.length !== b.days.length || a.events.length !== b.events.length) {
    return false;
  }

  for (let i = 0; i < a.days.length; i += 1) {
    if (!historyDayEqual(a.days[i], b.days[i])) {
      return false;
    }
  }

  for (let i = 0; i < a.events.length; i += 1) {
    const left = a.events[i];
    const right = b.events[i];
    if (
      left.timestamp !== right.timestamp ||
      left.date !== right.date ||
      left.type !== right.type ||
      left.task_id !== right.task_id ||
      left.title !== right.title ||
      left.from_status !== right.from_status ||
      left.to_status !== right.to_status ||
      left.duration !== right.duration ||
      left.deadline !== right.deadline
    ) {
      return false;
    }
  }

  return true;
}

export function mergeHistories(localInput: History, remoteInput: History): History {
  const local = normalizeHistory(localInput);
  const remote = normalizeHistory(remoteInput);

  const dayMap = new Map<string, HistoryDay>();
  for (const day of [...local.days, ...remote.days]) {
    const existing = dayMap.get(day.date);
    if (!existing) {
      dayMap.set(day.date, { ...day, tasks: day.tasks.map((task) => ({ ...task })) });
      continue;
    }

    const existingUpdatedAt = existing.updated_at || 0;
    const nextUpdatedAt = day.updated_at || 0;
    if (nextUpdatedAt > existingUpdatedAt || (nextUpdatedAt === existingUpdatedAt && day.tasks.length >= existing.tasks.length)) {
      dayMap.set(day.date, { ...day, tasks: day.tasks.map((task) => ({ ...task })) });
    }
  }

  const eventMap = new Map<string, HistoryEvent>();
  for (const event of [...local.events, ...remote.events]) {
    const key = [
      event.timestamp,
      event.date,
      event.type,
      event.task_id,
      event.title,
      event.from_status || '',
      event.to_status || '',
      event.duration || 0,
      event.deadline || '',
    ].join('|');
    if (!eventMap.has(key)) {
      eventMap.set(key, { ...event });
    }
  }

  const merged: History = {
    version: Math.max(local.version || HISTORY_VERSION, remote.version || HISTORY_VERSION),
    updated_at: Math.max(local.updated_at || 0, remote.updated_at || 0),
    days: Array.from(dayMap.values()),
    events: Array.from(eventMap.values()),
  };

  sortHistory(merged);
  return merged;
}

export function recordDataChange(historyInput: History, before: Data, after: Data, now = Date.now()): History {
  const history = normalizeHistory(historyInput);
  const normalizedBefore = normalizeData(cloneData(before));
  const normalizedAfter = normalizeData(cloneData(after));

  let changed = false;
  const initialEventCount = history.events.length;

  if (normalizedBefore.last_reset) {
    changed = upsertHistoryDay(history, normalizedBefore, now) || changed;
  }

  appendHistoryEvents(history, normalizedBefore, normalizedAfter, now);
  changed = upsertHistoryDay(history, normalizedAfter, now) || changed;

  if (!changed && history.events.length === initialEventCount) {
    return history;
  }

  history.updated_at = now;
  sortHistory(history);
  return history;
}

export function applyDailyResetWithHistory(dataInput: Data, historyInput: History, now = Date.now()): {
  data: Data;
  history: History;
  changed: boolean;
} {
  const before = normalizeData(cloneData(dataInput));
  const after = resetIfNeeded(cloneData(before));
  if (before.last_reset === after.last_reset) {
    return { data: after, history: normalizeHistory(historyInput), changed: false };
  }

  const history = recordDataChange(historyInput, before, after, now);
  return { data: after, history, changed: true };
}

export function buildStatsSummary(historyInput: History, current: Data, from: string, to: string): StatsSummary {
  const history = ensureHistorySnapshot(historyInput, current, Date.now());
  return aggregateStats(history, from, to);
}

export function aggregateStats(historyInput: History, from: string, to: string): StatsSummary {
  const history = normalizeHistory(historyInput);
  const filtered = history.days.filter((day) => {
    if (from && day.date < from) {
      return false;
    }
    if (to && day.date > to) {
      return false;
    }
    return true;
  });

  const summary: StatsSummary = {
    from,
    to,
    recorded_days: filtered.length,
    task_count: 0,
    todo_count: 0,
    done_count: 0,
    skipped_count: 0,
    todo_duration: 0,
    done_duration: 0,
    skipped_duration: 0,
    completion_rate: 0,
    daily: [],
    tasks: [],
  };

  const taskMap = new Map<number, TaskAccumulator>();

  for (const day of filtered) {
    const daily: DailyStats = {
      date: day.date,
      task_count: 0,
      todo_count: 0,
      done_count: 0,
      skipped_count: 0,
      todo_duration: 0,
      done_duration: 0,
      skipped_duration: 0,
      completion_rate: 0,
    };

    for (const task of day.tasks) {
      daily.task_count += 1;
      summary.task_count += 1;

      const currentTask = taskMap.get(task.id) ?? {
        task_id: task.id,
        title: task.title,
        recorded_days: 0,
        todo_days: 0,
        done_days: 0,
        skipped_days: 0,
        completion_rate: 0,
        total_duration: 0,
        done_duration: 0,
        skipped_duration: 0,
      };

      currentTask.title = task.title;
      currentTask.recorded_days += 1;
      currentTask.total_duration += task.duration;
      taskMap.set(task.id, currentTask);

      switch (task.status) {
        case 'done':
          daily.done_count += 1;
          daily.done_duration += task.duration;
          summary.done_count += 1;
          summary.done_duration += task.duration;
          currentTask.done_days += 1;
          currentTask.done_duration += task.duration;
          break;
        case 'skipped':
          daily.skipped_count += 1;
          daily.skipped_duration += task.duration;
          summary.skipped_count += 1;
          summary.skipped_duration += task.duration;
          currentTask.skipped_days += 1;
          currentTask.skipped_duration += task.duration;
          break;
        default:
          daily.todo_count += 1;
          daily.todo_duration += task.duration;
          summary.todo_count += 1;
          summary.todo_duration += task.duration;
          currentTask.todo_days += 1;
          break;
      }
    }

    if (daily.task_count > 0) {
      daily.completion_rate = daily.done_count / daily.task_count;
    }

    summary.daily.push(daily);
  }

  if (summary.task_count > 0) {
    summary.completion_rate = summary.done_count / summary.task_count;
  }

  summary.tasks = Array.from(taskMap.values())
    .map((task) => ({
      ...task,
      completion_rate: task.recorded_days > 0 ? task.done_days / task.recorded_days : 0,
    }))
    .sort((a, b) => {
      if (a.done_days === b.done_days) {
        if (a.recorded_days === b.recorded_days) {
          return a.task_id - b.task_id;
        }
        return b.recorded_days - a.recorded_days;
      }
      return b.done_days - a.done_days;
    });

  return summary;
}

function sortHistory(history: History): void {
  history.days.sort((a, b) => a.date.localeCompare(b.date));
  history.events.sort((a, b) => {
    if (a.timestamp === b.timestamp) {
      if (a.task_id === b.task_id) {
        return a.type.localeCompare(b.type);
      }
      return a.task_id - b.task_id;
    }
    return a.timestamp - b.timestamp;
  });
}

function upsertHistoryDay(history: History, data: Data, now: number): boolean {
  const day: HistoryDay = {
    date: data.last_reset,
    updated_at: now,
    tasks: snapshotsForVisibleTasks(data.tasks, data.last_reset),
  };

  const index = history.days.findIndex((entry) => entry.date === day.date);
  if (index >= 0) {
    if (historyDayEqual(history.days[index], day)) {
      return false;
    }
    history.days[index] = day;
    return true;
  }

  history.days.push(day);
  return true;
}

function historyDayEqual(a: HistoryDay, b: HistoryDay): boolean {
  if (a.date !== b.date || a.tasks.length !== b.tasks.length) {
    return false;
  }

  return a.tasks.every((task, index) => {
    const other = b.tasks[index];
    return (
      task.id === other.id &&
      task.title === other.title &&
      task.duration === other.duration &&
      task.status === other.status &&
      task.deadline === other.deadline
    );
  });
}

// Returns snapshots only for tasks visible on the given date (YYYY-MM-DD).
// Falls back to all tasks if the date can't be parsed.
function snapshotsForVisibleTasks(tasks: Task[], date: string): TaskSnapshot[] {
  const parsed = new Date(`${date}T00:00:00`);
  if (isNaN(parsed.getTime())) {
    return snapshotsForTasks(tasks);
  }
  const weekday = parsed.getDay(); // 0=Sun … 6=Sat
  const visible = tasks.filter(
    (t) => !t.visibility || t.visibility.length === 0 || t.visibility.includes(weekday),
  );
  return snapshotsForTasks(visible);
}

function snapshotsForTasks(tasks: Task[]): TaskSnapshot[] {
  return tasks
    .map((task) => ({
      id: task.id,
      title: task.title,
      duration: task.duration,
      status: task.status,
      deadline: task.deadline,
    }))
    .sort((a, b) => a.id - b.id);
}

function appendHistoryEvents(history: History, before: Data, after: Data, now: number): void {
  const beforeMap = new Map(before.tasks.map((task) => [task.id, task]));
  const afterMap = new Map(after.tasks.map((task) => [task.id, task]));

  for (const [id, previous] of beforeMap) {
    const next = afterMap.get(id);
    if (!next) {
      history.events.push({
        timestamp: now,
        date: before.last_reset,
        type: 'task_deleted',
        task_id: previous.id,
        title: previous.title,
        from_status: previous.status,
        duration: previous.duration,
        deadline: previous.deadline,
      });
      continue;
    }

    if (previous.status !== next.status) {
      history.events.push({
        timestamp: now,
        date: after.last_reset,
        type: 'status_changed',
        task_id: next.id,
        title: next.title,
        from_status: previous.status,
        to_status: next.status,
        duration: next.duration,
        deadline: next.deadline,
      });
    }

    if (
      previous.title !== next.title ||
      previous.duration !== next.duration ||
      previous.deadline !== next.deadline
    ) {
      history.events.push({
        timestamp: now,
        date: after.last_reset,
        type: 'task_updated',
        task_id: next.id,
        title: next.title,
        to_status: next.status,
        duration: next.duration,
        deadline: next.deadline,
      });
    }
  }

  for (const [id, task] of afterMap) {
    if (beforeMap.has(id)) {
      continue;
    }

    history.events.push({
      timestamp: now,
      date: after.last_reset,
      type: 'task_added',
      task_id: task.id,
      title: task.title,
      to_status: task.status,
      duration: task.duration,
      deadline: task.deadline,
    });
  }
}
