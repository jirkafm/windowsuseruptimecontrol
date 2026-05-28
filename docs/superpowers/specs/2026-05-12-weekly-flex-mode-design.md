# Weekly Flex Mode Design

## Context

WindowsUserUptimeControl currently enforces a per-user daily allowance. Each active console user has a daily state entry keyed by Windows SID, the service consumes one second per runtime tick, and enforcement occurs when that user's daily allowance is exhausted.

This design adds a separate weekly flexible mode. It does not replace the current daily behavior. The mode lets an ordinary logged-in user decide how their administrator-owned weekly allowance is distributed across Monday through Sunday while preserving service-side enforcement and local-only access.

## Goals

- Add a startup/config option that enables a new weekly flexible quota mode.
- Keep the current daily mode as the default and preserve existing API behavior where practical.
- Track weekly usage from Monday through Sunday per Windows user SID.
- Let the currently logged-in user view only their own weekly usage and allocation page.
- Let ordinary users redistribute, but not increase, their weekly allowance.
- Enforce 15-minute allocation increments.
- Prevent any day from receiving more than 50% of the weekly allowance.
- Expose the user web interface only on loopback, bound to `127.0.0.1`.
- Provide a modern dashboard-first UI with a weekly graph, statistics, and day sliders.

## Non-Goals

- Ordinary users cannot edit their total weekly allowance.
- The user UI is not an admin console and will not list other users.
- The user UI will not be exposed on the configured admin bind address if that address is non-loopback.
- This change will not remove daily mode.
- This change will not add remote identity providers or browser login flows.

## Configuration

Add config fields:

- `quota_mode`: `daily` or `weekly-flex`. Default is `daily`.
- `default_weekly_allowance_sec`: default weekly allowance for users first seen in weekly mode. The default value is 25,200 seconds, which is 7 hours.
- `user_ui_enabled`: enables the local user web UI. Defaults to true when `quota_mode` is `weekly-flex`, false otherwise.
- `user_ui_port`: local loopback UI port. If zero, reuse the existing API port only when the API bind address is already loopback; otherwise start a separate loopback server.

The service entrypoint should also accept a startup option equivalent to `quota_mode=weekly-flex` so installers and service arguments can enable the mode without changing existing default behavior. The config file remains the durable source of normal service configuration.

## State Model

Extend persisted state without deleting the daily fields:

- Add `WeeklyState` entries per user SID.
- Store `WeekStart` as the Monday date in `YYYY-MM-DD` format.
- Store `WeeklyAllowanceSec`.
- Store seven day allocations keyed by weekday, in seconds.
- Store seven day consumption counters keyed by weekday, in seconds.
- Store weekly consumed and remaining values, either persisted or derived consistently on load.
- Store warning and enforcement flags separately from daily flags where weekly mode needs different behavior.

On load:

- If weekly mode is active and no weekly state exists for a user, create a default weekly distribution from the weekly allowance. When no administrator-provided weekly allowance exists, use the 7-hour default.
- If the stored week start is not the current Monday, reset weekly consumption and initialize the new week while carrying forward the user's weekly allowance and last saved distribution when valid.
- If a legacy or corrupted distribution does not sum to the weekly allowance, normalize it into valid 15-minute increments and respect the 50% day cap.

## Policy Behavior

In `daily` mode, the current policy engine remains authoritative.

In `weekly-flex` mode:

- Each service tick consumes time for the active console user.
- Consumption is recorded against the current weekday and the current week.
- The current day has an allocation cap. Once the day's consumed time reaches that day's allocation, the user is out of time for that day unless they redistribute remaining weekly time through the UI.
- If total weekly consumed time reaches the weekly allowance, the user is exhausted until the next Monday reset.
- Enforcement uses the existing helper countdown and hibernate/shutdown fallback path.
- Restart reenforcement still applies after the user is already exhausted.
- Warning messages should be adjusted to say "today" or "this week" depending on which limit is approaching.

Redistribution rules:

- The weekly total is fixed by admin-owned `WeeklyAllowanceSec`.
- The submitted seven day allocation values must sum exactly to the weekly allowance.
- Each value must be a multiple of 900 seconds.
- Each value must be less than or equal to 50% of the weekly allowance.
- A day allocation cannot be lower than already consumed time for that day.
- A distribution update may clear day-level exhaustion if the new current-day allocation is above current-day consumption and weekly time remains.
- A distribution update cannot clear weekly exhaustion once weekly consumed equals weekly allowance.

## User UI

The selected UI direction is dashboard-first:

- Top summary: week range, weekly used, weekly remaining, today's used, today's remaining, and exhaustion status.
- Main chart: seven-day allocated versus consumed bars.
- Allocation panel: seven sliders, one per day, using 15-minute steps.
- Inline validation: over-cap days, sum mismatch, and attempts to set a day below consumed time.
- Save button: disabled until the submitted distribution is valid.
- Success/error feedback: short status region near the controls.

The UI should be implemented as embedded static HTML/CSS/JS served by Go. The first version can use accessible native range inputs and simple SVG or CSS bars rather than adding a JavaScript dependency.

## User UI Access Control

The local UI is only bound to `127.0.0.1`.

The service determines the currently active console user through the existing Windows session detector. UI status and update endpoints operate only on that active user's SID:

- `GET /user/` serves the HTML shell.
- `GET /user/api/status` returns only the active user's weekly view.
- `POST /user/api/distribution` updates only the active user's weekly distribution after validation.

The UI does not accept a user id from the browser. If no active console user is detected, the endpoints return an unavailable status rather than exposing stored users.

Because this is a loopback-only UI for the active desktop user, it does not use the admin bearer token. Admin APIs remain protected by bearer token and must not be weakened.

## Admin API

Keep existing admin endpoints working in daily mode.

In weekly mode, add or extend admin operations for:

- Reading a user's weekly status.
- Setting a user's total weekly allowance.
- Resetting the current week.

The existing `/v1/info` response should mention weekly-flex endpoints when available. The sanitized config view should include the quota mode and user UI bind settings.

## Runtime Wiring

The runtime chooses the policy implementation based on `Config.QuotaMode`:

- `daily`: current policy engine.
- `weekly-flex`: new weekly policy evaluator and weekly state mutation helpers.

The HTTP serving layer can use either:

- The existing server if its bind address is already `127.0.0.1` and the chosen port is shared.
- A second `http.Server` bound explicitly to `127.0.0.1:<user_ui_port>` when the admin API is bound to a wider interface.

Using a second server is acceptable only for the user UI bind guarantee; it does not change the selected architecture, because state and policy still live in the existing service runtime.

## Testing

Unit tests should cover:

- Config defaults and validation for `quota_mode`, weekly allowance, and UI bind settings.
- Week start calculation and Monday reset behavior.
- Default weekly distribution creation.
- Rejection of invalid user distributions: wrong sum, non-15-minute increment, day over 50%, day below consumed.
- Current-day exhaustion with remaining weekly time available for redistribution.
- Weekly exhaustion until next Monday.
- Runtime mode selection so daily mode behavior remains unchanged.
- User UI endpoints returning only active-user data and rejecting requests when no active user exists.
- User UI bind address selection, including the case where admin API is externally bound.

Manual validation on Windows should cover:

- Installing/running in default daily mode.
- Starting with weekly-flex mode enabled.
- Opening the UI at the loopback address as the logged-in user.
- Confirming another user is not visible.
- Redistributing time and observing enforcement when the day or week budget is exhausted.

## Rollout

The implementation should be incremental:

1. Add config and model fields with tests.
2. Add weekly state normalization and policy tests.
3. Wire runtime mode selection.
4. Add local user UI JSON endpoints.
5. Add embedded dashboard-first UI.
6. Update README and installer documentation.

Daily mode remains the default, so existing installations keep their current behavior unless weekly-flex mode is explicitly enabled.
