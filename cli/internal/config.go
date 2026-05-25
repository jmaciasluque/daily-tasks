package internal

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type BackendKind string

const (
	BackendUnconfigured BackendKind = ""
	BackendLocal        BackendKind = "local"
	BackendNextcloud    BackendKind = "nextcloud"
	BackendHosted       BackendKind = "hosted"
)

const DefaultHostedAPIURL = "https://daily-tasks-api.fly.dev"

var ErrBackendNotConfigured = errors.New("daily-tasks backend is not configured")
var ErrNextcloudNotConfigured = errors.New("nextcloud backend is not configured")

type NextcloudConfig struct {
	ServerURL   string `json:"server_url,omitempty"`
	LoginName   string `json:"login_name,omitempty"`
	AppPassword string `json:"app_password,omitempty"`
	RemotePath  string `json:"remote_path,omitempty"`
}

type HostedConfig struct {
	APIURL string `json:"api_url,omitempty"`
}

type AppConfig struct {
	Backend   BackendKind     `json:"backend,omitempty"`
	Nextcloud NextcloudConfig `json:"nextcloud,omitempty"`
	Hosted    HostedConfig    `json:"hosted,omitempty"`
}

func DefaultConfigPath() (string, error) {
	if envPath := strings.TrimSpace(os.Getenv("DAILY_TASKS_CONFIG")); envPath != "" {
		return envPath, nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "daily-tasks", "config.json"), nil
}

func DefaultRemotePath(loginName string) string {
	loginName = strings.TrimSpace(loginName)
	if loginName == "" {
		return ""
	}
	return "/remote.php/dav/files/" + url.PathEscape(loginName) + "/.daily-tasks.json"
}

func NormalizeServerURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimRight(raw, "/")
	}
	parsed.Path = ""
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/")
}

func NormalizeAppConfig(cfg AppConfig) AppConfig {
	cfg.Nextcloud.ServerURL = NormalizeServerURL(cfg.Nextcloud.ServerURL)
	cfg.Nextcloud.LoginName = strings.TrimSpace(cfg.Nextcloud.LoginName)
	cfg.Nextcloud.AppPassword = strings.TrimSpace(cfg.Nextcloud.AppPassword)
	cfg.Nextcloud.RemotePath = strings.TrimSpace(cfg.Nextcloud.RemotePath)
	cfg.Hosted.APIURL = NormalizeServerURL(cfg.Hosted.APIURL)
	if cfg.Nextcloud.RemotePath == "" && cfg.Nextcloud.LoginName != "" {
		cfg.Nextcloud.RemotePath = DefaultRemotePath(cfg.Nextcloud.LoginName)
	}

	switch cfg.Backend {
	case BackendLocal:
		cfg.Nextcloud = NextcloudConfig{}
		cfg.Hosted = HostedConfig{}
	case BackendNextcloud:
		cfg.Hosted = HostedConfig{}
		// keep nextcloud config as normalized above
	case BackendHosted:
		cfg.Nextcloud = NextcloudConfig{}
		if cfg.Hosted.APIURL == "" {
			cfg.Hosted.APIURL = DefaultHostedAPIURL
		}
	default:
		cfg.Backend = BackendUnconfigured
		cfg.Nextcloud = NextcloudConfig{}
		cfg.Hosted = HostedConfig{}
	}

	return cfg
}

func LoadAppConfig(path string) (AppConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return AppConfig{}, nil
		}
		return AppConfig{}, err
	}

	var cfg AppConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return AppConfig{}, err
	}
	return NormalizeAppConfig(cfg), nil
}

func SaveAppConfig(path string, cfg AppConfig) error {
	cfg = NormalizeAppConfig(cfg)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func LoadDefaultAppConfig() (AppConfig, string, error) {
	path, err := DefaultConfigPath()
	if err != nil {
		return AppConfig{}, "", err
	}
	cfg, err := LoadAppConfig(path)
	if err != nil {
		return AppConfig{}, path, err
	}
	return cfg, path, nil
}

func nextcloudConfigComplete(cfg NextcloudConfig) bool {
	return cfg.ServerURL != "" && cfg.LoginName != "" && cfg.AppPassword != "" && cfg.RemotePath != ""
}

func IsBackendConfigured(cfg AppConfig) bool {
	switch cfg.Backend {
	case BackendLocal:
		return true
	case BackendNextcloud:
		return nextcloudConfigComplete(cfg.Nextcloud)
	case BackendHosted:
		return strings.TrimSpace(cfg.Hosted.APIURL) != ""
	default:
		return false
	}
}

func LoadLegacyWebDAVConfigFromEnv() (AppConfig, bool) {
	rawURL := strings.TrimSpace(os.Getenv("DAILY_TASKS_WEBDAV_URL"))
	loginName := strings.TrimSpace(os.Getenv("DAILY_TASKS_WEBDAV_USER"))
	appPassword := strings.TrimSpace(os.Getenv("DAILY_TASKS_WEBDAV_PASS"))
	if rawURL == "" || loginName == "" || appPassword == "" {
		return AppConfig{}, false
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return AppConfig{}, false
	}

	cfg := AppConfig{
		Backend: BackendNextcloud,
		Nextcloud: NextcloudConfig{
			ServerURL:   parsed.Scheme + "://" + parsed.Host,
			LoginName:   loginName,
			AppPassword: appPassword,
			RemotePath:  parsed.EscapedPath(),
		},
	}
	if cfg.Nextcloud.RemotePath == "" {
		cfg.Nextcloud.RemotePath = DefaultRemotePath(loginName)
	}
	return NormalizeAppConfig(cfg), true
}

func LoadEffectiveAppConfig() (AppConfig, string, error) {
	cfg, path, err := LoadDefaultAppConfig()
	if err != nil {
		return AppConfig{}, path, err
	}
	if IsBackendConfigured(cfg) {
		return cfg, path, nil
	}
	if legacyCfg, ok := LoadLegacyWebDAVConfigFromEnv(); ok {
		return legacyCfg, path, nil
	}
	return cfg, path, nil
}
