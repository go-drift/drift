GO ?= go
ANDROID_SDK_ROOT ?= $(ANDROID_HOME)
ANDROID_NDK_HOME ?= $(ANDROID_NDK_ROOT)
HOST_TAG ?= linux-x86_64

.PHONY: all cli skia-release clean bridge-ios bridge-ios-sim bridge-android bridge-xtool

# Build the drift CLI tool
cli:
	$(GO) build -o bin/drift ./cmd/drift

# Build and package Skia release artifacts
skia-release:
	./scripts/build_skia_release.sh

# Clean build artifacts
clean:
	rm -rf bin/

# --------------------------------------------------------------------------
# Fast Bridge Iteration Targets
# These rebuild only the bridge code without rebuilding Skia.
# Requires Skia to be already built (libskia.a must exist).
# --------------------------------------------------------------------------

SKIA_DIR := third_party/skia
DRIFT_SKIA_DIR := third_party/drift_skia
BRIDGE_DIR := pkg/skia/bridge

# iOS device bridge rebuild (macOS only)
bridge-ios:
	@echo "Rebuilding iOS device bridge..."
	@test -f "$(SKIA_DIR)/out/ios/arm64/libskia.a" || (echo "libskia.a not found. Run scripts/build_skia_ios.sh first."; exit 1)
	cd $(SKIA_DIR) && xcrun clang++ -arch arm64 \
		-isysroot "$$(xcrun --sdk iphoneos --show-sdk-path)" \
		-miphoneos-version-min=14.0 \
		-std=c++17 -fPIC -DSKIA_METAL \
		-I. -I./include \
		-c ../../$(BRIDGE_DIR)/skia_metal.mm \
		-o out/ios/arm64/skia_bridge.o
	cd $(SKIA_DIR) && libtool -static -o out/ios/arm64/libdrift_skia.a \
		out/ios/arm64/libskia.a out/ios/arm64/skia_bridge.o
	rm -f $(SKIA_DIR)/out/ios/arm64/skia_bridge.o
	@mkdir -p $(DRIFT_SKIA_DIR)/ios/arm64
	@cp $(SKIA_DIR)/out/ios/arm64/libdrift_skia.a $(DRIFT_SKIA_DIR)/ios/arm64/libdrift_skia.a
	@echo "Created $(DRIFT_SKIA_DIR)/ios/arm64/libdrift_skia.a"

# iOS simulator bridge rebuild (macOS only)
bridge-ios-sim:
	@echo "Rebuilding iOS simulator bridge (arm64)..."
	@test -f "$(SKIA_DIR)/out/ios-simulator/arm64/libskia.a" || (echo "libskia.a not found. Run scripts/build_skia_ios.sh first."; exit 1)
	cd $(SKIA_DIR) && xcrun clang++ -arch arm64 \
		-isysroot "$$(xcrun --sdk iphonesimulator --show-sdk-path)" \
		-mios-simulator-version-min=14.0 \
		-std=c++17 -fPIC -DSKIA_METAL \
		-I. -I./include \
		-c ../../$(BRIDGE_DIR)/skia_metal.mm \
		-o out/ios-simulator/arm64/skia_bridge.o
	cd $(SKIA_DIR) && libtool -static -o out/ios-simulator/arm64/libdrift_skia.a \
		out/ios-simulator/arm64/libskia.a out/ios-simulator/arm64/skia_bridge.o
	rm -f $(SKIA_DIR)/out/ios-simulator/arm64/skia_bridge.o
	@mkdir -p $(DRIFT_SKIA_DIR)/ios-simulator/arm64
	@cp $(SKIA_DIR)/out/ios-simulator/arm64/libdrift_skia.a $(DRIFT_SKIA_DIR)/ios-simulator/arm64/libdrift_skia.a
	@echo "Created $(DRIFT_SKIA_DIR)/ios-simulator/arm64/libdrift_skia.a"

# Android bridge rebuild (requires NDK)
bridge-android:
	@echo "Rebuilding Android bridge (arm64)..."
	@test -n "$(ANDROID_NDK_HOME)" || (echo "ANDROID_NDK_HOME not set"; exit 1)
	@test -f "$(SKIA_DIR)/out/android/arm64/libskia.a" || (echo "libskia.a not found. Run scripts/build_skia_android.sh first."; exit 1)
	$(eval NDK_CLANG := $(ANDROID_NDK_HOME)/toolchains/llvm/prebuilt/$(HOST_TAG)/bin/clang++)
	cd $(SKIA_DIR) && $(NDK_CLANG) --target=aarch64-linux-android21 \
		-std=c++17 -fPIC -DSKIA_GL \
		-I. -I./include \
		-c ../../$(BRIDGE_DIR)/skia_gl.cc \
		-o out/android/arm64/skia_bridge.o
	cd $(SKIA_DIR)/out/android/arm64 && mkdir -p tmp && cd tmp && \
		ar x ../libskia.a && ar rcs ../libdrift_skia.a *.o ../skia_bridge.o && \
		cd .. && rm -rf tmp skia_bridge.o
	@mkdir -p $(DRIFT_SKIA_DIR)/android/arm64
	@cp $(SKIA_DIR)/out/android/arm64/libdrift_skia.a $(DRIFT_SKIA_DIR)/android/arm64/libdrift_skia.a
	@echo "Created $(DRIFT_SKIA_DIR)/android/arm64/libdrift_skia.a"

# xtool bridge rebuild (Linux cross-compile for iOS)
bridge-xtool:
	@echo "Rebuilding xtool bridge (iOS arm64)..."
	@test -f "$(SKIA_DIR)/out/ios/arm64/libskia.a" || (echo "libskia.a not found. Run scripts/build_skia_ios_xtool.sh first."; exit 1)
	$(eval XTOOL_CLANG := $(shell which clang++ 2>/dev/null || echo /opt/swift/usr/bin/clang++))
	$(eval XTOOL_SDK := $(shell ls -d ~/.xtool/sdk/iPhoneOS*.sdk 2>/dev/null | head -1))
	@test -n "$(XTOOL_SDK)" || (echo "iOS SDK not found in ~/.xtool/sdk/"; exit 1)
	cd $(SKIA_DIR) && $(XTOOL_CLANG) -target arm64-apple-ios14.0 \
		-isysroot $(XTOOL_SDK) \
		-std=c++17 -fPIC -DSKIA_METAL \
		-I. -I./include \
		-c ../../$(BRIDGE_DIR)/skia_metal.mm \
		-o out/ios/arm64/skia_bridge.o
	cd $(SKIA_DIR)/out/ios/arm64 && mkdir -p tmp && cd tmp && \
		llvm-ar x ../libskia.a && llvm-ar rcs ../libdrift_skia.a *.o ../skia_bridge.o && \
		cd .. && rm -rf tmp skia_bridge.o
	@mkdir -p $(DRIFT_SKIA_DIR)/ios/arm64
	@cp $(SKIA_DIR)/out/ios/arm64/libdrift_skia.a $(DRIFT_SKIA_DIR)/ios/arm64/libdrift_skia.a
	@echo "Created $(DRIFT_SKIA_DIR)/ios/arm64/libdrift_skia.a"
