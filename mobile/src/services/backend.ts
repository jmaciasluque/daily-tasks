// Backend is the data-plane abstraction over a remote storage system.
// Implementations (WebDAV today, Google Drive / our own server later)
// translate the two operations into protocol-specific calls.
//
// Backends deal in opaque bytes plus an ETag-like concurrency token.
// JSON encoding/decoding, merge logic, and sync orchestration live above
// this layer in sync.ts and operate on any Backend.
export interface Backend {
  // Fetch returns the bytes stored under key along with the resource's
  // current concurrency token. The returned blob has `bytes: null` when
  // the key does not exist on the remote.
  fetch(key: string): Promise<Blob>;

  // Push writes bytes to key. ifMatch carries a concurrency precondition:
  //   - undefined / "" pushes unconditionally (last-write-wins),
  //   - IF_NONE_MATCH_ANY requires the resource not yet exist (create-only),
  //   - any other value requires the resource still match that token.
  // On precondition failure (412 / equivalent) the implementation must
  // throw EtagMismatchError.
  push(key: string, body: string, ifMatch?: string): Promise<void>;
}

export type Blob = {
  // Null when the resource does not exist on the remote.
  bytes: string | null;
  etag: string;
};

// Well-known keys the sync layer uses. Each backend translates these to
// its own naming scheme (WebDAV file path, Google Drive file id, etc.).
export const KeyData = 'data';
export const KeyHistory = 'history';

// IF_NONE_MATCH_ANY is the value passed as ifMatch when the caller wants
// the push to succeed only if the resource does not yet exist.
export const IF_NONE_MATCH_ANY = '*';

// EtagMismatchError is thrown by Backend.push when the precondition fails
// because the remote resource changed since the etag we passed in. Sync
// callers catch this to refetch and merge.
export class EtagMismatchError extends Error {
  constructor(message = 'Remote etag changed since last fetch') {
    super(message);
    this.name = 'EtagMismatchError';
  }
}
