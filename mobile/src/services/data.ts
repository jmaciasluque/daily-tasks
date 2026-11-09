import type { Data, Task, TaskStatus } from '../types';
import { THEMES } from '../theme/themes';

// SCHEMA_VERSION is the current data file schema version.
// Mirrors SchemaVersion in cli/internal/data.go — keep them in sync.
//
// Version history:
//   0 (implicit): original format — no version field.
//   1: explicit version stamp. No structural changes from v0.
const SCHEMA_VERSION = 1;

function normalizeLastModified(ts?: number): number {
  if (!ts) return 0;
  return ts > 0 && ts < 100000000000 ? ts * 1000 : ts;
}

export function todayString(): string {
  return new Date().toISOString().slice(0, 10);
}

export function emptyData(): Data {
  return {
    version: SCHEMA_VERSION,
    last_reset: todayString(),
    next_id: 1,
    tasks: [],
    theme_index: 0,
    last_modified: 0, // Start with 0 so remote data always wins on first sync
  };
}

// migrateData upgrades data from older schema versions to SCHEMA_VERSION.
// Mirrors migrateData in cli/internal/data.go — keep them in sync.
function migrateData(data: Data): Data {
  const version = data.version ?? 0;
  if (version < 1) {
    // v0 → v1: no structural changes; stamp the version field.
    return { ...data, version: 1 };
  }
  // Future: add version < 2 steps here.
  return data;
}

export function normalizeData(input: Data): Data {
  const migrated = migrateData(input);
  const data: Data = {
    version: SCHEMA_VERSION,
    last_reset: migrated.last_reset || todayString(),
    next_id: migrated.next_id || 1,
    tasks: Array.isArray(migrated.tasks) ? migrated.tasks : [],
    theme_index: Number.isFinite(migrated.theme_index) ? migrated.theme_index : 0,
    last_modified: normalizeLastModified(migrated.last_modified), // Normalize old second-based timestamps
  };
  if (data.theme_index < 0 || data.theme_index >= THEMES.length) {
    data.theme_index = 0;
  }
  assignMissingOrders(data);
  return data;
}

export function assignMissingOrders(data: Data): void {
  let maxTodo = 0;
  let maxDone = 0;
  let maxSkipped = 0;
  data.tasks.forEach((task) => {
    if (task.order && task.order > 0) {
      if (task.status === 'done') {
        maxDone = Math.max(maxDone, task.order);
      } else if (task.status === 'skipped') {
        maxSkipped = Math.max(maxSkipped, task.order);
      } else {
        maxTodo = Math.max(maxTodo, task.order);
      }
    }
  });
  data.tasks.forEach((task) => {
    if (!task.order || task.order <= 0) {
      if (task.status === 'done') {
        maxDone += 1;
        task.order = maxDone;
      } else if (task.status === 'skipped') {
        maxSkipped += 1;
        task.order = maxSkipped;
      } else {
        maxTodo += 1;
        task.order = maxTodo;
      }
    }
  });
}

export function isAM(deadline: string): boolean {
  if (deadline.length < 5) return false;
  const h = parseInt(deadline.slice(0, 2), 10);
  return !isNaN(h) && h < 12;
}

export function isVisibleToday(task: Task): boolean {
  if (!task.visibility || task.visibility.length === 0) return true;
  return task.visibility.includes(new Date().getDay());
}

export function orderedAllTasks(data: Data): Task[] {
  const statusRank: Record<TaskStatus, number> = { todo: 0, done: 1, skipped: 2 };
  return [...data.tasks].sort((a, b) => {
    if (a.status !== b.status) return statusRank[a.status] - statusRank[b.status];
    if (a.status === 'todo') {
      if (a.deadline && b.deadline) return a.deadline.localeCompare(b.deadline);
      if (a.deadline) return -1;
      if (b.deadline) return 1;
    }
    return a.order === b.order ? a.id - b.id : a.order - b.order;
  });
}

export function orderedTasks(data: Data, status: TaskStatus): Task[] {
  return data.tasks
    .filter((task) => task.status === status && isVisibleToday(task))
    .sort((a, b) => {
      if (status === 'todo') {
        if (a.deadline && b.deadline) return a.deadline.localeCompare(b.deadline);
        if (a.deadline) return -1;
        if (b.deadline) return 1;
      }
      return a.order === b.order ? a.id - b.id : a.order - b.order;
    });
}

function allOrderedTasks(data: Data, status: TaskStatus): Task[] {
  return data.tasks
    .filter((task) => task.status === status)
    .sort((a, b) => {
      if (status === 'todo') {
        if (a.deadline && b.deadline) return a.deadline.localeCompare(b.deadline);
        if (a.deadline) return -1;
        if (b.deadline) return 1;
      }
      return a.order === b.order ? a.id - b.id : a.order - b.order;
    });
}

export function resetIfNeeded(data: Data): Data {
  const today = todayString();
  if (data.last_reset === today) {
    return data;
  }
  const todayWeekday = new Date().getDay();
  const resettable = [
    ...allOrderedTasks(data, 'todo'),
    ...allOrderedTasks(data, 'done'),
    ...allOrderedTasks(data, 'skipped'),
  ].filter((task) =>
    !task.visibility || task.visibility.length === 0 || task.visibility.includes(todayWeekday)
  );
  resettable.forEach((task, idx) => {
    task.status = 'todo';
    task.order = idx + 1;
  });
  return {
    ...data,
    last_reset: today,
    tasks: data.tasks,
    last_modified: Date.now(),
  };
}

export function nextOrder(data: Data, status: TaskStatus): number {
  let max = 0;
  data.tasks.forEach((task) => {
    if (task.status === status) {
      max = Math.max(max, task.order || 0);
    }
  });
  return max + 1;
}

export function cloneData(data: Data): Data {
  return {
    ...data,
    tasks: data.tasks.map(t => ({ ...t })),
  };
}
