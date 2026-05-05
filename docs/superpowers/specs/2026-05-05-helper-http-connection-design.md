# Helper HTTP Connection Design

## Goal

`activityhelper` must only remain running while it has an active service connection, and `activitysvc` must allow at most one active helper per Windows user.

## Design

Reuse the existing `activitysvc` HTTP listener for private helper communication. The admin API remains under `/v1/*`; helper communication is isolated under `/internal/helper/stream` and requires a service-generated helper token, not the configured admin bearer token.

The service owns an in-memory helper registry keyed by user SID. A helper connects with its user SID and session ID, then receives newline-delimited JSON commands over a long-lived HTTP response. Registering a new helper for the same user closes the previous helper's command stream, causing that older helper process to exit.

The service launcher passes the private helper URL and token on the `activityhelper.exe` command line. Helpers that start without those arguments, fail authentication, lose the stream, or cannot reach the service exit instead of staying alive independently.

## Components

- `internal/helperipc`: active helper registry, command delivery, duplicate replacement, and connection status.
- `internal/api`: private helper stream endpoint on the existing HTTP server.
- `internal/helper`: client-side stream reader that speaks received commands and exits when the stream ends.
- `internal/runtime`: service wiring that creates the helper token, starts the API with the helper registry, launches helpers with connection arguments, and uses the registry for speech delivery.
- `internal/windows/helper`: launcher command-line construction for helper URL and token.

## Error Handling

- Missing helper connection makes speech best-effort and returns `helper not connected`, preserving existing enforcement behavior.
- A duplicate helper connection replaces and disconnects the older helper for that user.
- A helper HTTP response other than `200 OK` is treated as a startup failure and the helper exits.
- Service shutdown closes active HTTP requests through normal server shutdown.

## Testing

Unit tests cover duplicate helper replacement, endpoint authentication, command streaming, helper client exit on closed streams, and launcher command-line arguments. The full Go test suite remains the primary verification command.
