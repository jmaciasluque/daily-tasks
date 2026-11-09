package rpc_test

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"

	v1 "daily-tasks-server/internal/gen/dailytasks/v1"
	"daily-tasks-server/internal/gen/dailytasks/v1/dailytasksv1connect"
	"daily-tasks-server/internal/testutil"
)

func login(t *testing.T, env *testutil.Env, sub string) string {
	t.Helper()
	res, err := http.Get(env.Server.URL + "/auth/test?secret=" + testutil.TestLoginSecret + "&sub=" + sub)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var body struct{ Token string }
	json.NewDecoder(res.Body).Decode(&body)
	return body.Token
}

func bearer(token string) connect.Option {
	return connect.WithInterceptors(connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			if token != "" {
				req.Header().Set("Authorization", "Bearer "+token)
			}
			return next(ctx, req)
		}
	}))
}

// h2cClient speaks HTTP/2 without TLS, like a gRPC client behind Fly's proxy.
func h2cClient() *http.Client {
	return &http.Client{Transport: &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, addr)
		},
	}}
}

type protocol struct {
	name   string
	client func() *http.Client
	opts   []connect.ClientOption
}

var protocols = []protocol{
	{"grpc", h2cClient, []connect.ClientOption{connect.WithGRPC()}},
	{"grpc-web", defaultClient, []connect.ClientOption{connect.WithGRPCWeb()}},
	{"connect-json", defaultClient, []connect.ClientOption{connect.WithProtoJSON()}},
	{"connect-proto", defaultClient, nil},
}

func TestSyncOverEveryProtocol(t *testing.T) {
	for _, p := range protocols {
		t.Run(p.name, func(t *testing.T) {
			env := testutil.New(t)
			ctx := context.Background()
			tok := login(t, env, "rpc-"+p.name)
			sync := dailytasksv1connect.NewSyncServiceClient(p.client(), env.Server.URL, append(p.opts, bearer(tok))...)

			got, err := sync.GetState(ctx, connect.NewRequest(&v1.GetStateRequest{}))
			if err != nil || got.Msg.Exists || got.Msg.Etag != `"empty"` {
				t.Fatalf("initial GetState: %+v %v", got, err)
			}
			created, err := sync.PutState(ctx, connect.NewRequest(&v1.PutStateRequest{
				Data: []byte(`{"tasks":[]}`), History: []byte(`{}`),
				Precondition: &v1.PutStateRequest_CreateOnly{CreateOnly: true},
			}))
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			_, err = sync.PutState(ctx, connect.NewRequest(&v1.PutStateRequest{
				Data: []byte(`{}`), History: []byte(`{}`),
				Precondition: &v1.PutStateRequest_CreateOnly{CreateOnly: true},
			}))
			if connect.CodeOf(err) != connect.CodeFailedPrecondition {
				t.Fatalf("second create: %v", err)
			}
			updated, err := sync.PutState(ctx, connect.NewRequest(&v1.PutStateRequest{
				Data: []byte(`{"tasks":[1]}`), History: []byte(`{"days":[]}`),
				Precondition: &v1.PutStateRequest_IfMatch{IfMatch: created.Msg.Etag},
			}))
			if err != nil || updated.Msg.Etag == created.Msg.Etag {
				t.Fatalf("update: %+v %v", updated, err)
			}
			_, err = sync.PutState(ctx, connect.NewRequest(&v1.PutStateRequest{
				Data: []byte(`{}`), History: []byte(`{}`),
				Precondition: &v1.PutStateRequest_IfMatch{IfMatch: created.Msg.Etag},
			}))
			if connect.CodeOf(err) != connect.CodeFailedPrecondition {
				t.Fatalf("stale update: %v", err)
			}
			_, err = sync.PutState(ctx, connect.NewRequest(&v1.PutStateRequest{Data: []byte(`[]`), History: []byte(`{}`)}))
			if connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Fatalf("invalid doc: %v", err)
			}
			got, err = sync.GetState(ctx, connect.NewRequest(&v1.GetStateRequest{}))
			if err != nil || !got.Msg.Exists || string(got.Msg.Data) != `{"tasks":[1]}` || got.Msg.Etag != updated.Msg.Etag || got.Msg.UpdatedAt == nil {
				t.Fatalf("final GetState: %+v %v", got.Msg, err)
			}
		})
	}
}

func TestUnauthenticatedCallsAreRejected(t *testing.T) {
	env := testutil.New(t)
	for _, tok := range []string{"", "garbage"} {
		sync := dailytasksv1connect.NewSyncServiceClient(http.DefaultClient, env.Server.URL, bearer(tok))
		_, err := sync.GetState(context.Background(), connect.NewRequest(&v1.GetStateRequest{}))
		if connect.CodeOf(err) != connect.CodeUnauthenticated {
			t.Fatalf("token %q: %v", tok, err)
		}
	}
}

func TestAccountLifecycle(t *testing.T) {
	env := testutil.New(t)
	ctx := context.Background()
	tok := login(t, env, "lifecycle")
	acct := dailytasksv1connect.NewAccountServiceClient(h2cClient(), env.Server.URL, connect.WithGRPC(), bearer(tok))

	me, err := acct.GetAccount(ctx, connect.NewRequest(&v1.GetAccountRequest{}))
	if err != nil || me.Msg.Provider != "test" || me.Msg.TokenExpiresAt == nil || me.Msg.RefreshRecommended {
		t.Fatalf("GetAccount: %+v %v", me, err)
	}
	refreshed, err := acct.RefreshSession(ctx, connect.NewRequest(&v1.RefreshSessionRequest{}))
	if err != nil || refreshed.Msg.Token == "" {
		t.Fatalf("RefreshSession: %v", err)
	}
	if _, err := acct.RevokeAllSessions(ctx, connect.NewRequest(&v1.RevokeAllSessionsRequest{})); err != nil {
		t.Fatalf("RevokeAllSessions: %v", err)
	}
	if _, err := acct.GetAccount(ctx, connect.NewRequest(&v1.GetAccountRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("revoked token still works: %v", err)
	}

	tok = login(t, env, "lifecycle")
	acct = dailytasksv1connect.NewAccountServiceClient(http.DefaultClient, env.Server.URL, bearer(tok))
	if _, err := acct.DeleteAccount(ctx, connect.NewRequest(&v1.DeleteAccountRequest{})); err != nil {
		t.Fatalf("DeleteAccount: %v", err)
	}
	if _, err := acct.GetAccount(ctx, connect.NewRequest(&v1.GetAccountRequest{})); connect.CodeOf(err) != connect.CodeUnauthenticated {
		t.Fatalf("deleted account still works: %v", err)
	}
}

func TestReflectionIsServed(t *testing.T) {
	env := testutil.New(t)
	req, _ := http.NewRequest("POST", env.Server.URL+"/grpc.reflection.v1.ServerReflection/ServerReflectionInfo", nil)
	req.Header.Set("Content-Type", "application/grpc")
	res, err := h2cClient().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("reflection status %d", res.StatusCode)
	}
}

func defaultClient() *http.Client { return http.DefaultClient }
