package auth

import (
	"strings"
	"testing"
	"time"
)

const secret = "test-secret-that-is-long-enough-123456"

func TestNewTokensRejectsEmptySecret(t *testing.T) {
	if _, err := NewTokens(""); err == nil {
		t.Fatal("expected error for empty secret")
	}
}

func TestIssueAndParse(t *testing.T) {
	tk, _ := NewTokens(secret)
	tok, exp, err := tk.Issue("user-1", 3, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if time.Until(exp) < 59*time.Minute {
		t.Fatalf("unexpected expiry %v", exp)
	}
	c, err := tk.Parse(tok)
	if err != nil || c.UserID != "user-1" || c.Version != 3 {
		t.Fatalf("Parse = %+v, %v", c, err)
	}
}

func TestParseRejectsExpiredForeignAndGarbage(t *testing.T) {
	tk, _ := NewTokens(secret)
	expired, _, _ := tk.Issue("u", 0, -time.Minute)
	if _, err := tk.Parse(expired); err == nil {
		t.Fatal("expired token accepted")
	}
	other, _ := NewTokens(strings.Repeat("x", 40))
	foreign, _, _ := other.Issue("u", 0, time.Hour)
	if _, err := tk.Parse(foreign); err == nil {
		t.Fatal("token signed with another secret accepted")
	}
	if _, err := tk.Parse("not.a.token"); err == nil {
		t.Fatal("garbage accepted")
	}
}

func TestLegacyTokenWithoutVersionParsesAsZero(t *testing.T) {
	// Tokens issued before token versions existed carry no "ver" claim.
	tk, _ := NewTokens(secret)
	tok, _, _ := tk.Issue("legacy", 0, time.Hour)
	c, err := tk.Parse(tok)
	if err != nil || c.Version != 0 {
		t.Fatalf("legacy parse = %+v, %v", c, err)
	}
}

func TestBearerToken(t *testing.T) {
	for in, want := range map[string]string{"Bearer abc": "abc", "bearer abc ": "abc"} {
		got, err := BearerToken(in)
		if err != nil || got != want {
			t.Fatalf("BearerToken(%q) = %q, %v", in, got, err)
		}
	}
	for _, bad := range []string{"", "Bearer ", "Basic abc", "abc"} {
		if _, err := BearerToken(bad); err == nil {
			t.Fatalf("BearerToken(%q) should fail", bad)
		}
	}
}

func TestStateRoundTrip(t *testing.T) {
	s := NewStateSigner(secret)
	state, nonce := s.New("daily-tasks://auth")
	redirect, err := s.Verify(state, nonce)
	if err != nil || redirect != "daily-tasks://auth" {
		t.Fatalf("Verify = %q, %v", redirect, err)
	}
	// Any other machine with the same secret can verify it.
	if _, err := NewStateSigner(secret).Verify(state, nonce); err != nil {
		t.Fatalf("verify on another instance: %v", err)
	}
}

func TestStateRejections(t *testing.T) {
	s := NewStateSigner(secret)
	state, nonce := s.New("")
	if _, err := s.Verify(state, ""); err == nil {
		t.Fatal("missing cookie nonce accepted")
	}
	if _, err := s.Verify(state, "other"); err == nil {
		t.Fatal("wrong cookie nonce accepted")
	}
	if _, err := s.Verify(state+"x", nonce); err == nil {
		t.Fatal("tampered signature accepted")
	}
	if _, err := NewStateSigner(strings.Repeat("y", 40)).Verify(state, nonce); err == nil {
		t.Fatal("state from another secret accepted")
	}
	s.now = func() time.Time { return time.Now().Add(StateTTL + time.Minute) }
	if _, err := s.Verify(state, nonce); err == nil {
		t.Fatal("expired state accepted")
	}
}
