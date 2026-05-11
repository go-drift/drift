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

# Bridge re-link recipe.
# Mirrors compile_bridge_* in scripts/build_skia_{ios,android,xtool}.sh:
# both skia_common.cc and the backend file are compiled, and the resulting
# libdrift_skia.a bundles every lib*.a produced by ninja (skia, svg,
# skparagraph, skshaper, skunicode, skottie, skresources), not just libskia.a.
# Keep these targets in sync with the scripts when the bridge sources change.

# iOS device bridge rebuild (macOS only)
bridge-ios:
	@echo "Rebuilding iOS device bridge..."
	@test -f "$(SKIA_DIR)/out/ios/arm64/libskia.a" || (echo "libskia.a not found. Run scripts/build_skia_ios.sh first."; exit 1)
	cd $(SKIA_DIR) && \
		FLAGS="-arch arm64 -isysroot $$(xcrun --sdk iphoneos --show-sdk-path) -miphoneos-version-min=16.0 -std=c++17 -fPIC -DSKIA_METAL -I. -I./include" && \
		xcrun clang++ $$FLAGS -c ../../$(BRIDGE_DIR)/skia_common.cc -o out/ios/arm64/skia_common.o && \
		xcrun clang++ $$FLAGS -c ../../$(BRIDGE_DIR)/skia_metal.mm -o out/ios/arm64/skia_backend.o && \
		rm -f out/ios/arm64/libdrift_skia.a && \
		libtool -static -o out/ios/arm64/libdrift_skia.a out/ios/arm64/lib*.a out/ios/arm64/skia_common.o out/ios/arm64/skia_backend.o && \
		rm -f out/ios/arm64/skia_common.o out/ios/arm64/skia_backend.o
	@mkdir -p $(DRIFT_SKIA_DIR)/ios/arm64
	@cp $(SKIA_DIR)/out/ios/arm64/libdrift_skia.a $(DRIFT_SKIA_DIR)/ios/arm64/libdrift_skia.a
	@echo "Created $(DRIFT_SKIA_DIR)/ios/arm64/libdrift_skia.a"

# iOS simulator bridge rebuild (macOS only)
bridge-ios-sim:
	@echo "Rebuilding iOS simulator bridge (arm64)..."
	@test -f "$(SKIA_DIR)/out/ios-simulator/arm64/libskia.a" || (echo "libskia.a not found. Run scripts/build_skia_ios.sh first."; exit 1)
	cd $(SKIA_DIR) && \
		FLAGS="-arch arm64 -isysroot $$(xcrun --sdk iphonesimulator --show-sdk-path) -mios-simulator-version-min=16.0 -std=c++17 -fPIC -DSKIA_METAL -I. -I./include" && \
		xcrun clang++ $$FLAGS -c ../../$(BRIDGE_DIR)/skia_common.cc -o out/ios-simulator/arm64/skia_common.o && \
		xcrun clang++ $$FLAGS -c ../../$(BRIDGE_DIR)/skia_metal.mm -o out/ios-simulator/arm64/skia_backend.o && \
		rm -f out/ios-simulator/arm64/libdrift_skia.a && \
		libtool -static -o out/ios-simulator/arm64/libdrift_skia.a out/ios-simulator/arm64/lib*.a out/ios-simulator/arm64/skia_common.o out/ios-simulator/arm64/skia_backend.o && \
		rm -f out/ios-simulator/arm64/skia_common.o out/ios-simulator/arm64/skia_backend.o
	@mkdir -p $(DRIFT_SKIA_DIR)/ios-simulator/arm64
	@cp $(SKIA_DIR)/out/ios-simulator/arm64/libdrift_skia.a $(DRIFT_SKIA_DIR)/ios-simulator/arm64/libdrift_skia.a
	@echo "Created $(DRIFT_SKIA_DIR)/ios-simulator/arm64/libdrift_skia.a"

# Android bridge rebuild (requires NDK)
bridge-android:
	@echo "Rebuilding Android bridge (arm64)..."
	@test -n "$(ANDROID_NDK_HOME)" || (echo "ANDROID_NDK_HOME not set"; exit 1)
	@test -f "$(SKIA_DIR)/out/android/arm64/libskia.a" || (echo "libskia.a not found. Run scripts/build_skia_android.sh first."; exit 1)
	$(eval NDK_CLANG := $(ANDROID_NDK_HOME)/toolchains/llvm/prebuilt/$(HOST_TAG)/bin/clang++)
	cd $(SKIA_DIR) && \
		FLAGS="--target=aarch64-linux-android21 -std=c++17 -fPIC -DSKIA_VULKAN -I. -I./include" && \
		$(NDK_CLANG) $$FLAGS -c ../../$(BRIDGE_DIR)/skia_common.cc -o out/android/arm64/skia_common.o && \
		$(NDK_CLANG) $$FLAGS -c ../../$(BRIDGE_DIR)/skia_vk.cc -o out/android/arm64/skia_backend.o
	cd $(SKIA_DIR)/out/android/arm64 && \
		rm -f libdrift_skia.a && mkdir -p tmp && cd tmp && \
		for lib in ../lib*.a; do [ -f "$$lib" ] && ar x "$$lib"; done && \
		ar rcs ../libdrift_skia.a *.o ../skia_common.o ../skia_backend.o && \
		cd .. && rm -rf tmp skia_common.o skia_backend.o
	@mkdir -p $(DRIFT_SKIA_DIR)/android/arm64
	@cp $(SKIA_DIR)/out/android/arm64/libdrift_skia.a $(DRIFT_SKIA_DIR)/android/arm64/libdrift_skia.a
	@echo "Created $(DRIFT_SKIA_DIR)/android/arm64/libdrift_skia.a"

# xtool bridge rebuild (Linux cross-compile for iOS)
bridge-xtool:
	@echo "Rebuilding xtool bridge (iOS arm64)..."
	@test -f "$(SKIA_DIR)/out/ios/arm64/libskia.a" || (echo "libskia.a not found. Run scripts/build_skia_xtool.sh first."; exit 1)
	$(eval XTOOL_CLANG := $(shell which clang++ 2>/dev/null || echo /opt/swift/usr/bin/clang++))
	$(eval XTOOL_SDK := $(shell ls -d ~/.xtool/sdk/iPhoneOS*.sdk 2>/dev/null | head -1))
	@test -n "$(XTOOL_SDK)" || (echo "iOS SDK not found in ~/.xtool/sdk/"; exit 1)
	cd $(SKIA_DIR) && \
		FLAGS="-target arm64-apple-ios16.0 -isysroot $(XTOOL_SDK) -std=c++17 -fPIC -DSKIA_METAL -I. -I./include" && \
		$(XTOOL_CLANG) $$FLAGS -c ../../$(BRIDGE_DIR)/skia_common.cc -o out/ios/arm64/skia_common.o && \
		$(XTOOL_CLANG) $$FLAGS -c ../../$(BRIDGE_DIR)/skia_metal.mm -o out/ios/arm64/skia_backend.o
	cd $(SKIA_DIR)/out/ios/arm64 && \
		rm -f libdrift_skia.a && mkdir -p tmp && cd tmp && \
		for lib in ../lib*.a; do [ -f "$$lib" ] && llvm-ar x "$$lib"; done && \
		llvm-ar rcs ../libdrift_skia.a *.o ../skia_common.o ../skia_backend.o && \
		cd .. && rm -rf tmp skia_common.o skia_backend.o
	@mkdir -p $(DRIFT_SKIA_DIR)/ios/arm64
	@cp $(SKIA_DIR)/out/ios/arm64/libdrift_skia.a $(DRIFT_SKIA_DIR)/ios/arm64/libdrift_skia.a
	@echo "Created $(DRIFT_SKIA_DIR)/ios/arm64/libdrift_skia.a"
