#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ge 1 && "$1" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  go run ./cmd/rel "$@"
  # AC122 Part B: re-install the binary AFTER the release commit is made so
  # the installed binary's vcs.revision matches the new release commit.
  # Without this, binaries installed during the release-prep build pipeline
  # carry programVersion (post-bump) + vcs.revision (pre-commit) — a stale
  # mismatched pair.
  go install ./cmd/governa
  installed_rev=$(go version -m "$(go env GOPATH)/bin/governa" | awk '$1=="vcs.revision"{print $2}')
  head_rev=$(git rev-parse HEAD)
  if [ "$installed_rev" != "$head_rev" ]; then
    echo "ERROR: installed binary's vcs.revision ($installed_rev) does not match HEAD ($head_rev) — Part B regression" >&2
    exit 1
  fi
  exit 0
fi

exec go run ./cmd/build "$@"
