# Windows Uptime Enforcer Design

**Date:** 2026-04-01

## Goal

Build a Windows-only application from scratch under the `wincontrol/` project directory that enforces a per-user daily uptime allowance for the active console user, exposes a network-reachable HTTP management API, announces remaining time with TTS, and hibernates the machine when the active user's daily limit is exhausted.

## Scope

This design covers one deployable product made of:

- a Windows service responsible for policy, persistence, enforcement, and the network API
- a per-user session helper responsible for TTS and user-visible notifications
- a one-time PowerShell installer that installs the service and configures startup

Out of scope:

- HTTPS termination
- a GUI or tray application
- domain-wide deployment tooling

## Requirements

### Functional

- The system runs on Windows only.
- The application starts automatically after setup and does not require reconfiguration on every login.
- The service exposes HTTP API endpoints for remote management over the network.
- Authentication uses a bearer token.
- The default daily uptime allowance is 3600 seconds per user.
- Time can be changed by API request:
  - set a user's daily allowance
  - add time
  - subtract time
  - reset the current day for a user
- Time counting applies to the active console user only.
- The policy is per-user quota with machine-wide enforcement.
- When the active user's remaining time reaches zero, the machine hibernates.
- If hibernation is unavailable or fails, the machine shuts down.
- The system logs runtime activity and API access.
- The system announces remaining time with TTS:
  - at the beginning of a timed session
  - at the halfway point
  - when 5 minutes remain
  - during the final 10-to-1 countdown before enforcement
- If the machine comes back online on the same day for a user who already exhausted their time:
  - the system logs and announces that no time remains
  - the system waits a configurable delay before enforcing again
  - the default reenforcement delay is 180 seconds
- If the machine comes back online on a new day, that user's allowance resets to the configured daily default.

### Security

- A standard user must not be able to terminate or stop the enforcement service.
- Only users with administrator rights may stop the service or uninstall the application.
- Standard users must not be able to modify installed binaries or config.

## High-Level Architecture

### 1. Windows Service: `activitysvc.exe`

The Windows service is the source of truth. It owns:

- daily allowance policy
- active console user detection
- per-user state persistence
- network HTTP API
- audit and runtime logging
- helper lifecycle management
- final hibernation or shutdown enforcement

The service runs under `LocalSystem`. This gives it the required rights to:

- query sessions
- start or monitor the helper for the active user session
- hibernate or shut down the machine
- resist termination by standard users through normal Service Control Manager rules

### 2. User Session Helper: `activityhelper.exe`

The helper runs in the logged-in user's interactive session. It is not authoritative. Its responsibilities are limited to:

- Windows TTS playback
- user-facing spoken warnings
- optional transient desktop notifications if added later
- reporting heartbeat/readiness to the service

The helper receives commands from the service over a local-only IPC channel. If a user kills the helper process, the service relaunches it for the active console session. Killing the helper never disables enforcement.

### 3. One-Time PowerShell Installer

The installer performs one-time machine setup:

- creates `C:\ProgramData\Activity`
- creates subdirectories for config, state, logs, and binaries
- copies binaries into the install directory
- writes initial config with generated or provided bearer token
- installs and starts the Windows service
- registers helper startup for user logon
- creates firewall rule(s) for the configured API port
- applies ACLs so standard users cannot modify the installation

After setup, the system starts automatically. No recurring installer action is required.

## Process Model

### Service Startup

On service startup:

1. load config
2. load or initialize persisted state
3. detect the active console session and active console user
4. start the HTTP API listener
5. ensure a helper is running in the active user's session if a user is logged in
6. start the policy evaluation loop

### Helper Startup

The helper starts on user logon and also may be started on demand by the service. On startup it:

1. connects to the local IPC endpoint exposed by the service
2. registers the current session id and username
3. waits for speech commands
4. sends heartbeats so the service can detect failure

The service treats the helper as replaceable and untrusted.

## Time Accounting Model

### Per-User Quota

Each Windows user gets an independent per-day allowance. Example:

- John has 3600 seconds per day
- Mary has 3600 seconds per day

State is tracked separately for each user, keyed by SID and cached with username for readability.

### Active Console User Only

Only the active console user consumes time.

Included:

- the user physically logged into the active desktop session
- the user after Fast User Switching when their session becomes active

Excluded:

- disconnected RDP sessions
- locked but non-active background sessions
- other logged-in users who are not currently the active console user

If no active console user exists, no time is consumed.

### Machine-Wide Enforcement

When the active user exhausts their allowance, the entire machine is hibernated or shut down. Enforcement is not scoped to a single session because Windows hibernation and shutdown are machine-wide actions.

## Multi-User Behavior

### John and Mary Example

If John uses 40 minutes in the morning and logs off, John has 20 minutes left for the rest of that day.

If Mary later logs in as the active console user, Mary still has her own full daily allowance because quotas are per-user.

If John becomes active again later that same day, the service restores John's consumed time and continues from his remaining 20 minutes.

If John reaches zero while active, the service warns John and then hibernates the whole machine. That enforcement affects Mary too because the action is machine-wide, but Mary's own quota remains unchanged unless she was the active user consuming time.

If the machine comes back the same day and John becomes active again while still exhausted, the service starts the reenforcement delay, announces that no time remains, and hibernates again unless an administrator changes John's time through the API before the delay expires.

If Mary becomes the active user later the same day and still has time, Mary continues normally.

## Persistence Design

### Config File

Location: `C:\ProgramData\Activity\config\config.json`

Fields:

- `api_bind_address`: default `0.0.0.0`
- `api_port`
- `bearer_token`
- `default_daily_allowance_sec`: default `3600`
- `reenforcement_delay_sec`: default `180`
- `warning_halfway_enabled`: default `true`
- `warning_five_min_enabled`: default `true`
- `helper_path`
- `log_level`

### State File

Location: `C:\ProgramData\Activity\state\state.json`

Top-level state contains:

- current local service date
- metadata for current enforcement cycle
- per-user records

Each per-user record contains:

- `user_sid`
- `username`
- `date`
- `daily_allowance_sec`
- `consumed_sec`
- `remaining_sec`
- `session_started_at`
- `exhausted`
- `startup_warning_sent`
- `halfway_warning_sent`
- `five_min_warning_sent`
- `reenforcement_pending`
- `reenforcement_deadline`
- `last_enforcement_reason`

### Restart and Recovery

On startup or restart the service loads state and reconciles it with the current local date.

- If the saved user record date is not today, that user's record resets for the new day and uses the configured default allowance unless explicitly overridden by an administrator.
- If the date is still today, the service preserves consumed time and warning markers.
- If the user was already exhausted today, the service enters reenforcement logic instead of resetting usage.

If the state file is unreadable or corrupted, the service:

1. renames the bad file with a timestamped `.corrupt` suffix
2. creates a fresh state file
3. logs the recovery event

## Warning and Enforcement Behavior

### Standard Daily Flow

At the start of a user's active timed session, the service instructs the helper to announce the remaining time.

While time is being consumed, the service emits warnings at these milestones:

- startup
- halfway through the user's allowance
- five minutes remaining

Warnings are emitted once per user per day per milestone.

### Exhaustion Flow

When `remaining_sec <= 0` for the active user:

1. mark the user exhausted in persisted state
2. log the exhaustion event
3. command the helper to speak a final warning
4. command the helper to speak a 10-to-1 countdown
5. attempt hibernation
6. if hibernation is unavailable or fails, perform shutdown

### Same-Day Return After Exhaustion

If the machine is started again on the same local day and the active user is already exhausted:

1. log that the user returned with no remaining time
2. set `reenforcement_pending=true`
3. compute `reenforcement_deadline = now + reenforcement_delay_sec`
4. instruct the helper to announce that no time remains and state the delay
5. expose the pending reenforcement state over the API
6. if an administrator adjusts the user's time before the deadline:
   - clear `exhausted`
   - clear `reenforcement_pending`
   - clear `reenforcement_deadline`
   - continue normal tracking
7. otherwise, at deadline:
   - speak the 10-to-1 countdown
   - hibernate, or shut down as fallback

### New Day Behavior

If the machine starts on a different local day, all per-user enforcement state resets for the new day:

- consumed time resets to zero
- remaining time becomes the current daily allowance
- warning flags clear
- exhausted clears
- reenforcement state clears

## API Design

### Authentication

All management endpoints require:

- `Authorization: Bearer <token>`

Requests with missing or invalid tokens return `401 Unauthorized`.

### Endpoints

#### `GET /v1/health`

Returns:

- service status
- active user identity if any
- current date
- helper connectivity status

#### `GET /v1/config`

Returns a safe view of configuration:

- bind address
- port
- default daily allowance
- reenforcement delay
- logging level

It does not return the raw bearer token.

#### `GET /v1/users`

Returns all tracked users for the current day with:

- username
- SID
- allowance
- consumed
- remaining
- exhausted
- active status

#### `GET /v1/users/{user}/status`

Returns the current-day state for the selected user.

User identifier resolution supports:

- SID
- exact username

#### `POST /v1/users/{user}/allowance`

Replaces that user's daily allowance for the current day.

Body:

- `daily_allowance_sec`

Behavior:

- recalculates remaining time based on the new limit and already consumed time
- if remaining time becomes positive, clears exhaustion and pending reenforcement

#### `POST /v1/users/{user}/adjust`

Adjusts the user's remaining time.

Body:

- `delta_sec`

Behavior:

- positive delta grants more time
- negative delta removes time
- remaining time never drops below zero
- if an exhausted user receives enough time to become positive, pending reenforcement is cleared

#### `POST /v1/users/{user}/reset-today`

Resets the user's current-day counters and warning flags without changing the configured default allowance.

#### `POST /v1/enforcement/hibernate-now`

Immediately triggers the final countdown and enforcement.

#### `POST /v1/announce`

Body:

- `message`

The service asks the active helper to speak the provided message. This is best-effort and does not affect enforcement state.

#### `GET /v1/logs/recent`

Returns recent service and API log entries with bounded line count.

## Logging

Log locations:

- `C:\ProgramData\Activity\logs\service.log`
- `C:\ProgramData\Activity\logs\api.log`

The service log records:

- startup and shutdown
- active user changes
- helper launches and failures
- time milestone warnings
- enforcement and fallback actions
- state recovery events

The API log records:

- timestamp
- remote address
- route
- method
- status code
- authenticated principal label, if configured

The raw bearer token is never written to logs.

## Security Design

### Admin-Only Termination

The service is installed as a Windows service. Standard users do not have rights to stop or reconfigure it through Service Control Manager. Administrators do.

The installer also applies ACLs to installation directories and service configuration so standard users cannot:

- replace binaries
- edit config
- remove logs
- unregister startup

### Helper Containment

The helper is deliberately non-authoritative. If it is terminated, the service restarts it. If the helper is unavailable during enforcement, the service still hibernates or shuts down the machine.

### Network Exposure

The API listens on a network-reachable interface over plain HTTP because that is an explicit requirement. This means the bearer token is exposed to anyone who can inspect network traffic on that segment. The design accepts this risk and does not attempt to hide it.

## Installer Design

The PowerShell installer performs:

1. preflight validation for administrative rights
2. creation of install directories under `C:\ProgramData\Activity`
3. binary copy
4. config creation
5. Windows service installation
6. service start
7. helper startup registration for user logon
8. firewall rule creation for the API port
9. ACL hardening

The installer is idempotent enough to support rerun for repair or upgrade, but the target user experience is one-time setup.

## Error Handling

- If no helper is connected, API and accounting continue; TTS is skipped and logged as unavailable.
- If active user detection temporarily fails, the service logs the error and retries without consuming time.
- If helper restart fails, the service retries with backoff and logs each failure.
- If hibernation fails, the service logs the failure and falls back to shutdown.
- If both hibernation and shutdown fail, the service logs a critical failure and retries enforcement at a short interval until success or shutdown.

## Testing Strategy

### Automated

- unit tests for per-user daily accounting
- unit tests for active console user switching
- unit tests for warning milestone emission
- unit tests for same-day reenforcement delay behavior
- unit tests for new-day reset behavior
- unit tests for API token authentication
- unit tests for API allowance and adjustment semantics
- integration tests with fake clock, fake persistence, fake helper, and fake power controller

### Manual Windows Validation

- install on a clean Windows machine
- verify service starts automatically
- verify helper starts on user logon
- verify TTS at startup, halfway, five minutes, and final countdown
- verify same-day reenforcement after reboot with default 180-second delay
- verify new-day reset
- verify admin can stop the service
- verify standard user cannot stop the service
- verify helper termination causes respawn
- verify hibernation fallback to shutdown when hibernation is unavailable

## Implementation Notes

- This project should be implemented from scratch in the `wincontrol/` directory and must not reuse code from `Activity/`.
- The preferred implementation language is Go because it is suitable for Windows services, HTTP APIs, persistence, and testable internal packages, but the design does not depend on a specific framework.
- Internal modules should be separated by responsibility:
  - service bootstrap
  - policy engine
  - active session detection
  - persistence
  - HTTP API
  - helper IPC
  - power control
  - logging

## Open Decisions Resolved

- Architecture: Windows service plus user-session helper
- Startup: one-time PowerShell installer
- API exposure: network reachable
- Auth: bearer token
- Transport: plain HTTP
- Quota model: per-user quota with machine-wide enforcement
- Time counting: active console user only
- Same-day return after exhaustion: delayed reenforcement with configurable delay, default 180 seconds
