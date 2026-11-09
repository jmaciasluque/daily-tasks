package internal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestBuildHostedLoginURLIncludesLoopbackRedirect(t *testing.T) {
	got, err := BuildHostedLoginURL("https://api.example.com/", "Google", "http://127.0.0.1:49152/callback")
	if err != nil {
		t.Fatalf("BuildHostedLoginURL: %v", err)
	}
	parsed, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse login URL: %v", err)
	}
	if parsed.String() != "https://api.example.com/auth/google?redirect_uri=http%3A%2F%2F127.0.0.1%3A49152%2Fcallback" {
		t.Fatalf("login URL = %q", parsed.String())
	}
}

func TestRunHostedLoginCapturesTokenFromBrowserCallback(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/facebook" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		redirectURI := r.URL.Query().Get("redirect_uri")
		if !strings.HasPrefix(redirectURI, "http://127.0.0.1:") || !strings.HasSuffix(redirectURI, "/callback") {
			t.Fatalf("redirect_uri = %q", redirectURI)
		}
		http.Redirect(w, r, redirectURI+"?token=jwt-from-server", http.StatusTemporaryRedirect)
	}))
	defer api.Close()

	token, err := RunHostedLogin(context.Background(), HostedLoginOptions{
		APIURL:   api.URL,
		Provider: "facebook",
		Timeout:  2 * time.Second,
		OpenBrowser: func(loginURL string) error {
			resp, err := http.Get(loginURL) // follows API redirect back to local callback
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			return nil
		},
	})
	if err != nil {
		t.Fatalf("RunHostedLogin: %v", err)
	}
	if token != "jwt-from-server" {
		t.Fatalf("token = %q", token)
	}
}
