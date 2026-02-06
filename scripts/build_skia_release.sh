#!/usr/bin/env bash

set -euo pipefail
trap 'echo "Error at line $LINENO: $BASH_COMMAND" >&2' ERR

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKIA_DIR="$ROOT_DIR/third_party/skia"
DRIFT_SKIA_DIR="$ROOT_DIR/third_party/drift_skia"
DIST_DIR="$ROOT_DIR/dist/drift_skia"

detect_drift_version() {
  if [[ -n "${DRIFT_VERSION:-}" ]]; then
    echo "$DRIFT_VERSION"
    return
  fi

  local base
  base="$(basename "$ROOT_DIR")"
  if [[ "$base" == *@* ]]; then
    echo "${base##*@}"
    return
  fi

  if git -C "$ROOT_DIR" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    local tag
    tag=$(git -C "$ROOT_DIR" describe --tags --abbrev=0 2>/dev/null || true)
    if [[ -n "$tag" ]]; then
      if [[ "$tag" == drift-* ]]; then
        echo "${tag#drift-}"
      else
        echo "$tag"
      fi
      return
    fi
  fi

  echo ""
}

usage() {
  cat <<EOF
Usage: $(basename "$0") [--android] [--ios] [--skip-build]

Builds and packages Skia static libraries for release. If no platform flags
are provided, both Android and iOS are built and packaged.
EOF
}

platforms=()
skip_build=false
for arg in "$@"; do
  case "$arg" in
    --android)
      platforms+=("android")
      ;;
    --ios)
      platforms+=("ios")
      ;;
    --skip-build)
      skip_build=true
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $arg" >&2
      usage
      exit 1
      ;;
  esac
done

if [[ ${#platforms[@]} -eq 0 ]]; then
  platforms=("android" "ios")
fi

if [[ ! -d "$SKIA_DIR" ]]; then
  "$ROOT_DIR/scripts/fetch_skia.sh"
fi

if [[ "$skip_build" = false ]]; then
  for platform in "${platforms[@]}"; do
    case "$platform" in
      android)
        "$ROOT_DIR/scripts/build_skia_android.sh"
        ;;
      ios)
        "$ROOT_DIR/scripts/build_skia_ios.sh"
        ;;
      *)
        echo "Unsupported platform: $platform" >&2
        exit 1
        ;;
    esac
  done
fi

drift_version="$(detect_drift_version)"
if [[ -z "$drift_version" ]]; then
  echo "Unable to determine Drift version. Set DRIFT_VERSION or run from a tagged checkout." >&2
  exit 1
fi
ndk_version="unknown"
ndk_api=21

if [[ -n "${ANDROID_NDK_HOME:-}" && -f "$ANDROID_NDK_HOME/source.properties" ]]; then
  ndk_version=$(grep -E '^Pkg\.Revision=' "$ANDROID_NDK_HOME/source.properties" | cut -d= -f2 | tr -d ' ') || true
fi

out_dir="$DIST_DIR/$drift_version"
mkdir -p "$DIST_DIR" "$out_dir"

copy_lib() {
  local platform="$1"
  local arch="$2"
  local src="$DRIFT_SKIA_DIR/$platform/$arch/libdrift_skia.a"
  local dst="$out_dir/$platform/$arch"

  if [[ ! -f "$src" ]]; then
    echo "Missing $src. Build Skia for $platform/$arch first." >&2
    exit 1
  fi

  mkdir -p "$dst"
  cp "$src" "$dst/libdrift_skia.a"
}

detect_host_tag() {
  local host_os host_arch
  host_os="$(uname -s)"
  host_arch="$(uname -m)"
  case "$host_os" in
    Linux*)
      case "$host_arch" in
        x86_64)  echo "linux-x86_64" ;;
        aarch64) echo "linux-aarch64" ;;
        *)       echo "linux-x86_64" ;;
      esac
      ;;
    Darwin*)
      case "$host_arch" in
        x86_64) echo "darwin-x86_64" ;;
        arm64)
          # Prefer native arm64, fall back to x86_64 (runs via Rosetta)
          if [[ -n "${ANDROID_NDK_HOME:-}" && -d "$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/darwin-arm64" ]]; then
            echo "darwin-arm64"
          elif [[ -n "${ANDROID_NDK_HOME:-}" && -d "$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/darwin-x86_64" ]]; then
            echo "darwin-x86_64"
          else
            echo "Error: No NDK toolchain found. Set ANDROID_NDK_HOME correctly." >&2
            exit 1
          fi
          ;;
        *)      echo "darwin-x86_64" ;;
      esac
      ;;
    *)
      echo "linux-x86_64"
      ;;
  esac
}

copy_cppshared() {
  local arch="$1"
  local triple="$2"
  local host_tag
  host_tag="$(detect_host_tag)"

  local src="$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/$host_tag/sysroot/usr/lib/$triple/libc++_shared.so"
  local dst="$out_dir/android/$arch"

  if [[ ! -f "$src" ]]; then
    echo "Error: libc++_shared.so not found at $src" >&2
    echo "Ensure ANDROID_NDK_HOME is set correctly." >&2
    exit 1
  fi

  cp "$src" "$dst/libc++_shared.so"
  echo "Copied libc++_shared.so for $arch"
}

include_android=false
include_ios=false

for platform in "${platforms[@]}"; do
  case "$platform" in
    android)
      include_android=true
      copy_lib android arm64
      copy_lib android arm
      copy_lib android amd64
      copy_cppshared arm64 aarch64-linux-android
      copy_cppshared arm arm-linux-androideabi
      copy_cppshared amd64 x86_64-linux-android
      ;;
    ios)
      include_ios=true
      # Device builds
      copy_lib ios arm64
      # Simulator builds (arm64 for Apple Silicon, amd64 for Intel)
      copy_lib ios-simulator arm64
      copy_lib ios-simulator amd64
      ;;
  esac
done

if [[ "$include_android" = true ]]; then
  android_tar="$DIST_DIR/drift-$drift_version-android.tar.gz"
  tar -C "$out_dir" -czf "$android_tar" android
fi

if [[ "$include_ios" = true ]]; then
  ios_tar="$DIST_DIR/drift-$drift_version-ios.tar.gz"
  tar -C "$out_dir" -czf "$ios_tar" ios ios-simulator
fi

if [[ "$include_android" = true && "$include_ios" = true ]]; then
  android_sha=$(sha256sum "$android_tar" | cut -d ' ' -f1)
  ios_sha=$(sha256sum "$ios_tar" | cut -d ' ' -f1)

  cat > "$out_dir/manifest.json" <<EOF
{
  "drift_version": "${drift_version}",
  "ndk_version": "${ndk_version}",
  "ndk_api": ${ndk_api},
  "android": {
    "tarball": "drift-${drift_version}-android.tar.gz",
    "sha256": "${android_sha}",
    "arches": ["arm64", "arm", "amd64"]
  },
  "ios": {
    "tarball": "drift-${drift_version}-ios.tar.gz",
    "sha256": "${ios_sha}",
    "device_arches": ["arm64"],
    "simulator_arches": ["arm64", "amd64"]
  }
}
EOF
fi

echo "Drift Skia release artifacts written to $out_dir"
if [[ "$include_android" = true ]]; then
  echo "  - $android_tar"
fi
if [[ "$include_ios" = true ]]; then
  echo "  - $ios_tar"
fi
if [[ "$include_android" = true && "$include_ios" = true ]]; then
  echo "  - $out_dir/manifest.json"
fi
