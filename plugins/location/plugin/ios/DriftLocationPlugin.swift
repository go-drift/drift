/// DriftLocationPlugin.swift
///
/// The location plugin's native half on iOS, on the main thread: one
/// CLLocationManager answers one-shot requests and another streams updates
/// to Go on `drift/location/updates`, in the background too when the
/// stream asks for it. Registers the `location` permission, and
/// `location_always` when the build added its usage string.
///
/// Errors Go maps to its sentinels (runtime package): `permission_denied`,
/// `location_disabled`, `background_not_configured` and `always_denied`.

import CoreLocation
import DriftPluginAPI
import UIKit

/// Public, with a public initializer: the app's generated registrant
/// creates it from outside this module (DriftPlugin_location).
public final class DriftLocationPlugin: NSObject, DriftPlugin, CLLocationManagerDelegate {
    private static let channel = "drift/location"
    private static let updatesChannel = "drift/location/updates"

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
                do {
                    try requireAuthorized()
                    result.success(oneShot.location.map(Self.payload))
                } catch {
                    result.error(error)
                }
            }
            channel.asyncMethod("isEnabled") { _, result in
                // Apple warns against asking on the main thread.
                DispatchQueue.global(qos: .userInitiated).async {
                    result.success(["enabled": CLLocationManager.locationServicesEnabled()])
                }
            }
            channel.method("startUpdates") { [self] args in
                try requireAuthorized()
                let args = args as? [String: Any] ?? [:]
                let background = args["background"] as? Bool ?? false
                if background {
                    try requireBackground()
                }
                let highAccuracy = args["highAccuracy"] as? Bool ?? true
                updates.desiredAccuracy = highAccuracy ? kCLLocationAccuracyBest : kCLLocationAccuracyHundredMeters
                updates.distanceFilter = (args["distanceFilter"] as? Double).flatMap { $0 > 0 ? $0 : nil }
                    ?? kCLDistanceFilterNone
                if Self.backgroundConfigured {
                    updates.allowsBackgroundLocationUpdates = background
                }
                // The status bar shows background use, as Android's
                // notification does. A paused stream does not resume by
                // itself, and an app in the background gets no chance to
                // restart it, so a background stream never pauses.
                updates.showsBackgroundLocationIndicator = background
                updates.pausesLocationUpdatesAutomatically = !background
                updates.startUpdatingLocation()
                return nil
            }
            channel.method("stopUpdates") { [self] _ in
                updates.stopUpdatingLocation()
                return nil
            }
        }
    }

    /// Runs body on the main thread once location services are known to be
    /// on; fails result with `location_disabled` otherwise, or with body's
    /// error.
    private func whenEnabled(_ result: DriftResult, _ body: @escaping () throws -> Void) {
        DispatchQueue.global(qos: .userInitiated).async {
            let enabled = CLLocationManager.locationServicesEnabled()
            DispatchQueue.main.async {
                guard enabled else {
                    result.error(Self.error("location_disabled", "location services are turned off"))
                    return
                }
                do {
                    try body()
                } catch {
                    result.error(error)
                }
            }
        }
    }

    private func requireAuthorized() throws {
        switch oneShot.authorizationStatus {
        case .authorizedWhenInUse, .authorizedAlways:
            return
        default:
            throw Self.error("permission_denied", "location permission not granted; request it first")
        }
    }

    private func requireBackground() throws {
        guard Self.backgroundConfigured else {
            throw Self.error("background_not_configured", "background location updates need the location plugin's always: config")
        }
        guard oneShot.authorizationStatus == .authorizedAlways else {
            throw Self.error("always_denied", "background location updates need the Always location permission; request it first")
        }
    }

    // MARK: - CLLocationManagerDelegate

    public func locationManager(_ manager: CLLocationManager, didUpdateLocations locations: [CLLocation]) {
        guard let location = locations.last else { return }
        if manager === oneShot {
            finishCurrent { $0.success(Self.payload(location)) }
        } else {
            host?.sendEvent(Self.updatesChannel, data: Self.payload(location))
        }
    }

    public func locationManager(_ manager: CLLocationManager, didFailWithError error: Error) {
        let failure: Error = (error as? CLError)?.code == .denied
            ? Self.error("permission_denied", "location permission not granted")
            : error
        if manager === oneShot {
            finishCurrent { $0.error(failure) }
        } else {
            host?.sendEventError(Self.updatesChannel, code: (failure as NSError).domain, message: failure.localizedDescription)
        }
    }

    private func finishCurrent(_ reply: (DriftResult) -> Void) {
        let waiting = pendingCurrent
        pendingCurrent.removeAll()
        waiting.forEach(reply)
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

    private static func error(_ code: String, _ message: String) -> NSError {
        NSError(domain: code, code: 0, userInfo: [NSLocalizedDescriptionKey: message])
    }
}

/// When-in-use or Always location access, on its own main-thread manager.
final class LocationPermission: NSObject, DriftPermission, CLLocationManagerDelegate {
    /// How long to wait for the app to go inactive behind a prompt before
    /// concluding iOS showed none.
    private static let promptGrace: TimeInterval = 1

    private let always: Bool
    private lazy var manager: CLLocationManager = {
        let manager = CLLocationManager()
        manager.delegate = self
        return manager
    }()
    private var pending: [(DriftPermissionStatus) -> Void] = []
    private var statusAtRequest: CLAuthorizationStatus = .notDetermined
    private var observers: [NSObjectProtocol] = []

    init(always: Bool) {
        self.always = always
    }

    func status() -> DriftPermissionStatus {
        map(manager.authorizationStatus)
    }

    func request(options: [String: Any], completion: @escaping (DriftPermissionStatus) -> Void) {
        pending.append(completion)
        guard pending.count == 1 else { return } // A prompt is already up.
        statusAtRequest = manager.authorizationStatus
        if always {
            manager.requestAlwaysAuthorization()
        } else {
            manager.requestWhenInUseAuthorization()
        }
        // iOS sends no callback when it shows no prompt (it already asked),
        // or when the user keeps the current level (upgrading to Always).
        // A prompt makes the app inactive: finish when it is active again,
        // or shortly after the request if it never went inactive.
        let center = NotificationCenter.default
        var prompted = false
        observers.append(center.addObserver(forName: UIApplication.willResignActiveNotification, object: nil, queue: .main) { _ in
            prompted = true
        })
        observers.append(center.addObserver(forName: UIApplication.didBecomeActiveNotification, object: nil, queue: .main) { [weak self] _ in
            self?.finish()
        })
        DispatchQueue.main.asyncAfter(deadline: .now() + Self.promptGrace) { [weak self] in
            if !prompted { self?.finish() }
        }
    }

    func locationManagerDidChangeAuthorization(_ manager: CLLocationManager) {
        // The delegate also reports the status it starts with; only a change
        // answers a request.
        if !pending.isEmpty, manager.authorizationStatus != statusAtRequest {
            finish()
        }
    }

    private func finish() {
        guard !pending.isEmpty else { return }
        observers.forEach { NotificationCenter.default.removeObserver($0) }
        observers.removeAll()
        let status = status()
        let completions = pending
        pending.removeAll()
        completions.forEach { $0(status) }
    }

    private func map(_ status: CLAuthorizationStatus) -> DriftPermissionStatus {
        switch status {
        case .authorizedAlways: return .granted
        // When-in-use can still be upgraded to Always.
        case .authorizedWhenInUse: return always ? .denied : .granted
        case .denied: return .permanentlyDenied
        case .restricted: return .restricted
        case .notDetermined: return .notDetermined
        @unknown default: return .unknown
        }
    }
}
