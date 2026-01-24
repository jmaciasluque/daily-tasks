import type { Data, Task } from '../types';
import { THEMES } from '../theme/themes';

export function todayString(): string {
  return new Date().toISOString().slice(0, 10);
}

export function emptyData(): Data {
  return {
    last_reset: todayString(),
    next_id: 1,
    tasks: [],
    theme_index: 0,
    last_modified: Date.now(),
  };
}

export function normalizeData(input: Data): Data {
  const data: Data = {
    last_reset: input.last_reset || todayString(),
    next_id: input.next_id || 1,
    tasks: Array.isArray(input.tasks) ? input.tasks : [],
    theme_index: Number.isFinite(input.theme_index) ? input.theme_index : 0,
    last_modified: input.last_modified || Date.now(),
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
  data.tasks.forEach((task) => {
    if (task.order && task.order > 0) {
      if (task.status === 'done') {
        maxDone = Math.max(maxDone, task.order);
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
      } else {
        maxTodo += 1;
        task.order = maxTodo;
      }
    }
  });
}

export function orderedTasks(data: Data, status: 'todo' | 'done'): Task[] {
  return data.tasks
    .filter((task) => task.status === status)
    .sort((a, b) => (a.order === b.order ? a.id - b.id : a.order - b.order));
}

export function resetIfNeeded(data: Data): Data {
  const today = todayString();
  if (data.last_reset === today) {
    return data;
  }
  const merged = [...orderedTasks(data, 'todo'), ...orderedTasks(data, 'done')];
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

export function nextOrder(data: Data, status: 'todo' | 'done'): number {
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
