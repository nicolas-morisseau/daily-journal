#!/usr/bin/env bash
#
# Read a PR's labels and print the release bump they ask for.
#
# The labels arrive as the JSON array GitHub exposes as
# `github.event.pull_request.labels.*.name`, on stdin:
#
#   echo '["minor","docs"]' | bump-from-labels.sh   -> minor
#
# Exactly one bump label is required. Zero is a mistake, and several are
# ambiguous, so both fail with a message aimed at whoever opened the PR.
#
# This is the single definition of the rule: the pull request gate and the
# release both call it, so they cannot disagree about what is mergeable.
set -euo pipefail

readonly BUMP_LABELS=(major minor patch)

labels_json="$(cat)"
if [ -z "$labels_json" ]; then
    labels_json='[]'
fi

# Keep only the labels that mean something to us, one per line.
found="$(jq -r --argjson wanted "$(printf '%s\n' "${BUMP_LABELS[@]}" | jq -R . | jq -sc .)" \
    '[.[] | select(. as $l | $wanted | index($l))] | .[]' <<<"$labels_json")"

count="$(printf '%s' "$found" | grep -c . || true)"

case "$count" in
    1)
        printf '%s\n' "$found"
        ;;
    0)
        cat >&2 <<EOF
This pull request carries no release label.

Add exactly one of: ${BUMP_LABELS[*]}
It decides how the version is bumped when the pull request is merged:

  patch   0.1.3 -> 0.1.4   a fix, no behaviour change
  minor   0.1.3 -> 0.2.0   a new command, option or behaviour
  major   0.1.3 -> 1.0.0   something that breaks an existing usage
EOF
        exit 1
        ;;
    *)
        {
            echo "This pull request carries several release labels:"
            printf '  %s\n' $found
            echo
            echo "Keep exactly one, so the version bump is unambiguous."
        } >&2
        exit 1
        ;;
esac
