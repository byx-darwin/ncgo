#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo unknown)"
BUILD_TIME="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

go install -ldflags "\
  -X github.com/byx-darwin/ncgo/internal/cli.BuildVersion=dev-${COMMIT} \
  -X github.com/byx-darwin/ncgo/internal/cli.BuildTime=${BUILD_TIME}" \
  .

echo "Installed: $(command -v ncgo)"
ncgo version
