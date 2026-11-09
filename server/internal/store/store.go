// Package store persists users and their (already encrypted) task blobs.
//
// The Store interface is the only thing the service layer depends on, so the
// HTTP/RPC layers can be tested against the in-memory implementation and the
// Postgres implementation can be exercised separately against a real database.
package store

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when a user or blob does not exist.
var ErrNotFound = errors.New("store: not found")

// ErrPreconditionFailed is returned by PutBlob when the caller's expected
// version no longer matches what is stored.
var ErrPreconditionFailed = errors.New("store: precondition failed")

// User is an account, identified by the external identity provider subject.
type User struct {
	ID           string
	Provider     string
	Sub          string
	Email        string
	TokenVersion int
	CreatedAt    time.Time
}

// Blob is the encrypted data + history documents of one user.
type Blob struct {
	UserID    string
	Data      []byte
	History   []byte
	UpdatedAt time.Time
}

// Expect states the concurrency precondition for PutBlob.
type Expect struct {
	// Any skips the check (unconditional write).
	Any bool
	// Absent requires that no blob exists yet (create-only).
	Absent bool
	// UpdatedAt requires the stored blob's UpdatedAt to equal this value.
	UpdatedAt time.Time
}

// Store is the persistence contract.
type Store interface {
	// UpsertUser finds the user for (provider, sub), creating it if needed,
	// and refreshes the stored email.
	UpsertUser(ctx context.Context, provider, sub, email string) (*User, error)
	// GetUser returns ErrNotFound for unknown or deleted users.
	GetUser(ctx context.Context, id string) (*User, error)
	// BumpTokenVersion invalidates every token issued so far for the user.
	BumpTokenVersion(ctx context.Context, id string) (int, error)
	// DeleteUser removes the user and all of their data.
	DeleteUser(ctx context.Context, id string) error

	// GetBlob returns ErrNotFound when the user has never stored data.
	GetBlob(ctx context.Context, userID string) (*Blob, error)
	// PutBlob writes data and history atomically if expect holds, and returns
	// the new UpdatedAt. It returns ErrPreconditionFailed otherwise.
	PutBlob(ctx context.Context, userID string, data, history []byte, expect Expect) (time.Time, error)

	// Ping reports whether the backing database is reachable.
	Ping(ctx context.Context) error
}
