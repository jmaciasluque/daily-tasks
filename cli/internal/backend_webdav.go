package internal

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// WebDAVBackend is the WebDAV implementation of Backend. It maps the
// well-known keys (KeyData, KeyHistory) onto a pair of URLs derived from
// a single configured data path: the history URL is the data URL with
// `.history` inserted before its extension.
type WebDAVBackend struct {
	DataURL string
	User    string
	Pass    string

	// Client is the http.Client used for requests. Zero value uses an
	// internal default with a 15s timeout.
	Client *http.Client
}

// NewWebDAVBackend builds a backend pointed at dataURL with basic-auth
// credentials. The history URL is derived automatically.
func NewWebDAVBackend(dataURL, user, pass string) *WebDAVBackend {
	return &WebDAVBackend{
		DataURL: dataURL,
		User:    user,
		Pass:    pass,
	}
}

func (b *WebDAVBackend) httpClient() *http.Client {
	if b.Client != nil {
		return b.Client
	}
	return &http.Client{Timeout: 15 * time.Second}
}

// urlFor returns the WebDAV URL for the given well-known key.
func (b *WebDAVBackend) urlFor(key string) (string, error) {
	switch key {
	case KeyData:
		return b.DataURL, nil
	case KeyHistory:
		return buildHistoryWebDAVURL(b.DataURL), nil
	default:
		return "", fmt.Errorf("webdav backend: unknown key %q", key)
	}
}

// Fetch implements Backend.
func (b *WebDAVBackend) Fetch(key string) (Blob, error) {
	target, err := b.urlFor(key)
	if err != nil {
		return Blob{}, err
	}
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return Blob{}, err
	}
	req.SetBasicAuth(b.User, b.Pass)

	resp, err := b.httpClient().Do(req)
	if err != nil {
		return Blob{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return Blob{}, ErrRemoteNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Blob{}, fmt.Errorf("remote status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Blob{}, err
	}
	return Blob{Bytes: body, Etag: resp.Header.Get("ETag")}, nil
}

// Push implements Backend.
func (b *WebDAVBackend) Push(key string, body []byte, ifMatch string) error {
	target, err := b.urlFor(key)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, target, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	setIfMatchHeader(req, ifMatch)
	req.SetBasicAuth(b.User, b.Pass)

	resp, err := b.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusPreconditionFailed {
		return ErrEtagMismatch
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("push failed with status %d", resp.StatusCode)
	}
	return nil
}

// setIfMatchHeader translates the ifMatch sentinel into the corresponding
// HTTP precondition header. "" means no precondition.
func setIfMatchHeader(req *http.Request, ifMatch string) {
	switch ifMatch {
	case "":
		// no precondition
	case IfNoneMatchAny:
		req.Header.Set("If-None-Match", "*")
	default:
		req.Header.Set("If-Match", ifMatch)
	}
}

// buildHistoryWebDAVURL derives the history-file URL from a data-file URL
// by inserting `.history` before the file's extension. Used internally by
// WebDAVBackend to map KeyHistory to a path adjacent to the data file.
func buildHistoryWebDAVURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw + ".history.json"
	}
	ext := path.Ext(parsed.Path)
	if ext == "" {
		parsed.Path += ".history.json"
	} else {
		base := strings.TrimSuffix(parsed.Path, ext)
		parsed.Path = base + ".history" + ext
	}
	return parsed.String()
}

// LoadWebDAVBackend builds a WebDAVBackend from the persisted Nextcloud
// configuration. Returns ErrNextcloudNotConfigured when the user has not
// completed setup.
func LoadWebDAVBackend() (*WebDAVBackend, error) {
	cfg, _, err := LoadEffectiveAppConfig()
	if err != nil {
		return nil, err
	}
	if cfg.Backend != BackendNextcloud || !nextcloudConfigComplete(cfg.Nextcloud) {
		return nil, ErrNextcloudNotConfigured
	}
	return NewWebDAVBackend(
		buildWebDAVURL(cfg.Nextcloud),
		cfg.Nextcloud.LoginName,
		cfg.Nextcloud.AppPassword,
	), nil
}

func LoadRemoteBackend() (Backend, error) {
	cfg, _, err := LoadEffectiveAppConfig()
	if err != nil {
		return nil, err
	}
	switch cfg.Backend {
	case BackendNextcloud:
		if !nextcloudConfigComplete(cfg.Nextcloud) {
			return nil, ErrNextcloudNotConfigured
		}
		return NewWebDAVBackend(
			buildWebDAVURL(cfg.Nextcloud),
			cfg.Nextcloud.LoginName,
			cfg.Nextcloud.AppPassword,
		), nil
	case BackendHosted:
		tokenPath, err := DefaultHostedTokenPath()
		if err != nil {
			return nil, err
		}
		token, err := LoadHostedToken(tokenPath)
		if err != nil || token == "" {
			return nil, ErrHostedTokenMissing
		}
		return NewHostedBackend(cfg.Hosted.APIURL, token), nil
	default:
		return nil, ErrBackendNotConfigured
	}
}

// HasBackendConfig reports whether a remote backend is configured and
// ready to use.
func HasBackendConfig() bool {
	_, err := LoadRemoteBackend()
	return err == nil
}

func buildWebDAVURL(cfg NextcloudConfig) string {
	base := strings.TrimRight(cfg.ServerURL, "/")
	rp := cfg.RemotePath
	if !strings.HasPrefix(rp, "/") {
		rp = "/" + rp
	}
	return base + rp
}
