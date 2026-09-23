#!/usr/bin/env bash
# Network-dependent generated-project verification. CI installs exact generator
# versions and selects the generated-project Go toolchain before invoking this.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

export NCGO_INTEGRATION=1
export GOTOOLCHAIN=local

# Network phase: prefetch only reviewed repository and generated-project locks.
go mod download
for fixture in generated-hertz generated-kitex; do
  ./scripts/prefetch-go-sum.sh "tools/verifyexamples/$fixture/go.sum"
  (
    cd "tools/verifyexamples/$fixture"
    go mod download all
    go mod verify
  )
done

go test ./internal/scaffold/mono -count=1 -timeout=30m \
  -run '^(TestResultNextStepsSafePrefixExecutes|TestPostGenerateResultNextStepsSafePrefixExecutes|TestGenerateHertzWithDatabaseRendersTopLevelDatabaseConfig|TestGenerateHertzCompiles|TestGenerateHertzWithDatabaseCompiles|TestGenerateKitexCompiles|TestGenerateKitexWithDatabaseCompiles|TestGenerate_AutoSteps_Default)$'

./scripts/verify-polaris-adapter.sh
