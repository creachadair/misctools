#!/usr/bin/env bash
#
# Build the SQLite3 CLI from source for linux/amd64.
#
# Optional parameters:
#
#   BUILDBASE: base image (e.g., ubuntu:24.04)
#   PLATFORM:  os/arch    (e.g., linux/amd64)
#
set -euo pipefail

# Find the latest release version from the SQLite download page.  The easiest
# way seems to be to grab the embedded product data.
base='https://www.sqlite.org'
latest="$(
  curl -Ls $base/download.html | \
    grep ^PRODUCT | grep sqlite-autoconf- | \
    cut -d, -f3
)"
year="$(echo "$latest" | cut -d/ -f1)"
vers="$(echo "$latest" | cut -d- -f3 | cut -d. -f1)"

img=sqlite3-builder:latest
buildbase="${BUILDBASE:-ubuntu:24.04}"
plat="${PLATFORM:-linux/amd64}"
out=./sqlite3-"$vers"
dl="$base/$latest"

cat <<EOF | docker build --platform="$plat" -t "$img" -
FROM "$buildbase" AS builder

WORKDIR /root

RUN apt-get update && apt-get install -y build-essential file libreadline-dev zlib1g-dev curl
RUN curl -sL "$dl" | tar xzv
RUN cd sqlite-autoconf-"$vers" && \
  ./configure CFLAGS="-DSQLITE_ENABLE_DBSTAT_VTAB=1" \
     --enable-readline && make -j
EOF

c="$(docker create --platform=$plat $img)"
trap "docker rm $c; docker image rm $img" EXIT
mkdir -p "$(dirname "$out")"
docker cp "$c":/root/sqlite-autoconf-"$vers"/sqlite3 "$out"
chmod +x "$out"
