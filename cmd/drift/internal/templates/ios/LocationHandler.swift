/// LocationHandler.swift
/// Handles location services for the Drift platform channel.
///
/// CLLocationManager delegate callbacks are delivered on the thread where the manager
/// was created. To avoid deadlocks when using synchronous APIs, we run the location
/// manager on a dedicated background thread with its own run loop.

import CoreLocation

final class LocationHandler: NSObject, CLLocationManagerDelegate {
    static let shared = LocationHandler()

    private var locationManager: CLLocationManager!
    private var locationThread: Thread!
    private var locationRunLoop: CFRunLoop!

    private let stateLock = NSLock()
    private var isUpdating = false
    private var pendingLocationCallback: ((CLLocation?, Error?) -> Void)?

    private override init() {
        super.init()
        startLocationThread()
    }

    // MARK: - Background Thread Setup

    /// Starts a dedicated background thread for CLLocationManager operations.
    /// This thread runs its own run loop to receive delegate callbacks.
    private func startLocationThread() {
        let semaphore = DispatchSemaphore(value: 0)

        locationThread = Thread { [weak self] in
            guard let self = self else { return }

            // Create location manager on this thread - callbacks will be delivered here
            self.locationManager = CLLocationManager()
            self.locationManager.delegate = self
            self.locationRunLoop = CFRunLoopGetCurrent()

            // Signal that setup is complete
            semaphore.signal()

            // Run the run loop indefinitely to process location callbacks.
            // Adding a port keeps the run loop alive when there are no other sources.
            let port = NSMachPort()
            RunLoop.current.add(port, forMode: .default)

            while true {
                RunLoop.current.run(mode: .default, before: .distantFuture)
            }
        }
        locationThread.name = "com.drift.location"
        locationThread.qualityOfService = .userInitiated
        locationThread.start()

        // Wait for the location manager to be initialized
        if semaphore.wait(timeout: .now() + 5) == .timedOut {
            // Fallback to avoid deadlock if the thread fails to start.
            locationThread = Thread.current
            locationRunLoop = CFRunLoopGetCurrent()
            locationManager = CLLocationManager()
            locationManager.delegate = self
        }
    }

    /// Executes a block synchronously on the location thread.
    private func performOnLocationThread(_ block: @escaping () -> Void) {
        if Thread.current == locationThread {
            block()
        } else {
            let semaphore = DispatchSemaphore(value: 0)
            CFRunLoopPerformBlock(locationRunLoop, CFRunLoopMode.defaultMode.rawValue) {
                block()
                semaphore.signal()
            }
            CFRunLoopWakeUp(locationRunLoop)
            semaphore.wait()
        }
    }

    // MARK: - Public API

    static func handle(method: String, args: Any?) -> (Any?, Error?) {
        switch method {
        case "getCurrentLocation":
            return shared.getCurrentLocation(args: args)
        case "startUpdates":
            return shared.startUpdates(args: args)
        case "stopUpdates":
            return shared.stopUpdates()
        case "isEnabled":
            return shared.isEnabled()
        case "getLastKnown":
            return shared.getLastKnown()
        default:
            return (nil, NSError(domain: "Location", code: 404, userInfo: [NSLocalizedDescriptionKey: "Unknown method: \(method)"]))
        }
    }

    // MARK: - Location Methods

    private func getCurrentLocation(args: Any?) -> (Any?, Error?) {
        // Validate authorization status
        if let authError = checkAuthorizationStatus() {
            return (nil, authError)
        }

        // Check if location services are enabled
        if !CLLocationManager.locationServicesEnabled() {
            return (nil, NSError(domain: "Location", code: 503, userInfo: [NSLocalizedDescriptionKey: "Location services are disabled"]))
        }

        let dict = args as? [String: Any] ?? [:]
        let highAccuracy = dict["highAccuracy"] as? Bool ?? true

        var result: [String: Any]? = nil
        var locationError: Error? = nil
        let semaphore = DispatchSemaphore(value: 0)

        stateLock.lock()
        pendingLocationCallback = { [weak self] location, err in
            if let location = location {
                result = self?.locationToDict(location)
            } else {
                locationError = err
            }
            semaphore.signal()
        }
        stateLock.unlock()

        // Configure and request location on the location thread
        performOnLocationThread { [self] in
            locationManager.desiredAccuracy = highAccuracy ? kCLLocationAccuracyBest : kCLLocationAccuracyHundredMeters
            locationManager.requestLocation()
        }

        // Wait for callback - this is safe because callbacks come on locationThread, not here
        let timeout = semaphore.wait(timeout: .now() + 30)

        stateLock.lock()
        pendingLocationCallback = nil
        stateLock.unlock()

        if timeout == .timedOut {
            return (nil, NSError(domain: "Location", code: 408, userInfo: [NSLocalizedDescriptionKey: "Location request timed out"]))
        }

        if let error = locationError {
            return (nil, error)
        }

        return (result, nil)
    }

    private func startUpdates(args: Any?) -> (Any?, Error?) {
        // Validate authorization status
        if let authError = checkAuthorizationStatus() {
            return (nil, authError)
        }

        stateLock.lock()
        if isUpdating {
            stateLock.unlock()
            return (nil, nil)
        }
        isUpdating = true
        stateLock.unlock()

        let dict = args as? [String: Any] ?? [:]
        let highAccuracy = dict["highAccuracy"] as? Bool ?? true
        let distanceFilter = dict["distanceFilter"] as? Double ?? kCLDistanceFilterNone

        performOnLocationThread { [self] in
            locationManager.desiredAccuracy = highAccuracy ? kCLLocationAccuracyBest : kCLLocationAccuracyHundredMeters
            locationManager.distanceFilter = distanceFilter
            locationManager.startUpdatingLocation()
        }

        return (nil, nil)
    }

    private func stopUpdates() -> (Any?, Error?) {
        stateLock.lock()
        if !isUpdating {
            stateLock.unlock()
            return (nil, nil)
        }
        isUpdating = false
        stateLock.unlock()

        performOnLocationThread { [self] in
            locationManager.stopUpdatingLocation()
        }

        return (nil, nil)
    }

    private func isEnabled() -> (Any?, Error?) {
        let enabled = CLLocationManager.locationServicesEnabled()
        return (["enabled": enabled], nil)
    }

    private func getLastKnown() -> (Any?, Error?) {
        var result: [String: Any]? = nil

        performOnLocationThread { [self] in
            if let location = locationManager.location {
                result = locationToDict(location)
            }
        }

        return (result, nil)
    }

    // MARK: - Authorization

    private func checkAuthorizationStatus() -> Error? {
        var authStatus: CLAuthorizationStatus = .notDetermined

        performOnLocationThread { [self] in
            authStatus = locationManager.authorizationStatus
        }

        switch authStatus {
        case .notDetermined:
            return NSError(domain: "Location", code: 401, userInfo: [NSLocalizedDescriptionKey: "Location permission not determined; request it first"])
        case .denied:
            return NSError(domain: "Location", code: 403, userInfo: [NSLocalizedDescriptionKey: "Location permission denied"])
        case .restricted:
            return NSError(domain: "Location", code: 403, userInfo: [NSLocalizedDescriptionKey: "Location access restricted"])
        case .authorizedWhenInUse, .authorizedAlways:
            return nil
        @unknown default:
            return nil
        }
    }

    // MARK: - Helpers

    private func locationToDict(_ location: CLLocation) -> [String: Any] {
        return [
            "latitude": location.coordinate.latitude,
            "longitude": location.coordinate.longitude,
            "altitude": location.altitude,
            "accuracy": location.horizontalAccuracy,
            "heading": location.course,
            "speed": location.speed,
            "timestamp": Int64(location.timestamp.timeIntervalSince1970 * 1000),
            "isMocked": false
        ]
    }

    // MARK: - CLLocationManagerDelegate

    func locationManager(_ manager: CLLocationManager, didUpdateLocations locations: [CLLocation]) {
        guard let location = locations.last else { return }

        stateLock.lock()
        let callback = pendingLocationCallback
        let updating = isUpdating
        if callback != nil {
            pendingLocationCallback = nil
        }
        stateLock.unlock()

        if let callback = callback {
            callback(location, nil)
        } else if updating {
            // Send location update event on main thread for UI safety
            DispatchQueue.main.async {
                PlatformChannelManager.shared.sendEvent(
                    channel: "drift/location/updates",
                    data: self.locationToDict(location)
                )
            }
        }
    }

    func locationManager(_ manager: CLLocationManager, didFailWithError error: Error) {
        stateLock.lock()
        let callback = pendingLocationCallback
        if callback != nil {
            pendingLocationCallback = nil
        }
        stateLock.unlock()

        if let callback = callback {
            callback(nil, error)
        }
    }
}
