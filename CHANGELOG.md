# Changelog

All notable changes to this project will be documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
Versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html):
- **Patch** (`0.0.x`): bug fixes, dependency bumps, docs
- **Minor** (`0.x.0`): new features, backwards-compatible
- **Major** (`x.0.0`): breaking changes to data format, CLI interface, or sync protocol

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
