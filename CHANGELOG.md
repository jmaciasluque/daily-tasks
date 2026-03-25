# Changelog

All notable changes to this project will be documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
Versioning follows [Semantic Versioning](https://semver.org/spec/v2.0.0.html):
- **Patch** (`0.0.x`): bug fixes, dependency bumps, docs
- **Minor** (`0.x.0`): new features, backwards-compatible
- **Major** (`x.0.0`): breaking changes to data format, CLI interface, or sync protocol

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
1. A version bump in `cli/internal/version.go` (and `mobile/app.json` if the mobile app changed)
2. A new entry in this file under the appropriate heading

Bump guide:
| Change type | Example | Bump |
|---|---|---|
| Bug fix, docs, dependency | Fix sync edge case | patch |
| New feature, new command | Add `archive` status | minor |
| Breaking data/protocol change | Rename field in schema | major |
