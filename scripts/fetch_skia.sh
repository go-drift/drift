#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
THIRD_PARTY="$ROOT_DIR/third_party"
SKIA_DIR="$THIRD_PARTY/skia"

mkdir -p "$THIRD_PARTY"

if [[ -d "$SKIA_DIR/.git" ]]; then
  echo "Skia already exists at $SKIA_DIR"
  cd "$SKIA_DIR"
  git fetch origin
  git checkout main
  git pull --ff-only origin main
  python3 tools/git-sync-deps
  exit 0
fi

echo "Cloning Skia into $SKIA_DIR"
git clone https://skia.googlesource.com/skia "$SKIA_DIR"

cd "$SKIA_DIR"
git checkout main
python3 tools/git-sync-deps
