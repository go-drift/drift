/// DriftPluginCoordinator.swift
/// Merges UIApplicationDelegate callback results across plugins, for
/// DriftPlugins.didReceiveRemoteNotification.
///
/// This file is hand-written and lives in the iOS template tree. The xtool
/// scaffold picks it up via WriteXtool's CopyTree("ios", ...) swift-glob, and
/// ejected projects receive it from EnsureRunnerSupport, so every build path
/// sees the same coordinator helpers.
///
/// The lock + timer + once-only race-condition surface lives here in one
/// place, testable via the harness at
/// cmd/drift/internal/plugin/coordinator_test/.

import UIKit

enum DriftPluginCoordinator {
    /// Maximum time dispatchBackgroundFetch waits for all handlers to invoke
    /// their per-handler completion before firing the real completionHandler
    /// with whatever results have accumulated. Sized under iOS's 30-second
    /// background-fetch deadline.
    static let backgroundFetchTimeout: TimeInterval = 25.0

    /// Picks the most-work-done UIBackgroundFetchResult from the results of
    /// the handlers that completed. `.newData` beats `.noData` beats
    /// `.failed`. Raw enum ordering is
    /// `.newData == 0 < .noData == 1 < .failed == 2`, which is counterintuitive,
    /// so an explicit priority merge is clearer than `max`.
    ///
    /// An empty list means handlers existed but none completed before the
    /// timeout, which is reported as `.failed`.
    static func mergeBackgroundFetch(_ results: [UIBackgroundFetchResult]) -> UIBackgroundFetchResult {
        if results.contains(.newData) { return .newData }
        if results.contains(.noData)  { return .noData }
        return .failed
    }

    /// One plugin's share of a remote-notification fetch. Returns true if it
    /// will call `completion` (exactly once), false to decline without
    /// contributing a result.
    typealias BackgroundFetchHandler = (
        _ userInfo: [AnyHashable: Any],
        _ completion: @escaping (UIBackgroundFetchResult) -> Void
    ) -> Bool

    /// Fan userInfo out to every plugin handler, accumulate the results of
    /// those that accept under a lock, and fire the real completionHandler
    /// exactly once with the priority-merged result. A safety timeout races
    /// group.notify, so a handler that never completes does not strand iOS
    /// waiting on background fetch.
    ///
    /// Invariants:
    ///   - completionHandler runs exactly once, outside the lock.
    ///   - A handler invoking its completion twice, or after declining, is a
    ///     no-op on the extra calls.
    ///   - When no handler accepts (including no handlers at all), completion
    ///     fires immediately with `.newData`, the result Drift has always
    ///     reported for its own notification dispatch.
    static func dispatchBackgroundFetch(
        userInfo: [AnyHashable: Any],
        handlers: [BackgroundFetchHandler],
        timeout: TimeInterval = backgroundFetchTimeout,
        completionQueue: DispatchQueue = .main,
        completionHandler: @escaping (UIBackgroundFetchResult) -> Void
    ) {
        let group = DispatchGroup()
        let lock = NSLock()
        var results: [UIBackgroundFetchResult] = []
        var fired = false
        var accepted = 0

        let finalize: () -> Void = {
            lock.lock()
            if fired {
                lock.unlock()
                return
            }
            fired = true
            let merged = mergeBackgroundFetch(results)
            lock.unlock()
            completionHandler(merged)
        }

        for handler in handlers {
            group.enter()
            let once = OnceFlag()
            // result is nil when the handler declined.
            let settle: (UIBackgroundFetchResult?) -> Void = { result in
                lock.lock()
                guard once.fire() else {
                    lock.unlock()
                    return
                }
                if let result = result {
                    results.append(result)
                }
                lock.unlock()
                group.leave()
            }
            if handler(userInfo, { settle($0) }) {
                accepted += 1
            } else {
                settle(nil)
            }
        }

        if accepted == 0 {
            completionHandler(.newData)
            return
        }
        group.notify(queue: completionQueue) { finalize() }
        completionQueue.asyncAfter(deadline: .now() + timeout) { finalize() }
    }

    /// One-shot flag guarding a handler's completion against
    /// double-invocation. Reference-typed so the handler's closure and every
    /// later call share one instance. Nested and private so it cannot clash
    /// with a type of the same name in plugin sources, which compile into
    /// the same module.
    private final class OnceFlag {
        private var fired = false

        /// Returns true on the first call, false thereafter. Caller must hold
        /// an external lock; the flag does not synchronise itself.
        func fire() -> Bool {
            if fired { return false }
            fired = true
            return true
        }
    }
}
