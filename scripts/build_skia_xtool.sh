#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKIA_DIR="$ROOT_DIR/third_party/skia"
DRIFT_SKIA_OUT="$ROOT_DIR/third_party/drift_skia"

if [[ ! -d "$SKIA_DIR" ]]; then
  echo "Skia not found at $SKIA_DIR. Run scripts/fetch_skia.sh first."
  exit 1
fi

# Auto-detect iOS SDK (same logic as cmd/drift/internal/xtool/xtool.go)
find_ios_sdk() {
  # Check XTOOL_SDK_PATH first
  if [[ -n "${XTOOL_SDK_PATH:-}" ]] && [[ -d "$XTOOL_SDK_PATH" ]]; then
    echo "$XTOOL_SDK_PATH"
    return
  fi

  local home="$HOME"
  local candidates=(
    "$home/.xtool/sdk/iPhoneOS.sdk"
    "$home/.xtool/SDKs/iPhoneOS.sdk"
    "/opt/xtool/SDKs/iPhoneOS.sdk"
  )

  # Check for versioned SDKs (e.g., iPhoneOS17.0.sdk)
  if [[ -d "$home/.xtool/sdk" ]]; then
    for sdk in "$home/.xtool/sdk"/iPhoneOS*.sdk; do
      [[ -d "$sdk" ]] && candidates=("$sdk" "${candidates[@]}")
    done
  fi

  for path in "${candidates[@]}"; do
    if [[ -d "$path" ]]; then
      echo "$path"
      return
    fi
  done

  echo ""
}

find_simulator_sdk() {
  local home="$HOME"
  local candidates=(
    "$home/.xtool/sdk/iPhoneSimulator.sdk"
    "$home/.xtool/SDKs/iPhoneSimulator.sdk"
    "/opt/xtool/SDKs/iPhoneSimulator.sdk"
  )

  # Check for versioned SDKs
  if [[ -d "$home/.xtool/sdk" ]]; then
    for sdk in "$home/.xtool/sdk"/iPhoneSimulator*.sdk; do
      [[ -d "$sdk" ]] && candidates=("$sdk" "${candidates[@]}")
    done
  fi

  for path in "${candidates[@]}"; do
    if [[ -d "$path" ]]; then
      echo "$path"
      return
    fi
  done

  echo ""
}

# Auto-detect clang (same logic as cmd/drift/internal/xtool/xtool.go)
find_clang() {
  local home="$HOME"

  # Priority 1: Swift toolchain's clang (has Objective-C support)
  local swift_paths=(
    "/opt/swift/usr/bin/clang"
    "$home/.swiftly/toolchains/swift-latest/usr/bin/clang"
    "/usr/share/swift/usr/bin/clang"
  )

  # Check swiftly versioned toolchains
  if [[ -d "$home/.swiftly/toolchains" ]]; then
    for tc in "$home/.swiftly/toolchains"/swift-*; do
      [[ -d "$tc" ]] && swift_paths+=("$tc/usr/bin/clang")
    done
  fi

  for path in "${swift_paths[@]}"; do
    if [[ -x "$path" ]]; then
      echo "$path"
      return
    fi
  done

  # Priority 2: xtool's bundled clang
  local xtool_paths=(
    "$home/.xtool/toolchain/bin/clang"
    "$home/.xtool/usr/bin/clang"
    "/opt/xtool/toolchain/bin/clang"
  )

  for path in "${xtool_paths[@]}"; do
    if [[ -x "$path" ]]; then
      echo "$path"
      return
    fi
  done

  # Priority 3: System clang
  for name in clang clang-18 clang-17 clang-16 clang-15 clang-14; do
    if command -v "$name" &>/dev/null; then
      command -v "$name"
      return
    fi
  done

  echo ""
}

# Detect paths
IPHONEOS_SDK=$(find_ios_sdk)
IPHONESIMULATOR_SDK=$(find_simulator_sdk)
CLANG=$(find_clang)

if [[ -z "$IPHONEOS_SDK" ]]; then
  echo "iOS SDK not found. Run 'xtool setup' with Xcode.xip or set XTOOL_SDK_PATH."
  echo "Checked: ~/.xtool/sdk/iPhoneOS.sdk, ~/.xtool/SDKs/iPhoneOS.sdk, /opt/xtool/SDKs/iPhoneOS.sdk"
  exit 1
fi

if [[ -z "$CLANG" ]]; then
  echo "clang not found. Install Swift toolchain from https://swift.org/download/"
  exit 1
fi

CLANGXX="${CLANG}++"

echo "Using iOS SDK: $IPHONEOS_SDK"
echo "Using Simulator SDK: ${IPHONESIMULATOR_SDK:-none}"
echo "Using clang: $CLANG"

# Create wrapper scripts that translate -arch to --target for Linux clang
WRAPPER_DIR="$ROOT_DIR/.xtool-wrappers"
mkdir -p "$WRAPPER_DIR"

create_wrapper() {
  local name="$1" real_clang="$2" target_suffix="$3"
  cat > "$WRAPPER_DIR/$name" << EOF
#!/usr/bin/env bash
# Translate -arch to --target for iOS cross-compilation
args=() arch=""
for arg; do
  case "\$arg" in -arch) ;; arm64|arm64e|x86_64|armv7) [[ -z "\$arch" ]] && arch="\$arg" ;;
    -miphoneos-version-min=*) args+=("--target=\${arch:-arm64}-apple-ios\${arg#*=}") ;;
    -miphonesimulator-version-min=*) args+=("--target=\${arch:-arm64}-apple-ios\${arg#*=}-simulator") ;;
    *) args+=("\$arg") ;;
  esac
done
[[ -n "\$arch" && ! " \${args[*]} " =~ " --target=" ]] && args=("--target=\${arch}-apple-ios14.0${target_suffix}" "\${args[@]}")
exec $real_clang "\${args[@]}"
EOF
  chmod +x "$WRAPPER_DIR/$name"
}

create_wrapper "clang-ios" "$CLANG" ""
create_wrapper "clang++-ios" "$CLANGXX" ""
[[ -n "$IPHONESIMULATOR_SDK" ]] && create_wrapper "clang-ios-sim" "$CLANG" "-simulator"
[[ -n "$IPHONESIMULATOR_SDK" ]] && create_wrapper "clang++-ios-sim" "$CLANGXX" "-simulator"

cd "$SKIA_DIR"
python3 tools/git-sync-deps

# Common Skia build args
COMMON_ARGS='is_official_build=true skia_use_metal=true skia_use_system_harfbuzz=false skia_use_system_expat=false skia_use_system_libpng=false skia_use_system_zlib=false skia_use_system_freetype2=false skia_use_system_libjpeg_turbo=false skia_use_libjpeg_turbo_decode=true skia_use_libjpeg_turbo_encode=true skia_use_system_libwebp=false skia_use_libwebp_decode=true skia_use_libwebp_encode=true'

# For cross-compilation, we need to tell GN where the toolchain is
# Use wrapper scripts that translate -arch to --target
XTOOL_ARGS="xcode_sysroot=\"$IPHONEOS_SDK\" cc=\"$WRAPPER_DIR/clang-ios\" cxx=\"$WRAPPER_DIR/clang++-ios\""
XTOOL_SIM_ARGS="xcode_sysroot=\"$IPHONESIMULATOR_SDK\" cc=\"$WRAPPER_DIR/clang-ios-sim\" cxx=\"$WRAPPER_DIR/clang++-ios-sim\""

build_device() {
  local out_dir="$1"
  local target_cpu="$2"
  echo "Building iOS device ($target_cpu)..."
  bin/gn gen "$out_dir" --args="target_os=\"ios\" target_cpu=\"$target_cpu\" $COMMON_ARGS $XTOOL_ARGS"
  ninja -C "$out_dir" skia
}

build_simulator() {
  local out_dir="$1"
  local target_cpu="$2"
  if [[ -z "$IPHONESIMULATOR_SDK" ]]; then
    echo "Skipping simulator build ($target_cpu): iPhoneSimulator.sdk not found"
    return
  fi
  echo "Building iOS simulator ($target_cpu)..."
  bin/gn gen "$out_dir" --args="target_os=\"ios\" target_cpu=\"$target_cpu\" ios_use_simulator=true $COMMON_ARGS $XTOOL_SIM_ARGS"
  ninja -C "$out_dir" skia
}

# Compile bridge code and combine with Skia into libdrift_skia.a (device)
compile_bridge_device() {
  local arch="$1"
  local out_dir="out/ios/$arch"

  echo "Compiling bridge for iOS device $arch..."

  "$CLANGXX" -target "${arch}-apple-ios14.0" \
    -isysroot "$IPHONEOS_SDK" \
    -std=c++17 -fPIC -DSKIA_METAL \
    -I. -I./include \
    -c "$ROOT_DIR/pkg/skia/bridge/skia_metal.mm" \
    -o "$out_dir/skia_bridge.o"

  # Combine using llvm-ar (required for Mach-O archives on Linux)
  mkdir -p "$out_dir/tmp"
  pushd "$out_dir/tmp" > /dev/null
  llvm-ar x ../libskia.a
  llvm-ar rcs ../libdrift_skia.a *.o ../skia_bridge.o
  popd > /dev/null
  rm -rf "$out_dir/tmp" "$out_dir/skia_bridge.o"

  echo "Created $out_dir/libdrift_skia.a"
}

# Compile bridge code and combine with Skia into libdrift_skia.a (simulator)
compile_bridge_simulator() {
  local arch="$1"
  local out_dir="out/ios-simulator/$arch"

  if [[ -z "$IPHONESIMULATOR_SDK" ]]; then
    echo "Skipping simulator build: iPhoneSimulator.sdk not found"
    return
  fi

  echo "Compiling bridge for iOS simulator $arch..."

  "$CLANGXX" -target "${arch}-apple-ios14.0-simulator" \
    -isysroot "$IPHONESIMULATOR_SDK" \
    -std=c++17 -fPIC -DSKIA_METAL \
    -I. -I./include \
    -c "$ROOT_DIR/pkg/skia/bridge/skia_metal.mm" \
    -o "$out_dir/skia_bridge.o"

  # Combine using llvm-ar (required for Mach-O archives on Linux)
  mkdir -p "$out_dir/tmp"
  pushd "$out_dir/tmp" > /dev/null
  llvm-ar x ../libskia.a
  llvm-ar rcs ../libdrift_skia.a *.o ../skia_bridge.o
  popd > /dev/null
  rm -rf "$out_dir/tmp" "$out_dir/skia_bridge.o"

  echo "Created $out_dir/libdrift_skia.a"
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
}

copy_lib ios arm64
if [[ -n "$IPHONESIMULATOR_SDK" ]]; then
  copy_lib ios-simulator arm64
  copy_lib ios-simulator x64
fi
