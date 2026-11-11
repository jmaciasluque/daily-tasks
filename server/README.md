# Daily Tasks sync server

Go service deployed to Fly.io at `https://daily-tasks-api.fly.dev`. It signs
users in with Google, stores each user's two JSON documents (task board +
history) encrypted at rest, and serves them over:

| API | Path | Used by |
|---|---|---|
| **Connect / gRPC** `dailytasks.v1` ([proto](../proto/dailytasks/v1)) | `/dailytasks.v1.SyncService/*`, `/dailytasks.v1.AccountService/*` | CLI (gRPC), mobile & web (Connect over fetch) |
| REST v1 (legacy, kept for installed apps) | `GET/PUT /api/v1/sync`, `GET/DELETE /api/v1/me`, `POST /api/v1/session/{refresh,revoke}` | app builds ≤ 0.17 |
| OAuth | `/auth/google?redirect_uri=…` → `/auth/google/callback` | all clients |
| Web app | `/app/` (embedded build of `../web`, `npm run build:hosted`) | browsers |
| Web pages | `/`, `/privacy`, `/terms`, `/account/delete` | Play Store listing, users |
| Probes | `/health` (process), `/ready` (database) | Fly checks, deploy smoke test |

The same handler speaks gRPC, gRPC-Web and the Connect protocol, so there is
no proxy in front of it. gRPC server reflection is enabled:

```bash
buf curl --protocol grpc --list-methods https://daily-tasks-api.fly.dev
buf curl --protocol grpc -H "Authorization: Bearer $TOKEN" \
  https://daily-tasks-api.fly.dev/dailytasks.v1.AccountService/GetAccount
# Connect protocol is plain HTTP + JSON:
curl -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $TOKEN" \
  -d '{}' https://daily-tasks-api.fly.dev/dailytasks.v1.SyncService/GetState
```

## Layout

```
cmd/server         wiring, config from env, graceful shutdown, h2c
internal/store     Store interface; Postgres (pgx, embedded migrations) and in-memory
internal/service   business logic shared by REST and RPC: auth, encryption, etags
internal/auth      session JWTs (with revocable token version), signed OAuth state, providers
internal/rpc       Connect handlers + auth interceptor
internal/httpapi   REST v1, OAuth flow, public pages, middleware (rate limit, headers, logs)
internal/webapp    serves the hosted web UI from dist/ (filled by CI before deploy)
internal/gen       generated from ../proto by `make proto` (do not edit)
```

## Design notes

- **Stateless**: OAuth `state` is HMAC-signed and bound to a short-lived
  cookie, so any machine can finish a login and machines can scale to zero.
- **Sessions** last 60 days and clients renew them after 7 days, so active
  users are never logged out. Every token carries the user's
  `token_version`; "sign out everywhere" and account deletion bump/remove it,
  revoking all tokens immediately.
- **Encryption at rest**: AES-256-GCM with a per-user key derived (HKDF) from
  `MASTER_KEY`. Losing `MASTER_KEY` makes all stored data unreadable.
- **Concurrency**: whole-document writes guarded by etags
  (`"sync-<updated_at ns>"`); stale writes fail with HTTP 412 /
  `FAILED_PRECONDITION` and the client merges and retries.
- **Abuse limits**: 512 KiB of task data and 4 MiB of history per account, JSON objects only, per-IP rate limits.

## Configuration

| Variable | Notes |
|---|---|
| `DATABASE_URL` | Postgres DSN. Migrations run on boot under an advisory lock. |
| `JWT_SECRET` | ≥ 32 chars recommended (a warning is logged below that). Rotating it signs everyone out. |
| `MASTER_KEY` | `openssl rand -base64 32`. **Never rotate** without re-encrypting. |
| `BASE_URL` | Public origin, used for OAuth callbacks and pages. |
| `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET` | Google OAuth web client. |
| `FACEBOOK_APP_ID`, `FACEBOOK_APP_SECRET` | Optional; the apps no longer offer Facebook. |
| `CONTACT_EMAIL` | Shown on the privacy and deletion pages. |
| `TEST_LOGIN_SECRET` | **Staging only.** Enables `/auth/test` for CI/E2E. |
| `DEV_MEMORY_STORE=1` | Local development without Postgres. |

## Develop

```bash
# fastest: in-memory store + test login
DEV_MEMORY_STORE=1 JWT_SECRET=$(openssl rand -hex 32) MASTER_KEY=$(openssl rand -base64 32) \
  BASE_URL=http://localhost:8080 TEST_LOGIN_SECRET=local go run ./cmd/server
curl "http://localhost:8080/auth/test?secret=local&sub=me"   # → {"token": ...}

go test ./...                                    # unit + handler + gRPC tests
TEST_DATABASE_URL=postgres://… go test ./...     # also runs the Postgres store contract
```

After editing `../proto`, run `make proto` from the repository root.

## Deploy

Merges to `main` that touch `server/` run the test suite and then
`flyctl deploy` (see `.github/workflows/deploy-server.yml`), followed by a
`/ready` smoke test. Fly scales machines to zero when idle (~1 s cold start).
