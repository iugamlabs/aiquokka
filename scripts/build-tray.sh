#!/bin/sh
# Build a separate desktop distribution without changing the default CLI.
set -eu

tray_os="${GOOS:-$(go env GOOS)}"
tray_arch="${GOARCH:-$(go env GOARCH)}"
case "$tray_os/$tray_arch" in
  linux/amd64|linux/arm64|darwin/amd64|darwin/arm64) ;;
  *) echo "unsupported tray target: $tray_os/$tray_arch" >&2; exit 1 ;;
esac
if [ "$tray_os" = darwin ] && [ "${CGO_ENABLED:-$(go env CGO_ENABLED)}" != 1 ]; then
  echo 'macOS tray builds require CGO_ENABLED=1 and the macOS SDK' >&2
  exit 1
fi

tray_label="$tray_os"
if [ "$tray_os" = darwin ]; then tray_label=macos; fi
tray_binary="aiquokka-tray-$tray_label-$tray_arch"
tray_dest="${TRAY_DIST_DIR:-dist/tray}"
mkdir -p "$tray_dest"
tray_stage=$(mktemp -d "$tray_dest/.stage.XXXXXX")
trap 'rm -rf "$tray_stage"' EXIT HUP INT TERM
tray_version="${AIQUOKKA_VERSION:-dev}"
tray_version="${tray_version#v}"
tray_commit=$(git rev-parse HEAD)
tray_date=$(date -u +%Y-%m-%dT%H:%M:%SZ)
tray_info=github.com/iugamlabs/aiquokka/internal/buildinfo
GOOS="$tray_os" GOARCH="$tray_arch" go build -tags tray -trimpath \
  -ldflags "-s -w -X $tray_info.Version=$tray_version -X $tray_info.Commit=$tray_commit -X $tray_info.Date=$tray_date" \
  -o "$tray_stage/$tray_binary" .
mkdir -p "$tray_stage/licenses"
cp LICENSE "$tray_stage/LICENSE"
cp third_party/NOTICE.md "$tray_stage/THIRD_PARTY_NOTICES.md"
cp third_party/systray/LICENSE "$tray_stage/licenses/systray-LICENSE"
cp third_party/systray/PATCHES.md "$tray_stage/licenses/systray-PATCHES.md"
cp third_party/LOBEHUB-LICENSE "$tray_stage/licenses/LOBEHUB-LICENSE"
tar -czf "$tray_dest/$tray_binary.tar.gz" -C "$tray_stage" .
echo "$tray_dest/$tray_binary.tar.gz"
