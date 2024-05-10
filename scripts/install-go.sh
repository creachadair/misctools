#!/usr/bin/env bash
##
## Usage: install-go.sh [<version>]
##  e.g.: install-go.sh 1.22.3
##
## If a version is omitted, the latest available release is installed.
##
## Environment variables:
##
##   GOOS/GOARCH: Which OS and architecture to install for.
##   INSTALL_DIR: Where to install the Go toolchain.
##
## You may also override GOOS and GOARCH.
##
set -euo pipefail

goversion="${1:-latest}"
if [[ "$goversion" = latest ]] ; then
    goversion="$(
      curl -sL https://golang.org/VERSION?m=text |
      head -1 |
      cut   -c3-
    )"
    echo "Latest Go release is $goversion" 1>&2
elif [[ -z "$goversion" ]] ; then
    echo "Missing required <version> argument (e.g., 1.22.3)" 1>&2
    exit 2
fi

: ${GOOS:="$(uname -s|tr A-Z a-z)"}
: ${GOARCH:="$(uname -m)"}
: ${INSTALL_DIR:=/usr/local/go}

# What Linux calls x86_64, Go calls amd64.
if [[ "$GOARCH" = x86_64 ]] ; then
    GOARCH=amd64
fi

vdir="${INSTALL_DIR}/_v"
target="${vdir}/${goversion}"
mkdir -p "$vdir"

setlink() {
    rm -f -- "${INSTALL_DIR}/current"
    ln -s -f "_v/${goversion}" "${INSTALL_DIR}/current"
}

if [[ ! -d "$target" ]] ; then
    # Fetch the tarball for the specified distribution.
    echo "- fetching Go ${goversion} ..." 1>&2
    dist="https://go.dev/dl/go${goversion}.${GOOS}-${GOARCH}.tar.gz"
    mkdir -p "$target"
    curl -sL "$dist" | tar -C "$target" -x --strip-components=1
else
    echo "- Go ${goversion} is already installed" 1>&2
fi
echo "- updating current link to ${goversion}" 1>&2
setlink

