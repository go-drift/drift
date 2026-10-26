#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/install-check.sh [--real] [--build <platforms>] [--keep] <version>

Installs Drift and the splash plugin from outside this repository, as a user
would after a release of <version>, and checks the CLI refuses version skew.

Rehearsal (default), before tagging:
  1. Clones HEAD and makes release commits for <version> and the next minor
     (scripts/release.sh prepare), then serves the released modules at both
     from a file GOPROXY.
  2. Installs the drift CLI at <version> from it.
  3. Creates an app with `drift init`, adds plugins/splash at <version> and
     runs `drift plugin sync`; with --build, also `drift build` for each
     platform (comma-separated: android, xtool, ios).
  4. Checks that a plugin requiring the next minor, and an app on the next
     minor, each fail `drift plugin sync` naming both versions.

--real skips the rehearsal proxy and the skew checks: the published tags are
installed through the default GOPROXY (Release sequence step 4).

Everything runs in a temporary directory with its own module, build and
Drift caches, so rehearsal versions never reach the real module cache. The
Skia libraries come from DRIFT_SKIA_DIR when set, else from this checkout's
third_party/drift_skia (a rehearsal version has no published Skia assets).

Options:
  --real              Install the published release instead of a rehearsal
  --build <platforms> Also build the app, e.g. --build android,xtool
  --keep              Keep the temporary directory
EOF
}

real=false
keep=false
platforms=""
version=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --real) real=true; shift ;;
    --keep) keep=true; shift ;;
    --build)
      [[ $# -ge 2 ]] || { echo "--build needs a value" >&2; exit 1; }
      platforms="$2"
      shift 2
      ;;
    -h|--help) usage; exit 0 ;;
    -*) echo "Unknown option: $1" >&2; usage; exit 1 ;;
    *)
      [[ -z "$version" ]] || { echo "Multiple versions specified" >&2; exit 1; }
      version="$1"
      shift
      ;;
  esac
done
[[ -n "$version" ]] || { usage; exit 1; }
if [[ ! "$version" =~ ^v([0-9]+)\.([0-9]+)\.[0-9]+$ ]]; then
  echo "Invalid version: $version (want vX.Y.Z)" >&2
  exit 1
fi
next="v${BASH_REMATCH[1]}.$((BASH_REMATCH[2] + 1)).0"

repo=$(git rev-parse --show-toplevel)
work=$(mktemp -d "${TMPDIR:-/tmp}/drift-install-check.XXXXXX")
if [[ "$keep" == "true" ]]; then
  echo "Working in $work (kept)"
else
  trap 'chmod -R u+w "$work" 2>/dev/null; rm -rf "$work"' EXIT
fi

export GOMODCACHE="$work/gomodcache"
export GOPATH="$work/gopath"
export GOBIN="$work/bin"
export GOCACHE="$work/gocache"
export GOFLAGS=-modcacherw
export GOWORK=off
export DRIFT_CACHE_DIR="$work/drift-cache"
export DRIFT_SKIA_DIR="${DRIFT_SKIA_DIR:-$repo/third_party/drift_skia}"
unset GOPRIVATE GONOPROXY

step() { printf '\n==> %s\n' "$*"; }
fail() { echo "install-check: $*" >&2; exit 1; }

# expect_failure <dir> <want>...: drift plugin sync in <dir> must fail with
# output containing every <want>.
expect_failure() {
  local dir="$1" out
  shift
  if out=$(cd "$dir" && drift plugin sync 2>&1); then
    fail "drift plugin sync in $dir succeeded; want a version skew error"
  fi
  for want in "$@"; do
    grep -qF -- "$want" <<<"$out" || fail "error lacks \"$want\":
$out"
  done
  printf '%s\n' "$out" | sed 's/^/    /'
}

if [[ "$real" == "true" ]]; then
  unset GOPROXY GONOSUMDB
else
  step "Release commits for $version and $next"
  git clone -q "$repo" "$work/src"
  (
    cd "$work/src"
    git checkout -q -b install-check
    for v in "$version" "$next"; do
      go run ./scripts/releasetool bump "$v" >/dev/null
      git -c user.name=install-check -c user.email=install-check@localhost \
        commit -q -a -m "release: $v"
      go run ./scripts/releasetool proxy -dir "$work/proxy" -rev HEAD "$v"
    done
  )
  export GOPROXY="file://$work/proxy,https://proxy.golang.org"
  export GONOSUMDB=github.com/go-drift/drift
fi

step "Install the drift CLI at $version"
go install "github.com/go-drift/drift/cmd/drift@$version"
export PATH="$GOBIN:$PATH"
drift version | grep -qF "version $version " || fail "drift version: $(drift version)"
drift version

step "New app with plugins/splash at $version"
cd "$work"
drift init app example.com/app >/dev/null
cd app
grep -qF "github.com/go-drift/drift $version" go.mod || fail "drift init did not require drift $version:
$(cat go.mod)"
go get "github.com/go-drift/drift/plugins/splash@$version"
mkdir -p assets
cp "$repo/examples/splash-demo/assets/splash.png" assets/
cat > drift.yaml <<'EOF'
app:
  name: app
  id: com.example.installcheck

plugins:
  - package: github.com/go-drift/drift/plugins/splash/plugin
    config:
      image: assets/splash.png
EOF
drift plugin sync --tidy
if grep -q '^replace\|^\s*github.com/go-drift/drift.* => ' go.mod; then
  fail "the app's go.mod has a replace:
$(cat go.mod)"
fi
grep -qF "github.com/go-drift/drift/plugins/splash $version" go.mod || fail "go.mod lacks splash $version"

IFS=, read -r -a build_platforms <<<"$platforms"
for p in "${build_platforms[@]}"; do
  step "drift build $p"
  drift build "$p"
done

if [[ "$real" == "true" ]]; then
  step "Installed $version from the published tags"
  exit 0
fi

step "Skew: a plugin requiring drift $next"
mkdir -p "$work/skew/plugin"
cat > "$work/skew/go.mod" <<EOF
module example.com/skew

go 1.24.0

require github.com/go-drift/drift $next
EOF
cat > "$work/skew/plugin/plugin.go" <<'EOF'
package plugin

import driftplugin "github.com/go-drift/drift/pkg/plugin"

type Config struct{}

type skew struct{}

func (skew) Name() string                                  { return "skew" }
func (skew) Build(ctx *driftplugin.BuildCtx, _ Config) error { return nil }

var Plugin driftplugin.Plugin[Config] = skew{}
EOF
cp -r "$work/app" "$work/app-skew-plugin"
(
  cd "$work/app-skew-plugin"
  go mod edit -require=example.com/skew@v0.0.0 -replace=example.com/skew=../skew
  printf '  - package: example.com/skew/plugin\n' >> drift.yaml
  printf 'package main\n\nimport _ "example.com/skew/plugin"\n' > skew_import.go
  go mod tidy
  rm skew_import.go
)
expect_failure "$work/app-skew-plugin" "example.com/skew/plugin" "requires drift $next" "this CLI ($version)"

step "Skew: an app on drift $next"
cp -r "$work/app" "$work/app-skew-drift"
(cd "$work/app-skew-drift" && go get "github.com/go-drift/drift@$next")
expect_failure "$work/app-skew-drift" "drift $next, but this CLI is $version"

step "Install check passed for $version"
