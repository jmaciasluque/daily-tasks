package store

import (
	"context"
	"errors"
	"os"
	"testing"
)

// Every Store implementation must pass the same contract.
func storeImplementations(t *testing.T) map[string]func(t *testing.T) Store {
	impls := map[string]func(t *testing.T) Store{
		"memory": func(t *testing.T) Store { return NewMemory() },
	}
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Log("TEST_DATABASE_URL not set; skipping Postgres contract tests")
		return impls
	}
	impls["postgres"] = func(t *testing.T) Store {
		ctx := context.Background()
		p, err := OpenPostgres(ctx, dsn)
		if err != nil {
			t.Fatalf("OpenPostgres: %v", err)
		}
		if _, err := p.pool.Exec(ctx, `TRUNCATE users CASCADE`); err != nil {
			t.Fatalf("truncate: %v", err)
		}
		t.Cleanup(p.Close)
		return p
	}
	return impls
}

func TestStoreContract(t *testing.T) {
	for name, open := range storeImplementations(t) {
		t.Run(name, func(t *testing.T) {
			t.Run("users", func(t *testing.T) { testUsers(t, open(t)) })
			t.Run("blobs", func(t *testing.T) { testBlobs(t, open(t)) })
			t.Run("delete cascades", func(t *testing.T) { testDelete(t, open(t)) })
		})
	}
}

func testUsers(t *testing.T, s Store) {
	ctx := context.Background()
	u1, err := s.UpsertUser(ctx, "google", "sub-1", "a@example.com")
	if err != nil {
		t.Fatalf("UpsertUser: %v", err)
	}
	u2, err := s.UpsertUser(ctx, "google", "sub-1", "b@example.com")
	if err != nil {
		t.Fatalf("UpsertUser again: %v", err)
	}
	if u1.ID != u2.ID || u2.Email != "b@example.com" {
		t.Fatalf("upsert should reuse the id and refresh email: %+v vs %+v", u1, u2)
	}
	other, _ := s.UpsertUser(ctx, "test", "sub-1", "")
	if other.ID == u1.ID {
		t.Fatal("different provider must yield a different user")
	}
	got, err := s.GetUser(ctx, u1.ID)
	if err != nil || got.TokenVersion != 0 {
		t.Fatalf("GetUser: %+v %v", got, err)
	}
	if v, err := s.BumpTokenVersion(ctx, u1.ID); err != nil || v != 1 {
		t.Fatalf("BumpTokenVersion = %d, %v", v, err)
	}
	if _, err := s.GetUser(ctx, "not-a-uuid"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetUser(bad id) err = %v", err)
	}
	if _, err := s.GetUser(ctx, "00000000-0000-4000-8000-000000000000"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetUser(unknown) err = %v", err)
	}
}

func testBlobs(t *testing.T, s Store) {
	ctx := context.Background()
	u, _ := s.UpsertUser(ctx, "google", "blob-user", "")
	if _, err := s.GetBlob(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetBlob before write: %v", err)
	}
	if _, err := s.PutBlob(ctx, u.ID, []byte("d"), []byte("h"), Expect{UpdatedAt: someTime}); !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("update of absent blob should fail precondition: %v", err)
	}
	t1, err := s.PutBlob(ctx, u.ID, []byte("d1"), []byte("h1"), Expect{Absent: true})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := s.PutBlob(ctx, u.ID, []byte("x"), []byte("x"), Expect{Absent: true}); !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("second create should fail: %v", err)
	}
	t2, err := s.PutBlob(ctx, u.ID, []byte("d2"), []byte("h2"), Expect{UpdatedAt: t1})
	if err != nil {
		t.Fatalf("conditional update: %v", err)
	}
	if !t2.After(t1) {
		t.Fatalf("updated_at must advance: %v -> %v", t1, t2)
	}
	if _, err := s.PutBlob(ctx, u.ID, []byte("x"), []byte("x"), Expect{UpdatedAt: t1}); !errors.Is(err, ErrPreconditionFailed) {
		t.Fatalf("stale update should fail: %v", err)
	}
	t3, err := s.PutBlob(ctx, u.ID, []byte("d3"), []byte("h3"), Expect{Any: true})
	if err != nil || !t3.After(t2) {
		t.Fatalf("unconditional update: %v %v", t3, err)
	}
	b, err := s.GetBlob(ctx, u.ID)
	if err != nil || string(b.Data) != "d3" || string(b.History) != "h3" || !b.UpdatedAt.Equal(t3) {
		t.Fatalf("GetBlob = %+v, %v", b, err)
	}
}

func testDelete(t *testing.T, s Store) {
	ctx := context.Background()
	u, _ := s.UpsertUser(ctx, "google", "doomed", "")
	if _, err := s.PutBlob(ctx, u.ID, []byte("d"), []byte("h"), Expect{Any: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteUser(ctx, u.ID); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if _, err := s.GetBlob(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("blob survived delete: %v", err)
	}
	if err := s.DeleteUser(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second delete: %v", err)
	}
	if _, err := s.PutBlob(ctx, u.ID, []byte("d"), []byte("h"), Expect{Any: true}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("PutBlob for deleted user: %v", err)
	}
	again, _ := s.UpsertUser(ctx, "google", "doomed", "")
	if again.ID == u.ID {
		t.Fatal("re-signup after deletion must create a fresh account")
	}
}

var someTime = mustTime("2024-01-01T00:00:00Z")
