#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKIA_DIR="$ROOT_DIR/third_party/skia"
DRIFT_SKIA_OUT="$ROOT_DIR/third_party/drift_skia"

if [[ ! -d "$SKIA_DIR" ]]; then
  echo "Skia not found at $SKIA_DIR. Run scripts/fetch_skia.sh first."
  exit 1
fi

if [[ -z "${ANDROID_NDK_HOME:-}" ]]; then
  echo "ANDROID_NDK_HOME is not set."
  exit 1
fi

cd "$SKIA_DIR"
python3 tools/git-sync-deps

build() {
  local out_dir="$1"
  local target_cpu="$2"
  bin/gn gen "$out_dir" --args="target_os=\"android\" target_cpu=\"$target_cpu\" ndk=\"$ANDROID_NDK_HOME\" ndk_api=21 is_official_build=true skia_use_gl=true skia_use_system_harfbuzz=false skia_use_harfbuzz=true skia_use_system_expat=false skia_use_system_libpng=false skia_use_system_zlib=false skia_use_system_freetype2=false skia_use_system_libjpeg_turbo=false skia_use_libjpeg_turbo_decode=true skia_use_libjpeg_turbo_encode=true skia_use_system_libwebp=false skia_use_libwebp_decode=true skia_use_libwebp_encode=true skia_enable_svg=true skia_use_expat=true skia_use_icu=false skia_use_libgrapheme=true skia_enable_skparagraph=true skia_enable_skshaper=true"
  ninja -C "$out_dir" skia svg skresources skparagraph skshaper skunicode
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
        arm64)  HOST_TAG="darwin-arm64" ;;
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

  echo "Compiling bridge for Android $arch..."
  echo "Skia out dir: $SKIA_DIR/$out_dir"

  local clang="$ANDROID_NDK_HOME/toolchains/llvm/prebuilt/$HOST_TAG/bin/clang++"

  # Compile bridge
  "$clang" --target="$target_triple" \
    -std=c++17 -fPIC -DSKIA_GL \
    -I. -I./include \
    -c "$ROOT_DIR/pkg/skia/bridge/skia_gl.cc" \
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
