import { SingleFlightQueue } from '../services/singleFlightQueue';

function deferred(): { promise: Promise<void>; resolve: () => void } {
  let resolve!: () => void;
  const promise = new Promise<void>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

describe('SingleFlightQueue', () => {
  it('runs one request at a time and coalesces pending work to the latest request', async () => {
    const queue = new SingleFlightQueue<{ value: number }>();
    const blockers = [deferred(), deferred()];
    const seen: number[] = [];
    const worker = jest.fn(async (request: { value: number }) => {
      seen.push(request.value);
      await blockers[seen.length - 1].promise;
    });

    const first = queue.enqueue({ value: 1 }, worker);
    await Promise.resolve();

    const second = queue.enqueue({ value: 2 }, worker);
    const third = queue.enqueue({ value: 3 }, worker);

    expect(second).toBe(first);
    expect(third).toBe(first);
    expect(worker).toHaveBeenCalledTimes(1);
    expect(seen).toEqual([1]);

    blockers[0].resolve();
    await Promise.resolve();
    await Promise.resolve();

    expect(worker).toHaveBeenCalledTimes(2);
    expect(seen).toEqual([1, 3]);

    blockers[1].resolve();
    await first;
  });

  it('lets callers merge metadata on a coalesced follow-up request', async () => {
    const queue = new SingleFlightQueue<{ value: number; showSyncing: boolean }>();
    const blockers = [deferred(), deferred()];
    const seen: Array<{ value: number; showSyncing: boolean }> = [];
    const worker = jest.fn(async (request: { value: number; showSyncing: boolean }) => {
      seen.push(request);
      await blockers[seen.length - 1].promise;
    });
    const merge = (
      previous: { value: number; showSyncing: boolean } | null,
      next: { value: number; showSyncing: boolean },
    ) => ({
      value: next.value,
      showSyncing: (previous?.showSyncing ?? false) || next.showSyncing,
    });

    const running = queue.enqueue({ value: 1, showSyncing: false }, worker, merge);
    await Promise.resolve();

    queue.enqueue({ value: 2, showSyncing: false }, worker, merge);
    queue.enqueue({ value: 3, showSyncing: true }, worker, merge);

    blockers[0].resolve();
    await Promise.resolve();
    await Promise.resolve();

    expect(seen).toEqual([
      { value: 1, showSyncing: false },
      { value: 3, showSyncing: true },
    ]);

    blockers[1].resolve();
    await running;
  });
});
