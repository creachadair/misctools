#!/usr/bin/env bash
##
## Usage: install-go.sh [<version>]
##  e.g.: install-go.sh 1.22.3
##
## If a version is omitted, the latest available release is installed.
##
## By default, the script installs a precompiled toolchain for the specified OS
## and architecture. Set SOURCE=1 to build from source instead. Source builds
## require a bootstrap toolchain, which the script will fetch if needed.
##
## Environment variables:
##
##   GOOS/GOARCH: Which OS and architecture to install for.
##   INSTALL_DIR: Where to install the Go toolchain.
##   BOOTSTRAP: The Go toolchain version to use for source bootstrap.
##   SOURCE: If nonzero, build from source.
##
set -euo pipefail

goversion="${1:-latest}"
if [[ "$goversion" = latest ]] ; then
    goversion="$(
      curl -sL https://golang.org/VERSION?m=text |
      head -1 |
      cut -c3-
    )"
    echo "Latest Go release is $goversion" 1>&2
elif [[ -z "$goversion" ]] ; then
    echo "Missing required <version> argument (e.g., 1.22.3)" 1>&2
    exit 2
fi

: ${GOOS:="$(uname -s|tr A-Z a-z)"}
: ${GOARCH:="$(uname -m)"}
: ${BOOTSTRAP:=1.20.14}
: ${SOURCE:=0}
: ${INSTALL_DIR:=/usr/local/go}

# What Linux calls x86_64, Go calls amd64.
# What Linux calls aarch64, Go calls arm64.
if [[ "$GOARCH" = x86_64 ]] ; then
    GOARCH=amd64
elif [[ "$GOARCH" = aarch64 ]] ; then
    GOARCH=arm64
fi

# If we are doing a source build, use _x as the target.
# Otherwise use _v for binary versions.
suffix=_v
if [[ "$SOURCE" -ne 0 ]] ; then
    suffix=_x
    echo "* Requested build from source." 1>&2
fi
vdir="${INSTALL_DIR}/${suffix}"
target="${vdir}/${goversion}"
mkdir -p "$vdir"

if [[ -d "$target" ]] ; then
    echo "- Go ${goversion} is already installed" 1>&2
elif [[ "$SOURCE" -ne 0 ]] ; then
    # Set up a bootstrap toolchain if necessary.
    if ! which go &>/dev/null ; then
        echo "* Go toolchain not found; installing bootstrap v${BOOTSTRAP} ..." 1>&2
        tmp="$(mktemp -p "${TMPDIR:-}" -d gobootstrap.XXXXXXXXXX)"
        trap "rm -fr -- '$tmp'" EXIT
        curl -sL "https://go.dev/dl/go${BOOTSTRAP}.${GOOS}-${GOARCH}.tar.gz" |
            tar -C "$tmp" -xz --strip-components=1
        export GOROOT_BOOTSTRAP="${tmp}"
    else
        echo "* Toolchain $(go version) found" 1>&2
    fi

    # Fetch the source tarball for the specified version.
    echo "- fetching Go ${goversion} source ..." 1>&2
    dist="https://go.dev/dl/go${goversion}.src.tar.gz"
    mkdir -p "$target"
    curl -sL "$dist" | tar -C "$target" -xz --strip-components=1

    echo "- building Go ${goversion} ..." 1>&2
    pushd "${target}/src"
    ./make.bash
    popd
else
    # Fetch the binary tarball for the specified version (no build required).
    echo "- fetching Go ${goversion} ..." 1>&2
    dist="https://go.dev/dl/go${goversion}.${GOOS}-${GOARCH}.tar.gz"
    mkdir -p "$target"
    curl -sL "$dist" | tar -C "$target" -xz --strip-components=1
fi

setlink() {
    rm -f -- "${INSTALL_DIR}/current"
    ln -s -f "${suffix}/${goversion}" "${INSTALL_DIR}/current"
}

echo "- updating current link to ${goversion}" 1>&2
setlink
