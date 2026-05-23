package db

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
)

// User represents a row in the users table.
type User struct {
	ID        string
	Provider  string
	Sub       string
	Email     string
	CreatedAt time.Time
}

// UserData represents a row in the user_data table.
type UserData struct {
	UserID    string
	Data      []byte
	History   []byte
	UpdatedAt time.Time
}

// Open connects to Postgres using DATABASE_URL and runs migrations.
func Open() (*sql.DB, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil, fmt.Errorf("DATABASE_URL not set")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("db ping failed: %w", err)
	}
	return db, nil
}

// Migrate runs the embedded SQL migrations.
func Migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			provider   TEXT NOT NULL,
			sub        TEXT NOT NULL,
			email      TEXT,
			created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
			UNIQUE(provider, sub)
		);
		CREATE TABLE IF NOT EXISTS user_data (
			user_id    UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			data       BYTEA NOT NULL,
			history    BYTEA NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);
	`)
	return err
}

// UpsertUser inserts or returns an existing user by provider+sub.
func UpsertUser(db *sql.DB, provider, sub, email string) (*User, error) {
	u := &User{}
	err := db.QueryRow(`
		INSERT INTO users (provider, sub, email)
		VALUES ($1, $2, $3)
		ON CONFLICT (provider, sub) DO UPDATE SET email = EXCLUDED.email
		RETURNING id, provider, sub, email, created_at
	`, provider, sub, email).Scan(&u.ID, &u.Provider, &u.Sub, &u.Email, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// GetUserData retrieves the encrypted blobs for a user.
func GetUserData(db *sql.DB, userID string) (*UserData, error) {
	ud := &UserData{}
	err := db.QueryRow(`
		SELECT user_id, data, history, updated_at
		FROM user_data WHERE user_id = $1
	`, userID).Scan(&ud.UserID, &ud.Data, &ud.History, &ud.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return ud, nil
}

// PutUserData upserts the encrypted blobs for a user.
func PutUserData(db *sql.DB, userID string, data, history []byte) error {
	_, err := db.Exec(`
		INSERT INTO user_data (user_id, data, history, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (user_id) DO UPDATE
			SET data = EXCLUDED.data,
			    history = EXCLUDED.history,
			    updated_at = now()
	`, userID, data, history)
	return err
}
