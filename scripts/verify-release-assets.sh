#!/usr/bin/env bash
# Verify the exact release asset contract and optionally write/check checksums.
set -euo pipefail

if [[ $# -lt 2 || $# -gt 3 ]]; then
  echo "usage: $0 <dist-dir> <version> [write|check]" >&2
  exit 2
fi

dist_dir="$1"
version="$2"
mode="${3:-names}"

if [[ ! -d "$dist_dir" || "$version" == *[!A-Za-z0-9._+-]* ]]; then
  echo "invalid release directory or version" >&2
  exit 2
fi

expected=(
  "ncgo_${version}_darwin_amd64.tar.gz"
  "ncgo_${version}_darwin_arm64.tar.gz"
  "ncgo_${version}_linux_amd64.tar.gz"
  "ncgo_${version}_linux_arm64.tar.gz"
  "ncgo_${version}_windows_amd64.zip"
)

actual="$(find "$dist_dir" -maxdepth 1 -type f \( -name '*.tar.gz' -o -name '*.zip' \) -exec basename {} \; | LC_ALL=C sort)"
expected_text="$(printf '%s\n' "${expected[@]}")"
if [[ "$actual" != "$expected_text" ]]; then
  echo "release archive set does not match the five expected targets" >&2
  diff -u <(printf '%s\n' "$expected_text") <(printf '%s\n' "$actual") || true
  exit 1
fi

case "$mode" in
  names)
    ;;
  write)
    (
      cd "$dist_dir"
      sha256sum "${expected[@]}" > checksums.txt
      sha256sum -c checksums.txt
    )
    ;;
  check)
    (
      cd "$dist_dir"
      test -f checksums.txt
      test "$(wc -l < checksums.txt | tr -d ' ')" -eq "${#expected[@]}"
      sha256sum -c checksums.txt
    )
    ;;
  *)
    echo "unknown verification mode: $mode" >&2
    exit 2
    ;;
esac
