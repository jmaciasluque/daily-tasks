# Backlog

This file mirrors GitHub issues for agent-friendly scanning. Use issues for discussion and tracking.

## Core architecture
- [x] #5 Define multi-backend interface and sync contract (labels: backend, architecture)
- [ ] #6 Add backend registry for additional backend types (labels: backend, configuration)
- [x] #7 Persist backend selection across clients (labels: backend, configuration, ui)
- [ ] #12 Event replay/aggregation for current state (labels: backend, architecture, dml)
- [ ] #13 Define multi-backend conflict/merge policy (labels: backend, architecture)

## Data model
- [x] #8 Extend task schema: visibility rules, AM/PM, deadline (labels: ddl, backend)
- [x] #9 Add history/event log schema (labels: ddl, dml, backend)
- [ ] #10 Add data/history migration + versioning (labels: backend, configuration)
- [x] #11 Record history for all mutations (labels: dml, backend)

## Behavior
- [x] #14 Implement visibility filters by day (labels: backend, ui)
- [ ] #15 Daily reset respects visibility rules (labels: dml, backend)
- [x] #66 Add task history and stats across CLI, web, and mobile (labels: backend, cli, mobile, web, ui, testing)

## Clients
### CLI
- [ ] #16 CLI first-run configuration TUI gating (labels: cli, ui, configuration)
- [x] #17 Add non-TUI CLI commands (labels: cli, ui)
- [x] #18 CLI: AM/PM grouping + deadline indicator (labels: cli, ui)

### Mobile
- [ ] #19 Mobile: backend chooser for new backends (labels: mobile, ui, configuration)
- [ ] #20 Mobile: AM/PM grouping for deadline tasks (labels: mobile, ui)

### Web
- [x] #21 Web app scaffold with local backend (labels: web, ui, backend)
- [ ] #22 Web: backend selection UI (labels: web, ui, configuration)
- [ ] #59 QR-based Nextcloud config transfer across clients (labels: cli, mobile, web, configuration)

## Docs & tests
- [ ] #23 Update schema/docs + migrations guide (labels: documentation, backend, configuration)
- [x] #24 Add tests for visibility/deadline/history/reset (labels: testing, backend)

## Consistency & hardening
- [x] #26 Align last_modified units across clients + schema (labels: documentation, backend, ddl)
- [ ] #27 Standardize last_reset date handling (local vs UTC) (labels: documentation, backend, dml, testing)
- [x] #28 Unify sync action semantics between CLI and mobile (labels: backend, cli, mobile)
- [x] #29 Harden CLI WebDAV not-found handling (labels: backend, cli, testing)
- [ ] #30 Add remaining mobile tests for hooks and notification sync edge cases (labels: backend, mobile, testing)
