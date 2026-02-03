#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
THIRD_PARTY="$ROOT_DIR/third_party"
SKIA_DIR="$THIRD_PARTY/skia"
SKIA_REV_FILE="$ROOT_DIR/SKIA_REV"

# Read pinned revision if SKIA_REV exists
skia_rev=""
if [[ -f "$SKIA_REV_FILE" ]]; then
  skia_rev="$(tr -d '[:space:]' < "$SKIA_REV_FILE")"
fi

mkdir -p "$THIRD_PARTY"

if [[ -d "$SKIA_DIR/.git" ]]; then
  echo "Skia already exists at $SKIA_DIR"
  cd "$SKIA_DIR"
  git fetch origin
  if [[ -n "$skia_rev" ]]; then
    echo "Checking out pinned revision: $skia_rev"
    git checkout "$skia_rev"
  else
    git checkout main
    git pull --ff-only origin main
  fi
  python3 tools/git-sync-deps
  exit 0
fi

echo "Cloning Skia into $SKIA_DIR"
git clone https://skia.googlesource.com/skia "$SKIA_DIR"

cd "$SKIA_DIR"
if [[ -n "$skia_rev" ]]; then
  echo "Checking out pinned revision: $skia_rev"
  git checkout "$skia_rev"
else
  git checkout main
fi
python3 tools/git-sync-deps
