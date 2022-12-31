#!/bin/sh

set -euo pipefail

: ${MODTIME:=30m}

readonly wd="$(dirname $0)"
readonly cf=".go-update"
cd "$wd" >/dev/null

presubmit() { git go check ; }

find . -depth 2 -type f -name "$cf" -mtime +"$MODTIME" -print | \
    cut -d/ -f2 |  while read -r pkg ; do
    (
        cd "$pkg"
        . "$cf"
        git pull
        find . -type f -name go.mod -print | while read -r mod ; do
            echo "-- $mod" 1>&2
            (
                cd "$(dirname $mod)"
                go get -u ./...
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
