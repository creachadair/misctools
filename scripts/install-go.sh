#!/usr/bin/env bash
##
## Usage: install-go.sh <version>
##  e.g.: install-go.sh 1.19.5
##
## To install an arbitrary remote branch, set LABEL to the name of the branch:
##
##   LABEL=origin/branch-name install.go.sh local-name
##
## Environment variables:
##
##   BUILD_DIR:   Where to check out source and build.
##   INSTALL_DIR: Where to install the Go toolchain.
##   LABEL:       The tag or branch name to check out (tagname, origin/branchname).
##
## You may also override GOOS and GOARCH.
##
set -euo pipefail

GO_VERSION="${1:?Missing Go version}"
: ${GOOS:="$(uname -s|tr A-Z a-z)"}
: ${GOARCH:="$(uname -m)"}
: ${GOARCH_BOOTSTRAP:=amd64}
: ${BUILD_DIR:=/usr/local/go/build}
: ${INSTALL_DIR:=/usr/local/go}
: ${LABEL:="go${GO_VERSION}"}

# What Linux calls x86_64, Go calls amd64.
if [[ "$GOARCH" = x86_64 ]] ; then
    GOARCH=amd64
fi

# The target directory where the code will be installed.
readonly target="${INSTALL_DIR}/${GO_VERSION}"

setlink() {
    rm -f -- "${INSTALL_DIR}/current"
    ln -s -f "$GO_VERSION" "${INSTALL_DIR}/current"
}

if [[ -d "$target" ]] ; then
    echo "--
-- Updating the current link to $target
--
-- Note: Go ${GO_VERSION} is already installed at $target
--       Remove or rename that directory if you want to force reinstallation
--" 1>&2
    setlink
    exit 1
fi

# To build a version of Go, we need a bootstrap compiler.
# This should not need to change unless the bootstrap requirements change.
if [[ "$(uname -s)" = Darwin ]] ; then
    # Bug workaround: https://github.com/golang/go/issues/59026
    export DSYMUTIL_REPRODUCER_PATH=/dev/null

    # Apple silicon wants a fairly recent toolchain to build anything, so just
    # default to that on Darwin generally.
    readonly bootstrap_version=1.20
elif [[ "$GO_VERSION" < 1.17 ]] ; then
    readonly bootstrap_version=1.7
elif [[ "$GO_VERSION" < 1.22 ]] ; then
    readonly bootstrap_version=1.17
else
    # Versions 1.22 and later require Go 1.20 to bootstrap.
    readonly bootstrap_version=1.20
fi
readonly bootstrap_tar="go${bootstrap_version}.${GOOS}-${GOARCH_BOOTSTRAP}.tar.gz"
readonly bootstrap_url="https://storage.googleapis.com/golang/${bootstrap_tar}"

# The location of the main Go repository.
readonly golang_url='https://github.com/golang/go'
readonly branch="build-${GO_VERSION}"

# Ensure we have a checkout of the main Go repository and a bootstrap compiler.
readonly repo_dir="${BUILD_DIR}/repo"
readonly bootstrap_dir="${BUILD_DIR}/bootstrap"
mkdir -p "$BUILD_DIR"
if [[ ! -d "$repo_dir" ]] ; then
    git -C "$BUILD_DIR" clone "$golang_url" repo
else
    git -C "$repo_dir" fetch origin
fi
if [[ ! -d "$bootstrap_dir" ]] ; then
    echo ">> Installing bootstrap compiler ${bootstrap_tar} ..." 1>&2
    mkdir -p "$bootstrap_dir"
    curl -L "$bootstrap_url" | tar -C "$bootstrap_dir" -xz
fi
export GOROOT_BOOTSTRAP="${bootstrap_dir}/go"

# Check out or update the build branch, and update the clone.
git -C "$repo_dir" checkout -B "$branch" "$LABEL"
git -C "$repo_dir" fetch

# Build. Note that we need to set GOROOT_FINAL so paths embedded in the built
# binaries will be correct, but this doesn't actually install the binaries.
export GOOS GOARCH GOROOT_FINAL="$INSTALL_DIR"
(cd "${repo_dir}/src" && ./all.bash)

# Install.

mkdir -p "$target"
tar -C "$repo_dir" -c -f- bin pkg src | tar -C "$target" -x
setlink
