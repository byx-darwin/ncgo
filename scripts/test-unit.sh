#!/usr/bin/env bash
# Hermetic repository checks. Run `go mod download` once before this script;
# every Go command below is forced to use the local toolchain and module cache.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

export GOTOOLCHAIN=local
export GOPROXY=off
export GOSUMDB=off

unformatted="$(git ls-files -z '*.go' | xargs -0 gofmt -l)"
if [[ -n "$unformatted" ]]; then
  echo "Go files need gofmt:" >&2
  echo "$unformatted" >&2
  exit 1
fi

go run ./internal/reference/cmd/generate --check
go vet ./...
go test -short ./... -count=1
go build ./...
./scripts/smoke.sh
