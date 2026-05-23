package handlers

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"os"

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
	state := auth.NewOAuthState()
	http.Redirect(w, r, cfg.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

// GoogleCallback handles the OAuth callback from Google.
// GET /auth/google/callback
func (s *Server) GoogleCallback(w http.ResponseWriter, r *http.Request) {
	if err := auth.ValidateOAuthState(r.URL.Query().Get("state")); err != nil {
		auth.JSONError(w, "invalid state", http.StatusBadRequest)
		return
	}
	cfg := auth.GoogleConfig(s.BaseURL + "/auth/google/callback")
	sub, email, err := auth.FetchGoogleUser(r.Context(), cfg, r.URL.Query().Get("code"))
	if err != nil {
		auth.JSONError(w, "google auth failed", http.StatusInternalServerError)
		return
	}
	s.finishLogin(w, "google", sub, email)
}

// FacebookLogin redirects to Facebook OAuth consent page.
// GET /auth/facebook
func (s *Server) FacebookLogin(w http.ResponseWriter, r *http.Request) {
	cfg := auth.FacebookConfig(s.BaseURL + "/auth/facebook/callback")
	state := auth.NewOAuthState()
	http.Redirect(w, r, cfg.AuthCodeURL(state), http.StatusTemporaryRedirect)
}

// FacebookCallback handles the OAuth callback from Facebook.
// GET /auth/facebook/callback
func (s *Server) FacebookCallback(w http.ResponseWriter, r *http.Request) {
	if err := auth.ValidateOAuthState(r.URL.Query().Get("state")); err != nil {
		auth.JSONError(w, "invalid state", http.StatusBadRequest)
		return
	}
	cfg := auth.FacebookConfig(s.BaseURL + "/auth/facebook/callback")
	sub, email, err := auth.FetchFacebookUser(r.Context(), cfg, r.URL.Query().Get("code"))
	if err != nil {
		auth.JSONError(w, "facebook auth failed", http.StatusInternalServerError)
		return
	}
	s.finishLogin(w, "facebook", sub, email)
}

func (s *Server) finishLogin(w http.ResponseWriter, provider, sub, email string) {
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
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
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
	if err := db.PutUserData(s.DB, userID, dataCipher, histCipher); err != nil {
		auth.JSONError(w, "db error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
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
