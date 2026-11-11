// Package httpapi exposes the REST v1 API, the OAuth login flow and the public
// web pages (privacy policy, terms, account deletion).
package httpapi

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"daily-tasks-server/internal/auth"
	"daily-tasks-server/internal/service"
	"daily-tasks-server/internal/webapp"
)

// Config holds the HTTP layer's settings.
type Config struct {
	// BaseURL is the public origin, e.g. https://daily-tasks-api.fly.dev.
	BaseURL string
	// Providers maps provider name ("google", "facebook") to its config.
	Providers map[string]*auth.Provider
	// TestLoginSecret enables GET /auth/test for CI and emulator tests.
	// It must never be set in production.
	TestLoginSecret string
	// ContactEmail is shown on the privacy and deletion pages.
	ContactEmail string
}

// Server serves every HTTP route except the Connect API, which is mounted on
// the same mux by the caller via Mount.
type Server struct {
	cfg    Config
	svc    *service.Service
	state  *auth.StateSigner
	logger *slog.Logger
	mux    *http.ServeMux
	https  bool
}

// New builds the server and its routes.
func New(cfg Config, svc *service.Service, state *auth.StateSigner, logger *slog.Logger) *Server {
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	s := &Server{cfg: cfg, svc: svc, state: state, logger: logger, mux: http.NewServeMux(), https: strings.HasPrefix(cfg.BaseURL, "https://")}

	authLimit := NewRateLimiter(1, 10)
	apiLimit := NewRateLimiter(10, 40)
	a := func(h http.HandlerFunc) http.Handler { return limit(authLimit, h) }
	api := func(h http.HandlerFunc) http.Handler { return limit(apiLimit, h) }

	s.mux.HandleFunc("GET /health", s.health)
	s.mux.HandleFunc("GET /ready", s.ready)

	s.mux.Handle("GET /auth/{provider}", a(s.oauthStart))
	s.mux.Handle("GET /auth/{provider}/callback", a(s.oauthCallback))
	if cfg.TestLoginSecret != "" {
		logger.Warn("TEST LOGIN ENABLED: /auth/test issues tokens without an identity provider")
		s.mux.Handle("GET /auth/test", a(s.testLogin))
	}

	s.mux.Handle("GET /api/v1/sync", api(s.getSync))
	s.mux.Handle("PUT /api/v1/sync", api(s.putSync))
	s.mux.Handle("GET /api/v1/me", api(s.getMe))
	s.mux.Handle("DELETE /api/v1/me", api(s.deleteMe))
	s.mux.Handle("POST /api/v1/session/refresh", api(s.refreshSession))
	s.mux.Handle("POST /api/v1/session/revoke", api(s.revokeSessions))

	s.mux.HandleFunc("GET /{$}", s.pageHome)
	s.mux.Handle("GET /app/", webapp.Handler())
	s.mux.Handle("GET /app", http.RedirectHandler("/app/", http.StatusMovedPermanently))
	s.mux.HandleFunc("GET /privacy", s.pagePrivacy)
	s.mux.HandleFunc("GET /terms", s.pageTerms)
	s.mux.HandleFunc("GET /account/delete", s.pageDeleteStart)
	s.mux.HandleFunc("GET /account/delete/confirm", s.pageDeleteConfirm)
	s.mux.Handle("POST /account/delete/confirm", a(s.pageDeleteSubmit))
	return s
}

// Mount adds an extra handler (the Connect API) under pattern, rate limited
// like the REST API.
func (s *Server) Mount(pattern string, h http.Handler) {
	s.mux.Handle(pattern, limit(NewRateLimiter(10, 40), h))
}

// Handler returns the fully wrapped handler.
func (s *Server) Handler() http.Handler {
	return logRequests(s.logger, securityHeaders(s.https, s.mux))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.Store.Ping(r.Context()); err != nil {
		writeError(w, http.StatusServiceUnavailable, "database unavailable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// authenticate resolves the bearer token or writes a 401.
func (s *Server) authenticate(w http.ResponseWriter, r *http.Request) (*service.Session, bool) {
	raw, err := auth.BearerToken(r.Header.Get("Authorization"))
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return nil, false
	}
	sess, err := s.svc.Authenticate(r.Context(), raw)
	if errors.Is(err, service.ErrUnauthenticated) {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return nil, false
	}
	if err != nil {
		s.internalError(w, "authenticate", err)
		return nil, false
	}
	return sess, true
}

func (s *Server) internalError(w http.ResponseWriter, op string, err error) {
	s.logger.Error("internal error", "op", op, "err", err)
	writeError(w, http.StatusInternalServerError, "internal error")
}

// --- REST v1 sync (kept wire-compatible with shipped clients) ---

type syncPayload struct {
	Data      string `json:"data"`
	History   string `json:"history"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

func (s *Server) getSync(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	st, err := s.svc.GetState(r.Context(), sess.User.ID)
	if err != nil {
		s.internalError(w, "get state", err)
		return
	}
	payload := syncPayload{
		Data:    base64.StdEncoding.EncodeToString([]byte("{}")),
		History: base64.StdEncoding.EncodeToString([]byte("{}")),
	}
	if st.Exists {
		payload.Data = base64.StdEncoding.EncodeToString(st.Data)
		payload.History = base64.StdEncoding.EncodeToString(st.History)
		payload.UpdatedAt = st.UpdatedAt.UTC().Format(time.RFC3339)
	}
	w.Header().Set("ETag", st.ETag)
	writeJSON(w, http.StatusOK, payload)
}

func (s *Server) putSync(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, (service.MaxDataBytes+service.MaxHistoryBytes)*4/3+1024)
	var req syncPayload
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	data, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid data encoding")
		return
	}
	history, err := base64.StdEncoding.DecodeString(req.History)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid history encoding")
		return
	}
	pre := service.Precondition{
		IfMatch:    r.Header.Get("If-Match"),
		CreateOnly: r.Header.Get("If-None-Match") == "*",
	}
	st, err := s.svc.PutState(r.Context(), sess.User.ID, data, history, pre)
	switch {
	case errors.Is(err, service.ErrPreconditionFailed):
		writeError(w, http.StatusPreconditionFailed, "sync precondition failed")
		return
	case errors.Is(err, service.ErrInvalidArgument):
		writeError(w, http.StatusBadRequest, err.Error())
		return
	case errors.Is(err, service.ErrUnauthenticated):
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	case err != nil:
		s.internalError(w, "put state", err)
		return
	}
	w.Header().Set("ETag", st.ETag)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// --- account & session ---

type meResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email,omitempty"`
	Provider  string `json:"provider"`
	CreatedAt string `json:"created_at"`
	// TokenExpiresAt and RefreshRecommended let clients renew in time.
	TokenExpiresAt     string `json:"token_expires_at,omitempty"`
	RefreshRecommended bool   `json:"refresh_recommended"`
}

func (s *Server) getMe(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	resp := meResponse{
		ID: sess.User.ID, Email: sess.User.Email, Provider: sess.User.Provider,
		CreatedAt:          sess.User.CreatedAt.UTC().Format(time.RFC3339),
		RefreshRecommended: !sess.IssuedAt.IsZero() && time.Since(sess.IssuedAt) > auth.RefreshAfter,
	}
	if !sess.Expires.IsZero() {
		resp.TokenExpiresAt = sess.Expires.UTC().Format(time.RFC3339)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) deleteMe(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	if err := s.svc.DeleteAccount(r.Context(), sess.User.ID); err != nil {
		s.internalError(w, "delete account", err)
		return
	}
	s.logger.Info("account deleted", "via", "api")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) refreshSession(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	tok, err := s.svc.Refresh(sess)
	if err != nil {
		s.internalError(w, "refresh", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": tok.Token, "expires_at": tok.Expires.UTC().Format(time.RFC3339)})
}

func (s *Server) revokeSessions(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.authenticate(w, r)
	if !ok {
		return
	}
	if err := s.svc.RevokeAllSessions(r.Context(), sess.User.ID); err != nil {
		s.internalError(w, "revoke", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
