// Package auth issues and verifies session tokens and OAuth login state.
package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/hkdf"
)

// SessionTTL is how long a session token stays valid. Clients renew it via
// the refresh endpoint while it is still valid, so active users never get
// logged out.
const SessionTTL = 60 * 24 * time.Hour

// RefreshAfter is the token age after which clients should renew.
const RefreshAfter = 7 * 24 * time.Hour

// Claims is the JWT payload.
type Claims struct {
	UserID string `json:"uid"`
	// Version must equal the user's current token_version; bumping it
	// revokes every outstanding token.
	Version int `json:"ver,omitempty"`
	jwt.RegisteredClaims
}

// Tokens signs and verifies session JWTs.
type Tokens struct {
	secret []byte
	now    func() time.Time
}

// RecommendedSecretLength is the minimum length new deployments should use;
// callers warn below it. Shorter (non-empty) secrets are still accepted so an
// existing deployment keeps validating the tokens it already issued.
const RecommendedSecretLength = 32

// NewTokens returns a signer for the given (non-empty) secret.
func NewTokens(secret string) (*Tokens, error) {
	if secret == "" {
		return nil, errors.New("JWT_SECRET not set")
	}
	return &Tokens{secret: []byte(secret), now: time.Now}, nil
}

// Issue creates a signed token for userID at the given token version.
func (t *Tokens) Issue(userID string, version int, ttl time.Duration) (string, time.Time, error) {
	now := t.now()
	exp := now.Add(ttl)
	claims := Claims{
		UserID:  userID,
		Version: version,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(t.secret)
	return signed, exp, err
}

// Parse validates signature and expiry. Callers must still check that the
// user exists and the version matches.
func (t *Tokens) Parse(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &Claims{}, func(tok *jwt.Token) (any, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", tok.Header["alg"])
		}
		return t.secret, nil
	}, jwt.WithTimeFunc(t.now), jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid || claims.UserID == "" {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}

// BearerToken extracts the token from an Authorization header value.
func BearerToken(header string) (string, error) {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", errors.New("missing bearer token")
	}
	return strings.TrimSpace(header[len(prefix):]), nil
}

// StateSigner produces OAuth "state" values that need no server-side storage:
// the redirect target and expiry are signed into the value itself, and a
// random nonce is also placed in a browser cookie so the callback can verify
// it is completing a login this browser started (login-CSRF protection).
// Any machine can therefore finish a login started on another one, which is
// what allows scale-to-zero and more than one machine.
type StateSigner struct {
	key []byte
	now func() time.Time
}

// StateTTL bounds how long a user may take on the provider's consent screen.
const StateTTL = 10 * time.Minute

type statePayload struct {
	Nonce    string `json:"n"`
	Redirect string `json:"r,omitempty"`
	Expires  int64  `json:"e"`
}

// NewStateSigner derives a dedicated key from the session secret.
func NewStateSigner(secret string) *StateSigner {
	key := make([]byte, 32)
	if _, err := io.ReadFull(hkdf.New(sha256.New, []byte(secret), nil, []byte("daily-tasks-oauth-state")), key); err != nil {
		panic(err)
	}
	return &StateSigner{key: key, now: time.Now}
}

// New returns a signed state value and the nonce to store in a cookie.
func (s *StateSigner) New(redirect string) (state, nonce string) {
	raw := make([]byte, 18)
	if _, err := rand.Read(raw); err != nil {
		panic(fmt.Sprintf("oauth state entropy: %v", err))
	}
	nonce = base64.RawURLEncoding.EncodeToString(raw)
	body, _ := json.Marshal(statePayload{Nonce: nonce, Redirect: redirect, Expires: s.now().Add(StateTTL).Unix()})
	encoded := base64.RawURLEncoding.EncodeToString(body)
	return encoded + "." + s.sign(encoded), nonce
}

// Verify checks signature, expiry and that cookieNonce matches, returning the
// redirect target that was requested when the login started.
func (s *StateSigner) Verify(state, cookieNonce string) (string, error) {
	encoded, sig, ok := strings.Cut(state, ".")
	if !ok || !hmac.Equal([]byte(sig), []byte(s.sign(encoded))) {
		return "", errors.New("invalid oauth state")
	}
	body, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return "", errors.New("invalid oauth state")
	}
	var p statePayload
	if err := json.Unmarshal(body, &p); err != nil {
		return "", errors.New("invalid oauth state")
	}
	if s.now().Unix() > p.Expires {
		return "", errors.New("oauth state expired")
	}
	if cookieNonce == "" || !hmac.Equal([]byte(p.Nonce), []byte(cookieNonce)) {
		return "", errors.New("oauth state not started in this browser")
	}
	return p.Redirect, nil
}

func (s *StateSigner) sign(encoded string) string {
	mac := hmac.New(sha256.New, s.key)
	mac.Write([]byte(encoded))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
