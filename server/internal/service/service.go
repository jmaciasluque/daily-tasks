// Package service holds the transport-independent business logic shared by
// the REST v1 API and the Connect/gRPC API.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"daily-tasks-server/internal/auth"
	"daily-tasks-server/internal/crypto"
	"daily-tasks-server/internal/store"
)

// Size caps for the two JSON documents. They keep the backend from becoming
// free file hosting while leaving real users plenty of headroom.
const (
	// MaxDataBytes caps the task list; a busy user's is a few KB.
	MaxDataBytes = 512 << 10
	// MaxHistoryBytes caps the stats history, which grows with every day of
	// use: roughly 1.5-2.5 MB per year for a user with ~25 daily tasks.
	MaxHistoryBytes = 4 << 20
)

// EmptyETag is the concurrency token of a user who has never stored data.
const EmptyETag = `"empty"`

var (
	ErrUnauthenticated    = errors.New("unauthenticated")
	ErrPreconditionFailed = errors.New("state changed since it was read")
	ErrInvalidArgument    = errors.New("invalid argument")
)

// Service is the application core.
type Service struct {
	Store     store.Store
	Tokens    *auth.Tokens
	MasterKey []byte
	Now       func() time.Time
}

// New wires a service; masterKey must be 32 bytes.
func New(st store.Store, tokens *auth.Tokens, masterKey []byte) (*Service, error) {
	if len(masterKey) != 32 {
		return nil, errors.New("MASTER_KEY must be 32 bytes")
	}
	return &Service{Store: st, Tokens: tokens, MasterKey: masterKey, Now: time.Now}, nil
}

// Session is an authenticated caller.
type Session struct {
	User     *store.User
	IssuedAt time.Time
	Expires  time.Time
}

// Authenticate validates a bearer token against the current user record, so
// deleted users and revoked tokens are rejected immediately.
func (s *Service) Authenticate(ctx context.Context, token string) (*Session, error) {
	claims, err := s.Tokens.Parse(token)
	if err != nil {
		return nil, ErrUnauthenticated
	}
	user, err := s.Store.GetUser(ctx, claims.UserID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrUnauthenticated
	}
	if err != nil {
		return nil, err
	}
	if user.TokenVersion != claims.Version {
		return nil, ErrUnauthenticated
	}
	sess := &Session{User: user}
	if claims.IssuedAt != nil {
		sess.IssuedAt = claims.IssuedAt.Time
	}
	if claims.ExpiresAt != nil {
		sess.Expires = claims.ExpiresAt.Time
	}
	return sess, nil
}

// IssuedToken is a freshly signed session token.
type IssuedToken struct {
	Token   string
	Expires time.Time
}

// Login records the identity and returns a session token.
func (s *Service) Login(ctx context.Context, id auth.Identity, ttl time.Duration) (*store.User, *IssuedToken, error) {
	user, err := s.Store.UpsertUser(ctx, id.Provider, id.Sub, id.Email)
	if err != nil {
		return nil, nil, err
	}
	tok, err := s.issue(user, ttl)
	return user, tok, err
}

// Refresh issues a new full-length token for an authenticated session.
func (s *Service) Refresh(sess *Session) (*IssuedToken, error) {
	return s.issue(sess.User, auth.SessionTTL)
}

func (s *Service) issue(user *store.User, ttl time.Duration) (*IssuedToken, error) {
	token, exp, err := s.Tokens.Issue(user.ID, user.TokenVersion, ttl)
	if err != nil {
		return nil, err
	}
	return &IssuedToken{Token: token, Expires: exp}, nil
}

// RevokeAllSessions invalidates every token of the user, including the
// caller's.
func (s *Service) RevokeAllSessions(ctx context.Context, userID string) error {
	_, err := s.Store.BumpTokenVersion(ctx, userID)
	return err
}

// DeleteAccount permanently removes the user and their data.
func (s *Service) DeleteAccount(ctx context.Context, userID string) error {
	err := s.Store.DeleteUser(ctx, userID)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	return err
}

// State is a user's decrypted documents.
type State struct {
	Exists    bool
	Data      []byte
	History   []byte
	ETag      string
	UpdatedAt time.Time
}

// GetState returns the user's documents, decrypted.
func (s *Service) GetState(ctx context.Context, userID string) (*State, error) {
	blob, err := s.Store.GetBlob(ctx, userID)
	if errors.Is(err, store.ErrNotFound) {
		return &State{ETag: EmptyETag}, nil
	}
	if err != nil {
		return nil, err
	}
	key, err := crypto.DeriveKey(s.MasterKey, userID)
	if err != nil {
		return nil, err
	}
	data, err := crypto.Decrypt(key, blob.Data)
	if err != nil {
		return nil, fmt.Errorf("decrypt data: %w", err)
	}
	history, err := crypto.Decrypt(key, blob.History)
	if err != nil {
		return nil, fmt.Errorf("decrypt history: %w", err)
	}
	return &State{Exists: true, Data: data, History: history, ETag: ETag(blob.UpdatedAt), UpdatedAt: blob.UpdatedAt}, nil
}

// Precondition guards PutState against lost updates.
type Precondition struct {
	// IfMatch is an etag from GetState; empty means unconditional.
	IfMatch string
	// CreateOnly requires that no state exists yet.
	CreateOnly bool
}

// PutState validates, encrypts and stores both documents atomically.
func (s *Service) PutState(ctx context.Context, userID string, data, history []byte, pre Precondition) (*State, error) {
	if err := validateDocument("data", data, MaxDataBytes); err != nil {
		return nil, err
	}
	if err := validateDocument("history", history, MaxHistoryBytes); err != nil {
		return nil, err
	}
	expect, err := expectation(pre)
	if err != nil {
		return nil, err
	}
	key, err := crypto.DeriveKey(s.MasterKey, userID)
	if err != nil {
		return nil, err
	}
	dataCipher, err := crypto.Encrypt(key, data)
	if err != nil {
		return nil, err
	}
	histCipher, err := crypto.Encrypt(key, history)
	if err != nil {
		return nil, err
	}
	updated, err := s.Store.PutBlob(ctx, userID, dataCipher, histCipher, expect)
	switch {
	case errors.Is(err, store.ErrPreconditionFailed):
		return nil, ErrPreconditionFailed
	case errors.Is(err, store.ErrNotFound):
		return nil, ErrUnauthenticated
	case err != nil:
		return nil, err
	}
	return &State{Exists: true, Data: data, History: history, ETag: ETag(updated), UpdatedAt: updated}, nil
}

func validateDocument(name string, doc []byte, limit int) error {
	if len(doc) > limit {
		return fmt.Errorf("%w: %s exceeds %d bytes", ErrInvalidArgument, name, limit)
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(doc, &probe); err != nil {
		return fmt.Errorf("%w: %s must be a JSON object", ErrInvalidArgument, name)
	}
	return nil
}

func expectation(pre Precondition) (store.Expect, error) {
	switch {
	case pre.CreateOnly:
		return store.Expect{Absent: true}, nil
	case pre.IfMatch == "":
		return store.Expect{Any: true}, nil
	case pre.IfMatch == EmptyETag:
		return store.Expect{Absent: true}, nil
	}
	t, ok := parseETag(pre.IfMatch)
	if !ok {
		// An etag we never issued can't match anything.
		return store.Expect{}, ErrPreconditionFailed
	}
	return store.Expect{UpdatedAt: t}, nil
}

// ETag formats the concurrency token for a stored state. The format matches
// what REST v1 clients already hold.
func ETag(updatedAt time.Time) string {
	return fmt.Sprintf(`"sync-%d"`, updatedAt.UTC().UnixNano())
}

func parseETag(etag string) (time.Time, bool) {
	inner := strings.TrimSuffix(strings.TrimPrefix(etag, `"sync-`), `"`)
	if inner == etag {
		return time.Time{}, false
	}
	n, err := strconv.ParseInt(inner, 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(0, n).UTC(), true
}
