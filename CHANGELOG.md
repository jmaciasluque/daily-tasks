# Changelog

All notable changes to this project will be documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
Versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html):
- **Patch** (`0.0.x`): bug fixes, dependency bumps, docs
- **Minor** (`0.x.0`): new features, backwards-compatible
- **Major** (`x.0.0`): breaking changes to data format, CLI interface, or sync protocol

---

## [0.17.0] - 2026-05-27

### Added
- Every CLI command now pulls the latest data from the server before reading
  the local file (`PullDataIfRemoteNewer`). This means `done`, `todo`, `list`,
  `stats`, `add`, `edit`, etc. always see changes made via the mobile app,
  without waiting for the desktop client to sync down.
- `PullDataIfRemoteNewer` is read-only (WebDAV GET only, never PUT) — safe
  alongside the desktop client and won't create conflicted copies.

## [0.16.0] - 2026-05-27

### Added
- Completely rewritten `daily-tasks stats` output: heatmap, streaks, weak spots, trends
- Calendar heatmap with shaded per-day completion blocks and weekly rate
- Streak tracking: current and longest streaks per task with visual indicators (🔥 💪 👍 💀)
- Week-over-week trend comparison with direction arrows
- Weak spots section: tasks sorted by lowest completion rate
- Per-task breakdown table with done/skip/rate/streak columns
- Terminal coloring via lipgloss (green/red/yellow thresholds)
- New internal functions: `ComputeTaskStreaks`, `ComputeWeeklyTrend`, `WeekdayLabel`,
  `DayDot`, `DayBlock`
- Test coverage: 6 new tests for streak and trend computation

### Changed
- `TaskStreak`, `WeekStats`, `WeeklyTrend` types added to internal package

## [0.15.0] - 2026-05-27

### Added
- New `daily-tasks edit <id>` command to update fields on existing tasks
- Supports `--title`, `--duration`, `--deadline`, `--visibility`, and `--status` flags
- Only flags explicitly provided are applied; uses `--visibility ""` to clear day restrictions
- Includes validation for all updatable fields

Closes #CLI_EDIT.

### Added
- Mobile hosted backend support with Google/Facebook OAuth via Expo auth session
  browser flow, secure JWT storage via `expo-secure-store`, and hosted
  `GET/PUT /api/v1/sync` integration using bearer-token auth.
- Hosted backend config persistence for mobile using the shared snake_case config
  schema while keeping JWTs out of AsyncStorage.
- Settings UI for hosted sign-in, connected account display, and sign-out.
- Server OAuth login support for mobile `redirect_uri` callbacks that return the
  hosted JWT and account email to the app deep link.
- Full CLI hosted OAuth login flow: `daily-tasks login` opens the hosted
  Google/Facebook OAuth flow, listens on a random localhost callback port, and
  stores the returned JWT with existing `0600` token-file permissions.
- Hosted login URLs now carry a validated loopback `redirect_uri` for CLI
  callbacks.

### Changed
- Server OAuth state now preserves an optional validated redirect target and
  redirects successful logins back to loopback/deep-link clients with the
  issued JWT and optional email.

Closes #99, #100.

---

## [0.13.0] - 2026-05-25

### Added
- CLI hosted backend config support with `backend: hosted` and configurable
  `hosted.api_url` defaulting to the Fly.io API.
- Hosted HTTP sync backend for `GET/PUT /api/v1/sync` using bearer-token auth,
  server ETags/precondition headers, and base64 payload translation into the
  existing `Backend` abstraction.
- `daily-tasks login --token <jwt>` and `daily-tasks logout` commands for storing
  and deleting hosted JWTs with `0600` token-file permissions.

### Changed
- CLI and TUI remote sync/push now load the configured remote backend instead of
  being hard-wired to WebDAV/Nextcloud.

Refs #99.

---

## [0.12.0] - 2026-05-25

### Added
- Hosted backend API server under `server/` with Google/Facebook OAuth entry points,
  JWT-protected sync endpoints, encrypted Postgres storage, Docker build, and Fly.io
  deployment configuration.
- GitHub Actions workflow to deploy the server to Fly.io on `main` changes under
  `server/**`.

### Changed
- Fly.io server config keeps one machine warm for the single-instance OAuth flow.

### Security
- Added random one-time OAuth state values, HTTP server timeouts, sync request body
  size limits, and `pgcrypto` migration setup for UUID generation.

Closes #98.

---

## [0.11.4] - 2026-05-22

### Added
- Data file schema versioning: `version` field added to `Data` (CLI Go struct,
  mobile TypeScript type, and `schema.json`).
- Migration framework: `migrateData` in Go (`cli/internal/data.go`) and
  `migrateData` in TypeScript (`mobile/src/services/data.ts`) apply incremental
  version steps on every load.
- v0 → v1 migration: existing files without a `version` field (v0) are
  automatically stamped to `version: 1` on first load — no data transformation
  required (all v0 fields are structurally compatible with v1).
- Tests: `TestMigrateData_V0ToV1`, `TestMigrateData_AlreadyAtCurrentVersion`,
  `TestLoadData_V0FileGetsVersionStamped`, `TestSaveData_WritesVersion`,
  `TestSchemaVersionIsOne`.

Closes #10.

---

## [0.11.3] - 2026-05-22

### Testing

#### Added
- CLI: new `commands_test.go` covering `parseVisibility`, `parseDeadline`, `parseID`,
  `wantsHelp`, `parseStatsPeriod`, `resolveStatsRange`, and CLI runners (`runList`,
  `runAdd`, `runMove`, `runSkip`, `runDelete`) with temp-file fixtures.
- CLI internal: tests for `IsVisibleToday`, `HistoryWithCurrentSnapshot`,
  `MergeHistories`, `EnsureHistorySnapshot`, `BuildStats`, `FetchRemoteHistory`,
  `PushRemoteHistory`, `PushRemoteState`, `SyncRemoteHistory`, `SyncStateWithRemote`.
- CLI total coverage: 41% → 60.2% (`daily-tasks/internal`: 63.5% → 75.5%).

---

## [0.11.2] - 2026-05-22

### Testing

#### Added
- Mobile: 44 new tests covering `useTaskData` hook (full lifecycle), `notificationTasks`
  background task callback and registration branches, and `storage` cache/config paths.
- Overall mobile statement coverage: 65.5% → 83.4%; branch coverage: 52.1% → 62.8%.
- Installed `react-test-renderer@19.1.0` as a dev dependency for hook testing.

---

## [0.11.1] - 2026-05-22

### CI

#### Changed
- Updated `actions/checkout` v4 → v6 and `actions/setup-node` v4 → v6 in all
  workflows to eliminate Node.js 20 deprecation warnings.
- Bumped Node.js runtime in mobile/EAS workflows from 20 → 22 (LTS).

---

## [0.11.0] - 2026-05-21

### Mobile

#### Added
- Todo list now groups deadline tasks under **AM** and **PM** section
  dividers, matching the CLI behaviour. Tasks with a deadline before noon
  appear under `── AM ──`; tasks with a deadline from noon onward appear
  under `── PM ──`. Tasks without a deadline continue to appear after the
  deadline groups, unchanged.

---

## [0.10.2] - 2026-05-21

### Mobile

#### Fixed
- Nextcloud WebDAV sync now normalizes compressed response ETags before sending
  `If-Match`, fixing repeated `412` conflicts when the server reports
  ETags with a `-gzip` suffix.

## [0.10.1] - 2026-05-20

### Mobile

#### Fixed
- Background notification action sync now uses the same single-flight queue as
  foreground sync, avoiding self-inflicted WebDAV ETag races.
- Mobile data sync now retries repeated ETag moves before surfacing a calm
  conflict message instead of the raw low-level ETag error.

## [0.10.0] - 2026-05-18

### CLI

#### Added
- Added a first-run Bubble Tea setup flow that requires choosing either
  local-only storage or Nextcloud before the normal TUI starts.
- The `daily-tasks setup` command now reopens the same setup TUI so backend
  configuration can be changed later.

## [0.9.8] - 2026-05-17

### CI

#### Added
- CLI workflow now reports the repository version, test pass/fail counts, total
  coverage, and a workflow-run link in the job output and PR summary comment.

## [0.9.7] - 2026-05-17

### CLI

#### Added
- Added regression tests covering weekday visibility filtering, deadline fields
  in task history, detailed history events, and daily reset behavior for
  visible and hidden tasks.

## [0.9.6] - 2026-05-15

### Mobile

#### Fixed
- Mobile launch, manual, and autosave sync now actually use the single-flight
  queue, preventing overlapping requests from racing their own WebDAV ETags.

## [0.9.5] - 2026-05-14

### All platforms

#### Fixed
- Tasks hidden by weekday visibility can now still be managed before their next
  visible day. Mobile and web add an All view for editing or deleting hidden
  tasks, and the CLI adds a today/all view toggle.

## [0.9.4] - 2026-05-12

### Mobile

#### Fixed
- Mobile deadline reminders now respect task weekday visibility. Tasks that
  only appear on selected weekdays use weekly notification triggers for those
  days instead of daily triggers that can fire while the task is hidden.

## [0.9.3] - 2026-05-11

### Mobile

#### Fixed
- Mobile Nextcloud sync now runs through a single-flight queue. Manual sync,
  launch sync, and auto-save requests no longer overlap with each other; extra
  requests made while a sync is running are coalesced into one follow-up sync
  using the latest local task state.

## [0.9.2] - 2026-05-11

### Mobile

#### Fixed
- Repeated Nextcloud etag races during mobile sync now defer the push and keep
  local changes on the device instead of surfacing a persistent raw
  `Remote etag changed since last fetch` error. History-file etag races now
  show a calmer retry message instead of exposing the low-level etag wording.

## [0.9.1] - 2026-05-06

### All platforms

#### Fixed
- Daily reset now only moves tasks visible today back to `todo`. Tasks hidden by
  weekday visibility keep their current status and order until a day when they
  are visible, preventing off-day tasks from creating misleading reset history.

## [0.9.0] - 2026-05-05

### Web

#### Added
- Added backend configuration to the React web app's Config modal. The web UI
  now shows the selected backend, local data/config paths, safe Nextcloud
  connection details, and can switch to local-only storage or start and finish
  the Nextcloud Login Flow through the local CLI server.

#### Changed
- The web Sync button now reflects local-only installations instead of sending a
  remote sync request when no Nextcloud backend is active.

## [0.8.2] - 2026-05-05

### Docs

#### Changed
- Cleaned up `BACKLOG.md` so already-shipped backend abstraction, task schema,
  history, visibility, CLI deadline grouping, web scaffold, sync semantics, and
  test coverage work is marked complete. Kept unresolved follow-up work open and
  narrowed stale umbrella titles to the remaining scope.

## [0.8.1] - 2026-05-05

### Mobile

#### Changed
- Replaced the main-screen `Backend` action with `Config` and moved backend,
  sync status, build, update, and test-build details into that configuration
  sheet.
- Removed the main-screen test notification button.

## [0.8.0] - 2026-04-26

### CLI + Mobile

#### Changed
- Extracted a `Backend` abstraction (CLI: `cli/internal/backend.go`,
  Mobile: `mobile/src/services/backend.ts`) with two operations —
  `Fetch(key)` and `Push(key, bytes, ifMatch)` — plus shared sentinels
  (`ErrRemoteNotFound`/`EtagMismatchError`, `IfNoneMatchAny`) and
  well-known keys (`KeyData`, `KeyHistory`). The previous WebDAV-specific
  `FetchRemoteData` / `PushRemoteData` / etc. now live in `sync.go` /
  `sync.ts` and operate on any `Backend`. The WebDAV implementation
  (`WebDAVBackend`) lives in `backend_webdav.go` / `backend_webdav.ts`
  and translates the well-known keys to its own URL scheme. The
  Nextcloud login flow helpers (`startNextcloudLogin`, `pollNextcloudLogin`)
  ride alongside the WebDAV backend since they're auth setup, not data
  plane.
- `LoadWebDAVSettings` → `LoadWebDAVBackend` (CLI). Mobile gains a
  `backendFromConfig(config)` factory in `storage.ts` that returns a
  ready-to-use `Backend` or `null` when setup is incomplete.
- `HasWebDAVConfig` → `HasBackendConfig` to match the new naming.
- The Nextcloud-desktop-client detection (`LocalPathInNextcloudSyncFolder`
  + `ErrWebDAVHandledByDesktopClient`) moved to a dedicated
  `nextcloud_desktop.go` since it's a Nextcloud-with-desktop-client
  quirk that lives outside the `Backend` abstraction.

No behavior change. No config migrations. Existing Nextcloud setups
continue to work as-is. The wire format (JSON bytes, etag headers,
`If-Match` / `If-None-Match: *`) is unchanged.

This is groundwork for adding additional backends (generic WebDAV,
Google Drive, Dropbox, our own server) in subsequent releases without
having to retouch the sync logic.

---

## [0.7.4] - 2026-04-26

### Mobile

#### Fixed
- A pure-pull sync that only failed on the secondary history merge no longer
  swallows the (already successful) data pull. Previously `syncWithRemoteState`
  let an `EtagMismatchError` from `syncRemoteHistory` propagate past the
  point where the data result would have been applied to the UI, so the
  user saw a "Sync error" footer and the screen stayed on stale state even
  though the WebDAV `GET` had succeeded. The history merge is now caught
  separately: data is applied as before, the message becomes "...; history
  merge deferred", and the next sync attempt reconciles the history file.
- The auto-save that fires after every mobile edit (mark done, skip, edit,
  delete, reorder) and after every notification action (Mark Done /
  Skip from a system notification) now goes through the etag-aware
  `syncWithRemoteState` path instead of the unconditional `pushRemoteState`
  overwrite. Concurrent writers — another mobile/CLI client, the Nextcloud
  desktop client uploading the laptop's copy — no longer get silently
  clobbered by a per-edit blind `PUT`. The dedicated "Save to Nextcloud"
  flow is unchanged and keeps `pushRemoteState`'s explicit overwrite
  semantics.

---

## [0.7.3] - 2026-04-22

### CLI

#### Changed
- WebDAV `PUT` requests are now etag-aware. `FetchRemoteData` /
  `FetchRemoteHistory` capture the response `ETag`, and `PushRemoteData` /
  `PushRemoteHistory` accept an `ifMatch` argument that translates to either
  `If-Match: <etag>` (concurrency check) or `If-None-Match: *` (create-only).
  A `412 Precondition Failed` from the server is mapped to a sentinel
  `ErrEtagMismatch`, which `SyncWithRemote` / `SyncRemoteHistory` catch and
  recover from by re-fetching, re-merging, and retrying the push once.
  Catches the case where another writer (another client, another machine, the
  Nextcloud desktop client) raced our PUT — instead of silently clobbering or
  emitting a conflicted-copy file, the second writer pulls the new state and
  reconciles. `PushRemoteState` (the dedicated "push" command) intentionally
  retains its unconditional overwrite semantics.

### Mobile

#### Changed
- `fetchRemoteData` / `fetchRemoteHistory` now return `{ data, etag }` /
  `{ history, etag }` so callers can pass the captured etag back on the next
  `PUT`. `pushRemoteData` / `pushRemoteHistory` accept an optional `ifMatch`
  string (or the `IF_NONE_MATCH_ANY` sentinel for create-only writes); on
  `412 Precondition Failed` they throw a typed `EtagMismatchError`.
  `syncWithRemote` and `syncRemoteHistory` catch that error, refetch, and
  retry once — same convergence behavior as the CLI.

---

## [0.7.2] - 2026-04-22

### CLI

#### Fixed
- Stop creating Nextcloud "conflicted copy" files when the data file lives
  inside a folder watched by the Nextcloud desktop client. Previously every
  save triggered two concurrent writes against the same server file — one
  from the desktop client uploading the local file, one from the CLI's
  WebDAV `PUT` — racing each other on etag. The CLI now detects when the
  local data path resolves inside `~/Nextcloud/` and skips its WebDAV
  push/sync for that installation, letting the desktop client be the sole
  syncer. `daily-tasks sync`/`push`, the TUI sync action, and the web UI
  sync endpoint all honor this.
- `PushRemoteData` and `PushRemoteHistory` no longer restamp
  `LastModified`/`UpdatedAt` when the caller has already set them. Before
  this fix the bytes pushed via WebDAV differed from the bytes `SaveData`
  had just written to disk, which guaranteed a content-level conflict even
  when the race was won.

### Mobile

#### Fixed
- `pushRemoteHistory` now preserves a caller-supplied `updated_at` on the
  history payload instead of unconditionally restamping it with
  `Date.now()`, matching the `pushRemoteData` behavior. Keeps bytes-on-wire
  consistent with the normalized history the rest of the app reasons about.

### Notes
- If you want the CLI to drive WebDAV sync directly, set `DAILY_TASKS_PATH`
  to a location outside `~/Nextcloud/` (e.g.
  `$HOME/.config/daily-tasks/data.json`).

---

## [0.7.1] - 2026-04-17

### CLI

#### Added
- Marking a task as done from the TUI now shows a motivational footer message
  with live completion progress (done count and percentage for today's visible
  tasks), including a special all-done message at 100%.

---

## [0.7.0] - 2026-04-17

### All platforms

#### Added
- Tasks can now have a `visibility` field — an array of weekday numbers
  (0=Sun … 6=Sat) that controls which days the task appears. Omitting the
  field (or leaving it empty) means the task is visible every day.
- Visibility is editable everywhere: `--visibility` flag on `daily-tasks add`,
  a text input in the CLI TUI edit/add modal, and a day-toggle button row in
  the web and mobile task editors. Day picker starts on Monday.
- Daily history snapshots now only include tasks visible on that day, so
  completion-rate stats accurately reflect the tasks that were actually
  relevant — a Monday-only task no longer dilutes Tuesday's percentage.
- New Go helpers: `IsVisibleOn`, `IsVisibleToday`, `VisibleTasksOn`,
  `WeekdayFromDate`.
- Stats panel now includes a **Today** period button (default); shows
  completion rate and status mix for the current day only.

---

## [0.6.0] - 2026-04-15

### Mobile

#### Added
- Added a dedicated Stats tab in the mobile app with preset `7d`, `30d`,
  `90d`, and `365d` ranges plus summaries for recorded days, completion rate,
  durations, daily activity bars, and top recurring tasks

### Shared History

#### Changed
- Nextcloud sync now treats the sibling history file as shared data, merging and
  pushing it alongside the main task file so CLI, web, and mobile stats stay in
  sync instead of drifting per client

### Mobile

#### Changed
- The mobile stats view now reads from the shared synced history stream while
  still caching that history locally for offline use

### Testing

#### Added
- Added mobile tests covering history snapshots, daily reset preservation, and
  local stats aggregation

---

## [0.5.0] - 2026-04-14

### CLI

#### Added
- Added a stats screen to the interactive TUI, with a `g` toggle between tasks
  and stats plus quick range switching for `7d`, `30d`, `90d`, and `365d`
- Added terminal-native stats summaries for recorded days, completion rate,
  durations, daily activity bars, and top recurring tasks
- The TUI now shows the app version directly in the header and footer

#### Changed
- Stats refresh in-session after task mutations, reloads, sync, undo, and daily
  reset so the dashboard stays current without restarting the CLI

### Testing

#### Added
- Added CLI view tests covering stats-screen rendering, visible version labels,
  and stats-range cycling behavior

---

## [0.4.0] - 2026-04-08

### CLI

#### Added
- AM/PM section dividers in the To Do column group tasks visually by
  morning and afternoon
- Each task with a deadline now shows a live time-remaining or overdue
  indicator (e.g. "in 2h 15m", "30m ago") that updates every minute

#### Changed
- Custom list delegate renders separator items as muted divider lines
- J/K reordering and H/L column moves correctly skip separator items

### Shared History

#### Added
- Added a reversible sibling history file that records task snapshots by day and
  mutation events without changing the main shared task data file
- Daily resets now preserve the pre-reset state in history before all tasks are
  returned to `todo`, making previous periods reportable

### CLI

#### Added
- Added a `stats` command for querying historical task performance over preset
  windows or explicit date ranges
- Local saves, status changes, edits, deletes, sync merges, and undo/reset flows
  now write to the history/audit store

### Web

#### Added
- Added a dedicated Stats screen to the local web app with range filters, status
  mix pie chart, daily histogram, and per-task frequency cards
- Added a local `/api/stats` endpoint backed by the shared Go aggregation logic

### Testing

#### Added
- Added Go test coverage for history persistence, reset snapshot preservation,
  aggregation output, and the web stats endpoint

---

## [0.3.2] - 2026-04-08

### CLI

#### Fixed
- Replaced fragile string matching for WebDAV 404 detection with a sentinel
  `ErrRemoteNotFound` error and `errors.Is` checks
- Added test coverage asserting that 404 and 500 responses produce distinct
  error types

---

## [0.3.1] - 2026-04-07

### CLI

#### Fixed
- Fixed TUI layout overflow that caused the top or bottom of the screen to be
  clipped when the footer status lines were present
- Dynamically measure footer height instead of using a hardcoded offset so the
  layout adapts correctly at any terminal size

#### Changed
- Sort tasks by deadline time (chronologically) instead of insertion order;
  tasks without a deadline appear after those with one

---

## [0.3.0] - 2026-03-30

### Shared Config

#### Added
- Added a shared `config.schema.json` backend configuration model for `local`
  and `nextcloud` client setups
- Added Nextcloud Login Flow v2 support so clients can authorize in the browser
  and receive a per-client app password instead of manually entering account
  credentials

### CLI

#### Added
- Added a blocking first-run setup flow and a new `daily-tasks setup` command
  to choose a backend and connect Nextcloud when desired

#### Changed
- CLI and local web sync now read persisted backend config first, while keeping
  the legacy `DAILY_TASKS_WEBDAV_*` environment variables as a compatibility
  fallback

### Web

#### Added
- The local web app now serves a setup screen until a backend is chosen and can
  complete the Nextcloud login flow directly from the browser

### Mobile

#### Changed
- Replaced the manual Nextcloud credential form with first-run backend gating
  and a browser-based Nextcloud connect flow
- Mobile now migrates legacy manually entered WebDAV settings into the shared
  backend config schema on first load

---

## [0.2.2] - 2026-03-28

### Mobile

#### Fixed
- Android notification actions now run through an Expo background notification
  task so **Mark Done** and **Skip for Today** can update tasks directly from
  the notification without foregrounding the app
- Notification action updates now persist to local cache and push to Nextcloud,
  so action-button changes are no longer lost on the next app sync or cold
  launch
- The mobile app now creates its Android notification channel explicitly and
  includes a `Test Notif` action in the UI to trigger an immediate local
  notification for end-to-end verification
- Upgraded the mobile app to the Expo 54-compatible `expo-notifications`
  package and added `expo-task-manager` support required for background action
  handling
- Android notification action taps now dismiss the delivered notification
  immediately after **Mark Done** or **Skip for Today** succeeds

---
## [0.2.1] - 2026-03-27

### CI

#### Changed
- Removed the GitHub Actions `EAS Build (APK)` workflow so mobile client build
  jobs no longer run from repository pushes or tags

---

## [0.2.0] - 2026-03-27

### CLI

#### Added
- Added a `web` command that serves the web UI locally from the `daily-tasks`
  binary and keeps WebDAV credentials on the server side instead of in the browser

### Web

#### Changed
- Reworked the web app to use a same-origin local API for load/save/sync instead
  of storing Nextcloud settings in the browser
- The web UI now shows local-server status and Nextcloud sync configuration from
  the CLI process rather than editing WebDAV credentials directly
- Added explicit local refresh actions in the CLI (`R`) and web UI (`Refresh`)
  so both clients can reload changes that arrived through another process or a
  local Nextcloud sync

### Shared Sync

#### Fixed
- Normalized `last_modified` timestamps to milliseconds across CLI, mobile, and
  web clients so a newer file written by one client is no longer mistaken for
  an older file by another during sync

---

## [0.1.5] - 2026-03-26

### Mobile

#### Changed
- Replaced text action labels with icons: ✅ (mark done), ⏭ (skip), 📝 (edit), 🗑️ (delete)
- Skipped items now show a faded ⏭ badge to visually connect the skip action with the skipped state

---

## [0.1.4] - 2026-03-25

### Mobile

#### Changed
- Todo tasks can now be reordered by drag-and-drop — long-press the ⠿ handle on
  the left of a row to start dragging, release to drop. The ↑/↓/Top buttons have
  been removed.

---

## [0.1.3] - 2026-03-25

### Mobile

#### Changed
- Reorder buttons (Top, ↑, ↓) are now hidden for Done tasks — only Todo tasks
  can be reordered.

---

## [0.1.2] - 2026-03-25

### Mobile

#### Changed
- Todo tasks are now sorted by deadline ascending — tasks with an earlier
  deadline appear first, tasks without a deadline appear last. Done and
  skipped lists retain their manual order.

---

## [0.1.1] - 2026-03-25

### Mobile

#### Fixed
- Notification action buttons (Mark Done / Skip for Today) now work reliably on
  Android — changed `opensAppToForeground` to `true` so the app launches and
  the JS handler fires even when the app process was killed
- Added `getLastNotificationResponseAsync()` fallback on app mount to catch
  notification responses that arrived before the listener was registered (cold
  start via notification button tap)

---

## [0.1.0] - 2026-03-25

First versioned release. Captures the full feature set that has accumulated
on `main` before versioning was introduced.

### CLI

#### Added
- Three-column TUI: **To Do**, **Done**, **Skipped**
- `s` key skips the selected To Do task; skipped tasks reset to To Do the next day
- Deadline field (`HH:MM`) on tasks — shown in all columns as `⏰ HH:MM`
- `H`/`L` keys move the selected task to the adjacent column
- `tab` / `shift+tab` cycle through all three columns
- `h`/`l` keys also navigate left/right across columns
- Non-TUI subcommands: `list`, `add`, `done`, `todo`, `skip`, `delete`, `sync`, `push`
- `--deadline HH:MM` flag on the `add` subcommand
- `--status skipped` support in `list`
- `version` / `--version` / `-v` flag prints the current version
- 25 built-in colour themes, cycle with `t`
- Undo (`u`) with up to 100-step history
- Bi-directional WebDAV sync (`r` to pull/push by timestamp, `p` to force-push)
- Daily reset: all tasks return to To Do at the start of each new day

#### Fixed
- Sync no longer overwrites remote tasks when the local state is empty

### Mobile

#### Added
- Skipped task status with its own tab and daily reset
- Deadline time field in task editor; local notification fires at that time
- Notification action buttons: **Mark Done** and **Skip for Today**
- Status tabs show live task counts

---

## Versioning policy going forward

Every pull request merged to `main` must include:
1. A version bump in the root `VERSION` file
2. A new entry in this file under the appropriate heading

Bump guide:
| Change type | Example | Bump |
|---|---|---|
| Bug fix, docs, dependency | Fix sync edge case | patch |
| New feature, new command | Add `archive` status | minor |
| Breaking data/protocol change | Rename field in schema | major |
