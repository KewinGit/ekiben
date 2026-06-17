#!/bin/sh
# Builds ekiben-archive-keyring.deb from an exported public key.
# Usage: build-keyring-deb.sh <pubkey.gpg> <out.deb>
set -eu

PUBKEY="$1"
OUT="$2"
VERSION="${KEYRING_VERSION:-1.0}"

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

mkdir -p "$WORK/DEBIAN" \
         "$WORK/usr/share/keyrings" \
         "$WORK/etc/apt/sources.list.d"

cp "$PUBKEY" "$WORK/usr/share/keyrings/ekiben-archive-keyring.gpg"

cat > "$WORK/etc/apt/sources.list.d/ekiben.sources" <<'EOF'
Types: deb
URIs: https://kewingit.github.io/ekiben
Suites: stable
Components: main
Architectures: amd64 arm64
Signed-By: /usr/share/keyrings/ekiben-archive-keyring.gpg
EOF

cat > "$WORK/DEBIAN/control" <<EOF
Package: ekiben-archive-keyring
Version: $VERSION
Architecture: all
Maintainer: Kevin Corso <kevin.froster.personal@outlook.it>
Section: utils
Priority: optional
Description: ekiben APT repository keyring and source list
 Installs the GPG keyring and APT source entry needed to install and
 update ekiben from https://kewingit.github.io/ekiben.
EOF

dpkg-deb --build --root-owner-group "$WORK" "$OUT"
