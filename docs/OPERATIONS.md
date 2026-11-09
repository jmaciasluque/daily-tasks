# Operations runbook

## Environments

| | Production | Staging (planned) | Local |
|---|---|---|---|
| URL | `https://daily-tasks-api.fly.dev` | `https://daily-tasks-api-staging.fly.dev` | `http://localhost:8080` |
| Fly app | `daily-tasks-api` (cdg) | `daily-tasks-api-staging` | — |
| Store | Postgres (`DATABASE_URL`) | `DEV_MEMORY_STORE=1` (ephemeral, free) | either |
| Sign-in | Google | `/auth/test` (`TEST_LOGIN_SECRET`) | `/auth/test` |
| Deployed by | merge to `main` touching `server/` | manual `flyctl deploy` | `go run ./cmd/server` |

## Deploy

Automatic: `.github/workflows/deploy-server.yml` runs the server test suite,
then `flyctl deploy --remote-only`, then polls `/ready`.

Manual (from `server/`): `flyctl deploy --remote-only`.

Rollback: `flyctl releases -a daily-tasks-api` then
`flyctl deploy --image <previous image ref>`.

## Secrets (Fly)

```bash
flyctl secrets list -a daily-tasks-api
flyctl secrets set CONTACT_EMAIL=privacy@example.com -a daily-tasks-api
```

| Secret | Rotate? |
|---|---|
| `MASTER_KEY` | **Never** — encrypts all stored data. Back it up offline. |
| `JWT_SECRET` | Rotating signs everybody out. Do it once if it is < 32 chars (the server logs a warning). |
| `GOOGLE_CLIENT_ID/SECRET` | When rotated in Google Cloud Console. |
| `DATABASE_URL` | With the database. |

The Google OAuth client must list
`https://daily-tasks-api.fly.dev/auth/google/callback` as an authorised
redirect URI, and its consent screen must be **In production** (not
"Testing") so any Google account can sign in. Only `openid` + `email` scopes
are requested, which do not need Google verification.

## Staging (one-time setup)

```bash
cd server
flyctl apps create daily-tasks-api-staging
flyctl secrets set -a daily-tasks-api-staging DEV_MEMORY_STORE=1 \
  JWT_SECRET=$(openssl rand -hex 32) MASTER_KEY=$(openssl rand -base64 32) \
  BASE_URL=https://daily-tasks-api-staging.fly.dev TEST_LOGIN_SECRET=$(openssl rand -hex 16)
flyctl deploy -a daily-tasks-api-staging --remote-only
```

## Android release

1. Bump `VERSION`, update `CHANGELOG.md`, merge.
2. `git tag v$(cat VERSION) && git push origin v$(cat VERSION)`.
3. `release.yml` builds the signed AAB, uploads it to the Play internal track
   and creates a GitHub release with the APK.
4. Promote in the Play Console.

Upload key: a JKS keystore plus an env file with its passwords, kept outside
the repository and backed up in a password manager. GitHub secrets:

```bash
D=/path/to/release-keys
base64 -i $D/upload.jks | gh secret set ANDROID_UPLOAD_KEYSTORE_BASE64
gh secret set -f $D/upload.env      # sets the three password/alias secrets
gh secret set PLAY_SERVICE_ACCOUNT_JSON < play-service-account.json
```

## Emulator smoke test

Needs JDK 17, the Android SDK and ~8 GB of free disk for local builds (keep
the SDK, Gradle cache and AVDs on a roomy volume if your system disk is tight).

```bash
emulator -avd <your-avd> -no-window -no-audio &               # Android 16 + Play image recommended
mobile/scripts/e2e-emulator.sh app-release.apk                # dev APK from CI or a local build
mobile/scripts/e2e-emulator.sh app-release.apk https://daily-tasks-api-staging.fly.dev <test-secret>
```

Local build:

```bash
export APP_VARIANT=testing   # must also be set for Gradle: it bundles the JS with the app config
cd mobile && npm ci && npx expo prebuild -p android --clean
cd android && ./gradlew assembleRelease -PreactNativeArchitectures=arm64-v8a
```

## Cost

Target < $5/month until real usage. Levers, in order:

1. Machines stop when idle (`auto_stop_machines = "stop"`,
   `min_machines_running = 0`): you pay per second of uptime.
2. 256 MB shared CPU is plenty; scale out with `flyctl scale count N`
   (stateless) before scaling up.
3. The database host is the fixed cost — check it with
   `flyctl postgres list` / the Fly dashboard. Each user's data is a few KB.

Check monthly: Fly dashboard → Billing.
