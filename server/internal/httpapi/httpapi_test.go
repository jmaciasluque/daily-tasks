package httpapi_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"daily-tasks-server/internal/auth"
	"daily-tasks-server/internal/service"
	"daily-tasks-server/internal/testutil"
)

var noRedirect = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

func testToken(t *testing.T, env *testutil.Env, sub string) string {
	t.Helper()
	res, err := http.Get(env.Server.URL + "/auth/test?secret=" + testutil.TestLoginSecret + "&sub=" + sub + "&email=" + sub + "@example.com")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var body struct{ Token, Email string }
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil || body.Token == "" {
		t.Fatalf("test login: %d %+v %v", res.StatusCode, body, err)
	}
	return body.Token
}

func do(t *testing.T, method, url, token string, body io.Reader, headers map[string]string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(method, url, body)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := noRedirect.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { res.Body.Close() })
	return res
}

func TestHealthReadyAndHeaders(t *testing.T) {
	env := testutil.New(t)
	for _, path := range []string{"/health", "/ready"} {
		res := do(t, "GET", env.Server.URL+path, "", nil, nil)
		if res.StatusCode != 200 {
			t.Fatalf("%s: %d", path, res.StatusCode)
		}
		if res.Header.Get("X-Content-Type-Options") != "nosniff" {
			t.Fatalf("%s: missing security headers", path)
		}
	}
}

func TestRESTv1SyncIsWireCompatible(t *testing.T) {
	env := testutil.New(t)
	tok := testToken(t, env, "rest")
	u := env.Server.URL + "/api/v1/sync"

	res := do(t, "GET", u, "", nil, nil)
	if res.StatusCode != 401 {
		t.Fatalf("unauthenticated GET: %d", res.StatusCode)
	}

	// Empty account: base64 "{}" documents and the "empty" etag.
	res = do(t, "GET", u, tok, nil, nil)
	var got map[string]string
	json.NewDecoder(res.Body).Decode(&got)
	if res.Header.Get("ETag") != `"empty"` || got["data"] != "e30=" || got["history"] != "e30=" {
		t.Fatalf("empty GET: etag=%q body=%v", res.Header.Get("ETag"), got)
	}

	put := func(data, etag, noneMatch string) *http.Response {
		body, _ := json.Marshal(map[string]string{
			"data":    base64.StdEncoding.EncodeToString([]byte(data)),
			"history": base64.StdEncoding.EncodeToString([]byte(`{"days":[]}`)),
		})
		h := map[string]string{"Content-Type": "application/json"}
		if etag != "" {
			h["If-Match"] = etag
		}
		if noneMatch != "" {
			h["If-None-Match"] = noneMatch
		}
		return do(t, "PUT", u, tok, strings.NewReader(string(body)), h)
	}

	res = put(`{"tasks":[]}`, "", "*")
	if res.StatusCode != 200 || res.Header.Get("ETag") == "" {
		t.Fatalf("create: %d", res.StatusCode)
	}
	etag := res.Header.Get("ETag")
	if res = put(`{"tasks":[]}`, "", "*"); res.StatusCode != 412 {
		t.Fatalf("second create-only: %d", res.StatusCode)
	}
	res = put(`{"tasks":[1]}`, etag, "")
	if res.StatusCode != 200 {
		t.Fatalf("conditional update: %d", res.StatusCode)
	}
	if res = put(`{"tasks":[2]}`, etag, ""); res.StatusCode != 412 {
		t.Fatalf("stale update: %d", res.StatusCode)
	}
	if res = put(`not json`, "", ""); res.StatusCode != 400 {
		t.Fatalf("invalid doc: %d", res.StatusCode)
	}
	res = do(t, "PUT", u, tok, strings.NewReader("garbage"), nil)
	if res.StatusCode != 400 {
		t.Fatalf("garbage body: %d", res.StatusCode)
	}

	res = do(t, "GET", u, tok, nil, nil)
	json.NewDecoder(res.Body).Decode(&got)
	data, _ := base64.StdEncoding.DecodeString(got["data"])
	if string(data) != `{"tasks":[1]}` || got["updated_at"] == "" {
		t.Fatalf("final GET: %s %v", data, got)
	}
}

// A long-lived account's history runs to megabytes and must fit in a REST PUT.
func TestRESTv1SyncAcceptsLargeHistory(t *testing.T) {
	env := testutil.New(t)
	tok := testToken(t, env, "rest-big")
	u := env.Server.URL + "/api/v1/sync"
	doc := func(n int) string { return `{"x":"` + strings.Repeat("a", n-len(`{"x":""}`)) + `"}` }
	put := func(data, history string) int {
		body, _ := json.Marshal(map[string]string{
			"data":    base64.StdEncoding.EncodeToString([]byte(data)),
			"history": base64.StdEncoding.EncodeToString([]byte(history)),
		})
		return do(t, "PUT", u, tok, strings.NewReader(string(body)), map[string]string{"Content-Type": "application/json"}).StatusCode
	}
	if code := put(doc(service.MaxDataBytes), doc(service.MaxHistoryBytes)); code != 200 {
		t.Fatalf("documents at the limit: %d", code)
	}
	if code := put(`{}`, doc(service.MaxHistoryBytes+1)); code != 400 {
		t.Fatalf("history over the limit: %d", code)
	}
	if code := put(doc(service.MaxDataBytes+1), `{}`); code != 400 {
		t.Fatalf("data over the limit: %d", code)
	}
	var got map[string]string
	json.NewDecoder(do(t, "GET", u, tok, nil, nil).Body).Decode(&got)
	history, _ := base64.StdEncoding.DecodeString(got["history"])
	if len(history) != service.MaxHistoryBytes {
		t.Fatalf("stored history: %d bytes", len(history))
	}
}

func TestAccountEndpoints(t *testing.T) {
	env := testutil.New(t)
	tok := testToken(t, env, "acct")

	res := do(t, "GET", env.Server.URL+"/api/v1/me", tok, nil, nil)
	var me map[string]any
	json.NewDecoder(res.Body).Decode(&me)
	if res.StatusCode != 200 || me["email"] != "acct@example.com" || me["provider"] != "test" {
		t.Fatalf("me: %d %v", res.StatusCode, me)
	}

	res = do(t, "POST", env.Server.URL+"/api/v1/session/refresh", tok, nil, nil)
	var refreshed map[string]string
	json.NewDecoder(res.Body).Decode(&refreshed)
	if res.StatusCode != 200 || refreshed["token"] == "" {
		t.Fatalf("refresh: %d %v", res.StatusCode, refreshed)
	}

	if res = do(t, "POST", env.Server.URL+"/api/v1/session/revoke", tok, nil, nil); res.StatusCode != 204 {
		t.Fatalf("revoke: %d", res.StatusCode)
	}
	for _, old := range []string{tok, refreshed["token"]} {
		if res = do(t, "GET", env.Server.URL+"/api/v1/me", old, nil, nil); res.StatusCode != 401 {
			t.Fatalf("revoked token still works: %d", res.StatusCode)
		}
	}

	tok = testToken(t, env, "acct")
	if res = do(t, "DELETE", env.Server.URL+"/api/v1/me", tok, nil, nil); res.StatusCode != 204 {
		t.Fatalf("delete: %d", res.StatusCode)
	}
	if res = do(t, "GET", env.Server.URL+"/api/v1/sync", tok, nil, nil); res.StatusCode != 401 {
		t.Fatalf("deleted account token still works: %d", res.StatusCode)
	}
}

func TestOAuthStartSetsSignedStateAndCookie(t *testing.T) {
	env := testutil.New(t)
	res := do(t, "GET", env.Server.URL+"/auth/google?redirect_uri="+url.QueryEscape("daily-tasks://auth"), "", nil, nil)
	if res.StatusCode != http.StatusFound {
		t.Fatalf("status %d", res.StatusCode)
	}
	loc, _ := url.Parse(res.Header.Get("Location"))
	if loc.Host != "accounts.google.com" {
		t.Fatalf("redirected to %s", loc)
	}
	var nonce string
	for _, c := range res.Cookies() {
		if c.Name == "dt_oauth_nonce" {
			nonce = c.Value
			if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
				t.Fatalf("cookie flags: %+v", c)
			}
		}
	}
	redirect, err := auth.NewStateSigner(testutil.Secret).Verify(loc.Query().Get("state"), nonce)
	if err != nil || redirect != "daily-tasks://auth" {
		t.Fatalf("state verify: %q %v", redirect, err)
	}
}

func TestOAuthStartDropsUntrustedRedirect(t *testing.T) {
	env := testutil.New(t)
	res := do(t, "GET", env.Server.URL+"/auth/google?redirect_uri="+url.QueryEscape("https://evil.example/cb"), "", nil, nil)
	loc, _ := url.Parse(res.Header.Get("Location"))
	nonce := res.Cookies()[0].Value
	redirect, err := auth.NewStateSigner(testutil.Secret).Verify(loc.Query().Get("state"), nonce)
	if err != nil || redirect != "" {
		t.Fatalf("untrusted redirect kept: %q %v", redirect, err)
	}
}

func TestOAuthCallbackRejectsForgedOrCookielessState(t *testing.T) {
	env := testutil.New(t)
	state, _ := auth.NewStateSigner(testutil.Secret).New("daily-tasks://auth")
	// No cookie: a login started in another browser (login CSRF).
	res := do(t, "GET", env.Server.URL+"/auth/google/callback?code=x&state="+url.QueryEscape(state), "", nil, nil)
	if res.StatusCode != 400 {
		t.Fatalf("cookieless callback: %d", res.StatusCode)
	}
	res = do(t, "GET", env.Server.URL+"/auth/google/callback?code=x&state=forged", "", nil, map[string]string{"Cookie": "dt_oauth_nonce=abc"})
	if res.StatusCode != 400 {
		t.Fatalf("forged state: %d", res.StatusCode)
	}
}

func TestUnknownOrUnconfiguredProvider(t *testing.T) {
	env := testutil.New(t)
	for _, p := range []string{"facebook", "myspace"} {
		if res := do(t, "GET", env.Server.URL+"/auth/"+p, "", nil, nil); res.StatusCode != 404 {
			t.Fatalf("%s: %d", p, res.StatusCode)
		}
	}
}

func TestTestLoginRequiresSecretAndRedirects(t *testing.T) {
	env := testutil.New(t)
	if res := do(t, "GET", env.Server.URL+"/auth/test?secret=wrong", "", nil, nil); res.StatusCode != 403 {
		t.Fatalf("wrong secret: %d", res.StatusCode)
	}
	res := do(t, "GET", env.Server.URL+"/auth/test?secret="+testutil.TestLoginSecret+"&sub=x&email=x@example.com&redirect_uri="+url.QueryEscape("daily-tasks-dev://auth"), "", nil, nil)
	loc, _ := url.Parse(res.Header.Get("Location"))
	if res.StatusCode != 302 || loc.Scheme != "daily-tasks-dev" || loc.Query().Get("token") == "" || loc.Query().Get("email") != "x@example.com" {
		t.Fatalf("redirect: %d %s", res.StatusCode, loc)
	}
}

func TestPublicPages(t *testing.T) {
	env := testutil.New(t)
	for path, want := range map[string]string{
		"/":               "reset every morning",
		"/privacy":        "AES-256-GCM",
		"/terms":          "Fair use",
		"/account/delete": "Sign in with Google to continue",
	} {
		res := do(t, "GET", env.Server.URL+path, "", nil, nil)
		body, _ := io.ReadAll(res.Body)
		if res.StatusCode != 200 || !strings.Contains(string(body), want) {
			t.Fatalf("%s: %d, missing %q", path, res.StatusCode, want)
		}
		if !strings.Contains(res.Header.Get("Content-Security-Policy"), "default-src 'none'") {
			t.Fatalf("%s: missing CSP", path)
		}
		if path == "/privacy" && !strings.Contains(string(body), "privacy@example.com") {
			t.Fatal("contact email not rendered")
		}
	}
}

func TestWebAccountDeletionFlow(t *testing.T) {
	env := testutil.New(t)
	// Sign-in for the web page lands on our own origin with a short-lived token.
	res := do(t, "GET", env.Server.URL+"/auth/test?secret="+testutil.TestLoginSecret+"&sub=web&email=web@example.com&redirect_uri="+url.QueryEscape(env.Server.URL+"/account/delete/confirm"), "", nil, nil)
	loc, _ := url.Parse(res.Header.Get("Location"))
	tok := loc.Query().Get("token")
	claims, err := mustTokens(t).Parse(tok)
	if err != nil || claims.ExpiresAt.Sub(claims.IssuedAt.Time) > 20*60e9 {
		t.Fatalf("web token should be short-lived: %+v %v", claims, err)
	}

	res = do(t, "GET", loc.String(), "", nil, nil)
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || !strings.Contains(string(body), "web@example.com") {
		t.Fatalf("confirm page: %d %s", res.StatusCode, body)
	}

	res = do(t, "POST", env.Server.URL+"/account/delete/confirm", "", strings.NewReader("token="+url.QueryEscape(tok)), map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	body, _ = io.ReadAll(res.Body)
	if res.StatusCode != 200 || !strings.Contains(string(body), "Account deleted") {
		t.Fatalf("delete submit: %d %s", res.StatusCode, body)
	}
	if _, err := env.Service.Authenticate(context.Background(), tok); err == nil {
		t.Fatal("account still exists")
	}

	res = do(t, "POST", env.Server.URL+"/account/delete/confirm", "", strings.NewReader("token=bogus"), map[string]string{"Content-Type": "application/x-www-form-urlencoded"})
	if res.StatusCode != 401 {
		t.Fatalf("bogus token delete: %d", res.StatusCode)
	}
}

func mustTokens(t *testing.T) *auth.Tokens {
	tk, err := auth.NewTokens(testutil.Secret)
	if err != nil {
		t.Fatal(err)
	}
	return tk
}

func TestWebAppRouteAndSessionLength(t *testing.T) {
	env := testutil.New(t)
	res := do(t, "GET", env.Server.URL+"/app", "", nil, nil)
	if res.StatusCode != http.StatusMovedPermanently || res.Header.Get("Location") != "/app/" {
		t.Fatalf("/app redirect: %d %q", res.StatusCode, res.Header.Get("Location"))
	}
	res = do(t, "GET", env.Server.URL+"/app/", "", nil, nil)
	if !strings.Contains(res.Header.Get("Content-Security-Policy"), "connect-src 'self'") {
		t.Fatalf("/app/ CSP: %q", res.Header.Get("Content-Security-Policy"))
	}

	// Signing in to the web app yields a full session, unlike the deletion page.
	res = do(t, "GET", env.Server.URL+"/auth/test?secret="+testutil.TestLoginSecret+"&sub=webapp&redirect_uri="+url.QueryEscape(env.Server.URL+"/app/"), "", nil, nil)
	loc, _ := url.Parse(res.Header.Get("Location"))
	claims, err := mustTokens(t).Parse(loc.Query().Get("token"))
	if err != nil || claims.ExpiresAt.Sub(claims.IssuedAt.Time) < auth.SessionTTL-time.Minute {
		t.Fatalf("web app session too short: %+v %v", claims, err)
	}
}
