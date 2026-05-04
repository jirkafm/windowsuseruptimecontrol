# Release Build

Use `scripts/build-release.sh` to prepare the Windows release bundle from a clean source checkout.

## Prerequisites

- Go matching `go.mod`
- `zip`
- `unzip`
- `file`
- `shasum`

## Command

```bash
./scripts/build-release.sh
```

The script uses `GOCACHE` if it is already set. Otherwise it uses `.gocache` under the repository root.

## What The Script Does

1. Runs `go test -count=1 ./...`.
2. Builds `dist/activitysvc.exe` for `GOOS=windows GOARCH=amd64`.
3. Builds `dist/activityhelper.exe` for `GOOS=windows GOARCH=amd64` with `-ldflags="-H=windowsgui"` so the helper does not open a console window.
4. Refreshes `release/windowsuseruptimecontrol-package`.
5. Creates `release/windowsuseruptimecontrol-windows-package.zip`.
6. Verifies the zip has no `.DS_Store` or `._*` files.
7. Verifies the service is a Windows console executable and the helper is a Windows GUI executable.
8. Verifies the binaries inside the zip match the binaries in `dist`.
9. Writes `release/windowsuseruptimecontrol-windows-package.sha256`.

## Release Outputs

- `dist/activitysvc.exe`
- `dist/activityhelper.exe`
- `release/windowsuseruptimecontrol-package/`
- `release/windowsuseruptimecontrol-windows-package.zip`
- `release/windowsuseruptimecontrol-windows-package.sha256`

## Expected Zip Layout

```text
windowsuseruptimecontrol-package/
windowsuseruptimecontrol-package/README.md
windowsuseruptimecontrol-package/dist/activitysvc.exe
windowsuseruptimecontrol-package/dist/activityhelper.exe
windowsuseruptimecontrol-package/installer/install.ps1
windowsuseruptimecontrol-package/installer/uninstall.ps1
```
