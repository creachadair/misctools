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
# Define shell functions in the .go-update file to replace the default behavior
# for the following steps.
#
#    update_mod  -- update Go module dependencies
#                   Default: go get -u ./...
#    presubmit   -- run tests prior to pushing an update
#                   Default: go mod check
#    push        -- push to the remote repository
#                   Default: git push --no-verify
#    cleanup     -- run after update completes
#                   Default: (empty)
#
# -- Environment:
#
#  MODTIME   -- how long between updates (default: 1 day)
#  MATCH     -- update matching directories (default: all)
#
set -euo pipefail

: ${MODTIME:=1d}
: ${MATCH:=}

unset CDPATH

readonly wd="$(dirname $0)"
readonly cf=".go-update"
cd "$wd" >/dev/null

update_mod() { go get -u ./... ; }
presubmit()  { git go check ; }
cleanup()    { : ; }
push()       { git push --no-verify ; }

find_matching() {
    find -s . -depth 2 -type f -path "*${MATCH}/$cf" -mtime +"$MODTIME" \
         -exec stat -f '%m %N' {} ';' | sort -n | cut -d/ -f2
}

find_matching | while read -r pkg ; do
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
            push
            cleanup
            touch "$cf"
        fi
    )
done
