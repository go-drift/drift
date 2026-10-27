/// DriftLocationPlugin.swift
///
/// The location plugin's native half on iOS, on the main thread: one
/// CLLocationManager answers one-shot requests and another streams updates
/// to Go on `drift/location` while Go listens, in the background too when
/// the options (`setUpdateOptions`) ask for it. Registers the `location` permission, and
/// `location_always` when the build added its usage string.
///
/// Errors Go maps to its sentinels (runtime package): `permission_denied`,
/// `location_disabled`, `location_unavailable`, `background_not_configured`
/// and `always_denied`. Core Location reads denied while location services
/// are off, so a denial reads `location_disabled` then, as on Android. Lost
/// access fails the update stream: `permission_denied` for a revoked
/// permission, `location_disabled` for services turned off.

import CoreLocation
import DriftPluginAPI
import UIKit

/// Public, with a public initializer: the app's generated registrant
/// creates it from outside this module (DriftPlugin_location).
public final class DriftLocationPlugin: NSObject, DriftPlugin, CLLocationManagerDelegate {
    private static let channel = "drift/location"

    /// Whether the build's always: config added the Always usage string and
    /// the location background mode. Background updates need both, and
    /// allowsBackgroundLocationUpdates must not be set without the mode
    /// (Core Location raises an exception).
    private static let backgroundConfigured: Bool = {
        let modes = Bundle.main.object(forInfoDictionaryKey: "UIBackgroundModes") as? [String] ?? []
        return modes.contains("location")
            && Bundle.main.object(forInfoDictionaryKey: "NSLocationAlwaysAndWhenInUseUsageDescription") != nil
    }()

    private var host: DriftPluginHost?
    private let oneShot = CLLocationManager()
    private let updates = CLLocationManager()
    /// Requests waiting for the one-shot manager's next fix.
    private var pendingCurrent: [DriftResult] = []
    /// The update stream's options, kept across starts.
    private var options = UpdateOptions()
    /// Whether the update stream runs (Go listens and it started).
    private var streaming = false
    /// Counts the update stream's starts and stops, so a failure reported
    /// late reaches only the listeners of the stream it belongs to.
    private var streamGeneration = 0

    public override init() {
        super.init()
        oneShot.delegate = self
        updates.delegate = self
    }

    public func register(host: DriftPluginHost) {
        self.host = host
        host.registerPermission("location", LocationPermission(always: false))
        if Bundle.main.object(forInfoDictionaryKey: "NSLocationAlwaysAndWhenInUseUsageDescription") != nil {
            host.registerPermission("location_always", LocationPermission(always: true))
        }
        host.registerChannel(Self.channel) { channel in
            channel.asyncMethod("getCurrentLocation") { [self] args, result in
                whenEnabled(result) { [self] in
                    try requireAuthorized()
                    let highAccuracy = (args as? [String: Any])?["highAccuracy"] as? Bool ?? true
                    oneShot.desiredAccuracy = highAccuracy ? kCLLocationAccuracyBest : kCLLocationAccuracyHundredMeters
                    pendingCurrent.append(result)
                    if pendingCurrent.count == 1 {
                        oneShot.requestLocation()
                    }
                }
            }
            channel.asyncMethod("getLastKnown") { [self] _, result in
                whenEnabled(result) { [self] in
                    try requireAuthorized()
                    result.success(oneShot.location.map(Self.payload))
                }
            }
            channel.asyncMethod("isEnabled") { [self] _, result in
                readServicesEnabled { result.success(["enabled": $0]) }
            }
            channel.method("setUpdateOptions") { [self] args in
                let next = UpdateOptions(args as? [String: Any] ?? [:])
                if next.background {
                    try requireBackground()
                }
                // A running stream that lost access takes the options too:
                // its listeners got the failure, and it resumes with them
                // when access returns.
                if streaming {
                    apply(next)
                }
                options = next
                return nil
            }
            channel.stream(
                onListen: { [self] in
                    streamGeneration += 1
                    guard authorized else {
                        // Not started. Which failure it is (permission or
                        // services) is read off the main thread.
                        let generation = streamGeneration
                        accessError(Self.permissionDenied) { [self] error in
                            if generation == streamGeneration {
                                self.host?.sendEventError(Self.channel, error)
                            }
                        }
                        return
                    }
                    if options.background {
                        try requireBackground()
                    }
                    apply(options)
                    updates.startUpdatingLocation()
                    streaming = true
                },
                onCancel: { [self] in
                    streamGeneration += 1
                    updates.stopUpdatingLocation()
                    streaming = false
                }
            )
        }
    }

    /// Configures the update manager for opts; Core Location applies it to
    /// a running stream at once.
    private func apply(_ opts: UpdateOptions) {
        updates.desiredAccuracy = opts.highAccuracy ? kCLLocationAccuracyBest : kCLLocationAccuracyHundredMeters
        updates.distanceFilter = opts.distanceFilter > 0 ? opts.distanceFilter : kCLDistanceFilterNone
        if Self.backgroundConfigured {
            updates.allowsBackgroundLocationUpdates = opts.background
        }
        // The status bar shows background use, as Android's notification
        // does. A paused stream does not resume by itself, and an app in
        // the background gets no chance to restart it, so a background
        // stream never pauses.
        updates.showsBackgroundLocationIndicator = opts.background
        updates.pausesLocationUpdatesAutomatically = !opts.background
    }

    private static let disabled = DriftCallError("location_disabled", "location services are turned off")
    private static let permissionDenied = DriftCallError("permission_denied", "location permission not granted; request it first")

    /// Calls back on the main thread with whether location services are on,
    /// read off it: Apple warns against reading it on the main thread.
    private func readServicesEnabled(_ completion: @escaping (Bool) -> Void) {
        DispatchQueue.global(qos: .userInitiated).async {
            let enabled = CLLocationManager.locationServicesEnabled()
            DispatchQueue.main.async { completion(enabled) }
        }
    }

    /// Runs body on the main thread once location services are known to be
    /// on; fails result with `location_disabled` otherwise, or with body's
    /// error.
    private func whenEnabled(_ result: DriftResult, _ body: @escaping () throws -> Void) {
        readServicesEnabled { enabled in
            guard enabled else {
                result.error(Self.disabled)
                return
            }
            do {
                try body()
            } catch {
                result.error(error)
            }
        }
    }

    /// Calls back on the main thread with the error for access Core
    /// Location denies: `location_disabled` while services are off (it
    /// then denies whatever the permission), otherwise denied.
    private func accessError(_ denied: DriftCallError, _ completion: @escaping (DriftCallError) -> Void) {
        readServicesEnabled { enabled in
            completion(enabled ? denied : Self.disabled)
        }
    }

    /// Whether the app may use location. True implies location services
    /// are on: while they are off, Core Location reads denied.
    private var authorized: Bool {
        switch oneShot.authorizationStatus {
        case .authorizedWhenInUse, .authorizedAlways:
            return true
        default:
            return false
        }
    }

    /// Throws `permission_denied` unless authorized. Call it once location
    /// services are known to be on (whenEnabled); otherwise the denial may
    /// be theirs.
    private func requireAuthorized() throws {
        guard authorized else { throw Self.permissionDenied }
    }

    private func requireBackground() throws {
        guard Self.backgroundConfigured else {
            throw DriftCallError("background_not_configured", "background location updates need the location plugin's always: config")
        }
        guard oneShot.authorizationStatus == .authorizedAlways else {
            throw DriftCallError("always_denied", "background location updates need the Always location permission; request it first")
        }
    }

    // MARK: - CLLocationManagerDelegate

    public func locationManager(_ manager: CLLocationManager, didUpdateLocations locations: [CLLocation]) {
        guard let location = locations.last else { return }
        if manager === oneShot {
            finishCurrent { $0.success(Self.payload(location)) }
        } else {
            host?.sendEvent(Self.channel, data: Self.payload(location))
        }
    }

    public func locationManager(_ manager: CLLocationManager, didFailWithError error: Error) {
        let code = (error as? CLError)?.code
        if manager === oneShot {
            switch code {
            case .denied?:
                finishCurrent { $0.error(DriftCallError("permission_denied", "location permission not granted")) }
            case .locationUnknown?:
                finishCurrent { $0.error(DriftCallError("location_unavailable", "no location fix available")) }
            default:
                finishCurrent { $0.error(error) }
            }
            return
        }
        switch code {
        case .locationUnknown?:
            // Transient: Core Location keeps trying, and the stream goes on.
            return
        case .denied?:
            // The permission was revoked or services turned off; the
            // updates resume if access returns.
            let generation = streamGeneration
            accessError(DriftCallError("permission_denied", "location permission revoked")) { [self] error in
                if generation == streamGeneration {
                    host?.sendEventError(Self.channel, error)
                }
            }
        default:
            host?.sendEventError(Self.channel, error)
        }
    }

    private func finishCurrent(_ reply: (DriftResult) -> Void) {
        let waiting = pendingCurrent
        pendingCurrent.removeAll()
        waiting.forEach(reply)
    }

    // MARK: - Options

    /// The update stream's options (the runtime package's UpdateOptions).
    private struct UpdateOptions {
        var highAccuracy = true
        var distanceFilter: CLLocationDistance = 0
        var background = false

        init() {}

        init(_ args: [String: Any]) {
            highAccuracy = args["highAccuracy"] as? Bool ?? true
            distanceFilter = args["distanceFilter"] as? Double ?? 0
            background = args["background"] as? Bool ?? false
        }
    }

    // MARK: - Payloads

    /// The wire form of a location (see the runtime package).
    private static func payload(_ location: CLLocation) -> [String: Any] {
        [
            "latitude": location.coordinate.latitude,
            "longitude": location.coordinate.longitude,
            "altitude": location.altitude,
            "accuracy": location.horizontalAccuracy,
            "heading": location.course,
            "speed": location.speed,
            "timestamp": Int64(location.timestamp.timeIntervalSince1970 * 1000),
            "isMocked": location.sourceInformation?.isSimulatedBySoftware ?? false,
        ]
    }
}

/// When-in-use or Always location access, on its own main-thread manager.
///
/// A request waits for the user's answer, and finishes at once when iOS
/// can show no prompt; it never guesses from timing, since a prompt does
/// not always make the app inactive while it is up.
///   - From not determined, both kinds prompt, and every answer changes
///     the status, which the delegate reports.
///   - From When In Use, iOS asks to upgrade to Always only on the app's
///     first requestAlwaysAuthorization call, which the user defaults
///     remember; once asked, Always reads permanently denied. "Change to
///     Always Allow" changes the status. "Keep Only While Using" leaves it
///     and iOS reports nothing, but the app goes inactive around the
///     prompt, so that answer is read when the app is active again.
///     After Allow Once, iOS shows no upgrade prompt and says nothing; the
///     request then finishes in the same way, or when that access ends.
///   - No prompt can change any other status (PermissionHandler does not
///     request those).
final class LocationPermission: NSObject, DriftPermission, CLLocationManagerDelegate {
    /// Set once the app has called requestAlwaysAuthorization: iOS ignores
    /// later calls, so it shows the Always upgrade prompt at most once.
    private static let alwaysRequestedKey = "drift.location.alwaysRequested"

    private let always: Bool
    private lazy var manager: CLLocationManager = {
        let manager = CLLocationManager()
        manager.delegate = self
        return manager
    }()
    /// The open request. PermissionHandler sends one per kind at a time and
    /// joins later ones to it.
    private var pending: (statusAtRequest: CLAuthorizationStatus, completion: (DriftPermissionStatus) -> Void)?
    private var observers: [NSObjectProtocol] = []

    init(always: Bool) {
        self.always = always
    }

    func status() -> DriftPermissionStatus {
        map(manager.authorizationStatus)
    }

    func request(options: [String: Any], completion: @escaping (DriftPermissionStatus) -> Void) {
        precondition(pending == nil, "drift: location permission requested while its request is open")
        let current = manager.authorizationStatus
        switch current {
        case .notDetermined:
            pending = (current, completion)
            if always {
                requestAlways()
            } else {
                // Back at not determined (Allow Once expired, or a reset
                // in Settings), iOS has forgotten any earlier Always
                // request.
                UserDefaults.standard.removeObject(forKey: Self.alwaysRequestedKey)
                manager.requestWhenInUseAuthorization()
            }
        case .authorizedWhenInUse where always && !Self.alwaysRequested:
            pending = (current, completion)
            finishWhenActiveAgain()
            requestAlways()
        default:
            completion(map(current))
        }
    }

    private static var alwaysRequested: Bool {
        UserDefaults.standard.bool(forKey: alwaysRequestedKey)
    }

    private func requestAlways() {
        UserDefaults.standard.set(true, forKey: Self.alwaysRequestedKey)
        manager.requestAlwaysAuthorization()
    }

    /// Finishes the open request when the app becomes active after going
    /// inactive. The upgrade prompt makes it inactive while it is up, or on
    /// recent iOS only briefly as it closes.
    private func finishWhenActiveAgain() {
        let center = NotificationCenter.default
        var wentInactive = false
        observers = [
            center.addObserver(forName: UIApplication.willResignActiveNotification, object: nil, queue: .main) { _ in
                wentInactive = true
            },
            center.addObserver(forName: UIApplication.didBecomeActiveNotification, object: nil, queue: .main) { [weak self] _ in
                if wentInactive { self?.finish() }
            },
        ]
    }

    func locationManagerDidChangeAuthorization(_ manager: CLLocationManager) {
        // The delegate also reports the status the manager starts with;
        // only a change answers a request.
        if let pending, manager.authorizationStatus != pending.statusAtRequest {
            finish()
        }
    }

    private func finish() {
        guard let completion = pending?.completion else { return }
        pending = nil
        observers.forEach { NotificationCenter.default.removeObserver($0) }
        observers.removeAll()
        completion(status())
    }

    private func map(_ status: CLAuthorizationStatus) -> DriftPermissionStatus {
        switch status {
        case .authorizedAlways: return .granted
        // A request joining an open upgrade prompt still sees denied.
        case .authorizedWhenInUse where always:
            return pending == nil && Self.alwaysRequested ? .permanentlyDenied : .denied
        case .authorizedWhenInUse: return .granted
        case .denied: return .permanentlyDenied
        case .restricted: return .restricted
        case .notDetermined: return .notDetermined
        @unknown default: return .unknown
        }
    }
}
