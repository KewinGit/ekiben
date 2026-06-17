#!/bin/sh
# End-to-end repo generation into a gh-pages working tree.
# Usage: gen-repo.sh <debs-dir> <pages-dir>
# Assumes the signing key is already in the gpg keyring.
set -eu

DEBS="$1"
PAGES="$2"
KEYID="ekiben repository signing"
HERE="$(cd "$(dirname "$0")" && pwd)"

mkdir -p "$PAGES/pool/main/e/ekiben"
cp "$DEBS"/*.deb "$PAGES/pool/main/e/ekiben/"

gpg --export "$KEYID" > "$PAGES/ekiben.gpg"
sh "$HERE/build-keyring-deb.sh" "$PAGES/ekiben.gpg" "$PAGES/ekiben-archive-keyring.deb"
sh "$HERE/build-index.sh" "$PAGES" "$HERE/apt-ftparchive.conf" "$KEYID"
cp "$HERE/index.html.tmpl" "$PAGES/index.html"

find "$PAGES" -name '*.db' -delete
