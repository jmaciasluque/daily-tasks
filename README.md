# Daily Tasks Monorepo

A simple Trello-style daily task board with terminal, mobile, and locally served web clients, synced via Nextcloud WebDAV.

## Features

- **Daily reset**: All tasks automatically move to "To Do" at the start of each day
- **Bi-directional sync**: CLI, mobile, and the local web UI can sync via WebDAV
- **Conflict detection**: Uses timestamps to avoid overwriting newer changes
- **25 themes**: Cycle through built-in color themes
- **Offline-first**: Works without network, syncs when available

## Applications

### CLI (Go)

Terminal UI built with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

```bash
cd cli
go run .
```

See [cli/README.md](cli/README.md) for details.

### Mobile (React Native / Expo)

Cross-platform mobile app.

```bash
cd mobile
npm install
npx expo start
```

See [mobile/README.md](mobile/README.md) for details.

### Web (React / Vite)

Browser client served by the `daily-tasks` binary.

```bash
daily-tasks web
```

The web UI talks only to the local CLI server, which reads and writes the same
JSON file as the terminal UI and owns the Nextcloud credentials on the server
side. The browser never talks to Nextcloud directly. Use `Refresh` in the web UI
to reload the local JSON if it changed via another client or the Nextcloud desktop app.

## Data File

All clients read/write the same JSON file. By default:
- **CLI**: `~/Nextcloud/.daily-tasks.json`
- **Mobile**: Configured via Settings screen
- **Web**: Served locally by `daily-tasks web`, which uses the same path as the CLI

See [schema.json](schema.json) for the data format specification.

## Sync via Nextcloud

1. Create an app password in Nextcloud (Settings → Security → App passwords)
2. Configure the CLI / local web server environment:
   - **URL**: `https://your-nextcloud.com/remote.php/dav/files/username/.daily-tasks.json`
   - **Username**: Your Nextcloud username
   - **Password**: The app password you created

### CLI Environment Variables

```bash
export DAILY_TASKS_WEBDAV_URL="https://cloud.example.com/remote.php/dav/files/user/.daily-tasks.json"
export DAILY_TASKS_WEBDAV_USER="your-username"
export DAILY_TASKS_WEBDAV_PASS="app-password"
```

### Custom Data Path (CLI)

```bash
export DAILY_TASKS_PATH="/path/to/data.json"
```

## Project Structure

```
daily-tasks/
├── README.md
├── schema.json           # JSON Schema for data file
├── cli/
│   ├── main.go           # CLI entry point
│   ├── internal/         # Internal packages
│   │   ├── data.go       # Data operations
│   │   ├── theme.go      # Theme definitions
│   │   └── webdav.go     # WebDAV sync
│   ├── Makefile
│   └── README.md
├── mobile/
│   ├── App.tsx
│   ├── src/
│   │   ├── components/   # UI components
│   │   ├── hooks/        # React hooks
│   │   ├── services/     # Business logic
│   │   ├── theme/        # Theme definitions
│   │   └── types/        # TypeScript types
│   ├── jest.config.js
│   └── README.md
├── web/
│   ├── src/
│   │   ├── components/   # UI components
│   │   ├── hooks/        # React hooks
│   │   ├── services/     # Browser-side API client
│   │   ├── config/       # Build-time defaults
│   │   └── types/        # TypeScript types
│   └── vite.config.ts
└── cli/webui/            # Bundled web assets served by the CLI binary
```

## Development

### CLI

```bash
cd cli
make test          # Run tests
make build         # Build binary
make help          # Show all commands
```

### Web UI Development

```bash
# Terminal 1: run the local API/static server
cd cli
go run . web --open=false

# Terminal 2: frontend dev server
cd web
npm install
npm run dev
```

### Mobile

```bash
cd mobile
npm test           # Run tests
npm run test:coverage  # Run tests with coverage
npx tsc --noEmit   # Type check
```

## License

MIT
