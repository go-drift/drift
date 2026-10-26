#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/release.sh prepare <version>
       scripts/release.sh [--force] <version>

Releases Drift: the root module and every plugin module under plugins/, in
lockstep at <version>.

  prepare   Makes the release commit on master: every module in the
            repository requires the released modules at <version>. Push
            it, then run the second form.

  (none)    Triggers the Drift release workflow via GitHub Actions. The
            workflow builds Skia assets, creates a GitHub release, and only
            then pushes the tags (<version> and plugins/<name>/<version>).
            This ensures `go install @latest` always resolves to a version
            with ready assets.

Arguments:
  <version>  SemVer tag like v0.1.0

Options:
  --force    Allow triggering a release for an existing tag
EOF
}

force=false
prepare=false
version=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --force)
      force=true
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    prepare)
      prepare=true
      shift
      ;;
    -*)
      echo "Unknown option: $1" >&2
      usage
      exit 1
      ;;
    *)
      if [[ -n "$version" ]]; then
        echo "Multiple versions specified" >&2
        usage
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

if [[ ! "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9.-]+)?$ ]]; then
  echo "Invalid version: $version" >&2
  echo "Expected format: vX.Y.Z or vX.Y.Z-suffix" >&2
  exit 1
fi

if ! git rev-parse --git-dir >/dev/null 2>&1; then
  echo "Not inside a git repository." >&2
  exit 1
fi
cd "$(git rev-parse --show-toplevel)"

if [[ -n "$(git status --porcelain)" ]]; then
  echo "Working tree is dirty. Commit or stash changes first." >&2
  exit 1
fi

# Enforce releases from master branch
current_branch=$(git rev-parse --abbrev-ref HEAD)
if [[ "$current_branch" != "master" ]]; then
  echo "Releases must be made from the master branch." >&2
  echo "Currently on: $current_branch" >&2
  exit 1
fi

# Ensure local master is up-to-date with remote
git fetch origin master --quiet
local_sha=$(git rev-parse HEAD)
remote_sha=$(git rev-parse origin/master)
if [[ "$local_sha" != "$remote_sha" ]]; then
  echo "Local master is not up-to-date with origin/master." >&2
  echo "Run 'git pull' first." >&2
  exit 1
fi

if [[ "$prepare" == "true" ]]; then
  go run ./scripts/releasetool bump "$version"
  if [[ -z "$(git status --porcelain)" ]]; then
    echo "Every module already requires the released modules at $version; nothing to commit."
  else
    git commit -q -a -m "release: $version"
    echo "Made the release commit:"
    git log --oneline -1
  fi
  echo ""
  echo "Push master, then run: scripts/release.sh $version"
  exit 0
fi

# Check if tag exists on remote
if git ls-remote --tags origin | grep -q "refs/tags/${version}$"; then
  if [[ "$force" == "true" ]]; then
    echo "Warning: Tag $version already exists on remote. Proceeding with --force."
  else
    echo "Tag already exists on remote: $version" >&2
    echo "Use --force to trigger a rebuild." >&2
    exit 1
  fi
else
  # The tagged commit must require the released modules at $version, or
  # the plugin modules cannot be installed outside this repository. (A
  # rebuild tags nothing.)
  go run ./scripts/releasetool check "$version"
fi

# Check gh CLI is available and authenticated
if ! command -v gh &>/dev/null; then
  echo "gh CLI not found. Install it from https://cli.github.com/" >&2
  exit 1
fi

if ! gh auth status &>/dev/null; then
  echo "gh CLI not authenticated. Run 'gh auth login' first." >&2
  exit 1
fi

echo "Triggering release workflow for $version..."
gh workflow run drift-skia-release.yml -f version="$version"

echo ""
echo "Release workflow started for $version"
echo "Monitor progress at: $(gh repo view --json url -q .url)/actions"
echo ""
echo "The Git tags will be created automatically after the build succeeds."
