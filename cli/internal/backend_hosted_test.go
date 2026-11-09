package internal

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"

	v1 "daily-tasks/internal/gen/dailytasks/v1"
	"daily-tasks/internal/gen/dailytasks/v1/dailytasksv1connect"
)

// fakeSyncService is an in-memory dailytasks.v1 server with the same
// precondition semantics as the real one.
type fakeSyncService struct {
	dailytasksv1connect.UnimplementedSyncServiceHandler
	dailytasksv1connect.UnimplementedAccountServiceHandler

	mu        sync.Mutex
	token     string
	exists    bool
	data      []byte
	history   []byte
	version   int
	lastAuth  string
	lastAgent string
	puts      []*v1.PutStateRequest
	refreshed bool
}

func (f *fakeSyncService) etag() string {
	if !f.exists {
		return `"empty"`
	}
	return fmt.Sprintf(`"sync-%d"`, f.version)
}

func (f *fakeSyncService) check(h http.Header) error {
	f.lastAuth = h.Get("Authorization")
	f.lastAgent = h.Get("User-Agent")
	if f.lastAuth != "Bearer "+f.token {
		return connect.NewError(connect.CodeUnauthenticated, errors.New("bad token"))
	}
	return nil
}

func (f *fakeSyncService) GetState(_ context.Context, req *connect.Request[v1.GetStateRequest]) (*connect.Response[v1.GetStateResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.check(req.Header()); err != nil {
		return nil, err
	}
	return connect.NewResponse(&v1.GetStateResponse{Exists: f.exists, Data: f.data, History: f.history, Etag: f.etag()}), nil
}

func (f *fakeSyncService) PutState(_ context.Context, req *connect.Request[v1.PutStateRequest]) (*connect.Response[v1.PutStateResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.check(req.Header()); err != nil {
		return nil, err
	}
	switch p := req.Msg.GetPrecondition().(type) {
	case *v1.PutStateRequest_CreateOnly:
		if f.exists {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("exists"))
		}
	case *v1.PutStateRequest_IfMatch:
		if p.IfMatch != f.etag() {
			return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("stale"))
		}
	}
	f.puts = append(f.puts, req.Msg)
	f.exists, f.data, f.history = true, req.Msg.GetData(), req.Msg.GetHistory()
	f.version++
	return connect.NewResponse(&v1.PutStateResponse{Etag: f.etag()}), nil
}

func (f *fakeSyncService) RefreshSession(_ context.Context, req *connect.Request[v1.RefreshSessionRequest]) (*connect.Response[v1.RefreshSessionResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.check(req.Header()); err != nil {
		return nil, err
	}
	f.refreshed = true
	f.token = "fresh-token"
	return connect.NewResponse(&v1.RefreshSessionResponse{Token: f.token}), nil
}

// startFake serves the fake over h2c so the client really speaks gRPC.
func startFake(t *testing.T, f *fakeSyncService) *HostedBackend {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle(dailytasksv1connect.NewSyncServiceHandler(f))
	mux.Handle(dailytasksv1connect.NewAccountServiceHandler(f))
	srv := httptest.NewUnstartedServer(mux)
	protocols := new(http.Protocols)
	protocols.SetUnencryptedHTTP2(true)
	srv.Config.Protocols = protocols
	srv.Start()
	t.Cleanup(srv.Close)
	return NewHostedBackend(srv.URL, f.token)
}

func TestHostedBackendFetchReportsMissingStateAsNotFound(t *testing.T) {
	f := &fakeSyncService{token: "tok"}
	b := startFake(t, f)
	if _, err := b.Fetch(KeyData); !errors.Is(err, ErrRemoteNotFound) {
		t.Fatalf("Fetch on empty account: %v", err)
	}
	if f.lastAuth != "Bearer tok" || f.lastAgent == "" {
		t.Fatalf("headers: auth=%q agent=%q", f.lastAuth, f.lastAgent)
	}
}

func TestHostedBackendCreateThenUpdateKeepsOtherDocument(t *testing.T) {
	f := &fakeSyncService{token: "tok"}
	b := startFake(t, f)

	if err := b.Push(KeyData, []byte(`{"tasks":[]}`), IfNoneMatchAny); err != nil {
		t.Fatalf("create data: %v", err)
	}
	if string(f.history) != "{}" {
		t.Fatalf("missing history should default to {}, got %q", f.history)
	}
	if err := b.Push(KeyData, []byte(`{}`), IfNoneMatchAny); !errors.Is(err, ErrEtagMismatch) {
		t.Fatalf("second create-only: %v", err)
	}

	blob, err := b.Fetch(KeyHistory)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Push(KeyHistory, []byte(`{"version":1}`), blob.Etag); err != nil {
		t.Fatalf("conditional history push: %v", err)
	}
	if string(f.data) != `{"tasks":[]}` || string(f.history) != `{"version":1}` {
		t.Fatalf("documents after update: %q %q", f.data, f.history)
	}
	if err := b.Push(KeyData, []byte(`{}`), blob.Etag); !errors.Is(err, ErrEtagMismatch) {
		t.Fatalf("stale etag push: %v", err)
	}
	data, err := b.Fetch(KeyData)
	if err != nil || string(data.Bytes) != `{"tasks":[]}` || data.Etag != `"sync-2"` {
		t.Fatalf("Fetch data: %+v %v", data, err)
	}
	if err := b.Push(KeyData, []byte(`{"tasks":[1]}`), ""); err != nil || f.puts[len(f.puts)-1].Precondition != nil {
		t.Fatalf("unconditional push: %v", err)
	}
}

func TestHostedBackendUnauthorizedAsksUserToLoginAgain(t *testing.T) {
	f := &fakeSyncService{token: "right"}
	b := startFake(t, f)
	b.Token = "wrong"
	if _, err := b.Fetch(KeyData); !errors.Is(err, ErrHostedTokenInvalid) {
		t.Fatalf("Fetch: %v", err)
	}
	if err := b.Push(KeyData, []byte(`{}`), ""); !errors.Is(err, ErrHostedTokenInvalid) {
		t.Fatalf("Push: %v", err)
	}
}

func jwtWithIssuedAt(iat time.Time) string {
	payload := base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf(`{"uid":"u","iat":%d}`, iat.Unix())))
	return "eyJhbGciOiJIUzI1NiJ9." + payload + ".sig"
}

func TestRefreshHostedTokenIfStale(t *testing.T) {
	now := time.Now()
	tokenPath := filepath.Join(t.TempDir(), "token")

	fresh := jwtWithIssuedAt(now.Add(-time.Hour))
	f := &fakeSyncService{token: fresh}
	b := startFake(t, f)
	RefreshHostedTokenIfStale(b, tokenPath, now)
	if f.refreshed {
		t.Fatal("fresh token should not be refreshed")
	}

	old := jwtWithIssuedAt(now.Add(-8 * 24 * time.Hour))
	f = &fakeSyncService{token: old}
	b = startFake(t, f)
	RefreshHostedTokenIfStale(b, tokenPath, now)
	if !f.refreshed || b.Token != "fresh-token" {
		t.Fatalf("stale token not refreshed: %v %q", f.refreshed, b.Token)
	}
	saved, err := LoadHostedToken(tokenPath)
	if err != nil || saved != "fresh-token" {
		t.Fatalf("saved token = %q, %v", saved, err)
	}
}
