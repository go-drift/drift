// swift-tools-version:5.9
// SPM package for exercising DriftPluginCoordinator's race-condition surface
// (merge function, dispatch lock + timer + once-only semantics) without
// going through the codegen path. The coordinator source lives in the iOS
// template tree; Sources/DriftPluginCoordinator/DriftPluginCoordinator.swift
// is a symlink to it, so a single hand-written source is shared between
// scaffolded apps and the XCTest harness. (SwiftPM rejects target paths
// outside the package root, so a relative `path:` cannot be used, and the
// package cannot live in templates/ios because the scaffold copies every
// .swift file there into apps.)
//
// Run on macOS with an iOS Simulator destination because DriftPluginCoordinator
// imports UIKit:
//
//   xcodebuild test \
//     -scheme DriftPluginCoordinatorTests \
//     -destination 'platform=iOS Simulator,name=iPhone 15'
//
// Or via SPM directly when the user has an iOS toolchain configured:
//
//   swift test --triple arm64-apple-ios16.0-simulator
//
// On Linux there is no UIKit, so the package cannot build there. CI must
// gate this harness on the iOS-toolchain runners that handle iOS builds.
import PackageDescription

let package = Package(
    name: "DriftPluginCoordinatorTests",
    platforms: [.iOS(.v16)],
    targets: [
        .target(name: "DriftPluginCoordinator"),
        .testTarget(
            name: "CoordinatorTests",
            dependencies: ["DriftPluginCoordinator"],
            path: "Tests/CoordinatorTests"
        ),
    ]
)
