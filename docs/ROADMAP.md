# Roadmap: from personal sideload to public Play Store app

Status as of 2026-11-09. This is the plan of record; GitHub issues carry the
detail and `BACKLOG.md` mirrors them.

## Status (2026-11-09, end of day)

| Milestone | State |
|---|---|
| M1 Emulator on Fly.io | Done in PRs #129/#130: SDK 58, `be.maclug.dailytasks`, icons, Google-first onboarding, Connect sync. Verified on the Android 16 emulator against the v2 server (`mobile/scripts/e2e-emulator.sh`). Against **production** it needs #128 deployed and a real Google sign-in. |
| M2 Public-ready backend | Done in #128 except: staging app, cost review, `CONTACT_EMAIL`, JWT secret rotation (need Fly access). |
| M3 Connect RPC API | Done: server, CLI (gRPC), mobile and web (Connect). REST v1 kept for installed apps. |
| M4 Play Store release | Pipeline ready (`release.yml`), listing copy ready. Needs: upload-key + Play service-account secrets, Play Console app entry, first manual AAB upload, screenshots. |
| M5 Remote follow-up | Done: hosted web app at `/app/` (#130); CLI `daily-tasks login` uses gRPC. |

### Starting point (before this work)

| Piece | State |
|---|---|
| Mobile (Expo SDK 54, RN 0.81) | Works on the owner's phone in **local** mode. Hosted (Fly.io) mode exists but is unproven on device. `expo-auth-session`, `expo-secure-store`, `expo-web-browser` are pinned to SDK-56 versions while `expo` is SDK 54 — a native-module mismatch. Default Expo placeholder icon. Package `com.dailytasks.app`. |
| Server (`server/`, Go) | Live at `https://daily-tasks-api.fly.dev`. Postgres, Google + Facebook OAuth, AES-GCM per-user encryption at rest, whole-blob `GET/PUT /api/v1/sync` with ETag preconditions. One always-on 256 MB machine because OAuth state lives in memory. Auto-deployed from `main` by GitHub Actions. |
| CLI (`cli/`, Go) | Supports local, Nextcloud and hosted backends. |
| Web (`web/`) | Served by the CLI on localhost only. |
| CI | Unit tests for CLI and mobile; server deploy; EAS OTA updates. No server tests in CI, no Android build, no release pipeline. |

## Decisions (2026-11-09)

| Topic | Decision | Why |
|---|---|---|
| Android application ID | `be.maclug.dailytasks` (debug/test variant: `be.maclug.dailytasks.dev`) | Permanent once on Play; must be a namespace we own. |
| Client ↔ server protocol | **Connect RPC** (protobuf contract, `connectrpc.com/connect`) | One Go handler speaks gRPC (used by the CLI), gRPC-Web and Connect-over-HTTP/JSON (used by React Native and the browser) — no Envoy/proxy, works through Fly's HTTP edge. REST v1 stays until all shipped clients have moved. |
| Sign-in | Google only at launch | Facebook Login requires Meta app review + business verification. Server endpoint stays; UI hides it. |
| Play account | Organization account (no 12-tester / 14-day closed-test gate) | — |
| Hosting | Stay on Fly.io | Already deployed and auto-deployed; tiny Go binary; per-second billing; scale-to-zero possible once state is stateless. |
| Android builds | Local Gradle (`expo prebuild`) and GitHub Actions | No EAS build quota; reproducible; release AAB signed in CI. |

## Cost model (target: < $5/month until there are real users)

- Go server on `shared-cpu-1x` / 256 MB: ~$2/month always-on, less with
  `auto_stop_machines = "stop"` + `min_machines_running = 0` (cold start ~1 s).
- Each user's data is a few KB of JSON; 10 000 users ≈ tens of MB. Storage is
  never the cost driver — the database *host* is. Keep Postgres as small as
  Fly allows (or move to SQLite + Litestream if Postgres is the dominant cost;
  see the cost-review issue).
- Horizontal scale = more stateless machines behind Fly's proxy. Requires the
  OAuth state to be signed rather than held in memory (M2).

## Milestones

### M1 — App runs on the emulator against Fly.io
1. Upgrade mobile to Expo SDK 58; align every `expo-*` module (`expo install --fix`).
2. Rename application ID to `be.maclug.dailytasks`; dev variant `.dev`.
3. Local Android build + emulator scripts (`mobile/scripts/`, Makefile targets).
4. Real icon set: launcher, adaptive (incl. monochrome), splash, notification.
5. Hosted backend as the default onboarding path (Google sign-in), local mode
   still available; Facebook hidden.
6. Verified: sign in, add/complete tasks, data visible from the CLI.

### M2 — Backend ready for strangers
1. Stateless, HMAC-signed OAuth state → enable scale-to-zero and >1 machine.
2. Account deletion (API, in-app button, public web page) — Play policy requirement.
3. Privacy policy + terms pages served by the backend.
4. Sessions that don't silently expire after 30 days (refresh/renew).
5. Rate limiting, security headers, structured request logs.
6. Staging app on Fly for CI/E2E with a test-only login.
7. Cost review of the database host; document the monthly bill.

### M3 — Connect RPC API (v2)
1. `proto/dailytasks/v1/*.proto` with `buf` lint/breaking checks + codegen (Go, TS).
2. Server: Connect handlers mounted next to REST v1 (`SyncService`, `AccountService`).
3. Mobile: Connect client (`@connectrpc/connect` + `connect-web` transport).
4. CLI: Connect client over gRPC protocol.
5. Deprecate REST v1 once no shipped client uses it.

### M4 — Play Store release
1. One CI pipeline on every PR: server (with Postgres), CLI, mobile (jest + tsc),
   web build, Android debug APK build.
2. E2E smoke test on an emulator (Maestro) against staging.
3. Upload keystore in GitHub secrets; tag `v*` → signed AAB → Play internal track.
4. Store listing: description, screenshots, feature graphic, data-safety form,
   content rating.
5. Promote internal → production.

### M5 — Follow-up from other computers
1. Hosted web UI on Fly (same Google login, same API).
2. CLI defaults to hosted backend on first run.
