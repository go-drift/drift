#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKIA_DIR="$ROOT_DIR/third_party/skia"
DRIFT_SKIA_OUT="$ROOT_DIR/third_party/drift_skia"

if [[ ! -d "$SKIA_DIR" ]]; then
  echo "Skia not found at $SKIA_DIR. Run scripts/fetch_skia.sh first."
  exit 1
fi

cd "$SKIA_DIR"
python3 tools/git-sync-deps

# Common Skia build args
COMMON_ARGS='is_official_build=true skia_use_metal=true skia_use_system_harfbuzz=false skia_use_harfbuzz=true skia_use_system_expat=false skia_use_system_libpng=false skia_use_system_zlib=false skia_use_system_freetype2=false skia_use_system_libjpeg_turbo=false skia_use_libjpeg_turbo_decode=true skia_use_libjpeg_turbo_encode=true skia_use_system_libwebp=false skia_use_libwebp_decode=true skia_use_libwebp_encode=true skia_enable_svg=true skia_use_expat=true skia_use_icu=false skia_use_libgrapheme=true skia_enable_skparagraph=true skia_enable_skshaper=true'

build_device() {
  local out_dir="$1"
  local target_cpu="$2"
  echo "Building iOS device ($target_cpu)..."
  bin/gn gen "$out_dir" --args="target_os=\"ios\" target_cpu=\"$target_cpu\" $COMMON_ARGS"
  ninja -C "$out_dir" skia svg skresources skparagraph skshaper skunicode
}

build_simulator() {
  local out_dir="$1"
  local target_cpu="$2"
  echo "Building iOS simulator ($target_cpu)..."
  bin/gn gen "$out_dir" --args="target_os=\"ios\" target_cpu=\"$target_cpu\" ios_use_simulator=true $COMMON_ARGS"
  ninja -C "$out_dir" skia svg skresources skparagraph skshaper skunicode
}

# Compile bridge code and combine with Skia into libdrift_skia.a (device)
compile_bridge_device() {
  local arch="$1"
  local out_dir="out/ios/$arch"

  echo "Compiling bridge for iOS device $arch..."
  echo "Skia out dir: $SKIA_DIR/$out_dir"

  xcrun clang++ -arch "$arch" \
    -isysroot "$(xcrun --sdk iphoneos --show-sdk-path)" \
    -miphoneos-version-min=14.0 \
    -std=c++17 -fPIC -DSKIA_METAL \
    -I. -I./include \
    -c "$ROOT_DIR/pkg/skia/bridge/skia_metal.mm" \
    -o "$out_dir/skia_bridge.o"

  # Combine using libtool (macOS) - include all static libraries
  rm -f "$out_dir/libdrift_skia.a"
  libtool -static -o "$out_dir/libdrift_skia.a" \
    "$out_dir"/lib*.a "$out_dir/skia_bridge.o"
  rm "$out_dir/skia_bridge.o"

  echo "Created $SKIA_DIR/$out_dir/libdrift_skia.a"
}

# Compile bridge code and combine with Skia into libdrift_skia.a (simulator)
compile_bridge_simulator() {
  local arch="$1"
  local out_dir="out/ios-simulator/$arch"

  # Map Skia's arch names to clang's -arch flag
  local clang_arch="$arch"
  if [[ "$arch" == "x64" ]]; then
    clang_arch="x86_64"
  fi

  echo "Compiling bridge for iOS simulator $arch..."

  xcrun clang++ -arch "$clang_arch" \
    -isysroot "$(xcrun --sdk iphonesimulator --show-sdk-path)" \
    -mios-simulator-version-min=14.0 \
    -std=c++17 -fPIC -DSKIA_METAL \
    -I. -I./include \
    -c "$ROOT_DIR/pkg/skia/bridge/skia_metal.mm" \
    -o "$out_dir/skia_bridge.o"

  rm -f "$out_dir/libdrift_skia.a"
  libtool -static -o "$out_dir/libdrift_skia.a" \
    "$out_dir"/lib*.a "$out_dir/skia_bridge.o"
  rm "$out_dir/skia_bridge.o"

  echo "Created $SKIA_DIR/$out_dir/libdrift_skia.a"
}

# Build for physical iOS devices
build_device out/ios/arm64 arm64

# Build for iOS Simulator
# arm64 for Apple Silicon Macs, x64 for Intel Macs
build_simulator out/ios-simulator/arm64 arm64
build_simulator out/ios-simulator/x64 x64

# Compile bridge and create combined libraries
compile_bridge_device arm64
compile_bridge_simulator arm64
compile_bridge_simulator x64

copy_lib() {
  local platform="$1"
  local arch="$2"
  local src="$SKIA_DIR/out/$platform/$arch/libdrift_skia.a"
  local dst="$DRIFT_SKIA_OUT/$platform/$arch"
  if [[ ! -f "$src" ]]; then
    echo "Missing $src" >&2
    exit 1
  fi
  mkdir -p "$dst"
  cp "$src" "$dst/libdrift_skia.a"
  echo "Copied $src -> $dst/libdrift_skia.a"
}

copy_lib ios arm64
copy_lib ios-simulator arm64
copy_lib ios-simulator x64
