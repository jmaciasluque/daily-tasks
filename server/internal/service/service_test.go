package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"daily-tasks-server/internal/auth"
	"daily-tasks-server/internal/store"
)

func newTestService(t *testing.T) (*Service, *store.Memory) {
	t.Helper()
	tokens, err := auth.NewTokens(strings.Repeat("s", 40))
	if err != nil {
		t.Fatal(err)
	}
	st := store.NewMemory()
	svc, err := New(st, tokens, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	return svc, st
}

func login(t *testing.T, svc *Service, sub string) (*store.User, string) {
	t.Helper()
	u, tok, err := svc.Login(context.Background(), auth.Identity{Provider: "google", Sub: sub, Email: sub + "@example.com"}, auth.SessionTTL)
	if err != nil {
		t.Fatal(err)
	}
	return u, tok.Token
}

func TestAuthenticateChecksUserAndVersion(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	u, tok := login(t, svc, "alice")
	sess, err := svc.Authenticate(ctx, tok)
	if err != nil || sess.User.ID != u.ID {
		t.Fatalf("Authenticate = %+v, %v", sess, err)
	}
	if _, err := svc.Authenticate(ctx, "garbage"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("garbage token: %v", err)
	}
	if err := svc.RevokeAllSessions(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(ctx, tok); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("revoked token still valid: %v", err)
	}
	_, fresh := login(t, svc, "alice")
	if _, err := svc.Authenticate(ctx, fresh); err != nil {
		t.Fatalf("new login after revoke: %v", err)
	}
}

func TestRefreshExtendsSession(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	u, _, _ := svc.Login(ctx, auth.Identity{Provider: "google", Sub: "bob"}, time.Hour)
	tok, _, _ := svc.Tokens.Issue(u.ID, 0, time.Hour)
	sess, err := svc.Authenticate(ctx, tok)
	if err != nil {
		t.Fatal(err)
	}
	renewed, err := svc.Refresh(sess)
	if err != nil {
		t.Fatal(err)
	}
	if renewed.Expires.Before(time.Now().Add(auth.SessionTTL - time.Minute)) {
		t.Fatalf("refresh did not extend: %v", renewed.Expires)
	}
}

func TestDeleteAccountRevokesAndRemovesData(t *testing.T) {
	svc, st := newTestService(t)
	ctx := context.Background()
	u, tok := login(t, svc, "carol")
	if _, err := svc.PutState(ctx, u.ID, []byte(`{"tasks":[]}`), []byte(`{}`), Precondition{}); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteAccount(ctx, u.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.Authenticate(ctx, tok); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("token survived deletion: %v", err)
	}
	if _, err := st.GetBlob(ctx, u.ID); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("data survived deletion: %v", err)
	}
	if err := svc.DeleteAccount(ctx, u.ID); err != nil {
		t.Fatalf("deleting twice should be a no-op: %v", err)
	}
}

func TestStateRoundTripAndPreconditions(t *testing.T) {
	svc, st := newTestService(t)
	ctx := context.Background()
	u, _ := login(t, svc, "dave")

	empty, err := svc.GetState(ctx, u.ID)
	if err != nil || empty.Exists || empty.ETag != EmptyETag {
		t.Fatalf("initial state = %+v, %v", empty, err)
	}
	created, err := svc.PutState(ctx, u.ID, []byte(`{"tasks":[1]}`), []byte(`{"days":[]}`), Precondition{IfMatch: EmptyETag})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Stored ciphertext must not contain the plaintext.
	raw, _ := st.GetBlob(ctx, u.ID)
	if strings.Contains(string(raw.Data), "tasks") {
		t.Fatal("data stored in plaintext")
	}
	got, err := svc.GetState(ctx, u.ID)
	if err != nil || string(got.Data) != `{"tasks":[1]}` || got.ETag != created.ETag {
		t.Fatalf("GetState = %+v, %v", got, err)
	}
	if _, err := svc.PutState(ctx, u.ID, []byte(`{}`), []byte(`{}`), Precondition{CreateOnly: true}); !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("create-only over existing: %v", err)
	}
	updated, err := svc.PutState(ctx, u.ID, []byte(`{"tasks":[2]}`), []byte(`{}`), Precondition{IfMatch: created.ETag})
	if err != nil {
		t.Fatalf("conditional update: %v", err)
	}
	if updated.ETag == created.ETag {
		t.Fatal("etag must change")
	}
	for _, stale := range []string{created.ETag, EmptyETag, `"bogus"`, `"sync-abc"`} {
		if _, err := svc.PutState(ctx, u.ID, []byte(`{}`), []byte(`{}`), Precondition{IfMatch: stale}); !errors.Is(err, ErrPreconditionFailed) {
			t.Fatalf("stale etag %s accepted: %v", stale, err)
		}
	}
}

func TestPutStateValidatesDocuments(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()
	u, _ := login(t, svc, "erin")
	cases := map[string][2][]byte{
		"not json":        {[]byte("nope"), []byte(`{}`)},
		"json array":      {[]byte(`[]`), []byte(`{}`)},
		"history broken":  {[]byte(`{}`), []byte(`{`)},
		"data too big":    {[]byte(`{"x":"` + strings.Repeat("a", MaxDataBytes) + `"}`), []byte(`{}`)},
		"history too big": {[]byte(`{}`), []byte(`{"x":"` + strings.Repeat("a", MaxHistoryBytes) + `"}`)},
	}
	for name, docs := range cases {
		if _, err := svc.PutState(ctx, u.ID, docs[0], docs[1], Precondition{}); !errors.Is(err, ErrInvalidArgument) {
			t.Fatalf("%s: err = %v", name, err)
		}
	}
}

func TestETagFormatIsBackwardsCompatible(t *testing.T) {
	ts := time.Date(2026, 5, 28, 21, 0, 0, 123456000, time.UTC)
	etag := ETag(ts)
	if etag != `"sync-1780002000123456000"` {
		t.Fatalf("etag format changed: %s", etag)
	}
	parsed, ok := parseETag(etag)
	if !ok || !parsed.Equal(ts) {
		t.Fatalf("parseETag = %v, %v", parsed, ok)
	}
}
