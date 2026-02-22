#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKIA_DIR="$ROOT_DIR/third_party/skia"
DRIFT_SKIA_OUT="$ROOT_DIR/third_party/drift_skia"
SKIA_REV_FILE="$ROOT_DIR/SKIA_REV"

if [[ ! -d "$SKIA_DIR" ]]; then
  echo "Skia not found at $SKIA_DIR. Run scripts/fetch_skia.sh first."
  exit 1
fi

if [[ -z "${ANDROID_NDK_HOME:-}" ]]; then
  echo "ANDROID_NDK_HOME is not set."
  exit 1
fi

# Checkout pinned Skia revision if SKIA_REV exists
# Set SKIP_SKIA_REV=1 to use your current Skia checkout (for local patching/testing)
if [[ -z "${SKIP_SKIA_REV:-}" ]] && [[ -f "$SKIA_REV_FILE" ]]; then
  skia_rev="$(tr -d '[:space:]' < "$SKIA_REV_FILE")"
  if [[ -n "$skia_rev" ]]; then
    echo "Checking out pinned Skia revision: $skia_rev"
    cd "$SKIA_DIR"
    # Only fetch if commit is not already present locally
    if ! git cat-file -e "$skia_rev^{commit}" 2>/dev/null; then
      git fetch origin
    fi
    git checkout "$skia_rev"
    cd "$ROOT_DIR"
  fi
fi

# Detect NDK major version from source.properties
NDK_MAJOR=0
if [[ -f "$ANDROID_NDK_HOME/source.properties" ]]; then
  NDK_MAJOR=$(grep "Pkg.Revision" "$ANDROID_NDK_HOME/source.properties" | cut -d= -f2 | cut -d. -f1 | tr -d ' ')
fi
echo "Using NDK r${NDK_MAJOR} from $ANDROID_NDK_HOME"

cd "$SKIA_DIR"
python3 tools/git-sync-deps

# COMPATIBILITY NOTE: -mno-outline-atomics
#
# NDK r23+ generates "outline atomics" for ARM64 by default - calls to helper
# functions like __aarch64_ldadd4_relax instead of inline atomic instructions.
# These helpers are only available in newer NDK runtimes, causing crashes when
# users build their apps with older NDKs (e.g., r22) against our prebuilt libs.
#
# We disable outline atomics for arm64 to ensure compatibility with older NDKs.
#
# Trade-offs:
# - May slightly reduce performance on some devices (inline atomics vs library calls)
# - Targets lowest common denominator rather than optimal code for modern toolchains
#
# Future consideration: Could provide separate "modern" builds for users on newer
# NDKs, or remove this flag once older NDK usage drops sufficiently.

build() {
  local out_dir="$1"
  local target_cpu="$2"
  local extra_cflags=""

  # Disable outline atomics for arm64 on NDK r23+ (see COMPATIBILITY NOTE above)
  # Older NDKs don't support this flag and don't generate outline atomics anyway.
  if [[ "$target_cpu" == "arm64" && "$NDK_MAJOR" -ge 23 ]]; then
    extra_cflags='extra_cflags=["-mno-outline-atomics"]'
  fi

  bin/gn gen "$out_dir" --args="target_os=\"android\" target_cpu=\"$target_cpu\" ndk=\"$ANDROID_NDK_HOME\" ndk_api=21 is_official_build=true skia_use_vulkan=true skia_use_backup_vma=true skia_use_gl=false skia_use_system_harfbuzz=false skia_use_harfbuzz=true skia_use_system_expat=false skia_use_system_libpng=false skia_use_system_zlib=false skia_use_system_freetype2=false skia_use_system_libjpeg_turbo=false skia_use_libjpeg_turbo_decode=true skia_use_libjpeg_turbo_encode=true skia_use_system_libwebp=false skia_use_libwebp_decode=true skia_use_libwebp_encode=true skia_enable_svg=true skia_use_expat=true skia_use_icu=false skia_use_libgrapheme=true skia_enable_skparagraph=true skia_enable_skshaper=true skia_enable_skottie=true $extra_cflags"
  ninja -C "$out_dir" skia svg skresources skparagraph skshaper skunicode skottie
}

# Detect host system for NDK toolchain path (honor HOST_TAG env var if set)
if [[ -z "${HOST_TAG:-}" ]]; then
  host_os="$(uname -s)"
  host_arch="$(uname -m)"
  case "$host_os" in
    Linux*)
      case "$host_arch" in
        x86_64)  HOST_TAG="linux-x86_64" ;;
        aarch64) HOST_TAG="linux-aarch64" ;;
        *)       echo "Unsupported host architecture: $host_arch" >&2; exit 1 ;;
      esac
      ;;
    Darwin*)
      case "$host_arch" in
        x86_64) HOST_TAG="darwin-x86_64" ;;
        arm64)
          # Prefer native arm64, fall back to x86_64 (runs via Rosetta)
          if [[ -n "$ANDROID_NDK_HOME" && -d "$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/darwin-arm64" ]]; then
            HOST_TAG="darwin-arm64"
          elif [[ -n "$ANDROID_NDK_HOME" && -d "$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/darwin-x86_64" ]]; then
            HOST_TAG="darwin-x86_64"
          else
            echo "No NDK toolchain found for macOS. Set ANDROID_NDK_HOME correctly." >&2
            exit 1
          fi
          ;;
        *)      echo "Unsupported host architecture: $host_arch" >&2; exit 1 ;;
      esac
      ;;
    *)
      echo "Unsupported host OS: $host_os" >&2
      exit 1
      ;;
  esac
fi

# Compile bridge code and combine with Skia into libdrift_skia.a
compile_bridge() {
  local arch="$1"
  local target_triple="$2"
  local out_dir="out/android/$arch"
  local arch_flags=""

  # Disable outline atomics for arm64 on NDK r23+ (see COMPATIBILITY NOTE above)
  # Older NDKs don't support this flag and don't generate outline atomics anyway.
  if [[ "$arch" == "arm64" && "$NDK_MAJOR" -ge 23 ]]; then
    arch_flags="-mno-outline-atomics"
  fi

  echo "Compiling bridge for Android $arch..."
  echo "Skia out dir: $SKIA_DIR/$out_dir"

  local clang="$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/$HOST_TAG/bin/clang++"

  # Compile bridge
  "$clang" --target="$target_triple" \
    -std=c++17 -fPIC -DSKIA_VULKAN $arch_flags \
    -I. -I./include \
    -c "$ROOT_DIR/pkg/skia/bridge/skia_vk.cc" \
    -o "$out_dir/skia_bridge.o"

  # Combine: extract all Skia libs, add bridge, repack
  mkdir -p "$out_dir/tmp"
  pushd "$out_dir/tmp" > /dev/null
  # Extract all static libraries produced by the build
  rm -f ../libdrift_skia.a
  for lib in ../lib*.a; do
    [ -f "$lib" ] && ar x "$lib"
  done
  ar rcs ../libdrift_skia.a *.o ../skia_bridge.o
  popd > /dev/null
  rm -rf "$out_dir/tmp" "$out_dir/skia_bridge.o"

  echo "Created $SKIA_DIR/$out_dir/libdrift_skia.a"
}

build out/android/arm64 arm64
build out/android/arm arm
build out/android/amd64 x64

compile_bridge arm64 aarch64-linux-android21
compile_bridge arm armv7a-linux-androideabi21
compile_bridge amd64 x86_64-linux-android21

copy_lib() {
  local arch="$1"
  local src="$SKIA_DIR/out/android/$arch/libdrift_skia.a"
  local dst="$DRIFT_SKIA_OUT/android/$arch"
  if [[ ! -f "$src" ]]; then
    echo "Missing $src" >&2
    exit 1
  fi
  mkdir -p "$dst"
  cp "$src" "$dst/libdrift_skia.a"
  echo "Copied $src -> $dst/libdrift_skia.a"
}

copy_lib arm64
copy_lib arm
copy_lib amd64
