# WinControl

Windows-only uptime enforcement service with a network HTTP API and a user-session TTS helper.

## Build

```bash
go test ./...
GOOS=windows GOARCH=amd64 go build -o dist/activitysvc.exe ./cmd/activitysvc
GOOS=windows GOARCH=amd64 go build -o dist/activityhelper.exe ./cmd/activityhelper
```

## Install

Run PowerShell as Administrator:

```powershell
pwsh -ExecutionPolicy Bypass -File .\installer\install.ps1 -ApiPort 8111 -BearerToken "<token>"
```
