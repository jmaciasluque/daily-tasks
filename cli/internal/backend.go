package internal

import "errors"

// Backend is the data-plane abstraction over a remote storage system.
// Implementations (WebDAV, eventually Google Drive, our own server, etc.)
// translate the two operations into protocol-specific calls.
//
// Backends deal in opaque bytes plus an ETag-like concurrency token.
// Encoding/decoding, merge logic, and sync orchestration live above this
// layer in sync.go and operate on any Backend.
type Backend interface {
	// Fetch returns the bytes stored under key along with the resource's
	// current concurrency token. ErrRemoteNotFound is returned when the
	// key does not exist on the remote.
	Fetch(key string) (Blob, error)

	// Push writes bytes to key. ifMatch carries a concurrency precondition:
	//   - "" pushes unconditionally (last-write-wins),
	//   - IfNoneMatchAny requires the resource not yet exist (create-only),
	//   - any other value requires the resource still match that token.
	// On precondition failure (412 / equivalent) the implementation must
	// return ErrEtagMismatch.
	Push(key string, body []byte, ifMatch string) error
}

// Blob is the fetched payload plus the resource's current concurrency
// token. The token is opaque — sync code only ever passes it back to
// the same backend's Push as ifMatch.
type Blob struct {
	Bytes []byte
	Etag  string
}

// Well-known keys the sync layer uses. Each backend translates these to
// its own naming scheme (WebDAV file path, Google Drive file id, etc.).
const (
	KeyData    = "data"
	KeyHistory = "history"
)

// ErrRemoteNotFound is returned by Backend.Fetch when the key does not
// exist on the remote.
var ErrRemoteNotFound = errors.New("remote file not found")

// ErrEtagMismatch is returned by Backend.Push when the precondition fails
// because the remote resource changed since the etag we passed in. Sync
// callers catch this to refetch and merge.
var ErrEtagMismatch = errors.New("remote etag changed since last fetch")

// IfNoneMatchAny is the value passed as ifMatch when the caller wants the
// Push to succeed only if the resource does not yet exist (i.e. mapped to
// If-None-Match: * by HTTP-based backends).
const IfNoneMatchAny = "*"
