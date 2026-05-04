#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DIST_DIR="$ROOT/dist"
PACKAGE_DIR="$ROOT/release/windowsuseruptimecontrol-package"
ZIP_PATH="$ROOT/release/windowsuseruptimecontrol-windows-package.zip"
CHECKSUM_PATH="$ROOT/release/windowsuseruptimecontrol-windows-package.sha256"
PACKAGE_ROOT_NAME="windowsuseruptimecontrol-package"
GO_CACHE="${GOCACHE:-$ROOT/.gocache}"

require_tool() {
  if ! command -v "$1" >/dev/null 2>&1; then
    printf 'missing required tool: %s\n' "$1" >&2
    exit 1
  fi
}

require_tool go
require_tool zip
require_tool unzip
require_tool shasum
require_tool file

mkdir -p "$DIST_DIR" "$PACKAGE_DIR/dist" "$PACKAGE_DIR/installer" "$ROOT/release" "$GO_CACHE"

printf 'Running tests...\n'
GOCACHE="$GO_CACHE" go test -count=1 ./...

printf 'Building Windows binaries...\n'
GOCACHE="$GO_CACHE" GOOS=windows GOARCH=amd64 go build -o "$DIST_DIR/activitysvc.exe" ./cmd/activitysvc
GOCACHE="$GO_CACHE" GOOS=windows GOARCH=amd64 go build -ldflags="-H=windowsgui" -o "$DIST_DIR/activityhelper.exe" ./cmd/activityhelper

printf 'Refreshing package directory...\n'
cp "$DIST_DIR/activitysvc.exe" "$PACKAGE_DIR/dist/activitysvc.exe"
cp "$DIST_DIR/activityhelper.exe" "$PACKAGE_DIR/dist/activityhelper.exe"
cp "$ROOT/installer/install.ps1" "$PACKAGE_DIR/installer/install.ps1"
cp "$ROOT/installer/uninstall.ps1" "$PACKAGE_DIR/installer/uninstall.ps1"
cp "$ROOT/packaging/README.md" "$PACKAGE_DIR/README.md"

tmpdir="$(mktemp -d "${TMPDIR:-/tmp}/windowsuseruptimecontrol-release-XXXXXXXX")"
cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

cp -R "$PACKAGE_DIR" "$tmpdir/$PACKAGE_ROOT_NAME"
tmpzip="$tmpdir/windowsuseruptimecontrol-windows-package.zip"

printf 'Creating clean release zip...\n'
(
  cd "$tmpdir"
  zip -r "$tmpzip" "$PACKAGE_ROOT_NAME" -x '*/.DS_Store' '*/._*'
)
mv "$tmpzip" "$ZIP_PATH"

printf 'Verifying release zip...\n'
archive_list="$(unzip -l "$ZIP_PATH")"
printf '%s\n' "$archive_list"
if printf '%s\n' "$archive_list" | grep -E '(^|/)(\.DS_Store|\._)' >/dev/null; then
  printf 'release zip contains macOS metadata files\n' >&2
  exit 1
fi

unzip -p "$ZIP_PATH" "$PACKAGE_ROOT_NAME/dist/activitysvc.exe" > "$tmpdir/activitysvc.exe"
unzip -p "$ZIP_PATH" "$PACKAGE_ROOT_NAME/dist/activityhelper.exe" > "$tmpdir/activityhelper.exe"

svc_type="$(file "$tmpdir/activitysvc.exe")"
helper_type="$(file "$tmpdir/activityhelper.exe")"
printf '%s\n%s\n' "$svc_type" "$helper_type"
case "$svc_type" in
  *"PE32+ executable (console)"*"x86-64"*"MS Windows"*) ;;
  *)
    printf 'unexpected service binary type: %s\n' "$svc_type" >&2
    exit 1
    ;;
esac
case "$helper_type" in
  *"PE32+ executable (GUI)"*"x86-64"*"MS Windows"*) ;;
  *)
    printf 'unexpected helper binary type: %s\n' "$helper_type" >&2
    exit 1
    ;;
esac

dist_svc_sum="$(shasum -a 256 "$DIST_DIR/activitysvc.exe" | awk '{print $1}')"
zip_svc_sum="$(shasum -a 256 "$tmpdir/activitysvc.exe" | awk '{print $1}')"
dist_helper_sum="$(shasum -a 256 "$DIST_DIR/activityhelper.exe" | awk '{print $1}')"
zip_helper_sum="$(shasum -a 256 "$tmpdir/activityhelper.exe" | awk '{print $1}')"
if [[ "$dist_svc_sum" != "$zip_svc_sum" ]]; then
  printf 'service checksum mismatch between dist and zip\n' >&2
  exit 1
fi
if [[ "$dist_helper_sum" != "$zip_helper_sum" ]]; then
  printf 'helper checksum mismatch between dist and zip\n' >&2
  exit 1
fi

{
  cd "$ROOT"
  shasum -a 256 dist/activitysvc.exe
  shasum -a 256 dist/activityhelper.exe
  shasum -a 256 release/windowsuseruptimecontrol-windows-package.zip
} > "$CHECKSUM_PATH"

printf 'Release bundle ready: %s\n' "$ZIP_PATH"
printf 'Checksums written: %s\n' "$CHECKSUM_PATH"
