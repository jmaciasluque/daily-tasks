// Package testutil builds a fully wired server on an in-memory store for
// handler-level tests.
package testutil

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"daily-tasks-server/internal/auth"
	"daily-tasks-server/internal/httpapi"
	"daily-tasks-server/internal/rpc"
	"daily-tasks-server/internal/service"
	"daily-tasks-server/internal/store"
)

// Secret is the JWT secret used by test servers.
var Secret = strings.Repeat("t", 40)

// TestLoginSecret enables /auth/test on test servers.
const TestLoginSecret = "let-me-in"

// Env is a running test server.
type Env struct {
	Server  *httptest.Server
	Service *service.Service
	Store   *store.Memory
	API     *httpapi.Server
}

// New starts an HTTP/1.1 + h2c test server with every route mounted.
func New(t *testing.T) *Env {
	t.Helper()
	st := store.NewMemory()
	tokens, err := auth.NewTokens(Secret)
	if err != nil {
		t.Fatal(err)
	}
	svc, err := service.New(st, tokens, make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	srv := httptest.NewUnstartedServer(nil)
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	srv.Config.Protocols = protocols
	srv.Start()
	t.Cleanup(srv.Close)

	api := httpapi.New(httpapi.Config{
		BaseURL:         srv.URL,
		Providers:       map[string]*auth.Provider{"google": auth.Google("client-id", "client-secret", srv.URL+"/auth/google/callback")},
		TestLoginSecret: TestLoginSecret,
		ContactEmail:    "privacy@example.com",
	}, svc, auth.NewStateSigner(Secret), logger)
	for pattern, h := range rpc.Handlers(svc, logger) {
		api.Mount(pattern, h)
	}
	srv.Config.Handler = api.Handler()
	return &Env{Server: srv, Service: svc, Store: st, API: api}
}
