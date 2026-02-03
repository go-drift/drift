#!/usr/bin/env bash
set -euo pipefail

# Update the pinned Skia revision in SKIA_REV
#
# Usage:
#   ./update_skia_rev.sh          # fetch latest from Skia main
#   ./update_skia_rev.sh --rev <hash>  # use specific commit
#   ./update_skia_rev.sh --sync   # also run fetch_skia.sh after updating

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKIA_REV_FILE="$ROOT_DIR/SKIA_REV"

rev=""
sync=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --rev)
      shift
      rev="${1:-}"
      if [[ -z "$rev" ]]; then
        echo "Error: --rev requires a commit hash" >&2
        exit 1
      fi
      shift
      ;;
    --sync)
      sync=true
      shift
      ;;
    -h|--help)
      echo "Usage: $0 [--rev <hash>] [--sync]"
      echo ""
      echo "Options:"
      echo "  --rev <hash>  Use a specific Skia commit hash"
      echo "  --sync        Run fetch_skia.sh after updating SKIA_REV"
      echo ""
      echo "Without --rev, fetches the latest commit from Skia main."
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      exit 1
      ;;
  esac
done

SKIA_REMOTE="https://skia.googlesource.com/skia"

if [[ -z "$rev" ]]; then
  echo "Fetching latest Skia main commit..."
  rev=$(git ls-remote "$SKIA_REMOTE" refs/heads/main | cut -f1)
  if [[ -z "$rev" ]]; then
    echo "Error: Failed to fetch Skia commit from remote" >&2
    exit 1
  fi
else
  # Validate hex format (allow short hashes, minimum 7 chars)
  if ! [[ "$rev" =~ ^[0-9a-fA-F]{7,40}$ ]]; then
    echo "Error: Invalid commit hash format: $rev" >&2
    exit 1
  fi
  # Resolve short hash to full hash via remote
  echo "Resolving commit on Skia remote..."
  matches=$(git ls-remote "$SKIA_REMOTE" | grep "^$rev" | cut -f1 | sort -u)
  match_count=$(echo "$matches" | grep -c . || true)
  if [[ "$match_count" -eq 0 ]]; then
    echo "Error: Commit $rev not found on Skia remote" >&2
    exit 1
  elif [[ "$match_count" -gt 1 ]]; then
    echo "Error: Ambiguous short hash $rev matches multiple commits:" >&2
    echo "$matches" >&2
    exit 1
  fi
  rev="$matches"
fi

echo "$rev" > "$SKIA_REV_FILE"
echo "Updated SKIA_REV to: $rev"

if $sync; then
  echo ""
  echo "Running fetch_skia.sh..."
  "$ROOT_DIR/scripts/fetch_skia.sh"
fi
