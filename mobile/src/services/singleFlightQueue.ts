export class SingleFlightQueue<T> {
  private pending: T | null = null;
  private running: Promise<void> | null = null;

  enqueue(
    request: T,
    worker: (request: T) => Promise<void>,
    merge: (previous: T | null, next: T) => T = (_previous, next) => next,
  ): Promise<void> {
    this.pending = merge(this.pending, request);
    if (this.running) {
      return this.running;
    }

    this.running = this.drain(worker).finally(() => {
      this.running = null;
    });
    return this.running;
  }

  isRunning(): boolean {
    return this.running !== null;
  }

  private async drain(worker: (request: T) => Promise<void>): Promise<void> {
    while (this.pending) {
      const request = this.pending;
      this.pending = null;
      await worker(request);
    }
  }
}
