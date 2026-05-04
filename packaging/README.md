# WindowsUserUptimeControl

Windows-only uptime enforcement service with a network HTTP API and a user-session TTS helper.

## Build

```bash
go test ./...
GOOS=windows GOARCH=amd64 go build -o dist/activitysvc.exe ./cmd/activitysvc
GOOS=windows GOARCH=amd64 go build -ldflags="-H=windowsgui" -o dist/activityhelper.exe ./cmd/activityhelper
```

## Install

Run PowerShell as Administrator:

```powershell
pwsh -ExecutionPolicy Bypass -File .\installer\install.ps1 -ApiPort 8111 -BearerToken "<token>"
```

## API

- `GET /v1/health` is public.
- `GET /v1/info` requires the bearer token and returns all endpoints with example usage.
- `GET /v1/config` requires the bearer token and returns a sanitized config view.
- `/v1/users/{userId}/...` endpoints accept either a Windows SID or a username. Username matching supports both exact stored usernames and simple-name matching for values like `MYPC\john` via `john`.

Note: `config\config.json` is intended to be readable only by `Administrators` and `SYSTEM` so standard users cannot read the API bearer token from disk.
