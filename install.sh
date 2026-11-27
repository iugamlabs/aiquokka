#!/bin/sh
# Install aiquokka from the published iugamlabs release.
#
# Picks the CLI binary for this OS and architecture, verifies its sha256
# checksum against SHA256SUMS, and moves it into INSTALL_DIR (~/.local/bin).
#
#   curl -fsSL https://raw.githubusercontent.com/iugamlabs/aiquokka/main/install.sh | sh
#
# AIQUOKKA_VERSION pins a release tag instead of the latest; INSTALL_DIR
# changes where the binary goes; AIQUOKKA_DOWNLOAD_BASE changes where the
# release files come from (the test points it at local fixtures).
set -eu

say() { printf '%s\n' "$*"; }
die() { printf 'aiquokka: %s\n' "$*" >&2; exit 1; }

command -v curl >/dev/null 2>&1 || die "need curl to download the release"

os=$(uname -s)
case "$os" in
  Darwin) os=macos ;;
  Linux) os=linux ;;
  *) die "unsupported OS: $os (releases cover macos and linux; Windows uses Scoop)" ;;
esac

arch=$(uname -m)
case "$arch" in
  arm64 | aarch64) arch=arm64 ;;
  amd64 | x86_64) arch=amd64 ;;
  *) die "unsupported architecture: $arch (releases cover arm64 and amd64)" ;;
esac

repo=iugamlabs/aiquokka
if [ -n "${AIQUOKKA_VERSION:-}" ]; then
  version=$AIQUOKKA_VERSION
else
  latest=$(curl -fsSL -o /dev/null -w '%{url_effective}' \
    "https://github.com/$repo/releases/latest") \
    || die "could not find the latest release (is one published yet?)"
  version=${latest##*/}
  case "$version" in
    "" | releases | latest) die "could not find the latest release (is one published yet?)" ;;
  esac
fi

base=${AIQUOKKA_DOWNLOAD_BASE:-https://github.com/$repo/releases/download}
binary=aiquokka-$os-$arch

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT TERM

curl -fsSL -o "$tmp/$binary" "$base/$version/$binary" \
  || die "could not download $binary from release $version"
curl -fsSL -o "$tmp/SHA256SUMS" "$base/$version/SHA256SUMS" \
  || die "could not download SHA256SUMS from release $version"

# Verify before anything is installed, so a failure leaves nothing behind.
want=$(awk -v a="$binary" '$NF == a { print $1; exit }' "$tmp/SHA256SUMS")
[ -n "$want" ] || die "no checksum for $binary in release $version"
if command -v sha256sum >/dev/null 2>&1; then
  sum=$(sha256sum "$tmp/$binary")
elif command -v shasum >/dev/null 2>&1; then
  sum=$(shasum -a 256 "$tmp/$binary")
else
  die "need sha256sum or shasum to verify the download"
fi
have=${sum%% *}
[ "$have" = "$want" ] || die "checksum mismatch for $binary, refusing to install"

dest=${INSTALL_DIR:-$HOME/.local/bin}
mkdir -p "$dest"
if [ -d "$dest/aiquokka" ]; then
  die "cannot install: $dest/aiquokka is a directory"
fi
chmod +x "$tmp/$binary"
mv "$tmp/$binary" "$dest/aiquokka"
say "installed aiquokka $version to $dest/aiquokka"

case ":${PATH:-}:" in
  *":$dest:"*) ;;
  *) say "note: $dest is not on PATH, so 'aiquokka' will not be found until you add it" ;;
esac
