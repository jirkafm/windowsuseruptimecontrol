# Custom Consumed Announcements Design

## Goal

Allow administrators to configure up to 10 additional consumed-time percentage announcements while preserving the existing startup, halfway, five-minute, and enforcement countdown announcements.

## Scope

- Add a service configuration array named `custom_consumed_warning_percentages`.
- Accept only values from `1` through `99`.
- Reject arrays with more than 10 items.
- Treat duplicate values as one value.
- Ignore `50` because the halfway warning already exists, and log a warning that the configured value is ignored.
- Track custom warning delivery per user, not per service runtime, so each Windows user receives each configured milestone once for their own daily quota.

## Architecture

Configuration loading normalizes the percentage list before it reaches policy evaluation. The normalized list is sorted, deduplicated, and excludes `50`.

The persisted per-user state records which custom percentages have already been announced for the current day. Policy evaluation compares consumed time against each configured percentage threshold and emits the normal remaining-time announcement text once per user and per configured percentage.

Existing warning controls remain unchanged. The built-in halfway warning continues to use `warning_halfway_enabled`, the five-minute warning continues to use `warning_five_min_enabled`, and custom percentages do not replace either behavior.

## Error Handling

Configuration loading fails when the submitted custom percentage array has more than 10 entries or contains a value outside `1..99`. Duplicate entries are not errors. Value `50` is not an error; it is ignored and logged as a warning because the built-in halfway announcement covers it.

## Testing

Tests cover config normalization, invalid config rejection, warning logging for ignored `50`, one-time custom policy messages, and per-user independence.
