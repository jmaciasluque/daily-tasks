# Daily Tasks Mobile (Expo)

Cross-platform React Native app that reads/writes the same JSON file via Nextcloud WebDAV.

## Run

```bash
cd mobile
npm install
npx expo start
```

## Project Structure

```
mobile/
├── App.tsx                 # Main app component
├── src/
│   ├── components/         # Reusable UI components
│   │   ├── TaskRow.tsx
│   │   ├── TaskEditor.tsx
│   │   └── SettingsModal.tsx
│   ├── hooks/
│   │   └── useTaskData.ts  # Main data management hook
│   ├── services/
│   │   ├── data.ts         # Data manipulation utilities
│   │   ├── storage.ts      # AsyncStorage operations
│   │   └── webdav.ts       # WebDAV sync operations
│   ├── theme/
│   │   └── themes.ts       # Theme definitions
│   ├── types/
│   │   └── index.ts        # TypeScript types
│   └── __tests__/          # Jest tests
│       ├── data.test.ts
│       └── themes.test.ts
└── jest.config.js
```

## Backend Setup

On first launch, the app now blocks usage until you choose a backend:

- `Local only`
- `Nextcloud`

For Nextcloud, enter only the server URL and continue in the browser. The app
uses Nextcloud Login Flow v2 to obtain a per-device app password, then stores
the resulting backend config locally and syncs automatically.

## Sync Behavior

- **On launch**: If settings are configured, syncs automatically
- **On edit**: Pushes changes to remote immediately
- **Manual sync**: Tap "Sync" button to pull latest changes

### Conflict Detection

The app uses a `last_modified` timestamp to detect conflicts:
- If the remote file is newer, it will be pulled
- If the local file is newer, it will be pushed
- If timestamps match, no action is taken

## Development

```bash
# Run tests
npm test

# Run tests with coverage
npm run test:coverage

# Type check
npx tsc --noEmit

# Start development server
npx expo start
```

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
      "order": 1
    }
  ],
  "theme_index": 0,
  "last_modified": 1737628800000
}
```

## Daily Reset

When the app detects a new day (compared to `last_reset`), all tasks are automatically moved back to "To Do" status.

## Themes

25 built-in themes are available. Tap "Theme" to cycle through them.
