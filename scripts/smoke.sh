#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

BIN="$TMP_DIR/ncgo"

log() {
  printf '\n==> %s\n' "$1"
}

write_manifest() {
  local path="$1"
  local module="$2"
  local kind="$3"
  local name="$4"
  mkdir -p "$(dirname "$path")"
  cat >"$path" <<YAML
ncgo:
  version: 0.1.0
  assets_version: 0.1.0
mode: mono
module: $module
service:
  name: $name
  kind: $kind
  idl: idl/app/$name.proto
generated_at: 2026-04-29T00:00:00Z
YAML
}

log "build CLI"
go build -o "$BIN" .

log "version includes build metadata"
"$BIN" version >"$TMP_DIR/version.out"
grep -q 'build:' "$TMP_DIR/version.out"
grep -q 'built:' "$TMP_DIR/version.out"
grep -q 'assets:' "$TMP_DIR/version.out"

log "CLI help exposes lifecycle flags"
"$BIN" upgrade --help >"$TMP_DIR/upgrade-help.out"
"$BIN" extract domain --help >"$TMP_DIR/extract-help.out"
"$BIN" add infra --help >"$TMP_DIR/add-infra-help.out"
grep -q -- '--plan' "$TMP_DIR/upgrade-help.out"
grep -q -- '--apply' "$TMP_DIR/extract-help.out"
grep -q -- '--wire' "$TMP_DIR/add-infra-help.out"
grep -q -- '--dry-run' "$TMP_DIR/add-infra-help.out"
grep -q -- '--output' "$TMP_DIR/add-infra-help.out"
grep -q -- '--plan' "$TMP_DIR/add-infra-help.out"

log "MCP tools/list exposes expected tools"
NCGO_BIN="$BIN" python3 - <<'PY'
import json
import os
import subprocess

bin_path = os.environ["NCGO_BIN"]
messages = [
    {"jsonrpc": "2.0", "id": 1, "method": "initialize"},
    {"jsonrpc": "2.0", "id": 2, "method": "tools/list"},
]
stdin = b""
for msg in messages:
    body = json.dumps(msg, separators=(",", ":")).encode()
    stdin += body + b"\n"

proc = subprocess.run([bin_path, "mcp", "serve"], input=stdin, stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=True)
raw = proc.stdout
responses = []
for line in raw.split(b"\n"):
    line = line.strip()
    if not line:
        continue
    responses.append(json.loads(line))

names = {tool["name"] for tool in responses[1]["result"]["tools"]}
required = {"ncgo_version", "ncgo_doctor", "ncgo_ai_sync", "ncgo_add_infra", "ncgo_add_method", "ncgo_ai_context"}
missing = sorted(required - names)
if missing:
    raise SystemExit(f"missing MCP tools: {missing}")
PY

log "upgrade --plan is read-only"
UPGRADE_ROOT="$TMP_DIR/upgrade-plan"
write_manifest "$UPGRADE_ROOT/.ncgo/manifest.yaml" github.com/acme/demo hertz demo
cp "$UPGRADE_ROOT/.ncgo/manifest.yaml" "$TMP_DIR/manifest.before.yaml"
"$BIN" upgrade --root "$UPGRADE_ROOT" --plan >"$TMP_DIR/upgrade.out"
grep -q 'upgrade plan for' "$TMP_DIR/upgrade.out"
grep -q '\[change\] metadata' "$TMP_DIR/upgrade.out"
cmp -s "$UPGRADE_ROOT/.ncgo/manifest.yaml" "$TMP_DIR/manifest.before.yaml"

log "ncgo check --help exposes --output"
"$BIN" check --help >"$TMP_DIR/check-help.out"
grep -q -- '--output' "$TMP_DIR/check-help.out"

log "ncgo check passes on a healthy service"
CHECK_ROOT="$TMP_DIR/check-ok"
write_manifest "$CHECK_ROOT/.ncgo/manifest.yaml" github.com/acme/demo hertz demo
cat >>"$CHECK_ROOT/.ncgo/manifest.yaml" <<'YAML'
domains:
  - demo
YAML
mkdir -p "$CHECK_ROOT/internal/usecase/demo"
cat >"$CHECK_ROOT/internal/usecase/demo/demo.go" <<'GO'
package demo

type UseCase struct{}

// ncgo:methods:start
// ncgo:methods:end
GO
"$BIN" check --root "$CHECK_ROOT" >"$TMP_DIR/check-ok.out"
grep -q 'all checks passed' "$TMP_DIR/check-ok.out"

log "ncgo check exits 2 on a non-project root"
"$BIN" check --root "$TMP_DIR" >"$TMP_DIR/check-err.out" 2>&1 && { echo "check should have failed"; exit 1; } || true

log "generated project passes ncgo check (dogfooding)"
GEN_ROOT="$TMP_DIR/gen-check"
"$BIN" new gen-check --module github.com/x/gen-check --kind hertz --no-generate --dir "$GEN_ROOT" >/dev/null
"$BIN" check --root "$GEN_ROOT" >"$TMP_DIR/gen-check.out"
grep -q 'all checks passed' "$TMP_DIR/gen-check.out"

log "broken anchor fails ncgo check (negative)"
# A fresh `ncgo new --no-generate` project writes no internal/usecase files,
# so add one with paired anchors to the generated project before breaking it.
cat >>"$GEN_ROOT/.ncgo/manifest.yaml" <<'YAML'
domains:
  - demo
YAML
mkdir -p "$GEN_ROOT/internal/usecase/demo"
cat >"$GEN_ROOT/internal/usecase/demo/demo.go" <<'GO'
package demo

type UseCase struct{}

// ncgo:methods:start
// ncgo:methods:end
GO
USE_CASE=$(find "$GEN_ROOT/internal/usecase" -name '*.go' | head -1)
test -n "$USE_CASE"
grep -q 'ncgo:methods:start' "$USE_CASE"
"$BIN" check --root "$GEN_ROOT" >/dev/null 2>&1 || { echo "healthy generated project failed ncgo check"; exit 1; }
# Remove the start anchor (portable grep+mv; works on GNU and BSD sed hosts).
grep -v 'ncgo:methods:start' "$USE_CASE" >"$USE_CASE.tmp" && mv "$USE_CASE.tmp" "$USE_CASE"
"$BIN" check --root "$GEN_ROOT" >/dev/null 2>&1 && { echo "check should have failed"; exit 1; } || true

log "add infra redis generates Redis data helper (hertz)"
REDIS_ROOT="$TMP_DIR/add-redis"
write_manifest "$REDIS_ROOT/.ncgo/manifest.yaml" github.com/acme/demo hertz demo
"$BIN" add infra redis --root "$REDIS_ROOT" >"$TMP_DIR/redis.out"
grep -q 'go get github.com/byx-darwin/go-tools/go-middleware' "$TMP_DIR/redis.out"
grep -q 'go mod tidy' "$TMP_DIR/redis.out"
grep -q 'type Redis struct' "$REDIS_ROOT/internal/base/data/redis.go"
grep -q 'func SharedRedisClient' "$REDIS_ROOT/internal/base/data/redis_shared.go"

log "add infra logging generates common core and framework adapters"
LOGGING_HERTZ_ROOT="$TMP_DIR/add-logging-hertz"
write_manifest "$LOGGING_HERTZ_ROOT/.ncgo/manifest.yaml" github.com/acme/demo hertz demo
"$BIN" add infra logging --root "$LOGGING_HERTZ_ROOT" >"$TMP_DIR/logging-hertz.out"
grep -q 'internal/base/logging/logging.go' "$TMP_DIR/logging-hertz.out"
grep -q 'internal/base/logging/hertz.go' "$TMP_DIR/logging-hertz.out"
grep -q 'goclog "github.com/byx-darwin/go-tools/go-common/log"' "$LOGGING_HERTZ_ROOT/internal/base/logging/logging.go"
grep -q 'func HertzAccessLog' "$LOGGING_HERTZ_ROOT/internal/base/logging/hertz.go"

LOGGING_WIRE_ROOT="$TMP_DIR/add-logging-wire"
write_manifest "$LOGGING_WIRE_ROOT/.ncgo/manifest.yaml" github.com/acme/demo hertz demo
mkdir -p "$LOGGING_WIRE_ROOT/internal/base/server"
cat >"$LOGGING_WIRE_ROOT/internal/base/server/server.go" <<'GO'
package server

import (
	"time"

	"github.com/cloudwego/hertz/pkg/app/server"
	"github.com/samber/do/v2"

	"github.com/acme/demo/internal/base/conf"
	"github.com/acme/demo/internal/pkg/middleware"
)

func Run() {
	injector := do.New()
	cfg := conf.Get()
	if cfg == nil {
		cfg = conf.Default()
	}
	do.ProvideValue(injector, cfg)
	h := server.Default()
	h.Use(middleware.Recovery())
	h.Use(middleware.RequestID())
	h.Use(middleware.AccessLog())
	h.Use(middleware.RequestTimeout(time.Second))
}
GO
"$BIN" add infra logging --root "$LOGGING_WIRE_ROOT" --wire --dry-run >"$TMP_DIR/logging-wire-dry-run.out"
grep -q 'would wire .*internal/base/server/server.go' "$TMP_DIR/logging-wire-dry-run.out"
grep -q 'dry-run: no files were written' "$TMP_DIR/logging-wire-dry-run.out"
test ! -e "$LOGGING_WIRE_ROOT/internal/base/logging/logging.go"
grep -q 'h.Use(middleware.AccessLog())' "$LOGGING_WIRE_ROOT/internal/base/server/server.go"
"$BIN" add infra logging --root "$LOGGING_WIRE_ROOT" --wire >"$TMP_DIR/logging-wire.out"
grep -q 'wired .*internal/base/server/server.go' "$TMP_DIR/logging-wire.out"
grep -q 'logging.HertzAccessLog' "$LOGGING_WIRE_ROOT/internal/base/server/server.go"
grep -q 'internal/base/logging' "$LOGGING_WIRE_ROOT/internal/base/server/server.go"

LOGGING_KITEX_ROOT="$TMP_DIR/add-logging-kitex"
write_manifest "$LOGGING_KITEX_ROOT/.ncgo/manifest.yaml" github.com/acme/demo kitex demo
"$BIN" add infra observability_logging --root "$LOGGING_KITEX_ROOT" >"$TMP_DIR/logging-kitex.out"
grep -q 'internal/base/logging/logging.go' "$TMP_DIR/logging-kitex.out"
grep -q 'internal/base/logging/kitex.go' "$TMP_DIR/logging-kitex.out"
grep -q 'func KitexAccessLog' "$LOGGING_KITEX_ROOT/internal/base/logging/kitex.go"

log "add infra canary generates release helper"
CANARY_ROOT="$TMP_DIR/add-canary"
write_manifest "$CANARY_ROOT/.ncgo/manifest.yaml" github.com/acme/demo kitex demo
"$BIN" add infra canary --root "$CANARY_ROOT" >"$TMP_DIR/canary.out"
grep -q 'internal/base/release/canary.go' "$TMP_DIR/canary.out"
grep -q 'internal/base/release/kitex.go' "$TMP_DIR/canary.out"
grep -q 'type RuleSet struct' "$CANARY_ROOT/internal/base/release/canary.go"
grep -q 'type Selector struct' "$CANARY_ROOT/internal/base/release/canary.go"
grep -q 'func Select(' "$CANARY_ROOT/internal/base/release/canary.go"
grep -q 'func SplitInstances' "$CANARY_ROOT/internal/base/release/canary.go"
grep -q 'ProviderNacos' "$CANARY_ROOT/internal/base/release/canary.go"
grep -q 'ProviderPolaris' "$CANARY_ROOT/internal/base/release/canary.go"
grep -q 'func KitexTraffic' "$CANARY_ROOT/internal/base/release/kitex.go"

CANARY_HERTZ_ROOT="$TMP_DIR/add-canary-hertz"
write_manifest "$CANARY_HERTZ_ROOT/.ncgo/manifest.yaml" github.com/acme/demo hertz demo
"$BIN" add infra release_canary --root "$CANARY_HERTZ_ROOT" >"$TMP_DIR/canary-hertz.out"
grep -q 'internal/base/release/canary.go' "$TMP_DIR/canary-hertz.out"
grep -q 'internal/base/release/hertz.go' "$TMP_DIR/canary-hertz.out"
grep -q 'func HertzTraffic' "$CANARY_HERTZ_ROOT/internal/base/release/hertz.go"

log "extract domain --apply copies files and rewrites imports"
EXTRACT_ROOT="$TMP_DIR/extract-apply"
write_manifest "$EXTRACT_ROOT/.ncgo/manifest.yaml" github.com/acme/demo hertz demo
cat >>"$EXTRACT_ROOT/.ncgo/manifest.yaml" <<'YAML'
domains:
  - device
YAML
mkdir -p "$EXTRACT_ROOT/internal/usecase/device" "$EXTRACT_ROOT/internal/repository/device" "$EXTRACT_ROOT/internal/base/data"
cat >"$EXTRACT_ROOT/internal/usecase/device/device.go" <<'GO'
package device

import _ "github.com/acme/demo/internal/repository/device"
GO
cat >"$EXTRACT_ROOT/internal/repository/device/device.go" <<'GO'
package devicerepo
GO
cat >"$EXTRACT_ROOT/internal/base/data/device_register.go" <<'GO'
package data

import _ "github.com/acme/demo/internal/usecase/device"
GO
write_manifest "$EXTRACT_ROOT/services/device-rpc/.ncgo/manifest.yaml" github.com/acme/device-rpc kitex device-rpc
"$BIN" extract domain device --root "$EXTRACT_ROOT" --apply >"$TMP_DIR/extract.out"
grep -q 'applied extraction' "$TMP_DIR/extract.out"
grep -q 'github.com/acme/device-rpc/internal/repository/device' "$EXTRACT_ROOT/services/device-rpc/internal/usecase/device/device.go"
if grep -q 'github.com/acme/demo/internal/repository/device' "$EXTRACT_ROOT/services/device-rpc/internal/usecase/device/device.go"; then
  echo "source module import was not rewritten" >&2
  exit 1
fi

log "smoke OK"
