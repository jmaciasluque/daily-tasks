import {
  todayString,
  emptyData,
  normalizeData,
  assignMissingOrders,
  orderedTasks,
  resetIfNeeded,
  nextOrder,
  cloneData,
} from '../services/data';
import type { Data, Task } from '../types';

describe('todayString', () => {
  it('returns date in YYYY-MM-DD format', () => {
    const result = todayString();
    expect(result).toMatch(/^\d{4}-\d{2}-\d{2}$/);
  });
});

describe('emptyData', () => {
  it('returns data with today as last_reset', () => {
    const data = emptyData();
    expect(data.last_reset).toBe(todayString());
    expect(data.next_id).toBe(1);
    expect(data.tasks).toEqual([]);
    expect(data.theme_index).toBe(0);
    expect(data.last_modified).toBeGreaterThan(0);
  });
});

describe('normalizeData', () => {
  it('sets defaults for empty data', () => {
    const data = normalizeData({} as Data);
    expect(data.last_reset).toBeDefined();
    expect(data.next_id).toBe(1);
    expect(data.tasks).toEqual([]);
    expect(data.theme_index).toBe(0);
  });

  it('clamps invalid theme index', () => {
    const data = normalizeData({ theme_index: 9999 } as Data);
    expect(data.theme_index).toBe(0);

    const data2 = normalizeData({ theme_index: -5 } as Data);
    expect(data2.theme_index).toBe(0);
  });

  it('preserves valid data', () => {
    const input: Data = {
      last_reset: '2026-01-01',
      next_id: 10,
      tasks: [{ id: 1, title: 'Test', duration: 5, status: 'todo', order: 1 }],
      theme_index: 5,
      last_modified: 12345,
    };
    const data = normalizeData(input);
    expect(data.last_reset).toBe('2026-01-01');
    expect(data.next_id).toBe(10);
    expect(data.theme_index).toBe(5);
  });
});

describe('assignMissingOrders', () => {
  it('assigns orders to tasks without them', () => {
    const data: Data = {
      last_reset: '2026-01-01',
      next_id: 4,
      tasks: [
        { id: 1, title: 'A', duration: 5, status: 'todo', order: 0 },
        { id: 2, title: 'B', duration: 5, status: 'todo', order: 0 },
        { id: 3, title: 'C', duration: 5, status: 'done', order: 0 },
      ],
      theme_index: 0,
    };

    assignMissingOrders(data);

    data.tasks.forEach(task => {
      expect(task.order).toBeGreaterThan(0);
    });
  });

  it('preserves existing orders', () => {
    const data: Data = {
      last_reset: '2026-01-01',
      next_id: 3,
      tasks: [
        { id: 1, title: 'A', duration: 5, status: 'todo', order: 5 },
        { id: 2, title: 'B', duration: 5, status: 'todo', order: 0 },
      ],
      theme_index: 0,
    };

    assignMissingOrders(data);

    expect(data.tasks[0].order).toBe(5);
    expect(data.tasks[1].order).toBe(6);
  });
});

describe('orderedTasks', () => {
  const data: Data = {
    last_reset: '2026-01-01',
    next_id: 5,
    tasks: [
      { id: 1, title: 'A', duration: 5, status: 'todo', order: 3 },
      { id: 2, title: 'B', duration: 5, status: 'done', order: 1 },
      { id: 3, title: 'C', duration: 5, status: 'todo', order: 1 },
      { id: 4, title: 'D', duration: 5, status: 'todo', order: 2 },
    ],
    theme_index: 0,
  };

  it('filters by status', () => {
    const todos = orderedTasks(data, 'todo');
    expect(todos.length).toBe(3);
    expect(todos.every(t => t.status === 'todo')).toBe(true);

    const done = orderedTasks(data, 'done');
    expect(done.length).toBe(1);
    expect(done[0].status).toBe('done');
  });

  it('sorts by order then id', () => {
    const todos = orderedTasks(data, 'todo');
    expect(todos[0].id).toBe(3); // order 1
    expect(todos[1].id).toBe(4); // order 2
    expect(todos[2].id).toBe(1); // order 3
  });
});

describe('resetIfNeeded', () => {
  it('does not reset on same day', () => {
    const data: Data = {
      last_reset: todayString(),
      next_id: 2,
      tasks: [{ id: 1, title: 'A', duration: 5, status: 'done', order: 1 }],
      theme_index: 0,
    };

    const result = resetIfNeeded(data);
    expect(result.tasks[0].status).toBe('done');
  });

  it('resets all tasks to todo on new day', () => {
    const data: Data = {
      last_reset: '2020-01-01',
      next_id: 3,
      tasks: [
        { id: 1, title: 'A', duration: 5, status: 'done', order: 1 },
        { id: 2, title: 'B', duration: 5, status: 'todo', order: 1 },
      ],
      theme_index: 0,
    };

    const result = resetIfNeeded(data);
    expect(result.last_reset).toBe(todayString());
    result.tasks.forEach(task => {
      expect(task.status).toBe('todo');
    });
  });
});

describe('nextOrder', () => {
  it('returns max order + 1 for status', () => {
    const data: Data = {
      last_reset: '2026-01-01',
      next_id: 4,
      tasks: [
        { id: 1, title: 'A', duration: 5, status: 'todo', order: 3 },
        { id: 2, title: 'B', duration: 5, status: 'todo', order: 5 },
        { id: 3, title: 'C', duration: 5, status: 'done', order: 2 },
      ],
      theme_index: 0,
    };

    expect(nextOrder(data, 'todo')).toBe(6);
    expect(nextOrder(data, 'done')).toBe(3);
  });

  it('returns 1 for empty list', () => {
    const data = emptyData();
    expect(nextOrder(data, 'todo')).toBe(1);
    expect(nextOrder(data, 'done')).toBe(1);
  });
});

describe('cloneData', () => {
  it('creates a deep copy', () => {
    const original: Data = {
      last_reset: '2026-01-01',
      next_id: 5,
      tasks: [{ id: 1, title: 'Test', duration: 5, status: 'todo', order: 1 }],
      theme_index: 2,
    };

    const clone = cloneData(original);

    // Modify clone
    clone.tasks[0].title = 'Modified';
    clone.next_id = 10;

    // Original should be unchanged
    expect(original.tasks[0].title).toBe('Test');
    expect(original.next_id).toBe(5);
  });
});
