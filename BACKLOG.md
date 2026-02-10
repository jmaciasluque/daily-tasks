# Backlog

This file mirrors GitHub issues for agent-friendly scanning. Use issues for discussion and tracking.

## Core architecture
- [ ] #5 Define multi-backend interface and sync contract (labels: backend, architecture)
- [ ] #6 Add backend registry + configuration schema (labels: backend, configuration)
- [ ] #7 Persist backend selection across clients (labels: backend, configuration, ui)
- [ ] #12 Event replay/aggregation for current state (labels: backend, architecture, dml)
- [ ] #13 Define multi-backend conflict/merge policy (labels: backend, architecture)

## Data model
- [ ] #8 Extend task schema: visibility rules, AM/PM, deadline (labels: ddl, backend)
- [ ] #9 Add history/event log schema (labels: ddl, dml, backend)
- [ ] #10 Add data/history migration + versioning (labels: backend, configuration)
- [ ] #11 Record history for all mutations (labels: dml, backend)

## Behavior
- [ ] #14 Implement visibility filters by day (labels: backend, ui)
- [ ] #15 Daily reset respects visibility rules (labels: dml, backend)

## Clients
### CLI
- [ ] #16 CLI first-run configuration TUI gating (labels: cli, ui, configuration)
- [ ] #17 Add non-TUI CLI commands (labels: cli, ui)
- [ ] #18 CLI: AM/PM grouping + deadline indicator (labels: cli, ui)

### Mobile
- [ ] #19 Mobile: backend chooser for new backends (labels: mobile, ui, configuration)
- [ ] #20 Mobile: visibility + AM/PM + deadline UI (labels: mobile, ui)

### Web
- [ ] #21 Web app scaffold with local backend (labels: web, ui, backend)
- [ ] #22 Web: backend selection UI (labels: web, ui, configuration)

## Docs & tests
- [ ] #23 Update schema/docs + migrations guide (labels: documentation, backend, configuration)
- [ ] #24 Add tests for visibility/deadline/history/reset (labels: testing, backend)
