# Daily Tasks TUI

Simple Trello-style daily task board in your terminal.

## Run

```bash
cd cli
go run .
```

Or build and run:

```bash
make build
./bin/daily-tasks
```

## Data File Location

By default, the data file is stored at `~/Nextcloud/.daily-tasks.json`.

To use a custom location, set the `DAILY_TASKS_PATH` environment variable:

```bash
export DAILY_TASKS_PATH="$HOME/.config/daily-tasks/data.json"
```

## Keys

| Key | Action |
|-----|--------|
| `a` | Add task (title, duration, optional deadline HH:MM) |
| `e` | Edit selected task |
| `d` | Delete selected task (asks for confirmation) |
| `s` | Skip selected To Do task → moves it to the Skipped column |
| `space` | Toggle selected task between To Do and Done |
| `J` | Move task down within its column |
| `K` | Move task up within its column |
| `H` | Move task to the left column |
| `L` | Move task to the right column |
| `h` | Focus left column |
| `l` | Focus right column |
| `tab` | Focus next column (To Do → Done → Skipped → To Do) |
| `shift+tab` | Focus previous column |
| `t` | Cycle through 25 colour themes |
| `u` | Undo last change (up to 100 steps) |
| `r` | Sync with Nextcloud (pull or push based on timestamps) |
| `p` | Force-push local data to Nextcloud |
| `q` / `ctrl+c` | Quit |

## Non-TUI Commands

You can run the CLI without the TUI using subcommands:

```bash
daily-tasks list
daily-tasks list --status todo
daily-tasks list --status skipped
daily-tasks list --status all
daily-tasks add --title "Write report" --duration 45
daily-tasks add --title "Standup" --duration 15 --deadline 09:00
daily-tasks done 3
daily-tasks skip 3
daily-tasks todo 3
daily-tasks delete 3
daily-tasks sync
daily-tasks push
daily-tasks version
```

## WebDAV Sync

To sync with Nextcloud via WebDAV, set these environment variables:

```bash
export DAILY_TASKS_WEBDAV_URL="https://cloud.example.com/remote.php/dav/files/your-username/.daily-tasks.json"
export DAILY_TASKS_WEBDAV_USER="your-username"
export DAILY_TASKS_WEBDAV_PASS="app-password"
```

Then:
- Press `r` to sync (pulls if remote is newer, pushes if local is newer)
- Press `p` to force push local changes

### Conflict Detection

The app uses a `last_modified` timestamp to detect conflicts:
- If the remote file is newer, it will be pulled
- If the local file is newer, it will be pushed
- If timestamps match, no action is taken

## Daily Reset

At startup and once per minute, the app checks the date. If the date changed, all tasks reset to "To Do" status.

## Data Format

The data file is a JSON file with this structure:

```json
{
  "last_reset": "2026-01-23",
  "next_id": 5,
  "tasks": [
    {
      "id": 1,
      "title": "Task title",
      "duration": 10,
      "status": "todo",
      "order": 1,
      "deadline": "09:00"
    }
  ],
  "theme_index": 0,
  "last_modified": 1737628800
}
```

Task statuses: `"todo"`, `"done"`, `"skipped"`. Skipped tasks reset to `"todo"` at the start of the next day, the same as done tasks.

## Development

```bash
# Run tests
make test

# Run tests with verbose output
make test-verbose

# Build binary
make build

# Format code
make fmt

# See all available commands
make help
```

## Themes

25 built-in themes are available. Press `t` to cycle through them:

Charcoal, Sand, Mint, Ocean, Ember, Mono Light, Solarized Dark, Solarized Light,
Forest, Plum, Slate, Coral, Meadow, Cobalt, Amber, Paper, Ice, Lavender, Rose,
Citrus, Steel, Redwood, Lagoon, Sunrise, Graphite
