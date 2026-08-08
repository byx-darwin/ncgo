package shared

import (
	"fmt"
	"os"
	"path/filepath"
)

const PreCommitConfig = `minimum_pre_commit_version: '3.7.0'
default_install_hook_types:
  - pre-commit
  - pre-push

repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v5.0.0
    hooks:
      - id: trailing-whitespace
        args:
          - --markdown-linebreak-ext=md
      - id: end-of-file-fixer
      - id: check-yaml
      - id: check-merge-conflict
      - id: check-added-large-files

  - repo: local
    hooks:
      - id: gofmt
        name: gofmt
        entry: bash -c 'gofmt -w "$@"' --
        language: system
        types:
          - go
        require_serial: true

      - id: ncgo-check
        name: ncgo check (anchors/manifest/context)
        entry: ncgo check --root . --output text
        language: system
        pass_filenames: false
        always_run: true
        stages: [pre-commit]

      - id: go-vet-all-modules
        name: go vet ./... (all Go modules)
        entry: ./scripts/run-go-module-checks.sh vet
        language: system
        pass_filenames: false
        stages:
          - pre-push

      - id: go-test-all-modules
        name: go test ./... -count=1 (all Go modules)
        entry: ./scripts/run-go-module-checks.sh test
        language: system
        pass_filenames: false
        stages:
          - pre-push

      - id: go-build-all-modules
        name: go build . (all Go modules)
        entry: ./scripts/run-go-module-checks.sh build
        language: system
        pass_filenames: false
        stages:
          - pre-push
`

const PrePushGoModulesScript = `#!/usr/bin/env bash
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
`

func WriteRepositoryHooks(dir string) error {
	if err := os.WriteFile(filepath.Join(dir, ".pre-commit-config.yaml"), []byte(PreCommitConfig), 0o644); err != nil {
		return fmt.Errorf("scaffold: write .pre-commit-config.yaml: %w", err)
	}
	scriptDir := filepath.Join(dir, "scripts")
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		return fmt.Errorf("scaffold: mkdir %s: %w", scriptDir, err)
	}
	scriptPath := filepath.Join(scriptDir, "run-go-module-checks.sh")
	if err := os.WriteFile(scriptPath, []byte(PrePushGoModulesScript), 0o755); err != nil {
		return fmt.Errorf("scaffold: write %s: %w", scriptPath, err)
	}
	return nil
}
