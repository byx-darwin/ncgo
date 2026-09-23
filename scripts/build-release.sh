#!/usr/bin/env bash
# Build one deterministic ncgo release archive.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$REPO_ROOT"

: "${GOOS:?GOOS is required}"
: "${GOARCH:?GOARCH is required}"
: "${ARCHIVE_FORMAT:?ARCHIVE_FORMAT must be tar.gz or zip}"

VERSION="${VERSION:-snapshot-$(git rev-parse --short=7 HEAD)}"
REVISION="${REVISION:-$(git rev-parse HEAD)}"
SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-$(git show -s --format=%ct HEAD)}"
DIST_DIR="${DIST_DIR:-$REPO_ROOT/dist}"

case "$ARCHIVE_FORMAT" in
  tar.gz|zip) ;;
  *) echo "unsupported ARCHIVE_FORMAT: $ARCHIVE_FORMAT" >&2; exit 2 ;;
esac

build_time="$(python3 - "$SOURCE_DATE_EPOCH" <<'PY'
import datetime
import sys
print(datetime.datetime.fromtimestamp(int(sys.argv[1]), datetime.timezone.utc).isoformat().replace("+00:00", "Z"))
PY
)"

name="ncgo_${VERSION}_${GOOS}_${GOARCH}"
package_dir="$DIST_DIR/$name"
binary="ncgo"
if [[ "$GOOS" == "windows" ]]; then
  binary="ncgo.exe"
fi

rm -rf "$package_dir"
mkdir -p "$package_dir"

CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X github.com/byx-darwin/ncgo/internal/cli.Version=$VERSION -X github.com/byx-darwin/ncgo/internal/cli.BuildVersion=${REVISION:0:7} -X github.com/byx-darwin/ncgo/internal/cli.BuildTime=$build_time" \
  -o "$package_dir/$binary" .
cp README.md README.zh-CN.md LICENSE "$package_dir/"

output="$DIST_DIR/$name.$ARCHIVE_FORMAT"
python3 scripts/package-release.py \
  --root "$package_dir" \
  --output "$output" \
  --format "$ARCHIVE_FORMAT" \
  --epoch "$SOURCE_DATE_EPOCH"
rm -rf "$package_dir"
printf '%s\n' "$output"
