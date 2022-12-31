#!/bin/sh
#
# Update Go modules in designated subdirectories.
#
# To install: Copy or link this script into a directory with subdirectories
# that contain Git repositories holding Go packages.
#
# In order to be updated, a subdirectory must contain a .go-update file at the
# top level. This file is sourced into the the update script.
#
# -- Overridable definitions:
#
#    update_mod  -- update Go module dependencies
#                   Default: go get -u ./...
#    presubmit   -- run tests prior to pushing an update
#                   Default: go mod check
#
# -- Environment:
#
#  MODTIME   -- how long between updates (default: 1 day)
#
set -euo pipefail

: ${MODTIME:=1d}
: ${MATCH:=}

unset CDPATH

readonly wd="$(dirname $0)"
readonly cf=".go-update"
cd "$wd" >/dev/null

update_mod() { go get -u ./... ; }
presubmit() { git go check ; }

find . -depth 2 -type f -path "*${MATCH}/$cf" -mtime +"$MODTIME" -print | \
    cut -d/ -f2 |  while read -r pkg ; do
    (
        cd "$pkg"
        printf "<> \033[1;93m%s\033[0m\n" "$pkg"
        . "$cf"
        git pull
        find . -type f -name go.mod -print | while read -r mod ; do
            echo "-- $mod" 1>&2
            (
                cd "$(dirname $mod)"
                update_mod
                go mod tidy
            )
        done

        if ! git diff --quiet ; then
            presubmit
            git commit -m "Update module dependencies." .
            git push --no-verify
            touch "$cf"
        fi
    )
done

# Clean up test and example binaries that get installed by Go upgrades, that
# aren't really meant to be installed for general use.
( cd "$GOBIN" && rm -vf -- \
   adder client copytree examples http mkenum mktree mktype server wshttp
)
