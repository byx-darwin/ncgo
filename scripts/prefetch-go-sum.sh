#!/usr/bin/env bash
# Populate the module cache from every exact module version recorded in a
# reviewed go.sum. This includes dependencies needed only while `go mod tidy`
# examines dependency tests, which `go mod download all` does not always fetch.
set -euo pipefail

if [[ $# -ne 1 || ! -f "$1" ]]; then
  echo "usage: $0 <go.sum>" >&2
  exit 2
fi

module_list="$(mktemp)"
trap 'rm -f "$module_list"' EXIT

# Archive checksum entries identify the exact module versions that can provide
# packages. Skip go.mod-only history: downloading those archives would be both
# unnecessary and very expensive for long-lived dependency graphs.
awk '$2 !~ /\/go.mod$/ { print $1 "@" $2 }' "$1" \
  | LC_ALL=C sort -u >"$module_list"

if [[ -s "$module_list" ]]; then
  xargs -n 20 -P 8 go mod download <"$module_list"
fi
