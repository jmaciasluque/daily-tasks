# AGENTS.md — Guide for AI Agents

This file tells AI coding agents how to work in this repository safely and
consistently.

---

## Repository overview

| Path | What it is |
|---|---|
| `cli/` | Go terminal UI (Bubble Tea). Entry point: `cli/main.go` |
| `cli/internal/` | Shared Go packages: data model, themes, WebDAV, version |
| `cli/commands.go` | Non-TUI subcommand handlers |
| `mobile/` | React Native / Expo mobile app |
| `mobile/src/services/` | Business logic (data, notifications, WebDAV) |
| `mobile/src/components/` | UI components |
| `schema.json` | JSON Schema for the shared data file |
| `BACKLOG.md` | Agent-readable mirror of GitHub issues |
| `CHANGELOG.md` | Release history — **must be updated on every merge to main** |

---

## Before you start

1. Read `BACKLOG.md` to understand open issues and labels.
2. Read `CHANGELOG.md` to understand what is already in the codebase.
3. Read `schema.json` before touching the data model — both clients share it.

> **IMPORTANT — every PR must include version bumps and a changelog entry.**
> Do not create a commit or open a PR without first bumping the version in
> `cli/internal/version.go` (and `mobile/app.json` if mobile changed) and
> adding an entry to `CHANGELOG.md`. See the versioning rules below.

---

## Branch naming

```
feat/<short-description>      # new feature
fix/<short-description>       # bug fix
chore/<short-description>     # refactor, deps, docs only
```

Always branch from `main`. Never commit directly to `main`.

---

## Versioning rules (mandatory on every PR)

The version lives in two places:

| File | Field |
|---|---|
| `cli/internal/version.go` | `const Version = "x.y.z"` |
| `mobile/app.json` | `"version"` and `"runtimeVersion"` (if mobile changed) |

Bump the **right** level:

| What changed | Bump |
|---|---|
| Bug fix, docs, dependency update | **patch** (`0.1.0 → 0.1.1`) |
| New feature, new command, new key binding | **minor** (`0.1.0 → 0.2.0`) |
| Breaking change to data schema, CLI interface, or sync protocol | **major** (`0.1.0 → 1.0.0`) |

Add an entry to `CHANGELOG.md` in every PR. Use the format already established
in that file (date, added/fixed/changed/removed sections).

---

## CLI development

```bash
cd cli
go build ./...          # must pass before committing
go test ./...           # run all tests
make test               # same via Makefile
make build              # produces bin/daily-tasks
```

- The CLI and the mobile app share the same JSON data file — any change to the
  data model (`cli/internal/data.go`) must stay backwards-compatible with
  `schema.json` and `mobile/src/services/data.ts`.
- `internal.Data`, `internal.Task` are the canonical Go types. Update
  `schema.json` if you add or rename fields.
- `internal.NormalizeData` is called after every load/sync — keep it safe for
  old data files that may be missing new fields.

### Key files

| File | Purpose |
|---|---|
| `cli/internal/data.go` | Data types, load/save, reset, ordering helpers |
| `cli/internal/version.go` | Single source of truth for the version string |
| `cli/internal/theme.go` | 25 built-in colour themes |
| `cli/internal/webdav.go` | WebDAV pull/push/sync logic |
| `cli/main.go` | Bubble Tea model: TUI modes, key bindings, rendering |
| `cli/commands.go` | Non-TUI subcommands and `parseDeadline` |

---

## Mobile development

```bash
cd mobile
npm test                # Jest
npx tsc --noEmit        # TypeScript type check
```

- Notifications are scheduled via `expo-notifications` in
  `mobile/src/services/notifications.ts`.
- Storage uses `@react-native-async-storage/async-storage`.
- Data sync (`mobile/src/services/data.ts`) mirrors the Go sync logic — keep
  them aligned (see backlog issue #28).

---

## Data model contract

Both clients read/write `~/.daily-tasks.json` (or a custom path via env var).
Any field you add must:

1. Be optional with a safe zero value (so old clients can read new files).
2. Be listed in `schema.json`.
3. Be handled by `internal.NormalizeData` (Go) and the equivalent JS
   normalisation logic in `mobile/src/services/data.ts`.

Current task statuses: `"todo"`, `"done"`, `"skipped"`.

---

## Tests

- **CLI**: `go test ./...` from `cli/`. Tests live in `cli/internal/*_test.go`.
- **Mobile**: `npm test` from `mobile/`. Tests live alongside source files.
- Do not merge a PR that breaks existing tests.
- Add tests for any new data-model behaviour or sync logic.

---

## Pull request checklist

- [ ] Branched from latest `main`
- [ ] `go build ./...` passes
- [ ] `go test ./...` passes (CLI)
- [ ] `npm test` passes (mobile, if touched)
- [ ] `cli/internal/version.go` bumped appropriately
- [ ] `CHANGELOG.md` entry added
- [ ] `BACKLOG.md` updated if an issue was closed or added
- [ ] `schema.json` updated if the data model changed
