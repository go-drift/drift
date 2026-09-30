#!/usr/bin/env bash
# Firebase iOS packaging spike (plugins v1, phase 1 item 6). macOS + Xcode 16+.
#
# Variants (all simulator builds, compile + link only, no GoogleService-Info):
#   A           current model: plugin Swift compiled into the Runner app target,
#               Firebase products reachable only via the Drift/Plugins sidecar.
#   A-explicit  A with SWIFT_ENABLE_EXPLICIT_MODULES=YES.
#   B           plugin Swift moved into its own SwiftPM target in the sidecar
#               (depends on DriftPluginAPI + Firebase products); the app
#               imports the plugin module and conforms to a protocol from
#               DriftPluginAPI.
#   B-explicit  B with SWIFT_ENABLE_EXPLICIT_MODULES=YES.
#
# Results: out/summary.txt, per-variant logs out/<variant>.log and the first
# 50 error lines in out/<variant>.errors.txt.
set -uo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
OUT="$HERE/out"
DRIFT="$REPO/bin/drift"
SLUG="example.com_firebasespike_app"

mkdir -p "$OUT"
: >"$OUT/summary.txt"

[ -x "$DRIFT" ] || { echo "missing $DRIFT: run 'make cli' in $REPO first" >&2; exit 1; }

record() {
	printf '%-12s %s\n' "$1" "$2" | tee -a "$OUT/summary.txt"
	if [ "$2" != PASS ] && [ -f "$OUT/$1.log" ]; then
		grep -m 50 'error:' "$OUT/$1.log" >"$OUT/$1.errors.txt" || true
	fi
}

{ xcodebuild -version; sw_vers; } | tee -a "$OUT/summary.txt"

# ---- A: stock drift pipeline -------------------------------------------------
echo "== A: drift build ios (first SwiftPM resolve of firebase-ios-sdk is slow)"
if (cd "$HERE/app" && "$DRIFT" build ios) >"$OUT/A.log" 2>&1; then
	record A PASS
else
	record A FAIL
fi

PROJ_DIR="$(ls -dt "${DRIFT_CACHE_DIR:-$HOME/.drift}/build/$SLUG/ios/"*/ios 2>/dev/null | head -1)"
if [ -z "$PROJ_DIR" ] || [ ! -d "$PROJ_DIR/Runner.xcodeproj" ]; then
	record SETUP "FAIL: generated Xcode project not found (see out/A.log)"
	exit 1
fi
echo "project: $PROJ_DIR" | tee -a "$OUT/summary.txt"

case "$(uname -m)" in
arm64) ARCH=arm64 ;;
*) ARCH=x86_64 ;;
esac

# xb <variant> [extra build settings...]: same xcodebuild invocation as
# `drift build ios` (simulator), with a fresh DerivedData per variant and a
# shared SwiftPM clone dir so firebase-ios-sdk is fetched once.
xb() {
	local variant="$1"
	shift
	rm -rf "$OUT/dd-$variant"
	echo "== $variant"
	if (cd "$PROJ_DIR" && xcodebuild \
		-project Runner.xcodeproj -scheme Runner -configuration Debug \
		-destination 'generic/platform=iOS Simulator' \
		-derivedDataPath "$OUT/dd-$variant" \
		-clonedSourcePackagesDirPath "$OUT/spm" \
		ARCHS="$ARCH" "$@" build) >"$OUT/$variant.log" 2>&1; then
		record "$variant" PASS
	else
		record "$variant" FAIL
	fi
}

xb A-explicit SWIFT_ENABLE_EXPLICIT_MODULES=YES

# ---- B: per-plugin SwiftPM target -------------------------------------------
PKG="$PROJ_DIR/Drift/Plugins"
PLUGIN_SRC="$PROJ_DIR/Runner/Plugins/firebasespike/FirebaseSpikePlugin.swift"
REGISTRANT="$(find "$PROJ_DIR/Runner" -maxdepth 1 -name DriftPluginRegistrant.swift | head -1)"
if [ ! -f "$PLUGIN_SRC" ] || [ ! -f "$PKG/Empty.swift" ] || [ -z "$REGISTRANT" ]; then
	record SETUP-B "FAIL: generated layout differs from what run.sh expects"
	ls -R "$PKG" "$PROJ_DIR/Runner" >"$OUT/layout.txt" 2>&1
	exit 1
fi

mkdir -p "$PKG/Sources/DriftPlugins" "$PKG/Sources/DriftPluginAPI" "$PKG/Sources/DriftPlugin_firebasespike"
mv "$PKG/Empty.swift" "$PKG/Sources/DriftPlugins/Empty.swift"
mv "$PLUGIN_SRC" "$PKG/Sources/DriftPlugin_firebasespike/"
rmdir "$PROJ_DIR/Runner/Plugins/firebasespike"

cat >"$PKG/Sources/DriftPluginAPI/DriftPluginAPI.swift" <<'SWIFT'
public protocol SpikePluginHost: AnyObject {
    func ping()
}
SWIFT

cat >"$PKG/Sources/DriftPlugin_firebasespike/SpikeUsesAPI.swift" <<'SWIFT'
import DriftPluginAPI

public func spikePing(_ host: SpikePluginHost) {
    host.ping()
}
SWIFT

cat >"$PKG/Package.swift" <<'SWIFT'
// swift-tools-version:5.9
import PackageDescription

let package = Package(
    name: "DriftPlugins",
    platforms: [.iOS(.v16)],
    products: [
        .library(
            name: "DriftPlugins",
            targets: ["DriftPlugins", "DriftPluginAPI", "DriftPlugin_firebasespike"]
        ),
    ],
    dependencies: [
        .package(url: "https://github.com/firebase/firebase-ios-sdk", from: "11.0.0"),
    ],
    targets: [
        .target(name: "DriftPlugins", path: "Sources/DriftPlugins"),
        .target(name: "DriftPluginAPI", path: "Sources/DriftPluginAPI"),
        .target(
            name: "DriftPlugin_firebasespike",
            dependencies: [
                "DriftPluginAPI",
                .product(name: "FirebaseCore", package: "firebase-ios-sdk"),
                .product(name: "FirebaseMessaging", package: "firebase-ios-sdk"),
            ],
            path: "Sources/DriftPlugin_firebasespike"
        ),
    ]
)
SWIFT

# The app target imports the plugin module (registrant call site) and
# conforms to a protocol declared in DriftPluginAPI (host adoption).
perl -pi -e 's/^import UIKit$/import UIKit\nimport DriftPlugin_firebasespike/' "$REGISTRANT"

cat >"$PROJ_DIR/Runner/SpikeImport.swift" <<'SWIFT'
import DriftPluginAPI
import DriftPlugin_firebasespike

final class SpikeHost: SpikePluginHost {
    func ping() {}
}

func driftSpikeTouch() {
    spikePing(SpikeHost())
}
SWIFT

xb B
xb B-explicit SWIFT_ENABLE_EXPLICIT_MODULES=YES

echo
echo "Done. Send back out/summary.txt and any out/*.errors.txt."
