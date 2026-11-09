# Google Play listing — Daily Tasks

Everything the Play Console asks for, ready to paste. Assets live next to this
file (`play-icon-512.png`, `feature-graphic.png`, `screenshots/`).

## Main store listing

**App name** (30): `Daily Tasks`

**Short description** (80):
`Your daily routine as a board that resets every morning. Syncs across devices.`

**Full description** (4000):

```
Daily Tasks is a tiny board for the things you do every day.

Every morning all of your tasks move back to "To do". Tick them off as the day goes by, skip the ones that don't apply, and see how your week went.

• A board that resets itself – no re-creating the same to-dos each day.
• Per-day visibility – show "Gym" only on Mon/Wed/Fri.
• Deadlines and reminders – optional time-of-day deadlines with notifications you can act on (done / skip) straight from the notification.
• Durations – see how much of your day your routine takes.
• Stats – completion rate, streaks, weak spots and trends.
• 25 colour themes.
• Works offline. Use it without an account, or sign in with Google to sync between your phone, any browser (daily-tasks-api.fly.dev/app) and the command-line client.
• Private by design: no ads, no trackers, no analytics. Synced data is encrypted at rest and you can delete your account from the app at any time.
• Open source.
```

**App category**: Productivity
**Tags**: To-do list, Habit tracker, Productivity
**Contact email**: _(choose; shown publicly)_
**Website**: `https://daily-tasks-api.fly.dev`
**Privacy policy**: `https://daily-tasks-api.fly.dev/privacy`

## Graphics

| Asset | File | Spec |
|---|---|---|
| App icon | `play-icon-512.png` | 512×512 PNG |
| Feature graphic | `feature-graphic.png` | 1024×500 PNG |
| Phone screenshots | `screenshots/*.png` | 2–8, 1080×2400 (from the emulator) |

## App content

**Account deletion**
- In-app: Config → Delete account
- Web URL: `https://daily-tasks-api.fly.dev/account/delete`

**Data safety** (answers):

| Question | Answer |
|---|---|
| Does your app collect or share any of the required user data types? | Yes (collects; does not share) |
| Is all user data encrypted in transit? | Yes |
| Do you provide a way for users to request that their data is deleted? | Yes (in-app and web URL above) |
| Personal info → Email address | Collected, not shared. Optional (only when signing in). Purpose: Account management. Not processed ephemerally. |
| Personal info → User IDs | Collected, not shared. Optional. Purpose: Account management, App functionality. |
| App activity → Other user-generated content (task titles, durations, status history) | Collected, not shared. Optional (only when signing in; local-only mode keeps data on device). Purpose: App functionality. |
| Everything else (location, contacts, photos, financial, health, messages, device IDs, app interactions/analytics, crash logs) | Not collected |

**Ads**: No ads.
**Target audience**: 18+ (avoids the Families policy; the app is not aimed at children).
**Content rating questionnaire**: Utility/Productivity, no user-to-user communication, no sharing of location, no purchases → expected rating "Everyone / PEGI 3".
**Government app / financial / health**: No.
**News app**: No.

## Release checklist

1. Play Console → Create app → name "Daily Tasks", default language English (US), App, Free.
2. Complete "App content" (above) and the store listing.
3. Setup → App signing: let Google manage the app signing key; upload key = the
   keystore whose base64 is in the `ANDROID_UPLOAD_KEYSTORE_BASE64` secret.
4. Setup → API access: create a service account with "Release to testing
   tracks" permission, store its JSON in `PLAY_SERVICE_ACCOUNT_JSON`.
5. The very first AAB must be uploaded manually (Play API can't create the
   app's first release): download it from the `release.yml` run artifacts.
6. Later releases: bump `VERSION`, merge, tag `vX.Y.Z` → internal track →
   promote to closed/production in the console.
