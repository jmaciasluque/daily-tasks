package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type LoginFlowV2Session struct {
	ServerURL    string `json:"server_url"`
	LoginURL     string `json:"login_url"`
	PollEndpoint string `json:"poll_endpoint"`
	PollToken    string `json:"poll_token"`
}

type LoginFlowV2Result struct {
	ServerURL   string `json:"server_url"`
	LoginName   string `json:"login_name"`
	AppPassword string `json:"app_password"`
}

type loginFlowV2StartResponse struct {
	Poll struct {
		Token    string `json:"token"`
		Endpoint string `json:"endpoint"`
	} `json:"poll"`
	Login string `json:"login"`
}

type loginFlowV2PollResponse struct {
	Server      string `json:"server"`
	LoginName   string `json:"loginName"`
	AppPassword string `json:"appPassword"`
}

func StartLoginFlowV2(serverURL string) (LoginFlowV2Session, error) {
	serverURL = NormalizeServerURL(serverURL)
	if serverURL == "" {
		return LoginFlowV2Session{}, fmt.Errorf("server URL is required")
	}

	req, err := http.NewRequest(http.MethodPost, serverURL+"/index.php/login/v2", bytes.NewReader(nil))
	if err != nil {
		return LoginFlowV2Session{}, err
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return LoginFlowV2Session{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return LoginFlowV2Session{}, fmt.Errorf("login flow start failed with status %d", resp.StatusCode)
	}

	var payload loginFlowV2StartResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return LoginFlowV2Session{}, err
	}
	if payload.Login == "" || payload.Poll.Token == "" || payload.Poll.Endpoint == "" {
		return LoginFlowV2Session{}, fmt.Errorf("login flow response was incomplete")
	}

	return LoginFlowV2Session{
		ServerURL:    serverURL,
		LoginURL:     payload.Login,
		PollEndpoint: payload.Poll.Endpoint,
		PollToken:    payload.Poll.Token,
	}, nil
}

func PollLoginFlowV2(session LoginFlowV2Session) (LoginFlowV2Result, bool, error) {
	values := url.Values{}
	values.Set("token", session.PollToken)

	req, err := http.NewRequest(http.MethodPost, session.PollEndpoint, strings.NewReader(values.Encode()))
	if err != nil {
		return LoginFlowV2Result{}, false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return LoginFlowV2Result{}, false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return LoginFlowV2Result{}, false, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return LoginFlowV2Result{}, false, fmt.Errorf("login flow poll failed with status %d", resp.StatusCode)
	}

	var payload loginFlowV2PollResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return LoginFlowV2Result{}, false, err
	}
	if payload.Server == "" || payload.LoginName == "" || payload.AppPassword == "" {
		return LoginFlowV2Result{}, false, fmt.Errorf("login flow poll response was incomplete")
	}

	return LoginFlowV2Result{
		ServerURL:   NormalizeServerURL(payload.Server),
		LoginName:   strings.TrimSpace(payload.LoginName),
		AppPassword: strings.TrimSpace(payload.AppPassword),
	}, true, nil
}

func AppConfigFromLogin(result LoginFlowV2Result) AppConfig {
	return NormalizeAppConfig(AppConfig{
		Backend: BackendNextcloud,
		Nextcloud: NextcloudConfig{
			ServerURL:   result.ServerURL,
			LoginName:   result.LoginName,
			AppPassword: result.AppPassword,
			RemotePath:  DefaultRemotePath(result.LoginName),
		},
	})
}
