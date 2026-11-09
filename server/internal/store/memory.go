package store

import (
	"context"
	"crypto/rand"
	"fmt"
	"sync"
	"time"
)

// Memory is an in-process Store used by tests and local development.
type Memory struct {
	mu     sync.Mutex
	users  map[string]*User
	byIdent map[string]string
	blobs  map[string]*Blob
	now    func() time.Time
}

// NewMemory returns an empty in-memory store.
func NewMemory() *Memory {
	return &Memory{
		users:   map[string]*User{},
		byIdent: map[string]string{},
		blobs:   map[string]*Blob{},
		now:     time.Now,
	}
}

func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func (m *Memory) UpsertUser(_ context.Context, provider, sub, email string) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := provider + "\x00" + sub
	if id, ok := m.byIdent[key]; ok {
		u := m.users[id]
		u.Email = email
		cp := *u
		return &cp, nil
	}
	u := &User{ID: newUUID(), Provider: provider, Sub: sub, Email: email, CreatedAt: m.now().UTC()}
	m.users[u.ID] = u
	m.byIdent[key] = u.ID
	cp := *u
	return &cp, nil
}

func (m *Memory) GetUser(_ context.Context, id string) (*User, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *u
	return &cp, nil
}

func (m *Memory) BumpTokenVersion(_ context.Context, id string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return 0, ErrNotFound
	}
	u.TokenVersion++
	return u.TokenVersion, nil
}

func (m *Memory) DeleteUser(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	u, ok := m.users[id]
	if !ok {
		return ErrNotFound
	}
	delete(m.byIdent, u.Provider+"\x00"+u.Sub)
	delete(m.users, id)
	delete(m.blobs, id)
	return nil
}

func (m *Memory) GetBlob(_ context.Context, userID string) (*Blob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.blobs[userID]
	if !ok {
		return nil, ErrNotFound
	}
	cp := *b
	return &cp, nil
}

func (m *Memory) PutBlob(_ context.Context, userID string, data, history []byte, expect Expect) (time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.users[userID]; !ok {
		return time.Time{}, ErrNotFound
	}
	current, exists := m.blobs[userID]
	if !expect.Any {
		if expect.Absent && exists {
			return time.Time{}, ErrPreconditionFailed
		}
		if !expect.Absent && (!exists || !current.UpdatedAt.Equal(expect.UpdatedAt)) {
			return time.Time{}, ErrPreconditionFailed
		}
	}
	// Postgres timestamps have microsecond precision; mirror that so etags
	// behave identically across implementations.
	updated := m.now().UTC().Truncate(time.Microsecond)
	if exists && !updated.After(current.UpdatedAt) {
		updated = current.UpdatedAt.Add(time.Microsecond)
	}
	m.blobs[userID] = &Blob{UserID: userID, Data: data, History: history, UpdatedAt: updated}
	return updated, nil
}

func (m *Memory) Ping(context.Context) error { return nil }
