#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/release-tags.sh [--dry-run] <version>

Tags a release at HEAD and pushes the tags to origin: <version> for the root
module and plugins/<name>/<version> for each plugin module. Run by the release
workflow once the Skia assets are built.

All tags go in one atomic push, so a release is never half-tagged. If the
root tag already exists, the release was tagged before (a rebuild) and
nothing is done.

Options:
  --dry-run  Print the tags without creating or pushing them
EOF
}

dry_run=false
version=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dry-run) dry_run=true; shift ;;
    -h|--help) usage; exit 0 ;;
    -*) echo "Unknown option: $1" >&2; usage; exit 1 ;;
    *)
      if [[ -n "$version" ]]; then
        echo "Multiple versions specified" >&2
        exit 1
      fi
      version="$1"
      shift
      ;;
  esac
done
if [[ -z "$version" ]]; then
  usage
  exit 1
fi

cd "$(git rev-parse --show-toplevel)"

remote_tags=$(git ls-remote --tags --refs origin | awk '{print $2}' | sed 's|^refs/tags/||')
has_remote_tag() { grep -qxF "$1" <<<"$remote_tags"; }

if has_remote_tag "$version"; then
  echo "Tag $version already exists on origin (rebuild); nothing to tag."
  exit 0
fi

go run ./scripts/releasetool check "$version"
mapfile -t tags < <(go run ./scripts/releasetool tags "$version")

for t in "${tags[@]}"; do
  if has_remote_tag "$t"; then
    echo "Tag $t exists on origin but $version does not; resolve by hand." >&2
    exit 1
  fi
done

if [[ "$dry_run" == "true" ]]; then
  printf 'Would tag %s at %s:\n' "$version" "$(git rev-parse --short HEAD)"
  printf '  %s\n' "${tags[@]}"
  exit 0
fi

for t in "${tags[@]}"; do
  git tag -a -f "$t" -m "Release $version"
done
git push --atomic origin "${tags[@]/#/refs/tags/}"
printf 'Pushed %d tags for %s.\n' "${#tags[@]}" "$version"
