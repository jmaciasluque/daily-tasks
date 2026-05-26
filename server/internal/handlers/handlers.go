package handlers

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"daily-tasks-server/internal/auth"
	"daily-tasks-server/internal/crypto"
	"daily-tasks-server/internal/db"
)

// Server holds shared dependencies for all HTTP handlers.
type Server struct {
	DB        *sql.DB
	MasterKey []byte
	BaseURL   string // e.g. https://api.daily-tasks.io
}

func NewServer(database *sql.DB) (*Server, error) {
	masterKeyHex := os.Getenv("MASTER_KEY")
	if masterKeyHex == "" {
		return nil, http.ErrNoCookie // sentinel; caller checks
	}
	masterKey, err := base64.StdEncoding.DecodeString(masterKeyHex)
	if err != nil || len(masterKey) != 32 {
		return nil, &validationError{"MASTER_KEY must be 32 bytes base64-encoded"}
	}
	return &Server{
		DB:        database,
		MasterKey: masterKey,
		BaseURL:   os.Getenv("BASE_URL"),
	}, nil
}

type validationError struct{ msg string }

func (e *validationError) Error() string { return e.msg }

var errSyncPreconditionFailed = errors.New("sync precondition failed")

const emptySyncETag = `"empty"`

func syncETag(updatedAt time.Time) string {
	return fmt.Sprintf(`"sync-%d"`, updatedAt.UTC().UnixNano())
}

func syncPreconditionsOK(ifMatch, ifNoneMatch, currentETag string, exists bool) bool {
	if ifNoneMatch == "*" && exists {
		return false
	}
	if ifMatch == "" {
		return true
	}
	if !exists {
		return ifMatch == emptySyncETag
	}
	return ifMatch == currentETag
}

// Health godoc
// GET /health
func (s *Server) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// GoogleLogin redirects to Google OAuth consent page.
// GET /auth/google
func (s *Server) GoogleLogin(w http.ResponseWriter, r *http.Request) {
	cfg := auth.GoogleConfig(s.BaseURL + "/auth/google/callback")
	state := auth.NewOAuthStateWithRedirect(s.validLoginRedirect(r.URL.Query().Get("redirect_uri")))
	http.Redirect(w, r, cfg.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

// GoogleCallback handles the OAuth callback from Google.
// GET /auth/google/callback
func (s *Server) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	redirectURI, err := auth.ConsumeOAuthState(r.URL.Query().Get("state"))
	if err != nil {
		auth.JSONError(w, "invalid state", http.StatusBadRequest)
		return
	}
	cfg := auth.GoogleConfig(s.BaseURL + "/auth/google/callback")
	sub, email, err := auth.FetchGoogleUser(r.Context(), cfg, r.URL.Query().Get("code"))
	if err != nil {
		auth.JSONError(w, "google auth failed", http.StatusInternalServerError)
		return
	}
	s.finishLogin(w, "google", sub, email, redirectURI)
}

// FacebookLogin redirects to Facebook OAuth consent page.
// GET /auth/facebook
func (s *Server) FacebookLogin(w http.ResponseWriter, r *http.Request) {
	cfg := auth.FacebookConfig(s.BaseURL + "/auth/facebook/callback")
	state := auth.NewOAuthStateWithRedirect(s.validLoginRedirect(r.URL.Query().Get("redirect_uri")))
	http.Redirect(w, r, cfg.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

// FacebookCallback handles the OAuth callback from Facebook.
// GET /auth/facebook/callback
func (s *Server) FacebookCallback(w http.ResponseWriter, r *http.Request) {
	redirectURI, err := auth.ConsumeOAuthState(r.URL.Query().Get("state"))
	if err != nil {
		auth.JSONError(w, "invalid state", http.StatusBadRequest)
		return
	}
	cfg := auth.FacebookConfig(s.BaseURL + "/auth/facebook/callback")
	sub, email, err := auth.FetchFacebookUser(r.Context(), cfg, r.URL.Query().Get("code"))
	if err != nil {
		auth.JSONError(w, "facebook auth failed", http.StatusInternalServerError)
		return
	}
	s.finishLogin(w, "facebook", sub, email, redirectURI)
}

func (s *Server) finishLogin(w http.ResponseWriter, provider, sub, email, redirectURI string) {
	user, err := db.UpsertUser(s.DB, provider, sub, email)
	if err != nil {
		auth.JSONError(w, "db error", http.StatusInternalServerError)
		return
	}
	token, err := auth.IssueToken(user.ID)
	if err != nil {
		auth.JSONError(w, "token error", http.StatusInternalServerError)
		return
	}
	if redirectURI != "" {
		dest, err := buildLoginRedirect(redirectURI, token, email)
		if err != nil {
			auth.JSONError(w, "invalid redirect", http.StatusBadRequest)
			return
		}
		http.Redirect(w, &http.Request{}, dest, http.StatusTemporaryRedirect)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func buildLoginRedirect(redirectURI, token, email string) (string, error) {
	dest, err := url.Parse(redirectURI)
	if err != nil {
		return "", err
	}
	q := dest.Query()
	q.Set("token", token)
	if email != "" {
		q.Set("email", email)
	}
	dest.RawQuery = q.Encode()
	return dest.String(), nil
}

func (s *Server) validLoginRedirect(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	if isLoopbackHTTPRedirect(u) {
		return raw
	}
	if strings.HasPrefix(raw, s.BaseURL+"/") {
		return raw
	}
	return ""
}

func isLoopbackHTTPRedirect(u *url.URL) bool {
	if u.Scheme != "http" {
		return false
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	return host == "localhost" || (ip != nil && ip.IsLoopback())
}

// syncRequest is the body for PUT /api/v1/sync.
type syncRequest struct {
	Data    string `json:"data"`    // base64-encoded plaintext JSON
	History string `json:"history"` // base64-encoded plaintext JSON
}

// syncResponse is the body for GET /api/v1/sync.
type syncResponse struct {
	Data      string `json:"data"`       // base64-encoded plaintext JSON
	History   string `json:"history"`    // base64-encoded plaintext JSON
	UpdatedAt string `json:"updated_at"` // RFC3339
}

// GetSync returns the user's data and history blobs, decrypted.
// GET /api/v1/sync
func (s *Server) GetSync(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	ud, err := db.GetUserData(s.DB, userID)
	if err != nil {
		auth.JSONError(w, "db error", http.StatusInternalServerError)
		return
	}
	if ud == nil {
		// No data yet — return empty JSON objects so clients can bootstrap.
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("ETag", emptySyncETag)
		json.NewEncoder(w).Encode(syncResponse{
			Data:    base64.StdEncoding.EncodeToString([]byte("{}")),
			History: base64.StdEncoding.EncodeToString([]byte("{}")),
		})
		return
	}
	key, err := crypto.DeriveKey(s.MasterKey, userID)
	if err != nil {
		auth.JSONError(w, "key error", http.StatusInternalServerError)
		return
	}
	dataPlain, err := crypto.Decrypt(key, ud.Data)
	if err != nil {
		auth.JSONError(w, "decrypt error", http.StatusInternalServerError)
		return
	}
	histPlain, err := crypto.Decrypt(key, ud.History)
	if err != nil {
		auth.JSONError(w, "decrypt error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", syncETag(ud.UpdatedAt))
	json.NewEncoder(w).Encode(syncResponse{
		Data:      base64.StdEncoding.EncodeToString(dataPlain),
		History:   base64.StdEncoding.EncodeToString(histPlain),
		UpdatedAt: ud.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// PutSync saves the user's data and history blobs, encrypted.
// PUT /api/v1/sync
func (s *Server) PutSync(w http.ResponseWriter, r *http.Request) {
	userID, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB is plenty for the daily-tasks JSON blobs.
	var req syncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		auth.JSONError(w, "invalid body", http.StatusBadRequest)
		return
	}
	dataPlain, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		auth.JSONError(w, "invalid data encoding", http.StatusBadRequest)
		return
	}
	histPlain, err := base64.StdEncoding.DecodeString(req.History)
	if err != nil {
		auth.JSONError(w, "invalid history encoding", http.StatusBadRequest)
		return
	}
	key, err := crypto.DeriveKey(s.MasterKey, userID)
	if err != nil {
		auth.JSONError(w, "key error", http.StatusInternalServerError)
		return
	}
	dataCipher, err := crypto.Encrypt(key, dataPlain)
	if err != nil {
		auth.JSONError(w, "encrypt error", http.StatusInternalServerError)
		return
	}
	histCipher, err := crypto.Encrypt(key, histPlain)
	if err != nil {
		auth.JSONError(w, "encrypt error", http.StatusInternalServerError)
		return
	}
	newETag, err := s.putUserDataWithPreconditions(
		r.Context(),
		userID,
		dataCipher,
		histCipher,
		r.Header.Get("If-Match"),
		r.Header.Get("If-None-Match"),
	)
	if errors.Is(err, errSyncPreconditionFailed) {
		auth.JSONError(w, "sync precondition failed", http.StatusPreconditionFailed)
		return
	}
	if err != nil {
		auth.JSONError(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("ETag", newETag)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) putUserDataWithPreconditions(ctx context.Context, userID string, dataCipher, histCipher []byte, ifMatch, ifNoneMatch string) (string, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1))`, userID); err != nil {
		return "", err
	}

	var updatedAt time.Time
	err = tx.QueryRowContext(ctx, `
		SELECT updated_at FROM user_data WHERE user_id = $1 FOR UPDATE
	`, userID).Scan(&updatedAt)
	exists := true
	currentETag := ""
	if errors.Is(err, sql.ErrNoRows) {
		exists = false
		currentETag = emptySyncETag
	} else if err != nil {
		return "", err
	} else {
		currentETag = syncETag(updatedAt)
	}

	if !syncPreconditionsOK(ifMatch, ifNoneMatch, currentETag, exists) {
		return "", errSyncPreconditionFailed
	}

	if exists {
		err = tx.QueryRowContext(ctx, `
			UPDATE user_data
			SET data = $2, history = $3, updated_at = now()
			WHERE user_id = $1
			RETURNING updated_at
		`, userID, dataCipher, histCipher).Scan(&updatedAt)
	} else {
		err = tx.QueryRowContext(ctx, `
			INSERT INTO user_data (user_id, data, history, updated_at)
			VALUES ($1, $2, $3, now())
			RETURNING updated_at
		`, userID, dataCipher, histCipher).Scan(&updatedAt)
	}
	if err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return syncETag(updatedAt), nil
}

// requireAuth validates the Bearer JWT and returns the userID, or writes an error and returns false.
func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request) (string, bool) {
	rawToken, err := auth.BearerToken(r.Header.Get("Authorization"))
	if err != nil {
		auth.JSONError(w, "unauthorized", http.StatusUnauthorized)
		return "", false
	}
	claims, err := auth.ValidateToken(rawToken)
	if err != nil {
		auth.JSONError(w, "unauthorized", http.StatusUnauthorized)
		return "", false
	}
	return claims.UserID, true
}
