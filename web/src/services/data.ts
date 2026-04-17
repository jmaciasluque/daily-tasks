import type { Data, Task, TaskStatus } from '../types';
import { THEMES } from '../theme/themes';

function normalizeLastModified(ts?: number): number {
  if (!ts) return 0;
  return ts > 0 && ts < 100000000000 ? ts * 1000 : ts;
}

export function todayString(): string {
  return new Date().toISOString().slice(0, 10);
}

export function emptyData(): Data {
  return {
    last_reset: todayString(),
    next_id: 1,
    tasks: [],
    theme_index: 0,
    last_modified: 0,
  };
}

export function normalizeData(input: Data): Data {
  const data: Data = {
    last_reset: input.last_reset || todayString(),
    next_id: input.next_id || 1,
    tasks: Array.isArray(input.tasks) ? input.tasks : [],
    theme_index: Number.isFinite(input.theme_index) ? input.theme_index : 0,
    last_modified: normalizeLastModified(input.last_modified),
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

export function isVisibleToday(task: Task): boolean {
  if (!task.visibility || task.visibility.length === 0) return true;
  return task.visibility.includes(new Date().getDay());
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
  const merged = [
    ...allOrderedTasks(data, 'todo'),
    ...allOrderedTasks(data, 'done'),
    ...allOrderedTasks(data, 'skipped'),
  ];
  merged.forEach((task, idx) => {
    task.status = 'todo';
    task.order = idx + 1;
  });
  return {
    ...data,
    last_reset: today,
    tasks: merged,
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
