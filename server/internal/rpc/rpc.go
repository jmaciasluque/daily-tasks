// Package rpc serves the Connect API (dailytasks.v1). Each handler speaks the
// gRPC, gRPC-Web and Connect protocols, so the Go CLI can use plain gRPC while
// React Native and browsers use Connect over fetch without any proxy.
package rpc

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"connectrpc.com/connect"
	"connectrpc.com/grpcreflect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"daily-tasks-server/internal/auth"
	v1 "daily-tasks-server/internal/gen/dailytasks/v1"
	"daily-tasks-server/internal/gen/dailytasks/v1/dailytasksv1connect"
	"daily-tasks-server/internal/service"
)

type sessionKey struct{}

func sessionFrom(ctx context.Context) *service.Session {
	s, _ := ctx.Value(sessionKey{}).(*service.Session)
	return s
}

// authInterceptor authenticates every call from the Authorization header.
func authInterceptor(svc *service.Service) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			raw, err := auth.BearerToken(req.Header().Get("Authorization"))
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing bearer token"))
			}
			sess, err := svc.Authenticate(ctx, raw)
			if err != nil {
				return nil, toConnectError(err)
			}
			return next(context.WithValue(ctx, sessionKey{}, sess), req)
		}
	}
}

// errorLogger logs internal errors server-side and hides their detail from
// clients.
func errorLogger(logger *slog.Logger) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			res, err := next(ctx, req)
			if err != nil && connect.CodeOf(err) == connect.CodeInternal {
				logger.Error("rpc internal error", "procedure", req.Spec().Procedure, "err", err)
				return nil, connect.NewError(connect.CodeInternal, errors.New("internal error"))
			}
			return res, err
		}
	}
}

func toConnectError(err error) error {
	switch {
	case errors.Is(err, service.ErrUnauthenticated):
		return connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	case errors.Is(err, service.ErrPreconditionFailed):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case errors.Is(err, service.ErrInvalidArgument):
		return connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewError(connect.CodeInternal, err)
}

// Handlers returns the Connect handlers keyed by their mux pattern.
func Handlers(svc *service.Service, logger *slog.Logger) map[string]http.Handler {
	opts := connect.WithHandlerOptions(
		connect.WithInterceptors(errorLogger(logger), authInterceptor(svc)),
		// Bound request bodies before they are buffered; the service enforces
		// the exact per-document limits. The 4/3 allows for the JSON codec,
		// which carries bytes fields as base64.
		connect.WithReadMaxBytes((service.MaxDataBytes+service.MaxHistoryBytes)*4/3+64<<10),
	)
	out := map[string]http.Handler{}
	path, h := dailytasksv1connect.NewSyncServiceHandler(&syncServer{svc: svc}, opts)
	out[path] = h
	path, h = dailytasksv1connect.NewAccountServiceHandler(&accountServer{svc: svc}, opts)
	out[path] = h

	reflector := grpcreflect.NewStaticReflector(dailytasksv1connect.SyncServiceName, dailytasksv1connect.AccountServiceName)
	path, h = grpcreflect.NewHandlerV1(reflector)
	out[path] = h
	path, h = grpcreflect.NewHandlerV1Alpha(reflector)
	out[path] = h
	return out
}

type syncServer struct{ svc *service.Service }

func (s *syncServer) GetState(ctx context.Context, _ *connect.Request[v1.GetStateRequest]) (*connect.Response[v1.GetStateResponse], error) {
	st, err := s.svc.GetState(ctx, sessionFrom(ctx).User.ID)
	if err != nil {
		return nil, toConnectError(err)
	}
	res := &v1.GetStateResponse{Exists: st.Exists, Data: st.Data, History: st.History, Etag: st.ETag}
	if st.Exists {
		res.UpdatedAt = timestamppb.New(st.UpdatedAt)
	}
	return connect.NewResponse(res), nil
}

func (s *syncServer) PutState(ctx context.Context, req *connect.Request[v1.PutStateRequest]) (*connect.Response[v1.PutStateResponse], error) {
	pre := service.Precondition{}
	switch p := req.Msg.GetPrecondition().(type) {
	case *v1.PutStateRequest_IfMatch:
		if p.IfMatch == "" {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("if_match must not be empty"))
		}
		pre.IfMatch = p.IfMatch
	case *v1.PutStateRequest_CreateOnly:
		pre.CreateOnly = p.CreateOnly
	}
	st, err := s.svc.PutState(ctx, sessionFrom(ctx).User.ID, req.Msg.GetData(), req.Msg.GetHistory(), pre)
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1.PutStateResponse{Etag: st.ETag, UpdatedAt: timestamppb.New(st.UpdatedAt)}), nil
}

type accountServer struct{ svc *service.Service }

func (a *accountServer) GetAccount(ctx context.Context, _ *connect.Request[v1.GetAccountRequest]) (*connect.Response[v1.GetAccountResponse], error) {
	sess := sessionFrom(ctx)
	res := &v1.GetAccountResponse{
		Id: sess.User.ID, Email: sess.User.Email, Provider: sess.User.Provider,
		CreatedAt:          timestamppb.New(sess.User.CreatedAt),
		RefreshRecommended: !sess.IssuedAt.IsZero() && a.svc.Now().Sub(sess.IssuedAt) > auth.RefreshAfter,
	}
	if !sess.Expires.IsZero() {
		res.TokenExpiresAt = timestamppb.New(sess.Expires)
	}
	return connect.NewResponse(res), nil
}

func (a *accountServer) RefreshSession(ctx context.Context, _ *connect.Request[v1.RefreshSessionRequest]) (*connect.Response[v1.RefreshSessionResponse], error) {
	tok, err := a.svc.Refresh(sessionFrom(ctx))
	if err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1.RefreshSessionResponse{Token: tok.Token, ExpiresAt: timestamppb.New(tok.Expires)}), nil
}

func (a *accountServer) RevokeAllSessions(ctx context.Context, _ *connect.Request[v1.RevokeAllSessionsRequest]) (*connect.Response[v1.RevokeAllSessionsResponse], error) {
	if err := a.svc.RevokeAllSessions(ctx, sessionFrom(ctx).User.ID); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1.RevokeAllSessionsResponse{}), nil
}

func (a *accountServer) DeleteAccount(ctx context.Context, _ *connect.Request[v1.DeleteAccountRequest]) (*connect.Response[v1.DeleteAccountResponse], error) {
	if err := a.svc.DeleteAccount(ctx, sessionFrom(ctx).User.ID); err != nil {
		return nil, toConnectError(err)
	}
	return connect.NewResponse(&v1.DeleteAccountResponse{}), nil
}
