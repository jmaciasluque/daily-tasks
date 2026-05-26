package handlers

import (
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"daily-tasks-server/internal/auth"
	"daily-tasks-server/internal/crypto"
)

func setupTestServer(t *testing.T) *Server {
	t.Helper()
	os.Setenv("JWT_SECRET", "test-secret-for-handlers-testing-ok")
	os.Setenv("MASTER_KEY", base64.StdEncoding.EncodeToString(make([]byte, 32)))
	os.Setenv("BASE_URL", "http://localhost:8080")
	t.Cleanup(func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("MASTER_KEY")
		os.Unsetenv("BASE_URL")
	})
	return &Server{
		DB:        nil,
		MasterKey: make([]byte, 32),
		BaseURL:   "http://localhost:8080",
	}
}

func issueTestToken(t *testing.T, userID string) string {
	t.Helper()
	tok, err := auth.IssueToken(userID)
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}
	return tok
}

func TestHealth(t *testing.T) {
	s := setupTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	s.Health(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ok") {
		t.Fatalf("expected ok body, got %q", w.Body.String())
	}
}

func TestRequireAuth_Missing(t *testing.T) {
	s := setupTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync", nil)
	w := httptest.NewRecorder()
	_, ok := s.requireAuth(w, req)
	if ok {
		t.Fatal("expected auth to fail without header")
	}
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 got %d", w.Code)
	}
}

func TestRequireAuth_InvalidToken(t *testing.T) {
	s := setupTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync", nil)
	req.Header.Set("Authorization", "Bearer not.valid.token")
	w := httptest.NewRecorder()
	_, ok := s.requireAuth(w, req)
	if ok {
		t.Fatal("expected auth to fail for invalid token")
	}
}

func TestRequireAuth_ValidToken(t *testing.T) {
	s := setupTestServer(t)
	token := issueTestToken(t, "user-abc-123")
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sync", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	userID, ok := s.requireAuth(w, req)
	if !ok {
		t.Fatal("expected auth to succeed")
	}
	if userID != "user-abc-123" {
		t.Fatalf("expected user-abc-123 got %q", userID)
	}
}

func TestPutSync_InvalidBody(t *testing.T) {
	s := setupTestServer(t)
	token := issueTestToken(t, "user-test-id")
	req := httptest.NewRequest(http.MethodPut, "/api/v1/sync", strings.NewReader("not json"))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.PutSync(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestPutSync_InvalidBase64(t *testing.T) {
	s := setupTestServer(t)
	token := issueTestToken(t, "user-test-id")
	body := `{"data":"!!!notbase64","history":"also bad"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/sync", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.PutSync(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 got %d", w.Code)
	}
}

func TestEncryptDecryptViaHandlerHelpers(t *testing.T) {
	masterKey := make([]byte, 32)
	userID := "abc-123"
	key, _ := crypto.DeriveKey(masterKey, userID)

	plaintext := []byte(`{"tasks":[],"version":1}`)
	cipherBlob, err := crypto.Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	got, err := crypto.Decrypt(key, cipherBlob)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Fatalf("roundtrip failed")
	}
}

func TestSyncResponseEncoding(t *testing.T) {
	resp := syncResponse{
		Data:      base64.StdEncoding.EncodeToString([]byte(`{"tasks":[]}`)),
		History:   base64.StdEncoding.EncodeToString([]byte(`{}`)),
		UpdatedAt: "2024-01-01T00:00:00Z",
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]string
	json.Unmarshal(b, &out)
	if out["updated_at"] != "2024-01-01T00:00:00Z" {
		t.Fatalf("expected updated_at got %q", out["updated_at"])
	}
}

func TestSyncPreconditions(t *testing.T) {
	cases := []struct {
		name        string
		ifMatch     string
		ifNoneMatch string
		current     string
		exists      bool
		want        bool
	}{
		{name: "unconditional create", current: emptySyncETag, exists: false, want: true},
		{name: "empty etag allows create only while absent", ifMatch: emptySyncETag, current: emptySyncETag, exists: false, want: true},
		{name: "empty etag rejects after create", ifMatch: emptySyncETag, current: `"sync-1"`, exists: true, want: false},
		{name: "matching etag allows update", ifMatch: `"sync-1"`, current: `"sync-1"`, exists: true, want: true},
		{name: "stale etag rejects update", ifMatch: `"sync-0"`, current: `"sync-1"`, exists: true, want: false},
		{name: "if none match star allows create", ifNoneMatch: "*", current: emptySyncETag, exists: false, want: true},
		{name: "if none match star rejects existing", ifNoneMatch: "*", current: `"sync-1"`, exists: true, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := syncPreconditionsOK(tc.ifMatch, tc.ifNoneMatch, tc.current, tc.exists)
			if got != tc.want {
				t.Fatalf("syncPreconditionsOK() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestGoogleLoginIncludesRedirectState(t *testing.T) {
	s := setupTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/auth/google?redirect_uri=daily-tasks://auth", nil)
	w := httptest.NewRecorder()
	s.GoogleLogin(w, req)
	if w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected redirect got %d", w.Code)
	}
	loc := w.Header().Get("Location")
	if !strings.Contains(loc, "state=") {
		t.Fatalf("expected OAuth state in redirect URL, got %q", loc)
	}
}

func TestLoginRedirectURLValidation(t *testing.T) {
	s := setupTestServer(t)
	if got := s.validLoginRedirect("daily-tasks://auth"); got != "daily-tasks://auth" {
		t.Fatalf("expected app redirect, got %q", got)
	}
	if got := s.validLoginRedirect("https://evil.example/callback"); got != "" {
		t.Fatalf("expected untrusted redirect to be rejected, got %q", got)
	}
}

// Compile-time check: sql.DB is used in the Server struct.
var _ *sql.DB = (*sql.DB)(nil)
