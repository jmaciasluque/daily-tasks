package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

// Postgres is the production Store.
type Postgres struct {
	pool *pgxpool.Pool
}

// OpenPostgres connects, verifies the connection and applies migrations.
func OpenPostgres(ctx context.Context, dsn string) (*Postgres, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse DATABASE_URL: %w", err)
	}
	// Small instances: keep the pool tiny so many app machines can share one
	// small database without exhausting its connection limit.
	if cfg.MaxConns > 5 {
		cfg.MaxConns = 5
	}
	cfg.MaxConnIdleTime = 5 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}
	p := &Postgres{pool: pool}
	if err := p.migrate(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return p, nil
}

// Close releases the connection pool.
func (p *Postgres) Close() { p.pool.Close() }

// migrate applies every embedded migration not yet recorded, holding an
// advisory lock so concurrently booting machines don't race.
func (p *Postgres) migrate(ctx context.Context) error {
	conn, err := p.pool.Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	const lockKey = 7_215_301_119 // arbitrary, stable
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, lockKey); err != nil {
		return fmt.Errorf("migration lock: %w", err)
	}
	defer conn.Exec(context.Background(), `SELECT pg_advisory_unlock($1)`, lockKey) //nolint:errcheck

	if _, err := conn.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		name       TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return err
	}
	names, err := fs.Glob(migrationFS, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for _, name := range names {
		base := strings.TrimPrefix(name, "migrations/")
		var done bool
		if err := conn.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE name=$1)`, base).Scan(&done); err != nil {
			return err
		}
		if done {
			continue
		}
		sqlBytes, err := migrationFS.ReadFile(name)
		if err != nil {
			return err
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			tx.Rollback(ctx) //nolint:errcheck
			return fmt.Errorf("migration %s: %w", base, err)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO schema_migrations (name) VALUES ($1)`, base); err != nil {
			tx.Rollback(ctx) //nolint:errcheck
			return err
		}
		if err := tx.Commit(ctx); err != nil {
			return err
		}
	}
	return nil
}

const userColumns = `id::text, provider, sub, COALESCE(email, ''), token_version, created_at`

func scanUser(row pgx.Row) (*User, error) {
	u := &User{}
	if err := row.Scan(&u.ID, &u.Provider, &u.Sub, &u.Email, &u.TokenVersion, &u.CreatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return u, nil
}

// isInvalidUUID reports whether err is Postgres rejecting a malformed UUID,
// which for lookups simply means "no such user".
func isInvalidUUID(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22P02"
}

func (p *Postgres) UpsertUser(ctx context.Context, provider, sub, email string) (*User, error) {
	return scanUser(p.pool.QueryRow(ctx, `
		INSERT INTO users (provider, sub, email) VALUES ($1, $2, NULLIF($3, ''))
		ON CONFLICT (provider, sub) DO UPDATE SET email = COALESCE(EXCLUDED.email, users.email)
		RETURNING `+userColumns, provider, sub, email))
}

func (p *Postgres) GetUser(ctx context.Context, id string) (*User, error) {
	u, err := scanUser(p.pool.QueryRow(ctx, `SELECT `+userColumns+` FROM users WHERE id = $1`, id))
	if isInvalidUUID(err) {
		return nil, ErrNotFound
	}
	return u, err
}

func (p *Postgres) BumpTokenVersion(ctx context.Context, id string) (int, error) {
	var v int
	err := p.pool.QueryRow(ctx, `UPDATE users SET token_version = token_version + 1 WHERE id = $1 RETURNING token_version`, id).Scan(&v)
	if errors.Is(err, pgx.ErrNoRows) || isInvalidUUID(err) {
		return 0, ErrNotFound
	}
	return v, err
}

func (p *Postgres) DeleteUser(ctx context.Context, id string) error {
	tag, err := p.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id)
	if isInvalidUUID(err) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *Postgres) GetBlob(ctx context.Context, userID string) (*Blob, error) {
	b := &Blob{UserID: userID}
	err := p.pool.QueryRow(ctx, `SELECT data, history, updated_at FROM user_data WHERE user_id = $1`, userID).
		Scan(&b.Data, &b.History, &b.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) || isInvalidUUID(err) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (p *Postgres) PutBlob(ctx context.Context, userID string, data, history []byte, expect Expect) (time.Time, error) {
	var updated time.Time
	err := pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		var current time.Time
		err := tx.QueryRow(ctx, `SELECT updated_at FROM user_data WHERE user_id = $1 FOR UPDATE`, userID).Scan(&current)
		exists := err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if !expect.Any {
			if expect.Absent && exists {
				return ErrPreconditionFailed
			}
			if !expect.Absent && (!exists || !current.Equal(expect.UpdatedAt)) {
				return ErrPreconditionFailed
			}
		}
		if exists {
			// GREATEST guarantees a new etag even if two writes land in the
			// same microsecond.
			return tx.QueryRow(ctx, `
				UPDATE user_data SET data = $2, history = $3,
					updated_at = GREATEST(now(), updated_at + interval '1 microsecond')
				WHERE user_id = $1 RETURNING updated_at`, userID, data, history).Scan(&updated)
		}
		err = tx.QueryRow(ctx, `
			INSERT INTO user_data (user_id, data, history, updated_at) VALUES ($1, $2, $3, now())
			RETURNING updated_at`, userID, data, history).Scan(&updated)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505": // concurrent create won the race
				return ErrPreconditionFailed
			case "23503": // user deleted
				return ErrNotFound
			}
		}
		return err
	})
	if isInvalidUUID(err) {
		return time.Time{}, ErrNotFound
	}
	return updated, err
}

func (p *Postgres) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }
