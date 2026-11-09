package internal

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"

	v1 "daily-tasks/internal/gen/dailytasks/v1"
	"daily-tasks/internal/gen/dailytasks/v1/dailytasksv1connect"
)

var ErrHostedTokenInvalid = errors.New("hosted token is invalid or expired; run daily-tasks login again")
var ErrHostedTokenMissing = errors.New("hosted token missing; run daily-tasks login")

// hostedRequestTimeout bounds every call to the hosted backend. It is
// generous because an idle Fly machine needs a moment to start.
const hostedRequestTimeout = 20 * time.Second

// HostedBackend talks to the Daily Tasks sync service over gRPC (via
// connect-go). The service stores data and history atomically under a single
// etag; this adapter exposes them as the two keys the sync layer expects.
type HostedBackend struct {
	APIURL string
	Token  string

	sync    dailytasksv1connect.SyncServiceClient
	account dailytasksv1connect.AccountServiceClient
}

// NewHostedBackend returns a gRPC client for apiURL authenticated with token.
func NewHostedBackend(apiURL, token string) *HostedBackend {
	return newHostedBackend(apiURL, token, hostedHTTPClient())
}

func newHostedBackend(apiURL, token string, client connect.HTTPClient) *HostedBackend {
	b := &HostedBackend{
		APIURL: strings.TrimRight(strings.TrimSpace(apiURL), "/"),
		Token:  strings.TrimSpace(token),
	}
	opts := []connect.ClientOption{connect.WithGRPC(), connect.WithInterceptors(b.authInterceptor())}
	b.sync = dailytasksv1connect.NewSyncServiceClient(client, b.APIURL, opts...)
	b.account = dailytasksv1connect.NewAccountServiceClient(client, b.APIURL, opts...)
	return b
}

// hostedHTTPClient speaks HTTP/2, as gRPC requires: over TLS for https URLs
// and cleartext (h2c, prior knowledge) for local http URLs.
func hostedHTTPClient() *http.Client {
	protocols := new(http.Protocols)
	protocols.SetHTTP2(true)
	protocols.SetUnencryptedHTTP2(true)
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Protocols = protocols
	return &http.Client{Transport: transport}
}

func (b *HostedBackend) authInterceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if b.Token != "" {
				req.Header().Set("Authorization", "Bearer "+b.Token)
			}
			req.Header().Set("User-Agent", "daily-tasks-cli/"+Version)
			return next(ctx, req)
		}
	}
}

func hostedError(err error) error {
	switch connect.CodeOf(err) {
	case connect.CodeUnauthenticated, connect.CodePermissionDenied:
		return ErrHostedTokenInvalid
	case connect.CodeFailedPrecondition:
		return ErrEtagMismatch
	}
	return fmt.Errorf("hosted backend: %w", err)
}

func (b *HostedBackend) getState() (*v1.GetStateResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hostedRequestTimeout)
	defer cancel()
	res, err := b.sync.GetState(ctx, connect.NewRequest(&v1.GetStateRequest{}))
	if err != nil {
		return nil, hostedError(err)
	}
	return res.Msg, nil
}

// Fetch returns the document for key and the shared etag.
func (b *HostedBackend) Fetch(key string) (Blob, error) {
	if key != KeyData && key != KeyHistory {
		return Blob{}, fmt.Errorf("hosted backend: unknown key %q", key)
	}
	st, err := b.getState()
	if err != nil {
		return Blob{}, err
	}
	if !st.GetExists() {
		return Blob{}, ErrRemoteNotFound
	}
	body := st.GetData()
	if key == KeyHistory {
		body = st.GetHistory()
	}
	if len(body) == 0 {
		return Blob{}, ErrRemoteNotFound
	}
	return Blob{Bytes: body, Etag: st.GetEtag()}, nil
}

// Push replaces one document, carrying the other one over unchanged.
func (b *HostedBackend) Push(key string, body []byte, ifMatch string) error {
	if key != KeyData && key != KeyHistory {
		return fmt.Errorf("hosted backend: unknown key %q", key)
	}
	st, err := b.getState()
	if err != nil {
		return err
	}
	data, history := st.GetData(), st.GetHistory()
	if len(data) == 0 {
		data = []byte("{}")
	}
	if len(history) == 0 {
		history = []byte("{}")
	}
	if key == KeyData {
		data = body
	} else {
		history = body
	}
	req := &v1.PutStateRequest{Data: data, History: history}
	switch ifMatch {
	case "":
	case IfNoneMatchAny:
		req.Precondition = &v1.PutStateRequest_CreateOnly{CreateOnly: true}
	default:
		req.Precondition = &v1.PutStateRequest_IfMatch{IfMatch: ifMatch}
	}
	ctx, cancel := context.WithTimeout(context.Background(), hostedRequestTimeout)
	defer cancel()
	if _, err := b.sync.PutState(ctx, connect.NewRequest(req)); err != nil {
		return hostedError(err)
	}
	return nil
}

// HostedAccount describes the signed-in account.
type HostedAccount struct {
	ID        string
	Email     string
	Provider  string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Account returns the signed-in account.
func (b *HostedBackend) Account() (*HostedAccount, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hostedRequestTimeout)
	defer cancel()
	res, err := b.account.GetAccount(ctx, connect.NewRequest(&v1.GetAccountRequest{}))
	if err != nil {
		return nil, hostedError(err)
	}
	return &HostedAccount{
		ID: res.Msg.GetId(), Email: res.Msg.GetEmail(), Provider: res.Msg.GetProvider(),
		CreatedAt: res.Msg.GetCreatedAt().AsTime(), ExpiresAt: res.Msg.GetTokenExpiresAt().AsTime(),
	}, nil
}

// RefreshToken returns a new full-length session token.
func (b *HostedBackend) RefreshToken() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hostedRequestTimeout)
	defer cancel()
	res, err := b.account.RefreshSession(ctx, connect.NewRequest(&v1.RefreshSessionRequest{}))
	if err != nil {
		return "", hostedError(err)
	}
	return res.Msg.GetToken(), nil
}

// RevokeAllSessions signs the account out everywhere.
func (b *HostedBackend) RevokeAllSessions() error {
	ctx, cancel := context.WithTimeout(context.Background(), hostedRequestTimeout)
	defer cancel()
	_, err := b.account.RevokeAllSessions(ctx, connect.NewRequest(&v1.RevokeAllSessionsRequest{}))
	if err != nil {
		return hostedError(err)
	}
	return nil
}

// DeleteAccount permanently deletes the account and its synced data.
func (b *HostedBackend) DeleteAccount() error {
	ctx, cancel := context.WithTimeout(context.Background(), hostedRequestTimeout)
	defer cancel()
	_, err := b.account.DeleteAccount(ctx, connect.NewRequest(&v1.DeleteAccountRequest{}))
	if err != nil {
		return hostedError(err)
	}
	return nil
}

// tokenRefreshAge is how old a session token may get before the CLI renews it.
const tokenRefreshAge = 7 * 24 * time.Hour

// tokenIssuedAt reads the "iat" claim without verifying the signature; it is
// only used to decide when to ask the server for a fresh token.
func tokenIssuedAt(token string) (time.Time, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		IssuedAt int64 `json:"iat"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.IssuedAt == 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.IssuedAt, 0), true
}

// RefreshHostedTokenIfStale renews the stored token once it is older than
// tokenRefreshAge, so active users are never logged out. Failures are
// ignored: the current token keeps working until it expires.
func RefreshHostedTokenIfStale(b *HostedBackend, tokenPath string, now time.Time) {
	issued, ok := tokenIssuedAt(b.Token)
	if !ok || now.Sub(issued) < tokenRefreshAge {
		return
	}
	fresh, err := b.RefreshToken()
	if err != nil || fresh == "" {
		return
	}
	if SaveHostedToken(tokenPath, fresh) == nil {
		b.Token = fresh
	}
}
