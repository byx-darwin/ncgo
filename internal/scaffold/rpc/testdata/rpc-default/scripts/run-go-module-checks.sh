#!/usr/bin/env bash
set -euo pipefail

mode="${1:-}"
case "$mode" in
  vet)
    label='go vet ./...'
    cmd=(go vet ./...)
    ;;
  test)
    label='go test ./... -count=1'
    cmd=(go test ./... -count=1)
    ;;
  build)
    label='go build .'
    cmd=(go build .)
    ;;
  *)
    echo "usage: $0 <vet|test|build>" >&2
    exit 1
    ;;
esac

mods="$(find . -name go.mod -not -path './.git/*' -not -path './vendor/*' | LC_ALL=C sort)"
if [ -z "$mods" ]; then
  echo "no go.mod files found; skipping ${label}"
  exit 0
fi

while IFS= read -r mod; do
  [ -n "$mod" ] || continue
  dir="$(dirname "$mod")"
  echo "==> (cd ${dir} && ${label})"
  (cd "$dir" && "${cmd[@]}")
done <<EOF
$mods
EOF
