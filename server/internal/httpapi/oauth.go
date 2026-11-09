package httpapi

import (
	"crypto/subtle"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"daily-tasks-server/internal/auth"
)

const stateCookie = "dt_oauth_nonce"

// webSessionTTL is used when the login lands on one of our own web pages
// (e.g. account deletion): the token travels in a URL, so keep it short-lived.
const webSessionTTL = 15 * time.Minute

// allowedRedirect returns raw if it is a destination we may hand a token to:
// the mobile apps' custom schemes, a loopback HTTP listener (CLI), or a page
// on our own origin. Anything else is dropped.
func (s *Server) allowedRedirect(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	switch {
	case u.Scheme == "daily-tasks" || u.Scheme == "daily-tasks-dev":
		return raw
	case isLoopbackHTTP(u):
		return raw
	case s.cfg.BaseURL != "" && strings.HasPrefix(raw, s.cfg.BaseURL+"/"):
		return raw
	}
	return ""
}

func isLoopbackHTTP(u *url.URL) bool {
	if u.Scheme != "http" {
		return false
	}
	host := u.Hostname()
	ip := net.ParseIP(host)
	return host == "localhost" || (ip != nil && ip.IsLoopback())
}

func (s *Server) provider(w http.ResponseWriter, r *http.Request) (*auth.Provider, bool) {
	p := s.cfg.Providers[r.PathValue("provider")]
	if !p.Enabled() {
		writeError(w, http.StatusNotFound, "unknown login provider")
		return nil, false
	}
	return p, true
}

func (s *Server) oauthStart(w http.ResponseWriter, r *http.Request) {
	p, ok := s.provider(w, r)
	if !ok {
		return
	}
	state, nonce := s.state.New(s.allowedRedirect(r.URL.Query().Get("redirect_uri")))
	http.SetCookie(w, &http.Cookie{
		Name: stateCookie, Value: nonce, Path: "/auth/",
		MaxAge: int(auth.StateTTL.Seconds()), HttpOnly: true, Secure: s.https,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, p.AuthCodeURL(state), http.StatusFound)
}

func (s *Server) oauthCallback(w http.ResponseWriter, r *http.Request) {
	p, ok := s.provider(w, r)
	if !ok {
		return
	}
	nonce := ""
	if c, err := r.Cookie(stateCookie); err == nil {
		nonce = c.Value
	}
	http.SetCookie(w, &http.Cookie{Name: stateCookie, Value: "", Path: "/auth/", MaxAge: -1, HttpOnly: true, Secure: s.https, SameSite: http.SameSiteLaxMode})
	redirect, err := s.state.Verify(r.URL.Query().Get("state"), nonce)
	if err != nil {
		s.logger.Warn("oauth state rejected", "provider", p.Name, "err", err)
		s.renderMessage(w, http.StatusBadRequest, "Sign-in expired", "That sign-in link is no longer valid. Go back to the app and try again.")
		return
	}
	if errParam := r.URL.Query().Get("error"); errParam != "" {
		s.renderMessage(w, http.StatusBadRequest, "Sign-in cancelled", "Sign-in was cancelled. Go back to the app and try again.")
		return
	}
	ident, err := p.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		s.logger.Error("oauth exchange failed", "provider", p.Name, "err", err)
		s.renderMessage(w, http.StatusBadGateway, "Sign-in failed", "We could not complete sign-in with "+p.Name+". Please try again.")
		return
	}
	s.finishLogin(w, r, *ident, redirect)
}

// testLogin issues a token for a synthetic "test" identity. It is only
// routed when TEST_LOGIN_SECRET is configured (staging), never in production.
func (s *Server) testLogin(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if subtle.ConstantTimeCompare([]byte(q.Get("secret")), []byte(s.cfg.TestLoginSecret)) != 1 {
		writeError(w, http.StatusForbidden, "forbidden")
		return
	}
	sub := q.Get("sub")
	if sub == "" {
		sub = "e2e"
	}
	s.finishLogin(w, r, auth.Identity{Provider: "test", Sub: sub, Email: q.Get("email")}, s.allowedRedirect(q.Get("redirect_uri")))
}

func (s *Server) finishLogin(w http.ResponseWriter, r *http.Request, ident auth.Identity, redirect string) {
	ttl := auth.SessionTTL
	// One-off web pages (account deletion) get a short-lived token because it
	// travels in a URL; the web app (/app/) gets a normal session.
	if redirect != "" && strings.HasPrefix(redirect, s.cfg.BaseURL+"/account/") {
		ttl = webSessionTTL
	}
	user, tok, err := s.svc.Login(r.Context(), ident, ttl)
	if err != nil {
		s.internalError(w, "login", err)
		return
	}
	s.logger.Info("login", "provider", ident.Provider)
	if redirect == "" {
		writeJSON(w, http.StatusOK, map[string]string{"token": tok.Token, "email": user.Email})
		return
	}
	dest, err := url.Parse(redirect)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid redirect")
		return
	}
	q := dest.Query()
	q.Set("token", tok.Token)
	if user.Email != "" {
		q.Set("email", user.Email)
	}
	dest.RawQuery = q.Encode()
	w.Header().Set("Cache-Control", "no-store")
	http.Redirect(w, r, dest.String(), http.StatusFound)
}
