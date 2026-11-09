#!/usr/bin/env bash
# End-to-end smoke test: dev APK on a running Android emulator against a
# server with test login enabled.
#
#   mobile/scripts/e2e-emulator.sh path/to/app-release.apk [api-url] [test-secret]
#
# Without api-url it builds and starts a local server (in-memory store) on
# :18080, reachable from the emulator as http://10.0.2.2:18080.
# Screenshots land in $OUT_DIR (default: ./e2e-out).
set -euo pipefail

APK=${1:?usage: e2e-emulator.sh app.apk [api-url] [secret]}
API=${2:-}
SECRET=${3:-local}
OUT_DIR=${OUT_DIR:-e2e-out}
PKG=be.maclug.dailytasks.dev
ROOT=$(cd "$(dirname "$0")/../.." && pwd)
mkdir -p "$OUT_DIR"

log() { printf '\n== %s\n' "$*"; }
fail() { echo "FAIL: $*" >&2; exit 1; }

SERVER_PID=""
cleanup() { [ -n "$SERVER_PID" ] && kill "$SERVER_PID" 2>/dev/null || true; }
trap cleanup EXIT

if [ -z "$API" ]; then
  log "starting local server on :18080"
  (cd "$ROOT/server" && go build -o "$OUT_DIR/../.e2e-server" ./cmd/server) 2>/dev/null \
    || (cd "$ROOT/server" && go build -o /tmp/daily-tasks-e2e-server ./cmd/server)
  BIN="$OUT_DIR/../.e2e-server"; [ -x "$BIN" ] || BIN=/tmp/daily-tasks-e2e-server
  DEV_MEMORY_STORE=1 JWT_SECRET=$(openssl rand -hex 32) MASTER_KEY=$(openssl rand -base64 32) \
    BASE_URL=http://localhost:18080 PORT=18080 TEST_LOGIN_SECRET="$SECRET" \
    "$BIN" > "$OUT_DIR/server.log" 2>&1 &
  SERVER_PID=$!
  for _ in $(seq 1 20); do curl -fsS http://localhost:18080/health >/dev/null 2>&1 && break; sleep 0.5; done
  HOST_API=http://localhost:18080
  API=http://10.0.2.2:18080
else
  HOST_API=$API
fi

SUB="e2e-$(date +%s)"
token() { curl -fsS "$HOST_API/auth/test?secret=$SECRET&sub=$SUB" | python3 -c 'import sys,json;print(json.load(sys.stdin)["token"])'; }
rpc() { curl -fsS -X POST -H 'Content-Type: application/json' -H "Authorization: Bearer $TOKEN" -d "$2" "$HOST_API/dailytasks.v1.$1"; }
ui_has() { adb exec-out uiautomator dump /dev/tty 2>/dev/null | grep -q "$1"; }
wait_ui() { for _ in $(seq 1 "${2:-30}"); do ui_has "$1" && return 0; sleep 1; done; return 1; }
shot() { adb exec-out screencap -p > "$OUT_DIR/$1.png"; echo "screenshot: $OUT_DIR/$1.png"; }

log "device"
adb wait-for-device
[ "$(adb shell getprop sys.boot_completed | tr -d '\r')" = "1" ] || fail "emulator not booted"
adb shell getprop ro.build.version.release

log "install $APK"
adb uninstall "$PKG" >/dev/null 2>&1 || true
adb install -r "$APK" >/dev/null
adb shell pm grant "$PKG" android.permission.POST_NOTIFICATIONS 2>/dev/null || true

log "first launch shows onboarding"
adb shell am start -W -n "$PKG/.MainActivity" >/dev/null
wait_ui "Sign in with Google" 60 || { shot fail-onboarding; fail "onboarding not shown"; }
shot 01-onboarding

log "dev sign-in as $SUB against $API"
adb shell am start -a android.intent.action.VIEW \
  -d "'daily-tasks-dev://dev-login?api=$API&secret=$SECRET&sub=$SUB'" "$PKG" >/dev/null
wait_ui "Daily Tasks" 30 && ! ui_has "Sign in with Google" || { shot fail-login; fail "dev login did not reach the board"; }
shot 02-empty-board

log "seed tasks server-side (as another device would)"
TOKEN=$(token)
NOW_MS=$(($(date +%s) * 1000))
TODAY=$(date +%F)
DATA=$(python3 - "$NOW_MS" "$TODAY" <<'PY'
import base64, json, sys
now, today = int(sys.argv[1]), sys.argv[2]
data = {"version": 1, "last_reset": today, "next_id": 4, "theme_index": 0, "last_modified": now,
        "tasks": [
          {"id": 1, "title": "Stretch 10 min", "duration": 10, "status": "todo", "order": 1},
          {"id": 2, "title": "Read 20 pages", "duration": 25, "status": "todo", "order": 2, "deadline": "21:00"},
          {"id": 3, "title": "Inbox zero", "duration": 15, "status": "done", "order": 1}]}
print(json.dumps({"data": base64.b64encode(json.dumps(data).encode()).decode(),
                  "history": base64.b64encode(b'{"version":1,"days":[],"events":[]}').decode()}))
PY
)
rpc SyncService/PutState "$DATA" >/dev/null

log "foreground sync pulls them into the app"
adb shell input keyevent KEYCODE_HOME
sleep 1
adb shell am start -n "$PKG/.MainActivity" >/dev/null
wait_ui "Read 20 pages" 30 || { shot fail-pull; fail "server-side tasks did not appear"; }
shot 03-board-synced

log "complete a task in the app and verify the server sees it"
# Tap the row's "done" button, found by its accessibility label.
BOUNDS=$(adb exec-out uiautomator dump /dev/tty 2>/dev/null | python3 -c '
import re,sys
xml=sys.stdin.read()
m=re.search(r"content-desc=\"Mark Stretch 10 min done\"[^>]*bounds=\"\[(\d+),(\d+)\]\[(\d+),(\d+)\]\"", xml)
print("" if not m else f"{(int(m.group(1))+int(m.group(3)))//2} {(int(m.group(2))+int(m.group(4)))//2}")')
[ -n "$BOUNDS" ] || fail "task row not found"
adb shell input tap $BOUNDS
for _ in $(seq 1 20); do
  STATUS=$(rpc SyncService/GetState '{}' | python3 -c 'import sys,json,base64; d=json.load(sys.stdin); t=[x for x in json.loads(base64.b64decode(d["data"]))["tasks"] if x["id"]==1]; print(t[0]["status"] if t else "")')
  [ "$STATUS" = "done" ] && break
  sleep 1
done
shot 04-after-tap
[ "$STATUS" = "done" ] || fail "server still has status '$STATUS' for task 1"

log "PASS"
