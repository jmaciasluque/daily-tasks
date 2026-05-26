package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"daily-tasks-server/internal/auth"
)

func TestLoginRedirectURIAllowsLoopbackCallbackAndRejectsRemote(t *testing.T) {
	s := setupTestServer(t)
	allowed := "http://127.0.0.1:49152/callback"
	if got := s.validLoginRedirect(allowed); got != allowed {
		t.Fatalf("loopback redirect = %q, want %q", got, allowed)
	}
	if got := s.validLoginRedirect("https://evil.example.com/callback"); got != "" {
		t.Fatalf("evil redirect accepted: %q", got)
	}
}

func TestGoogleLoginStoresValidatedRedirectURIInOAuthState(t *testing.T) {
	s := setupTestServer(t)
	redirectURI := "http://127.0.0.1:49152/callback"
	req := httptest.NewRequest(http.MethodGet, "/auth/google?redirect_uri="+url.QueryEscape(redirectURI), nil)
	w := httptest.NewRecorder()

	s.GoogleLogin(w, req)
	if w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("status = %d", w.Code)
	}
	location := w.Header().Get("Location")
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	state := parsed.Query().Get("state")
	if state == "" {
		t.Fatalf("missing state in Location %q", location)
	}
	gotRedirect, err := auth.ConsumeOAuthState(state)
	if err != nil {
		t.Fatalf("ConsumeOAuthState: %v", err)
	}
	if gotRedirect != redirectURI {
		t.Fatalf("stored redirect = %q, want %q", gotRedirect, redirectURI)
	}
}

func TestBuildLoginRedirectAddsTokenAndEmail(t *testing.T) {
	redirectURI := "http://127.0.0.1:49152/callback?existing=1"
	location, err := buildLoginRedirect(redirectURI, "jwt-token", "user@example.com")
	if err != nil {
		t.Fatalf("buildLoginRedirect: %v", err)
	}
	if !strings.HasPrefix(location, "http://127.0.0.1:49152/callback?") {
		t.Fatalf("Location = %q", location)
	}
	parsed, err := url.Parse(location)
	if err != nil {
		t.Fatalf("parse Location: %v", err)
	}
	if parsed.Query().Get("existing") != "1" || parsed.Query().Get("token") != "jwt-token" || parsed.Query().Get("email") != "user@example.com" {
		t.Fatalf("query = %v", parsed.Query())
	}
}
