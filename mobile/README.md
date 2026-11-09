# Daily Tasks Mobile (Expo / React Native)

Android app (iOS-ready) for the Daily Tasks board. Expo SDK 58, React Native
0.88, continuous native generation (`android/` is generated, never committed).

| Variant | Application ID | URL scheme | Notes |
|---|---|---|---|
| production | `be.maclug.dailytasks` | `daily-tasks://` | Play Store build, no cleartext HTTP |
| testing (dev) | `be.maclug.dailytasks.dev` | `daily-tasks-dev://` | installs next to production; cleartext allowed; dev sign-in deep link |

Select with `APP_VARIANT=production|testing` (default `production`). The
version comes from the root `VERSION` file; `versionCode` is derived from it
(`major*1e6 + minor*1e3 + patch`).

## Backends

On first launch the app asks how to store tasks:

- **Sign in with Google** (default): syncs with the hosted service at
  `https://daily-tasks-api.fly.dev` over the Connect protocol
  (`dailytasks.v1`, see `../proto`). Tasks are also visible from the CLI and
  web.
- **Use without an account**: tasks stay on the device.
- **Advanced: Nextcloud**: your own server via WebDAV (Login Flow v2).

Edits push immediately; the app pulls on launch, on returning to the
foreground and on "Sync". Conflicts are detected with etags and resolved by
`last_modified` (newest wins) with history merged.

Settings → Delete account permanently deletes the hosted account (also
available at `/account/delete` on the server).

## Develop

```bash
npm ci
npm test               # Jest (unit + in-memory Connect server)
npx tsc --noEmit       # types
npx expo start         # Metro for a dev client
```

`src/gen/` is generated from `../proto` with `make proto` at the repo root —
do not edit it by hand.

### Run on the Android emulator

APKs are built by CI (`.github/workflows/android.yml`) for every change:
download the `daily-tasks-testing-<sha>` artifact, then

```bash
adb install -r app-release.apk
```

Sign the dev build into a local server without a Google account (the server
must run with `TEST_LOGIN_SECRET`, see `../server/README.md`):

```bash
adb shell am start -a android.intent.action.VIEW \
  -d "daily-tasks-dev://dev-login?api=http://10.0.2.2:8080\&secret=local\&sub=me"
```

(`10.0.2.2` is the host machine as seen from the emulator.) This deep link is
ignored by production builds.

To build locally instead (JDK 17, Android SDK, ~8 GB of disk; see `docs/OPERATIONS.md`):

```bash
export APP_VARIANT=testing   # keep it set for Gradle too: the JS bundle embeds the variant
npx expo prebuild -p android --clean
cd android && ./gradlew assembleRelease -PreactNativeArchitectures=arm64-v8a
```

### Integration test against a running server

```bash
DAILY_TASKS_IT_URL=http://localhost:8080 DAILY_TASKS_IT_TOKEN=<token> npx jest hostedBackend.integration
```

## Release

Tag `vX.Y.Z` (equal to `VERSION`) → `.github/workflows/release.yml` builds the
signed AAB with the upload key from repository secrets, uploads it to the Play
Console internal track and attaches the APK to a GitHub release.

## Icons

All icon assets come from `scripts/generate-icons.mjs`
(`npm i --no-save sharp && node scripts/generate-icons.mjs`); Play listing art
is in `store/`.
