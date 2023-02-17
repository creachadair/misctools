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
#                   Default: go get -u -t ./...
#    presubmit   -- run tests prior to pushing an update
#                   Default: git go check
#    push        -- push to the remote repository
#                   Default: git push --no-verify
#    cleanup     -- run after update completes
#                   Default: (empty)
#
# -- Environment:
#
#  MODTIME   -- how long between updates (default: 5 days)
#  MATCH     -- update matching directories (default: all)
#
set -euo pipefail

: ${MODTIME:=5d}
: ${MATCH:=}

unset CDPATH

readonly wd="$(dirname $0)"
readonly cf=".go-update"
cd "$wd" >/dev/null

update_mod() { go get -u -t ./... ; }
presubmit()  { git go check ; }
cleanup()    { : ; }
push()       { git push --no-verify ; }

find_matching() {
    find . -depth 2 -type f -path "*${MATCH}/$cf" -mtime +"$MODTIME" \
         -exec stat -f '%m %N' {} ';' | sort -n | cut -d/ -f2
}

case "${1:-}" in
    ('')
        ;;
    (--list|-list)
        find_matching
        exit 0
        ;;
    (*)
        echo "Usage: update.sh [--list]" 1>&2
        exit 2
        ;;
esac

find_matching | while read -r pkg ; do
    (
        cd "$pkg"
        printf "<> \033[1;93m%s\033[0m\n" "$pkg"
        . "$cf"

        # Before doing anything destructive, make sure the working directory is clean.
        if ! git diff --quiet ; then
            printf " !! \033[1;91mThere are uncommitted changes in %s\033[0m\n" "$pkg"
            printf "    Please stash or commit them before upgrading.\n"
            exit 1
        fi

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
