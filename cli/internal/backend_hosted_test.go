package internal

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHostedBackendFetchUsesBearerTokenAndDecodesSyncPayload(t *testing.T) {
	var gotAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/sync" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("ETag", `"server-etag"`)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"data":    base64.StdEncoding.EncodeToString([]byte(`{"tasks":[]}`)),
			"history": base64.StdEncoding.EncodeToString([]byte(`{"version":1}`)),
		})
	}))
	defer server.Close()

	backend := NewHostedBackend(server.URL, "test-token")
	blob, err := backend.Fetch(KeyData)
	if err != nil {
		t.Fatalf("Fetch(KeyData): %v", err)
	}

	if gotAuth != "Bearer test-token" {
		t.Fatalf("Authorization header = %q, want Bearer token", gotAuth)
	}
	if string(blob.Bytes) != `{"tasks":[]}` {
		t.Fatalf("data bytes = %q", string(blob.Bytes))
	}
	if blob.Etag != `"server-etag"` {
		t.Fatalf("etag = %q, want server ETag", blob.Etag)
	}

	historyBlob, err := backend.Fetch(KeyHistory)
	if err != nil {
		t.Fatalf("Fetch(KeyHistory): %v", err)
	}
	if string(historyBlob.Bytes) != `{"version":1}` {
		t.Fatalf("history bytes = %q", string(historyBlob.Bytes))
	}
}

func TestHostedBackendPushMergesExistingPayloadAndEncodesBase64(t *testing.T) {
	var putBody struct {
		Data    string `json:"data"`
		History string `json:"history"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]string{
				"data":    base64.StdEncoding.EncodeToString([]byte(`{"tasks":[]}`)),
				"history": base64.StdEncoding.EncodeToString([]byte(`{"version":1}`)),
			})
		case http.MethodPut:
			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Fatalf("Authorization header = %q", r.Header.Get("Authorization"))
			}
			if r.Header.Get("If-Match") != `"etag-123"` {
				t.Fatalf("If-Match header = %q", r.Header.Get("If-Match"))
			}
			if err := json.NewDecoder(r.Body).Decode(&putBody); err != nil {
				t.Fatalf("decode PUT body: %v", err)
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	backend := NewHostedBackend(server.URL, "test-token")
	if err := backend.Push(KeyData, []byte(`{"tasks":[{"id":1}]}`), `"etag-123"`); err != nil {
		t.Fatalf("Push(KeyData): %v", err)
	}

	decodedData, err := base64.StdEncoding.DecodeString(putBody.Data)
	if err != nil {
		t.Fatalf("decode data: %v", err)
	}
	decodedHistory, err := base64.StdEncoding.DecodeString(putBody.History)
	if err != nil {
		t.Fatalf("decode history: %v", err)
	}
	if string(decodedData) != `{"tasks":[{"id":1}]}` {
		t.Fatalf("PUT data = %q", string(decodedData))
	}
	if string(decodedHistory) != `{"version":1}` {
		t.Fatalf("PUT history = %q", string(decodedHistory))
	}
}

func TestHostedBackendPushSendsIfNoneMatchAny(t *testing.T) {
	var gotIfNoneMatch string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]string{
				"data":    base64.StdEncoding.EncodeToString([]byte(`{"tasks":[]}`)),
				"history": base64.StdEncoding.EncodeToString([]byte(`{"version":1}`)),
			})
		case http.MethodPut:
			gotIfNoneMatch = r.Header.Get("If-None-Match")
			w.WriteHeader(http.StatusOK)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	defer server.Close()

	backend := NewHostedBackend(server.URL, "test-token")
	if err := backend.Push(KeyHistory, []byte(`{"version":1}`), IfNoneMatchAny); err != nil {
		t.Fatalf("Push(KeyHistory): %v", err)
	}
	if gotIfNoneMatch != "*" {
		t.Fatalf("If-None-Match header = %q, want *", gotIfNoneMatch)
	}
}

func TestHostedBackendUnauthorizedAsksUserToLoginAgain(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	backend := NewHostedBackend(server.URL, "expired-token")
	_, err := backend.Fetch(KeyData)
	if err == nil {
		t.Fatal("expected error")
	}
	if err != ErrHostedTokenInvalid {
		t.Fatalf("error = %v, want ErrHostedTokenInvalid", err)
	}
}
