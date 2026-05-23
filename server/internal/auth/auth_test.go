package auth

import (
	"os"
	"testing"
)

func TestIssueAndValidateToken(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-that-is-long-enough")
	defer os.Unsetenv("JWT_SECRET")

	token, err := IssueToken("user-uuid-123")
	if err != nil {
		t.Fatalf("IssueToken: %v", err)
	}

	claims, err := ValidateToken(token)
	if err != nil {
		t.Fatalf("ValidateToken: %v", err)
	}

	if claims.UserID != "user-uuid-123" {
		t.Fatalf("expected uid=user-uuid-123 got %q", claims.UserID)
	}
}

func TestValidateToken_Invalid(t *testing.T) {
	os.Setenv("JWT_SECRET", "test-secret-that-is-long-enough")
	defer os.Unsetenv("JWT_SECRET")

	_, err := ValidateToken("not.a.token")
	if err == nil {
		t.Fatal("expected error for invalid token")
	}
}

func TestBearerToken(t *testing.T) {
	tok, err := BearerToken("Bearer abc123")
	if err != nil || tok != "abc123" {
		t.Fatalf("BearerToken: got %q, %v", tok, err)
	}
}

func TestBearerToken_Missing(t *testing.T) {
	_, err := BearerToken("nobearer")
	if err == nil {
		t.Fatal("expected error for missing bearer prefix")
	}
}

func TestOAuthState(t *testing.T) {
	state := NewOAuthState()
	if err := ValidateOAuthState(state); err != nil {
		t.Fatalf("ValidateOAuthState: %v", err)
	}
	// second use should fail (deleted after validation)
	if err := ValidateOAuthState(state); err == nil {
		t.Fatal("expected error on second use of state")
	}
}
