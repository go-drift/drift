#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/release.sh <version>

Creates and pushes an annotated git tag for a Drift release.

Arguments:
  <version>  SemVer tag like v0.1.0
EOF
}

if [[ $# -ne 1 ]]; then
  usage
  exit 1
fi

version="$1"

if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Invalid version: $version" >&2
  echo "Expected format: vX.Y.Z" >&2
  exit 1
fi

if ! git rev-parse --git-dir >/dev/null 2>&1; then
  echo "Not inside a git repository." >&2
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  echo "Working tree is dirty. Commit or stash changes first." >&2
  exit 1
fi

if git tag -l "$version" | grep -q "^${version}$"; then
  echo "Tag already exists: $version" >&2
  exit 1
fi

git tag -a "$version" -m "Release $version"
git push origin "$version"

echo "Release tag created and pushed: $version"
