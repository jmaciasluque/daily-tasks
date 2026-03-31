package internal

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStartLoginFlowV2(t *testing.T) {
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/index.php/login/v2" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"poll": map[string]string{
				"token":    "poll-token",
				"endpoint": server.URL + "/login/v2/poll",
			},
			"login": server.URL + "/login/v2/flow/token",
		})
	}))
	defer server.Close()

	session, err := StartLoginFlowV2(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if session.ServerURL != server.URL {
		t.Fatalf("expected server URL %q, got %q", server.URL, session.ServerURL)
	}
	if session.PollToken != "poll-token" {
		t.Fatalf("expected poll token to round-trip, got %q", session.PollToken)
	}
}

func TestPollLoginFlowV2(t *testing.T) {
	t.Run("pending returns not complete", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		_, complete, err := PollLoginFlowV2(LoginFlowV2Session{
			PollEndpoint: server.URL,
			PollToken:    "poll-token",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if complete {
			t.Fatal("expected poll to remain pending")
		}
	})

	t.Run("completed returns credentials", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("failed to parse form: %v", err)
			}
			if r.Form.Get("token") != "poll-token" {
				t.Fatalf("expected poll token, got %q", r.Form.Get("token"))
			}
			_ = json.NewEncoder(w).Encode(map[string]string{
				"server":      "https://cloud.example.com/",
				"loginName":   "user",
				"appPassword": "app-pass",
			})
		}))
		defer server.Close()

		result, complete, err := PollLoginFlowV2(LoginFlowV2Session{
			PollEndpoint: server.URL,
			PollToken:    "poll-token",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !complete {
			t.Fatal("expected poll to complete")
		}
		if result.ServerURL != "https://cloud.example.com" {
			t.Fatalf("expected normalized server URL, got %q", result.ServerURL)
		}
		if result.LoginName != "user" {
			t.Fatalf("expected login name to round-trip, got %q", result.LoginName)
		}
	})
}
