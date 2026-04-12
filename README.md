# WindowsUserUptimeControl

WindowsUserUptimeControl is a Windows-only uptime enforcer built around a background service plus a per-session helper. The service tracks how long the active console user has been allowed to use the machine on the current day, persists that state per Windows user, exposes a bearer-token HTTP API for administration, and triggers hibernation or shutdown when the allowance is exhausted. The helper process runs inside the logged-in user session so warning announcements can be spoken with native Windows text-to-speech before enforcement happens.

## Highlights
- **Per-user daily uptime enforcement** — the state store keeps a separate daily allowance, consumed time, remaining time, and warning/enforcement flags for each tracked Windows SID so one user exhausting their time does not overwrite another user’s quota.
- **Active console detection with spoken warnings** — the service only counts usage for the active console session, launches `activityhelper.exe` into that session, and uses the helper IPC path plus heartbeat tracking to deliver startup, halfway, five-minute, and countdown announcements.
- **Admin API plus restart reenforcement** — the HTTP API exposes health, config, quota, reset, announcement, logs, and immediate-enforcement operations behind a bearer token, while the runtime also supports same-day reenforcement after reboot with a configurable delay before enforcing again.

## Quick Start
1. Build the binaries with `GOOS=windows GOARCH=amd64 go build -o dist/activitysvc.exe ./cmd/activitysvc` and `GOOS=windows GOARCH=amd64 go build -o dist/activityhelper.exe ./cmd/activityhelper`.
2. Open an elevated PowerShell window and run `pwsh -ExecutionPolicy Bypass -File .\installer\install.ps1 -ApiPort 8111 -BearerToken "<token>"`.
3. Confirm the Windows service `WindowsUserUptimeControlActivityService` is running, then query `GET /v1/info` with the bearer token to inspect the available API endpoints.

## Usage Walkthrough
1. Install and verify the service: the installer copies both binaries into `C:\ProgramData\Activity\bin`, creates `config\config.json`, applies ACLs to config and state paths, opens the configured firewall port, and starts `WindowsUserUptimeControlActivityService`.
2. Let a user log in normally: the service detects the active console session, relaunches the helper if its heartbeat is missing or stale, and begins recording usage into the per-user daily state for that Windows SID.
3. Adjust or inspect quotas through the API: use `GET /v1/users` or `GET /v1/users/{userId}/status` to inspect current usage, then `POST /v1/users/{userId}/allowance`, `POST /v1/users/{userId}/adjust`, or `POST /v1/users/{userId}/reset-today` to change the current day’s allowance or state.
4. Observe warning and enforcement behavior: as the allowance is consumed, the helper speaks warning milestones and the final `10` to `1` countdown. When the timer is exhausted, the service attempts hibernation first and falls back to shutdown if hibernation is unavailable.

## Example API Flow
1. Install the service with a known bearer token and API port, for example `8111`.
2. Check that the API is up:

```bash
curl http://localhost:8111/v1/health
```

3. List tracked users:

```bash
curl -H "Authorization: Bearer <token>" http://localhost:8111/v1/users
```

4. Grant five more minutes to a user:

```bash
curl -X POST \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  http://localhost:8111/v1/users/john/adjust \
  -d '{"delta_sec":300}'
```

5. Trigger a manual spoken announcement:

```bash
curl -X POST \
  -H "Authorization: Bearer <token>" \
  -H "Content-Type: application/json" \
  http://localhost:8111/v1/announce \
  -d '{"message":"WindowsUserUptimeControl test announcement"}'
```

The service resolves usernames or SIDs, writes the updated state to disk, and routes spoken messages through the active user’s helper process.

## Build Requirements
- Go 1.25+ as declared in `go.mod`.
- A Windows target environment for building and running `activitysvc.exe` and `activityhelper.exe`.
- PowerShell with Administrator access for installation and validation.

## Build/Test From CLI
- `go test ./...` — runs the Go unit test suite across API, policy, runtime, helper IPC, state, and Windows abstraction layers.
- `GOOS=windows GOARCH=amd64 go build -o dist/activitysvc.exe ./cmd/activitysvc` — builds the Windows service executable.
- `GOOS=windows GOARCH=amd64 go build -o dist/activityhelper.exe ./cmd/activityhelper` — builds the helper executable for the user session.

## Install/Operate From CLI
- Install the service: `pwsh -ExecutionPolicy Bypass -File .\installer\install.ps1 -ApiPort 8111 -BearerToken "<token>"`.
- Uninstall the service: `pwsh -ExecutionPolicy Bypass -File .\installer\uninstall.ps1`.
- Inspect the API surface: `curl -H "Authorization: Bearer <token>" http://localhost:8111/v1/info`.
- Trigger immediate enforcement: `curl -X POST -H "Authorization: Bearer <token>" http://localhost:8111/v1/enforcement/hibernate-now`.

## Configuration Notes
The installer writes `C:\ProgramData\Activity\config\config.json` with defaults for `api_bind_address`, `api_port`, `default_daily_allowance_sec`, `reenforcement_delay_sec`, `helper_launch_cooldown_sec`, warning toggles, helper path, and log level. The config directory is intended to be readable only by `Administrators` and `SYSTEM` so standard users cannot read the raw bearer token.

Manual validation steps for installation, ACLs, helper relaunch, API auth, user quota behavior, and enforcement flow are documented in [docs/windows-validation-checklist.md](docs/windows-validation-checklist.md).

## Project Layout
- `cmd/activitysvc` — service entrypoint that starts the Windows service runner and main runtime loop.
- `cmd/activityhelper` — helper entrypoint that polls the IPC bus, reports heartbeats, and speaks queued messages.
- `internal/runtime` — wiring for config loading, HTTP server startup, helper launch checks, and the top-level service/helper loops.
- `internal/service` — quota runtime logic, quota mutation methods used by the API, and hibernation/shutdown orchestration.
- `internal/policy` — daily allowance evaluation, warning thresholds, and enforcement decision rules.
- `internal/api` — HTTP handlers for health, config, users, announcements, logs, and immediate enforcement.
- `internal/state` — JSON-backed persisted daily user state.
- `internal/helperfs`, `internal/helperipc`, `internal/helperstatus` — message spooling, helper command protocol, and helper heartbeat tracking.
- `internal/windows/session`, `internal/windows/helper`, `internal/windows/power`, `internal/windows/service` — Windows-specific adapters for active user detection, helper launching, power actions, and service hosting.
- `installer` — PowerShell install, uninstall, and installer script tests.
- `docs` — validation checklist and project planning/spec artifacts.
