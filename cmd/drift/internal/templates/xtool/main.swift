/// main.swift
/// Explicit entry point for SwiftPM executable.
///
/// SwiftPM executables require an explicit main.swift file with UIApplicationMain
/// instead of using the @main attribute on AppDelegate.

import UIKit

UIApplicationMain(
    CommandLine.argc,
    CommandLine.unsafeArgv,
    nil,
    NSStringFromClass(AppDelegate.self)
)
