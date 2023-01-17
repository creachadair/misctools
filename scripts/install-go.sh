#!/bin/bash
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
set -e
set -o pipefail

GO_VERSION="${1:?Missing Go version}"
: ${GOOS:="$(uname -s|tr A-Z a-z)"}
: ${GOARCH:="$(uname -m)"}
: ${GOARCH_BOOTSTRAP:=amd64}
: ${BUILD_DIR:=/usr/local/go/build}
: ${INSTALL_DIR:=/usr/local/go}
: ${LABEL:="go${GO_VERSION}"}

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
    # Apple Silicon appears not to bootstrap with versions < 1.16.
    if [[ "$(uname -p)" = arm ]] ; then
	readonly bootstrap_version=1.16
    else
	# Versions of macOS > 11 require a newer bootstrap due to changes in the
	# handling of dynamic libraries.
	readonly bootstrap_version=1.11
    fi
else
    readonly bootstrap_version=1.7
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
