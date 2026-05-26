package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/facebook"
	"golang.org/x/oauth2/google"
)

// Claims is the JWT payload.
type Claims struct {
	UserID string `json:"uid"`
	jwt.RegisteredClaims
}

func jwtSecret() []byte {
	s := os.Getenv("JWT_SECRET")
	if s == "" {
		panic("JWT_SECRET not set")
	}
	return []byte(s)
}

// IssueToken creates a signed JWT valid for 30 days.
func IssueToken(userID string) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(30 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret())
}

// ValidateToken parses and validates a JWT, returning the claims.
func ValidateToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return jwtSecret(), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	return claims, nil
}

// GoogleConfig returns the OAuth2 config for Google.
func GoogleConfig(redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  redirectURL,
		Scopes:       []string{"openid", "email"},
		Endpoint:     google.Endpoint,
	}
}

// FacebookConfig returns the OAuth2 config for Facebook.
func FacebookConfig(redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     os.Getenv("FACEBOOK_APP_ID"),
		ClientSecret: os.Getenv("FACEBOOK_APP_SECRET"),
		RedirectURL:  redirectURL,
		Scopes:       []string{"email"},
		Endpoint:     facebook.Endpoint,
	}
}

// GoogleUserInfo fetches the user's profile from Google.
type googleUserInfo struct {
	Sub   string `json:"sub"`
	Email string `json:"email"`
}

func FetchGoogleUser(ctx context.Context, cfg *oauth2.Config, code string) (sub, email string, err error) {
	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return "", "", fmt.Errorf("google token exchange: %w", err)
	}
	client := cfg.Client(ctx, token)
	resp, err := client.Get("https://openidconnect.googleapis.com/v1/userinfo")
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var info googleUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return "", "", err
	}
	return info.Sub, info.Email, nil
}

// facebookUserInfo fetches the user's profile from Facebook.
type facebookUserInfo struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

func FetchFacebookUser(ctx context.Context, cfg *oauth2.Config, code string) (sub, email string, err error) {
	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return "", "", fmt.Errorf("facebook token exchange: %w", err)
	}
	client := cfg.Client(ctx, token)
	resp, err := client.Get("https://graph.facebook.com/me?fields=id,email")
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var info facebookUserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return "", "", err
	}
	return info.ID, info.Email, nil
}

// BearerToken extracts the Bearer token from an Authorization header value.
func BearerToken(header string) (string, error) {
	const prefix = "Bearer "
	if len(header) <= len(prefix) {
		return "", fmt.Errorf("missing bearer token")
	}
	if header[:len(prefix)] != prefix {
		return "", fmt.Errorf("invalid authorization header")
	}
	return header[len(prefix):], nil
}

// stateStore keeps one-time OAuth state values for the single-machine Fly.io deployment.
// For multi-instance deployment, replace with a Redis or DB-backed store.
type oauthStateValue struct {
	ExpiresAt   time.Time
	RedirectURI string
}

var stateStore = struct {
	sync.Mutex
	values map[string]oauthStateValue
}{values: map[string]oauthStateValue{}}

func NewOAuthState() string {
	return NewOAuthStateWithRedirect("")
}

func NewOAuthStateWithRedirect(redirectURI string) string {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		panic(fmt.Sprintf("oauth state entropy: %v", err))
	}
	state := base64.RawURLEncoding.EncodeToString(raw)
	stateStore.Lock()
	stateStore.values[state] = oauthStateValue{ExpiresAt: time.Now().Add(10 * time.Minute), RedirectURI: redirectURI}
	stateStore.Unlock()
	return state
}

func ValidateOAuthState(state string) error {
	_, err := ConsumeOAuthState(state)
	return err
}

func ConsumeOAuthState(state string) (string, error) {
	stateStore.Lock()
	value, ok := stateStore.values[state]
	if ok {
		delete(stateStore.values, state)
	}
	stateStore.Unlock()
	if !ok {
		return "", fmt.Errorf("unknown oauth state")
	}
	if time.Now().After(value.ExpiresAt) {
		return "", fmt.Errorf("oauth state expired")
	}
	return value.RedirectURI, nil
}

// JSONError writes a JSON error response.
func JSONError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	fmt.Fprintf(w, `{"error":%q}`, msg)
}
