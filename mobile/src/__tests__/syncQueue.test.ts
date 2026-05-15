import { RemoteStateSyncQueue } from '../services/syncQueue';
import type { Backend } from '../services/backend';
import type { Data, History } from '../types';

function deferred(): { promise: Promise<void>; resolve: () => void } {
  let resolve!: () => void;
  const promise = new Promise<void>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

async function flushPromises(): Promise<void> {
  await Promise.resolve();
  await Promise.resolve();
  await Promise.resolve();
}

const backend = {} as Backend;
const history: History = { version: 1, days: [], events: [] };

function data(id: number): Data {
  return {
    last_reset: '2026-05-15',
    next_id: id + 1,
    tasks: [{ id, title: `Task ${id}`, duration: 5, status: 'todo', order: 1 }],
    theme_index: 0,
    last_modified: 1700000000000 + id,
  };
}

describe('RemoteStateSyncQueue', () => {
  it('serializes sync calls and keeps only the newest pending payload', async () => {
    const blockers = [deferred(), deferred()];
    const syncedIds: number[] = [];
    const appliedIds: number[] = [];
    const worker = jest.fn(async (_backend: Backend, nextData: Data) => {
      const id = nextData.tasks[0].id;
      syncedIds.push(id);
      await blockers[syncedIds.length - 1].promise;
      return {
        data: nextData,
        history,
        action: 'pushed' as const,
        message: `Pushed ${id}`,
      };
    });
    const queue = new RemoteStateSyncQueue(worker);

    const first = queue.enqueue({
      backend,
      data: data(1),
      history,
      applyResult: (result) => {
        appliedIds.push(result.data.tasks[0].id);
      },
    });
    await Promise.resolve();

    const second = queue.enqueue({
      backend,
      data: data(2),
      history,
      applyResult: (result) => {
        appliedIds.push(result.data.tasks[0].id);
      },
    });
    const third = queue.enqueue({
      backend,
      data: data(3),
      history,
      applyResult: (result) => {
        appliedIds.push(result.data.tasks[0].id);
      },
    });

    expect(second).toBe(first);
    expect(third).toBe(first);
    expect(worker).toHaveBeenCalledTimes(1);
    expect(syncedIds).toEqual([1]);

    blockers[0].resolve();
    await flushPromises();

    expect(worker).toHaveBeenCalledTimes(2);
    expect(syncedIds).toEqual([1, 3]);
    expect(appliedIds).toEqual([1]);

    blockers[1].resolve();
    await first;

    expect(appliedIds).toEqual([1, 3]);
  });
});
