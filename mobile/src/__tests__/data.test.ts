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
import type { Data } from '../types';

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
    expect(data.last_modified).toBe(0); // 0 so remote data wins on first sync
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

  it('converts second timestamps to milliseconds', () => {
    const data = normalizeData({ last_modified: 1700000000 } as Data);
    expect(data.last_modified).toBe(1700000000000);
  });

  it('preserves deadline field on tasks', () => {
    const input: Data = {
      last_reset: '2026-01-01',
      next_id: 2,
      tasks: [{ id: 1, title: 'Morning run', duration: 30, status: 'todo', order: 1, deadline: '07:00' }],
      theme_index: 0,
    };
    const data = normalizeData(input);
    expect(data.tasks[0].deadline).toBe('07:00');
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

  it('assigns orders to skipped tasks independently from todo and done', () => {
    const data: Data = {
      last_reset: '2026-01-01',
      next_id: 4,
      tasks: [
        { id: 1, title: 'A', duration: 5, status: 'todo', order: 0 },
        { id: 2, title: 'B', duration: 5, status: 'skipped', order: 0 },
        { id: 3, title: 'C', duration: 5, status: 'skipped', order: 0 },
      ],
      theme_index: 0,
    };

    assignMissingOrders(data);

    expect(data.tasks[0].order).toBe(1); // todo: 1
    expect(data.tasks[1].order).toBe(1); // skipped: 1
    expect(data.tasks[2].order).toBe(2); // skipped: 2
  });

  it('preserves existing skipped orders and appends after them', () => {
    const data: Data = {
      last_reset: '2026-01-01',
      next_id: 3,
      tasks: [
        { id: 1, title: 'A', duration: 5, status: 'skipped', order: 3 },
        { id: 2, title: 'B', duration: 5, status: 'skipped', order: 0 },
      ],
      theme_index: 0,
    };

    assignMissingOrders(data);

    expect(data.tasks[0].order).toBe(3);
    expect(data.tasks[1].order).toBe(4);
  });
});

describe('orderedTasks', () => {
  const data: Data = {
    last_reset: '2026-01-01',
    next_id: 6,
    tasks: [
      { id: 1, title: 'A', duration: 5, status: 'todo', order: 3 },
      { id: 2, title: 'B', duration: 5, status: 'done', order: 1 },
      { id: 3, title: 'C', duration: 5, status: 'todo', order: 1 },
      { id: 4, title: 'D', duration: 5, status: 'todo', order: 2 },
      { id: 5, title: 'E', duration: 5, status: 'skipped', order: 1 },
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

  it('returns skipped tasks filtered correctly', () => {
    const skipped = orderedTasks(data, 'skipped');
    expect(skipped.length).toBe(1);
    expect(skipped[0].id).toBe(5);
    expect(skipped[0].status).toBe('skipped');
  });

  it('returns empty array when no tasks match the status', () => {
    const emptySkipped = orderedTasks(data, 'skipped');
    const dataNoSkipped: Data = { ...data, tasks: data.tasks.filter(t => t.status !== 'skipped') };
    expect(orderedTasks(dataNoSkipped, 'skipped')).toEqual([]);
  });

  it('sorts todo tasks by deadline ascending, no-deadline tasks last', () => {
    const d: Data = {
      last_reset: '2026-01-01',
      next_id: 5,
      tasks: [
        { id: 1, title: 'A', duration: 5, status: 'todo', order: 1 },
        { id: 2, title: 'B', duration: 5, status: 'todo', order: 2, deadline: '14:00' },
        { id: 3, title: 'C', duration: 5, status: 'todo', order: 3, deadline: '08:00' },
        { id: 4, title: 'D', duration: 5, status: 'todo', order: 4, deadline: '11:30' },
      ],
      theme_index: 0,
    };
    const todos = orderedTasks(d, 'todo');
    expect(todos[0].id).toBe(3); // deadline 08:00
    expect(todos[1].id).toBe(4); // deadline 11:30
    expect(todos[2].id).toBe(2); // deadline 14:00
    expect(todos[3].id).toBe(1); // no deadline, last
  });

  it('sorts multiple no-deadline todo tasks by order then id', () => {
    const d: Data = {
      last_reset: '2026-01-01',
      next_id: 4,
      tasks: [
        { id: 1, title: 'A', duration: 5, status: 'todo', order: 3 },
        { id: 2, title: 'B', duration: 5, status: 'todo', order: 1 },
        { id: 3, title: 'C', duration: 5, status: 'todo', order: 2 },
      ],
      theme_index: 0,
    };
    const todos = orderedTasks(d, 'todo');
    expect(todos[0].id).toBe(2); // order 1
    expect(todos[1].id).toBe(3); // order 2
    expect(todos[2].id).toBe(1); // order 3
  });

  it('does not apply deadline sorting to done or skipped tasks', () => {
    const d: Data = {
      last_reset: '2026-01-01',
      next_id: 4,
      tasks: [
        { id: 1, title: 'A', duration: 5, status: 'done', order: 2, deadline: '08:00' },
        { id: 2, title: 'B', duration: 5, status: 'done', order: 1, deadline: '20:00' },
      ],
      theme_index: 0,
    };
    const done = orderedTasks(d, 'done');
    expect(done[0].id).toBe(2); // order 1, even though deadline is later
    expect(done[1].id).toBe(1); // order 2
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

  it('resets todo and done tasks to todo on a new day', () => {
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

  it('resets skipped tasks back to todo on a new day', () => {
    const data: Data = {
      last_reset: '2020-01-01',
      next_id: 4,
      tasks: [
        { id: 1, title: 'A', duration: 5, status: 'todo', order: 1 },
        { id: 2, title: 'B', duration: 5, status: 'done', order: 1 },
        { id: 3, title: 'C', duration: 5, status: 'skipped', order: 1 },
      ],
      theme_index: 0,
    };

    const result = resetIfNeeded(data);
    expect(result.last_reset).toBe(todayString());
    result.tasks.forEach(task => {
      expect(task.status).toBe('todo');
    });
    expect(result.tasks).toHaveLength(3);
  });

  it('does not reset skipped tasks when already reset today', () => {
    const data: Data = {
      last_reset: todayString(),
      next_id: 2,
      tasks: [{ id: 1, title: 'A', duration: 5, status: 'skipped', order: 1 }],
      theme_index: 0,
    };

    const result = resetIfNeeded(data);
    expect(result.tasks[0].status).toBe('skipped');
  });

  it('preserves task deadline field after reset', () => {
    const data: Data = {
      last_reset: '2020-01-01',
      next_id: 2,
      tasks: [{ id: 1, title: 'Morning run', duration: 30, status: 'done', order: 1, deadline: '07:00' }],
      theme_index: 0,
    };

    const result = resetIfNeeded(data);
    expect(result.tasks[0].deadline).toBe('07:00');
    expect(result.tasks[0].status).toBe('todo');
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

  it('returns max skipped order + 1', () => {
    const data: Data = {
      last_reset: '2026-01-01',
      next_id: 4,
      tasks: [
        { id: 1, title: 'A', duration: 5, status: 'skipped', order: 2 },
        { id: 2, title: 'B', duration: 5, status: 'skipped', order: 4 },
        { id: 3, title: 'C', duration: 5, status: 'todo', order: 1 },
      ],
      theme_index: 0,
    };

    expect(nextOrder(data, 'skipped')).toBe(5);
  });

  it('returns 1 for skipped when no skipped tasks exist', () => {
    const data: Data = {
      last_reset: '2026-01-01',
      next_id: 2,
      tasks: [{ id: 1, title: 'A', duration: 5, status: 'todo', order: 1 }],
      theme_index: 0,
    };

    expect(nextOrder(data, 'skipped')).toBe(1);
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

  it('copies deadline field', () => {
    const original: Data = {
      last_reset: '2026-01-01',
      next_id: 2,
      tasks: [{ id: 1, title: 'A', duration: 5, status: 'todo', order: 1, deadline: '08:30' }],
      theme_index: 0,
    };

    const clone = cloneData(original);
    clone.tasks[0].deadline = '09:00';

    expect(original.tasks[0].deadline).toBe('08:30');
  });
});
