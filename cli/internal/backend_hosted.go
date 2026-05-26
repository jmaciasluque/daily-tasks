package internal

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

var ErrHostedTokenInvalid = errors.New("hosted token is invalid or expired; run daily-tasks login again")
var ErrHostedTokenMissing = errors.New("hosted token missing; run daily-tasks login")

type HostedBackend struct {
	APIURL string
	Token  string
	Client *http.Client
}

type hostedSyncPayload struct {
	Data       string `json:"data"`
	History    string `json:"history"`
	remoteETag string
}

func NewHostedBackend(apiURL, token string) *HostedBackend {
	return &HostedBackend{
		APIURL: strings.TrimRight(strings.TrimSpace(apiURL), "/"),
		Token:  strings.TrimSpace(token),
	}
}

func (b *HostedBackend) httpClient() *http.Client {
	if b.Client != nil {
		return b.Client
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func (b *HostedBackend) syncURL() string {
	return strings.TrimRight(b.APIURL, "/") + "/api/v1/sync"
}

func (b *HostedBackend) Fetch(key string) (Blob, error) {
	payload, err := b.fetchPayload()
	if err != nil {
		return Blob{}, err
	}
	body, err := payload.bytesFor(key)
	if err != nil {
		return Blob{}, err
	}
	return Blob{Bytes: body, Etag: payload.etag()}, nil
}

func (b *HostedBackend) Push(key string, body []byte, ifMatch string) error {
	payload, err := b.fetchPayload()
	if errors.Is(err, ErrRemoteNotFound) {
		payload = hostedSyncPayload{}
	} else if err != nil {
		return err
	}
	if err := payload.setBytes(key, body); err != nil {
		return err
	}
	return b.putPayload(payload, ifMatch)
}

func (b *HostedBackend) fetchPayload() (hostedSyncPayload, error) {
	req, err := http.NewRequest(http.MethodGet, b.syncURL(), nil)
	if err != nil {
		return hostedSyncPayload{}, err
	}
	b.authorize(req)
	resp, err := b.httpClient().Do(req)
	if err != nil {
		return hostedSyncPayload{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return hostedSyncPayload{}, ErrHostedTokenInvalid
	}
	if resp.StatusCode == http.StatusNotFound {
		return hostedSyncPayload{}, ErrRemoteNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return hostedSyncPayload{}, fmt.Errorf("hosted backend status %d", resp.StatusCode)
	}
	var payload hostedSyncPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return hostedSyncPayload{}, err
	}
	payload.remoteETag = resp.Header.Get("ETag")
	return payload, nil
}

func (b *HostedBackend) putPayload(payload hostedSyncPayload, ifMatch string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, b.syncURL(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	b.authorize(req)
	req.Header.Set("Content-Type", "application/json")
	setIfMatchHeader(req, ifMatch)
	resp, err := b.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		_, _ = io.Copy(io.Discard, resp.Body)
		return ErrHostedTokenInvalid
	}
	if resp.StatusCode == http.StatusPreconditionFailed {
		return ErrEtagMismatch
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("hosted push failed with status %d", resp.StatusCode)
	}
	return nil
}

func (b *HostedBackend) authorize(req *http.Request) {
	if b.Token != "" {
		req.Header.Set("Authorization", "Bearer "+b.Token)
	}
}

func (p hostedSyncPayload) bytesFor(key string) ([]byte, error) {
	switch key {
	case KeyData:
		return decodeHostedBlob(p.Data, key)
	case KeyHistory:
		return decodeHostedBlob(p.History, key)
	default:
		return nil, fmt.Errorf("hosted backend: unknown key %q", key)
	}
}

func (p *hostedSyncPayload) setBytes(key string, body []byte) error {
	encoded := base64.StdEncoding.EncodeToString(body)
	switch key {
	case KeyData:
		p.Data = encoded
	case KeyHistory:
		p.History = encoded
	default:
		return fmt.Errorf("hosted backend: unknown key %q", key)
	}
	return nil
}

func (p hostedSyncPayload) etag() string {
	if p.remoteETag != "" {
		return p.remoteETag
	}
	sum := sha256.Sum256([]byte(p.Data + "\x00" + p.History))
	return `"hosted-` + hex.EncodeToString(sum[:]) + `"`
}

func decodeHostedBlob(encoded, key string) ([]byte, error) {
	if encoded == "" {
		return nil, ErrRemoteNotFound
	}
	body, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf("hosted backend: invalid %s payload: %w", key, err)
	}
	return body, nil
}
