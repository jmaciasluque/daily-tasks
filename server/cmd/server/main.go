// Command server runs the Daily Tasks sync backend.
//
// Environment:
//
//	DATABASE_URL          Postgres DSN (required unless DEV_MEMORY_STORE=1)
//	JWT_SECRET            >= 32 chars; signs session tokens and OAuth state
//	MASTER_KEY            base64 of 32 random bytes; encrypts user data at rest
//	BASE_URL              public origin, e.g. https://daily-tasks-api.fly.dev
//	GOOGLE_CLIENT_ID / GOOGLE_CLIENT_SECRET
//	FACEBOOK_APP_ID / FACEBOOK_APP_SECRET   (optional)
//	CONTACT_EMAIL         shown on the privacy / deletion pages (optional)
//	TEST_LOGIN_SECRET     enables /auth/test — staging only, never production
//	DEV_MEMORY_STORE=1    use an in-memory store (local development only)
//	PORT                  default 8080
package main

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"daily-tasks-server/internal/auth"
	"daily-tasks-server/internal/httpapi"
	"daily-tasks-server/internal/rpc"
	"daily-tasks-server/internal/service"
	"daily-tasks-server/internal/store"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	if err := run(logger); err != nil {
		logger.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var st store.Store
	if os.Getenv("DEV_MEMORY_STORE") == "1" {
		logger.Warn("using in-memory store; data is lost on restart")
		st = store.NewMemory()
	} else {
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			return errors.New("DATABASE_URL not set")
		}
		connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		pg, err := store.OpenPostgres(connectCtx, dsn)
		cancel()
		if err != nil {
			return err
		}
		defer pg.Close()
		st = pg
	}

	secret := os.Getenv("JWT_SECRET")
	if len(secret) < auth.RecommendedSecretLength {
		logger.Warn("JWT_SECRET is shorter than recommended; rotate it", "recommended_length", auth.RecommendedSecretLength)
	}
	tokens, err := auth.NewTokens(secret)
	if err != nil {
		return err
	}
	masterKey, err := base64.StdEncoding.DecodeString(os.Getenv("MASTER_KEY"))
	if err != nil {
		return fmt.Errorf("MASTER_KEY: %w", err)
	}
	svc, err := service.New(st, tokens, masterKey)
	if err != nil {
		return err
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		return errors.New("BASE_URL not set")
	}
	providers := map[string]*auth.Provider{
		"google":   auth.Google(os.Getenv("GOOGLE_CLIENT_ID"), os.Getenv("GOOGLE_CLIENT_SECRET"), baseURL+"/auth/google/callback"),
		"facebook": auth.Facebook(os.Getenv("FACEBOOK_APP_ID"), os.Getenv("FACEBOOK_APP_SECRET"), baseURL+"/auth/facebook/callback"),
	}

	api := httpapi.New(httpapi.Config{
		BaseURL:         baseURL,
		Providers:       providers,
		TestLoginSecret: os.Getenv("TEST_LOGIN_SECRET"),
		ContactEmail:    os.Getenv("CONTACT_EMAIL"),
	}, svc, auth.NewStateSigner(secret), logger)
	for pattern, handler := range rpc.Handlers(svc, logger) {
		api.Mount(pattern, handler)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	protocols := new(http.Protocols)
	protocols.SetHTTP1(true)
	// Fly's proxy talks HTTP/2 cleartext to the app (h2_backend), which gRPC
	// clients need end to end.
	protocols.SetUnencryptedHTTP2(true)
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           api.Handler(),
		Protocols:         protocols,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    32 << 10,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("listening", "port", port)
		errCh <- srv.ListenAndServe()
	}()
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
