#!/usr/bin/env bash
# Compile + unit-test verification gate for the polaris canary adapter assets.
#
# The adapter (internal/assets/_data/kitex/optional/polaris_canary_adapter.go)
# is the ONLY place ncgo reconciles with the real polaris-go SDK API. ncgo's
# own CI has no live Polaris server, so this script copies the assets into a
# dedicated test module that pins polaris-go, runs `go build ./...`, then
# `go test ./release/ -count=1` for the SDK-neutral ops + OTel observer + the
# runtime canary harness (AC5). If this passes, the assets compile against the
# pinned SDK version and the engine/cache/observer invariants hold.
#
# Usage: ./scripts/verify-polaris-adapter.sh

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
TESTMOD="${REPO_ROOT}/tools/verifyexamples/polaris-adapter"
RELEASE_PKG="${TESTMOD}/release"
ADAPTER_ASSET="${REPO_ROOT}/internal/assets/_data/kitex/optional/polaris_canary_adapter.go"
CANARY_ASSET="${REPO_ROOT}/internal/assets/_data/optional/release_canary.go"
OPS_ASSET="${REPO_ROOT}/internal/assets/_data/optional/release_ops.go"
OTEL_ASSET="${REPO_ROOT}/internal/assets/_data/kitex/optional/polaris_canary_observer_otel.go"

if [[ ! -f "${ADAPTER_ASSET}" ]]; then
  echo "ERROR: adapter asset not found: ${ADAPTER_ASSET}" >&2
  exit 1
fi
if [[ ! -f "${CANARY_ASSET}" ]]; then
  echo "ERROR: canary seam asset not found: ${CANARY_ASSET}" >&2
  exit 1
fi
if [[ ! -f "${OPS_ASSET}" ]]; then
  echo "ERROR: ops asset not found: ${OPS_ASSET}" >&2
  exit 1
fi
if [[ ! -f "${OTEL_ASSET}" ]]; then
  echo "ERROR: otel observer asset not found: ${OTEL_ASSET}" >&2
  exit 1
fi

mkdir -p "${RELEASE_PKG}"

# Refresh the release package from embedded assets (single source of truth).
cp "${CANARY_ASSET}" "${RELEASE_PKG}/release_canary.go"
cp "${ADAPTER_ASSET}" "${RELEASE_PKG}/polaris_canary_adapter.go"
cp "${OPS_ASSET}" "${RELEASE_PKG}/release_ops.go"
cp "${OTEL_ASSET}" "${RELEASE_PKG}/polaris_canary_observer_otel.go"

# Substitute any {{.Module}} template placeholders (the adapter is
# module-agnostic today, but keep this for forward compatibility).
for f in "${RELEASE_PKG}"/*.go; do
  if grep -q '{{\.Module}}' "${f}"; then
    sed -i.bak 's#{{\.Module}}#example#g' "${f}"
    rm -f "${f}.bak"
  fi
done

cd "${TESTMOD}"

# Resolve transitive dependencies against GOPROXY.
go mod tidy

# Compile everything in the test module. The main package references
# release.NewPolarisSelector, so the adapter's sdkClient / instanceFromPolaris
# bodies are fully type-checked against the pinned polaris-go.
go build ./...

# Run the SDK-neutral ops tests (cache, observer, engine).
go test ./release/ -count=1

echo "polaris-adapter compile + unit OK"
