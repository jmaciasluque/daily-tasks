import type { Backend } from './backend';
import type { Data, History } from '../types';
import { syncWithRemoteState, type SyncStateResult } from './sync';
import { SingleFlightQueue } from './singleFlightQueue';

export type RemoteStateSyncRequest = {
  backend: Backend;
  data: Data;
  history: History;
  applyResult: (result: SyncStateResult) => void | Promise<void>;
};

export type RemoteStateSyncWorker = (
  backend: Backend,
  data: Data,
  history: History,
) => Promise<SyncStateResult>;

export class RemoteStateSyncQueue {
  private readonly queue = new SingleFlightQueue<RemoteStateSyncRequest>();

  constructor(private readonly worker: RemoteStateSyncWorker = syncWithRemoteState) {}

  enqueue(request: RemoteStateSyncRequest): Promise<void> {
    return this.queue.enqueue(
      request,
      async (next) => {
        const result = await this.worker(next.backend, next.data, next.history);
        await next.applyResult(result);
      },
      (_previous, next) => next,
    );
  }

  isRunning(): boolean {
    return this.queue.isRunning();
  }
}

export const remoteStateSyncQueue = new RemoteStateSyncQueue();
