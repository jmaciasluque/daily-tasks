package httpapi

import (
	"embed"
	"errors"
	"html/template"
	"net/http"
	"net/url"

	"daily-tasks-server/internal/service"
)

//go:embed templates/*.html
var templateFS embed.FS

var pages = template.Must(template.ParseFS(templateFS, "templates/*.html"))

type pageData struct {
	Title        string
	Heading      string
	Message      string
	ContactEmail string
	BaseURL      string
	LoginURL     string
	Token        string
	Email        string
	Updated      string
}

// policyUpdated is the "last updated" date shown on the legal pages.
const policyUpdated = "9 November 2026"

func (s *Server) render(w http.ResponseWriter, status int, name string, d pageData) {
	d.ContactEmail = s.cfg.ContactEmail
	d.BaseURL = s.cfg.BaseURL
	d.Updated = policyUpdated
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src 'self' data:; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
	w.WriteHeader(status)
	if err := pages.ExecuteTemplate(w, name, d); err != nil {
		s.logger.Error("render", "page", name, "err", err)
	}
}

func (s *Server) renderMessage(w http.ResponseWriter, status int, heading, message string) {
	s.render(w, status, "message.html", pageData{Title: heading, Heading: heading, Message: message})
}

func (s *Server) pageHome(w http.ResponseWriter, _ *http.Request) {
	s.render(w, http.StatusOK, "home.html", pageData{Title: "Daily Tasks"})
}

func (s *Server) pagePrivacy(w http.ResponseWriter, _ *http.Request) {
	s.render(w, http.StatusOK, "privacy.html", pageData{Title: "Privacy policy · Daily Tasks"})
}

func (s *Server) pageTerms(w http.ResponseWriter, _ *http.Request) {
	s.render(w, http.StatusOK, "terms.html", pageData{Title: "Terms of use · Daily Tasks"})
}

func (s *Server) pageDeleteStart(w http.ResponseWriter, _ *http.Request) {
	login := s.cfg.BaseURL + "/auth/google?redirect_uri=" + url.QueryEscape(s.cfg.BaseURL+"/account/delete/confirm")
	s.render(w, http.StatusOK, "delete.html", pageData{Title: "Delete your account · Daily Tasks", LoginURL: login})
}

func (s *Server) pageDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	sess, err := s.svc.Authenticate(r.Context(), token)
	if err != nil {
		s.renderMessage(w, http.StatusUnauthorized, "Sign-in required", "Your sign-in has expired. Start again from the account deletion page.")
		return
	}
	s.render(w, http.StatusOK, "delete_confirm.html", pageData{Title: "Confirm deletion · Daily Tasks", Token: token, Email: sess.User.Email})
}

func (s *Server) pageDeleteSubmit(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8<<10)
	if err := r.ParseForm(); err != nil {
		s.renderMessage(w, http.StatusBadRequest, "Invalid request", "Start again from the account deletion page.")
		return
	}
	sess, err := s.svc.Authenticate(r.Context(), r.PostForm.Get("token"))
	if errors.Is(err, service.ErrUnauthenticated) {
		s.renderMessage(w, http.StatusUnauthorized, "Sign-in required", "Your sign-in has expired. Start again from the account deletion page.")
		return
	}
	if err != nil {
		s.internalError(w, "delete page auth", err)
		return
	}
	if err := s.svc.DeleteAccount(r.Context(), sess.User.ID); err != nil {
		s.internalError(w, "delete page", err)
		return
	}
	s.logger.Info("account deleted", "via", "web")
	s.renderMessage(w, http.StatusOK, "Account deleted", "Your Daily Tasks account and all of its synced data have been permanently deleted. Tasks stored locally on your devices are not affected; uninstall the app to remove them.")
}
