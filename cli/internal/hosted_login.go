package internal

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type HostedLoginOptions struct {
	APIURL      string
	Provider    string
	OpenBrowser func(string) error
	Timeout     time.Duration
}

func BuildHostedLoginURL(apiURL, provider, redirectURI string) (string, error) {
	apiURL = NormalizeServerURL(apiURL)
	if apiURL == "" {
		apiURL = DefaultHostedAPIURL
	}
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider != "google" && provider != "facebook" {
		return "", errors.New("provider must be google or facebook")
	}
	u, err := url.Parse(apiURL + "/auth/" + provider)
	if err != nil {
		return "", err
	}
	q := u.Query()
	if strings.TrimSpace(redirectURI) != "" {
		q.Set("redirect_uri", redirectURI)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func RunHostedLogin(ctx context.Context, opts HostedLoginOptions) (string, error) {
	provider := strings.ToLower(strings.TrimSpace(opts.Provider))
	if provider == "" {
		provider = "facebook"
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer listener.Close()

	redirectURI := "http://" + listener.Addr().String() + "/callback"
	authURL, err := BuildHostedLoginURL(opts.APIURL, provider, redirectURI)
	if err != nil {
		return "", err
	}

	tokenCh := make(chan string, 1)
	errCh := make(chan error, 1)
	server := &http.Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if errText := strings.TrimSpace(r.URL.Query().Get("error")); errText != "" {
			http.Error(w, "daily-tasks login failed", http.StatusBadRequest)
			select {
			case errCh <- fmt.Errorf("hosted login failed: %s", errText):
			default:
			}
			return
		}
		token := strings.TrimSpace(r.URL.Query().Get("token"))
		if token == "" {
			http.Error(w, "daily-tasks login did not return a token", http.StatusBadRequest)
			select {
			case errCh <- errors.New("hosted login did not return a token"):
			default:
			}
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<html><body><h1>daily-tasks login complete</h1><p>You can close this window.</p></body></html>"))
		select {
		case tokenCh <- token:
		default:
		}
	})
	server.Handler = mux

	serveDone := make(chan struct{})
	go func() {
		defer close(serveDone)
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			select {
			case errCh <- err:
			default:
			}
		}
	}()
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
		<-serveDone
	}()

	open := opts.OpenBrowser
	if open == nil {
		return "", errors.New("hosted login browser opener is required")
	}
	if err := open(authURL); err != nil {
		return "", err
	}

	select {
	case token := <-tokenCh:
		return token, nil
	case err := <-errCh:
		return "", err
	case <-ctx.Done():
		return "", errors.New("hosted login timed out waiting for browser callback")
	}
}
