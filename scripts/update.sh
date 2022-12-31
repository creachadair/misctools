#!/bin/sh

set -euo pipefail

readonly wd="$(dirname $0)"
readonly cf=".go-update"
cd "$wd" >/dev/null

update() {
    for pkg in "$@" ; do
	~/software/shell/go-get-u.sh "$pkg"
    done
}

find . -depth 2 -type f -name "$cf" -print | cut -d/ -f2 | while read -r pkg ; do
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
            git go check
            git commit -m "Update module dependencies."
            git push --no-verify
        fi
    )
done

# Clean up test and example binaries that get installed by Go upgrades, that
# aren't really meant to be installed for general use.
( cd "$GOBIN" && rm -vf -- \
   adder client copytree examples http mkenum mktree mktype server wshttp
)
