package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"daily-tasks-server/internal/db"
	"daily-tasks-server/internal/handlers"
)

func main() {
	database, err := db.Open()
	if err != nil {
		log.Fatalf("db.Open: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		log.Fatalf("db.Migrate: %v", err)
	}

	srv, err := handlers.NewServer(database)
	if err != nil {
		log.Fatalf("handlers.NewServer: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", srv.Health)
	mux.HandleFunc("GET /auth/google", srv.GoogleLogin)
	mux.HandleFunc("GET /auth/google/callback", srv.GoogleCallback)
	mux.HandleFunc("GET /auth/facebook", srv.FacebookLogin)
	mux.HandleFunc("GET /auth/facebook/callback", srv.FacebookCallback)
	mux.HandleFunc("GET /api/v1/sync", srv.GetSync)
	mux.HandleFunc("PUT /api/v1/sync", srv.PutSync)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("daily-tasks server listening on :%s\n", port)
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("ListenAndServe: %v", err)
	}
}
