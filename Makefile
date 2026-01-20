GO ?= go
GRADLE ?= ./gradlew
ANDROID_SDK_ROOT ?= $(ANDROID_HOME)
ANDROID_NDK_HOME ?= $(ANDROID_NDK_ROOT)
HOST_TAG ?= linux-x86_64

ADB ?= $(ANDROID_SDK_ROOT)/platform-tools/adb
EMULATOR ?= $(ANDROID_SDK_ROOT)/emulator/emulator

ANDROID_ABIS := arm64-v8a armeabi-v7a x86_64
NDK_TOOLCHAIN := $(ANDROID_NDK_HOME)/toolchains/llvm/prebuilt/$(HOST_TAG)/bin
NDK_SYSROOT_LIB := $(ANDROID_NDK_HOME)/toolchains/llvm/prebuilt/$(HOST_TAG)/sysroot/usr/lib

# Showcase project paths
SHOWCASE_DIR := showcase
ANDROID_JNI_DIR := $(SHOWCASE_DIR)/android/app/src/main/jniLibs
ANDROID_APK := $(SHOWCASE_DIR)/android/app/build/outputs/apk/debug/app-debug.apk

.PHONY: all cli skia-release android-libs android-build android-install android-run android-log android-emulator clean bridge-ios bridge-ios-sim bridge-android bridge-xtool

# Build the drift CLI tool
cli:
	$(GO) build -o bin/drift ./cmd/drift

# Build and package Skia release artifacts
skia-release:
	./scripts/build_skia_release.sh

# Build Go shared libraries for all Android ABIs
android-libs:
	@test -d "$(NDK_TOOLCHAIN)" || (echo "Missing NDK toolchain. Set ANDROID_NDK_HOME and HOST_TAG."; exit 1)
	@for abi in $(ANDROID_ABIS); do \
		case $$abi in \
			arm64-v8a) GOARCH=arm64; GOARM=; CC=$(NDK_TOOLCHAIN)/aarch64-linux-android21-clang; CXX=$(NDK_TOOLCHAIN)/aarch64-linux-android21-clang++; NDK_TRIPLE=aarch64-linux-android ;; \
			armeabi-v7a) GOARCH=arm; GOARM=7; CC=$(NDK_TOOLCHAIN)/armv7a-linux-androideabi21-clang; CXX=$(NDK_TOOLCHAIN)/armv7a-linux-androideabi21-clang++; NDK_TRIPLE=arm-linux-androideabi ;; \
			x86_64) GOARCH=amd64; GOARM=; CC=$(NDK_TOOLCHAIN)/x86_64-linux-android21-clang; CXX=$(NDK_TOOLCHAIN)/x86_64-linux-android21-clang++; NDK_TRIPLE=x86_64-linux-android ;; \
			*) echo "Unknown ABI $$abi"; exit 1 ;; \
		esac; \
		outdir=$(ANDROID_JNI_DIR)/$$abi; \
		mkdir -p $$outdir; \
		cd $(SHOWCASE_DIR) && CGO_ENABLED=1 GOOS=android GOARCH=$$GOARCH GOARM=$$GOARM CC=$$CC CXX=$$CXX \
			$(GO) build -buildmode=c-shared -o ../$$outdir/libdrift.so . && cd ..; \
		cpp_shared=$(NDK_SYSROOT_LIB)/$$NDK_TRIPLE/libc++_shared.so; \
		if [ -f $$cpp_shared ]; then \
			cp $$cpp_shared $$outdir/; \
		else \
			echo "Missing libc++_shared.so for $$abi at $$cpp_shared"; exit 1; \
		fi; \
		rm -f $$outdir/libdrift.h; \
	done

# Build the Android APK
android-build: android-libs
	cd $(SHOWCASE_DIR)/android && $(GRADLE) assembleDebug

# Install APK on connected device
android-install: android-build
	$(ADB) install -r $(ANDROID_APK)

# Build, install, and run on Android device
android-run: android-install
	$(ADB) shell am start -n com.drift.showcase/.MainActivity

# View Android logs
android-log:
	$(ADB) logcat -v time DriftJNI:* Go:* drift:* AndroidRuntime:E *:S

# Start Android emulator (requires AVD environment variable)
android-emulator:
	@test -n "$(AVD)" || (echo "Set AVD=<name>"; exit 1)
	$(EMULATOR) -avd $(AVD)

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf $(SHOWCASE_DIR)/android/app/build
	rm -rf $(SHOWCASE_DIR)/android/.gradle
	rm -rf $(SHOWCASE_DIR)/build
	rm -f $(ANDROID_JNI_DIR)/*/*.so

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
