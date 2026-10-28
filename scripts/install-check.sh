#!/usr/bin/env bash

set -euo pipefail

usage() {
  cat <<'EOF'
Usage: scripts/install-check.sh [--real] [--build <platforms>] [--keep] <version>

Installs Drift from outside this repository, as a user would after a
release of <version>, and checks the CLI refuses version skew.

Rehearsal (default), before tagging:
  1. Serves HEAD from a file GOPROXY as drift <version> and the next minor,
     and a copy of examples/plugins/hello as the third-party plugin
     example.com/hello: v1.0.0 requiring drift <version>, v1.1.0 requiring
     the next minor.
  2. Installs the drift CLI at <version> from it.
  3. Creates an app with `drift init`, configures plugins/splash (part of
     the drift module) and runs `drift plugin sync`; with --build, also
     `drift build` for each platform (comma-separated: android, xtool, ios).
  4. Creates an app using example.com/hello v1.0.0 and checks that, with
     hello v1.1.0 or with the app on the next minor, `drift plugin sync`
     fails naming both versions.

--real skips the rehearsal proxy and the skew checks: the published tags are
installed through the default GOPROXY (Release sequence step 4).

Everything runs in a temporary directory with its own module, build and
Drift caches, so rehearsal versions never reach the real module cache. The
Skia libraries come from DRIFT_SKIA_DIR when set, else from this checkout's
third_party/drift_skia (a rehearsal version has no published Skia assets).
With --real, DRIFT_SKIA_DIR is unset, so the CLI downloads the release's
Skia assets as a user's would.

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
if [[ "$real" == "true" ]]; then
  unset DRIFT_SKIA_DIR
else
  export DRIFT_SKIA_DIR="${DRIFT_SKIA_DIR:-$repo/third_party/drift_skia}"
fi
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
  step "Serve drift $version and $next, and example.com/hello"
  for v in "$version" "$next"; do
    (cd "$repo" && go run ./scripts/fileproxy -dir "$work/proxy" "$v")
  done
  # A third-party plugin: examples/plugins/hello under its own module path,
  # in its own repository, without the in-repo replace.
  cp -r "$repo/examples/plugins/hello" "$work/hello"
  (
    cd "$work/hello"
    grep -rlF github.com/go-drift/drift/examples/plugins/hello . |
      xargs sed -i.orig 's|github.com/go-drift/drift/examples/plugins/hello|example.com/hello|g'
    find . -name '*.orig' -delete
    go mod edit -dropreplace=github.com/go-drift/drift
    git init -q
    for hv in "v1.0.0:$version" "v1.1.0:$next"; do
      go mod edit -require="github.com/go-drift/drift@${hv#*:}"
      git add -A
      git -c user.name=install-check -c user.email=install-check@localhost \
        commit -q -m "hello ${hv%%:*}"
      (cd "$repo" && go run ./scripts/fileproxy -dir "$work/proxy" -repo "$work/hello" "${hv%%:*}")
    done
  )
  export GOPROXY="file://$work/proxy,https://proxy.golang.org"
  export GONOSUMDB=github.com/go-drift/drift,example.com/hello
fi

step "Install the drift CLI at $version"
go install "github.com/go-drift/drift/cmd/drift@$version"
export PATH="$GOBIN:$PATH"
drift version | grep -qF "version $version " || fail "drift version: $(drift version)"
drift version

step "New app with plugins/splash from drift $version"
cd "$work"
drift init app example.com/app >/dev/null
cd app
grep -qF "github.com/go-drift/drift $version" go.mod || fail "drift init did not require drift $version:
$(cat go.mod)"
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
if [[ "$(go list -m all | grep -c '^github.com/go-drift/drift')" != 1 ]]; then
  fail "the app's module graph has modules besides github.com/go-drift/drift:
$(go list -m all | grep '^github.com/go-drift/drift')"
fi

IFS=, read -r -a build_platforms <<<"$platforms"
for p in "${build_platforms[@]}"; do
  step "drift build $p"
  drift build "$p"
done

if [[ "$real" == "true" ]]; then
  step "Installed $version from the published tags"
  exit 0
fi

step "New app with example.com/hello v1.0.0"
cd "$work"
drift init hello-app example.com/hello-app >/dev/null
cd hello-app
go get example.com/hello@v1.0.0
cat > drift.yaml <<'EOF'
app:
  name: HelloApp
  id: com.example.installcheck.hello

plugins:
  - package: example.com/hello/plugin
    config:
      greeting: Hello from install-check
EOF
drift plugin sync --tidy

step "Skew: example.com/hello v1.1.0, requiring drift $next"
cp -r "$work/hello-app" "$work/app-skew-plugin"
(cd "$work/app-skew-plugin" && go get example.com/hello@v1.1.0)
expect_failure "$work/app-skew-plugin" "example.com/hello/plugin" "requires drift $next" "this CLI ($version)"

step "Skew: an app on drift $next"
cp -r "$work/hello-app" "$work/app-skew-drift"
(cd "$work/app-skew-drift" && go get "github.com/go-drift/drift@$next")
expect_failure "$work/app-skew-drift" "drift $next, but this CLI is $version"

step "Install check passed for $version"
