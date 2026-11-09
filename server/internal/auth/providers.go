package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/facebook"
	"golang.org/x/oauth2/google"
)

// Identity is what we keep from an identity provider.
type Identity struct {
	Provider string
	Sub      string
	Email    string
}

// Provider is an OAuth2 identity provider.
type Provider struct {
	Name       string
	config     *oauth2.Config
	userInfo   string
	parseIdent func(body []byte) (sub, email string, err error)
}

// Enabled reports whether credentials are configured.
func (p *Provider) Enabled() bool { return p != nil && p.config.ClientID != "" && p.config.ClientSecret != "" }

// AuthCodeURL is the consent-screen URL for the given state.
func (p *Provider) AuthCodeURL(state string) string {
	return p.config.AuthCodeURL(state, oauth2.SetAuthURLParam("prompt", "select_account"))
}

// Exchange trades an authorization code for the user's identity.
func (p *Provider) Exchange(ctx context.Context, code string) (*Identity, error) {
	if code == "" {
		return nil, errors.New("missing authorization code")
	}
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("%s token exchange: %w", p.Name, err)
	}
	resp, err := p.config.Client(ctx, token).Get(p.userInfo)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s userinfo: status %d", p.Name, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil, err
	}
	sub, email, err := p.parseIdent(body)
	if err != nil {
		return nil, err
	}
	if sub == "" {
		return nil, fmt.Errorf("%s userinfo: missing subject", p.Name)
	}
	return &Identity{Provider: p.Name, Sub: sub, Email: email}, nil
}

// Google returns the Google OpenID Connect provider.
func Google(clientID, clientSecret, redirectURL string) *Provider {
	return &Provider{
		Name: "google",
		config: &oauth2.Config{
			ClientID: clientID, ClientSecret: clientSecret, RedirectURL: redirectURL,
			Scopes: []string{"openid", "email"}, Endpoint: google.Endpoint,
		},
		userInfo: "https://openidconnect.googleapis.com/v1/userinfo",
		parseIdent: func(body []byte) (string, string, error) {
			var info struct {
				Sub           string `json:"sub"`
				Email         string `json:"email"`
				EmailVerified bool   `json:"email_verified"`
			}
			if err := json.Unmarshal(body, &info); err != nil {
				return "", "", err
			}
			if !info.EmailVerified {
				info.Email = ""
			}
			return info.Sub, info.Email, nil
		},
	}
}

// Facebook returns the Facebook Login provider.
func Facebook(appID, appSecret, redirectURL string) *Provider {
	return &Provider{
		Name: "facebook",
		config: &oauth2.Config{
			ClientID: appID, ClientSecret: appSecret, RedirectURL: redirectURL,
			Scopes: []string{"email"}, Endpoint: facebook.Endpoint,
		},
		userInfo: "https://graph.facebook.com/me?fields=id,email",
		parseIdent: func(body []byte) (string, string, error) {
			var info struct {
				ID    string `json:"id"`
				Email string `json:"email"`
			}
			err := json.Unmarshal(body, &info)
			return info.ID, info.Email, err
		},
	}
}
