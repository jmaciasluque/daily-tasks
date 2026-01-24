# Daily Tasks Monorepo

A simple Trello-style daily task board with a terminal UI and mobile app, synced via Nextcloud WebDAV.

## Features

- **Daily reset**: All tasks automatically move to "To Do" at the start of each day
- **Bi-directional sync**: Both apps can push and pull changes via WebDAV
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

## Data File

Both apps read/write the same JSON file. By default:
- **CLI**: `~/Nextcloud/.daily-tasks.json`
- **Mobile**: Configured via Settings screen

See [schema.json](schema.json) for the data format specification.

## Sync via Nextcloud

1. Create an app password in Nextcloud (Settings → Security → App passwords)
2. Configure the WebDAV settings:
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
└── mobile/
    ├── App.tsx
    ├── src/
    │   ├── components/   # UI components
    │   ├── hooks/        # React hooks
    │   ├── services/     # Business logic
    │   ├── theme/        # Theme definitions
    │   └── types/        # TypeScript types
    ├── jest.config.js
    └── README.md
```

## Development

### CLI

```bash
cd cli
make test          # Run tests
make build         # Build binary
make help          # Show all commands
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
