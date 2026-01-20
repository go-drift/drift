/// LocationHandler.swift
/// Handles location services for the Drift platform channel.

import CoreLocation

final class LocationHandler: NSObject, CLLocationManagerDelegate {
    static let shared = LocationHandler()

    private let locationManager = CLLocationManager()
    private var isUpdating = false
    private var pendingLocationCallback: ((CLLocation?, Error?) -> Void)?

    private override init() {
        super.init()
        locationManager.delegate = self
    }

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

    private func getCurrentLocation(args: Any?) -> (Any?, Error?) {
        let dict = args as? [String: Any] ?? [:]
        let highAccuracy = dict["highAccuracy"] as? Bool ?? true

        locationManager.desiredAccuracy = highAccuracy ? kCLLocationAccuracyBest : kCLLocationAccuracyHundredMeters

        var result: [String: Any]? = nil
        var error: Error? = nil
        let semaphore = DispatchSemaphore(value: 0)

        pendingLocationCallback = { location, err in
            if let location = location {
                result = self.locationToDict(location)
            } else {
                error = err
            }
            semaphore.signal()
        }

        locationManager.requestLocation()

        let timeout = semaphore.wait(timeout: .now() + 30)
        pendingLocationCallback = nil

        if timeout == .timedOut {
            return (nil, NSError(domain: "Location", code: 408, userInfo: [NSLocalizedDescriptionKey: "Location request timed out"]))
        }

        if let error = error {
            return (nil, error)
        }

        return (result, nil)
    }

    private func startUpdates(args: Any?) -> (Any?, Error?) {
        if isUpdating {
            return (nil, nil)
        }

        let dict = args as? [String: Any] ?? [:]
        let highAccuracy = dict["highAccuracy"] as? Bool ?? true
        let distanceFilter = dict["distanceFilter"] as? Double ?? kCLDistanceFilterNone

        locationManager.desiredAccuracy = highAccuracy ? kCLLocationAccuracyBest : kCLLocationAccuracyHundredMeters
        locationManager.distanceFilter = distanceFilter
        locationManager.startUpdatingLocation()
        isUpdating = true

        return (nil, nil)
    }

    private func stopUpdates() -> (Any?, Error?) {
        if !isUpdating {
            return (nil, nil)
        }

        locationManager.stopUpdatingLocation()
        isUpdating = false

        return (nil, nil)
    }

    private func isEnabled() -> (Any?, Error?) {
        let enabled = CLLocationManager.locationServicesEnabled()
        return (["enabled": enabled], nil)
    }

    private func getLastKnown() -> (Any?, Error?) {
        if let location = locationManager.location {
            return (locationToDict(location), nil)
        }
        return (nil, nil)
    }

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

        if let callback = pendingLocationCallback {
            callback(location, nil)
            pendingLocationCallback = nil
        } else if isUpdating {
            PlatformChannelManager.shared.sendEvent(
                channel: "drift/location/updates",
                data: locationToDict(location)
            )
        }
    }

    func locationManager(_ manager: CLLocationManager, didFailWithError error: Error) {
        if let callback = pendingLocationCallback {
            callback(nil, error)
            pendingLocationCallback = nil
        }
    }
}
