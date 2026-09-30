/// DriftPluginCoordinator.swift
/// Helpers used by the generated DriftPluginRegistrant.swift to merge
/// UIApplicationDelegate callback results across multiple plugin registrants.
///
/// This file is hand-written and lives in the iOS template tree. The xtool
/// scaffold picks it up via WriteXtool's CopyTree("ios", ...) swift-glob, and
/// ejected projects receive it from EnsureRunnerSupport, so every build path
/// sees the same coordinator helpers.
///
/// The dispatch + merge scaffolding lives here (not in the codegen) so all
/// the lock + timer + once-only race-condition surface is in one place,
/// testable via the harness at cmd/drift/internal/plugin/coordinator_test/.
/// The generated DriftPluginRegistrant.didReceiveRemoteNotification just
/// hands an array of plugin closures to dispatchBackgroundFetch.

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

    typealias BackgroundFetchHandler = (
        _ userInfo: [AnyHashable: Any],
        _ completion: @escaping (UIBackgroundFetchResult) -> Void
    ) -> Void

    /// Fan userInfo out to every plugin handler, accumulate their results
    /// under a lock, and fire the real completionHandler exactly once with
    /// the priority-merged result. A safety timeout races group.notify, so a
    /// handler that never completes does not strand iOS waiting on
    /// background fetch.
    ///
    /// Invariants:
    ///   - completionHandler runs exactly once, outside the lock.
    ///   - A handler invoking its completion twice is a no-op on the second
    ///     call.
    ///   - No handlers (no plugin hooks this callback) completes immediately
    ///     with `.newData`, the result Drift has always reported for its own
    ///     notification dispatch.
    static func dispatchBackgroundFetch(
        userInfo: [AnyHashable: Any],
        handlers: [BackgroundFetchHandler],
        timeout: TimeInterval = backgroundFetchTimeout,
        completionQueue: DispatchQueue = .main,
        completionHandler: @escaping (UIBackgroundFetchResult) -> Void
    ) {
        if handlers.isEmpty {
            completionHandler(.newData)
            return
        }
        let group = DispatchGroup()
        let lock = NSLock()
        var results: [UIBackgroundFetchResult] = []
        var fired = false

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
            handler(userInfo) { result in
                lock.lock()
                guard once.fire() else {
                    lock.unlock()
                    return
                }
                results.append(result)
                lock.unlock()
                group.leave()
            }
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
