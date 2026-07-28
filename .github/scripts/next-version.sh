#!/usr/bin/env bash
#
# Print the tag the next release should carry.
#
#   next-version.sh patch 1.2.3   -> 1.2.4
#   next-version.sh minor 1.2.3   -> 1.3.0
#   next-version.sh major 1.2.3   -> 2.0.0
#
# Tags carry the bare semantic version, no `v` prefix: 1.2.3, not v1.2.3.
#
# With no current version, the newest version tag in the repository is used, and
# a repository without any tag starts from 0.0.0 — so the first patch release is
# 0.0.1 and the first minor one is 0.1.0.
set -euo pipefail

usage() {
    echo "usage: $(basename "$0") <major|minor|patch> [current-version]" >&2
    exit 2
}

bump="${1:-}"
current="${2:-}"

case "$bump" in
    major | minor | patch) ;;
    *) usage ;;
esac

if [ -z "$current" ]; then
    # --sort=-version:refname orders 1.10.0 after 1.9.0, which a plain sort does not.
    current="$(git tag --list '[0-9]*' --sort=-version:refname | head -n1)"
fi
if [ -z "$current" ]; then
    current="0.0.0"
fi

# A leading `v` is tolerated on input, so an old tag passed by hand still reads,
# but it is never written back out.
if [[ ! "$current" =~ ^v?([0-9]+)\.([0-9]+)\.([0-9]+)$ ]]; then
    echo "cannot read a version out of '$current'; expected something like 1.2.3" >&2
    exit 1
fi
major="${BASH_REMATCH[1]}"
minor="${BASH_REMATCH[2]}"
patch="${BASH_REMATCH[3]}"

case "$bump" in
    major) major=$((major + 1)); minor=0; patch=0 ;;
    minor) minor=$((minor + 1)); patch=0 ;;
    patch) patch=$((patch + 1)) ;;
esac

printf '%d.%d.%d\n' "$major" "$minor" "$patch"
