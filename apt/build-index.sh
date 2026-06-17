#!/bin/sh
# Regenerates dists/ from pool/ and GPG-signs the Release.
# Usage: build-index.sh <pages-dir> <apt-ftparchive.conf> <gpg-key-id>
set -eu

PAGES="$1"
CONF="$2"
KEYID="$3"

CONF_ABS="$(readlink -f "$CONF")"
cd "$PAGES"

rm -rf dists
mkdir -p dists/stable/main/binary-amd64 dists/stable/main/binary-arm64
apt-ftparchive generate "$CONF_ABS"

mkdir -p dists/stable
apt-ftparchive -c="$CONF_ABS" release dists/stable > dists/stable/Release

rm -f dists/stable/InRelease dists/stable/Release.gpg
gpg --batch --yes --default-key "$KEYID" --clearsign -o dists/stable/InRelease dists/stable/Release
gpg --batch --yes --default-key "$KEYID" -abs -o dists/stable/Release.gpg dists/stable/Release
