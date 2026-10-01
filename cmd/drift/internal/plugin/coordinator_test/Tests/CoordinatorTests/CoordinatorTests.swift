/// CoordinatorTests.swift
/// XCTests for DriftPluginCoordinator's race-condition-sensitive scaffold.
/// See coordinator_test/Package.swift for run instructions.

import XCTest
import UIKit
@testable import DriftPluginCoordinator

final class MergeBackgroundFetchTests: XCTestCase {
    func testEmptyReturnsFailed() {
        // Empty results with handlers present means every handler missed the
        // timeout.
        XCTAssertEqual(DriftPluginCoordinator.mergeBackgroundFetch([]), .failed)
    }

    func testSingleNewData() {
        XCTAssertEqual(DriftPluginCoordinator.mergeBackgroundFetch([.newData]), .newData)
    }

    func testSingleNoData() {
        XCTAssertEqual(DriftPluginCoordinator.mergeBackgroundFetch([.noData]), .noData)
    }

    func testSingleFailed() {
        XCTAssertEqual(DriftPluginCoordinator.mergeBackgroundFetch([.failed]), .failed)
    }

    func testNewDataWinsOverNoData() {
        XCTAssertEqual(DriftPluginCoordinator.mergeBackgroundFetch([.noData, .newData]), .newData)
        XCTAssertEqual(DriftPluginCoordinator.mergeBackgroundFetch([.newData, .noData]), .newData)
    }

    func testNewDataWinsOverFailed() {
        XCTAssertEqual(DriftPluginCoordinator.mergeBackgroundFetch([.failed, .newData]), .newData)
    }

    func testNoDataWinsOverFailed() {
        XCTAssertEqual(DriftPluginCoordinator.mergeBackgroundFetch([.failed, .noData]), .noData)
    }

    func testAllFailedReturnsFailed() {
        XCTAssertEqual(DriftPluginCoordinator.mergeBackgroundFetch([.failed, .failed]), .failed)
    }
}

final class DispatchBackgroundFetchTests: XCTestCase {
    private let userInfo: [AnyHashable: Any] = ["k": "v"]

    /// Empty handlers list (no plugins) fires completion immediately with
    /// .newData, Drift's result for its own dispatch.
    func testEmptyHandlersShortCircuits() {
        let exp = expectation(description: "completion")
        DriftPluginCoordinator.dispatchBackgroundFetch(
            userInfo: userInfo,
            handlers: [],
            completionQueue: .main
        ) { result in
            XCTAssertEqual(result, .newData)
            exp.fulfill()
        }
        wait(for: [exp], timeout: 0.5)
    }

    /// Single handler completing synchronously: result merged through.
    func testSingleHandlerSyncComplete() {
        let exp = expectation(description: "completion")
        DriftPluginCoordinator.dispatchBackgroundFetch(
            userInfo: userInfo,
            handlers: [
                { _, completion in completion(.newData); return true }
            ],
            completionQueue: .main
        ) { result in
            XCTAssertEqual(result, .newData)
            exp.fulfill()
        }
        wait(for: [exp], timeout: 0.5)
    }

    /// Two handlers returning different results: priority merge.
    func testTwoHandlersPriorityMerge() {
        let exp = expectation(description: "completion")
        DriftPluginCoordinator.dispatchBackgroundFetch(
            userInfo: userInfo,
            handlers: [
                { _, completion in completion(.noData); return true },
                { _, completion in completion(.newData); return true }
            ],
            completionQueue: .main
        ) { result in
            XCTAssertEqual(result, .newData, "newData must win priority merge")
            exp.fulfill()
        }
        wait(for: [exp], timeout: 0.5)
    }

    /// A handler that never completes: timeout fires, real completion runs
    /// with the partial results from handlers that did complete.
    func testNonCompletingHandlerTriggersTimeout() {
        let exp = expectation(description: "completion")
        DriftPluginCoordinator.dispatchBackgroundFetch(
            userInfo: userInfo,
            handlers: [
                { _, completion in completion(.newData); return true },
                { _, _ in true /* accepts, never fires */ }
            ],
            timeout: 0.1,
            completionQueue: .main
        ) { result in
            XCTAssertEqual(result, .newData, "completing handler's result survives the timeout")
            exp.fulfill()
        }
        wait(for: [exp], timeout: 0.5)
    }

    /// Every handler misses the timeout: completion still fires, with .failed.
    func testAllHandlersTimeOutReportsFailed() {
        let exp = expectation(description: "completion")
        DriftPluginCoordinator.dispatchBackgroundFetch(
            userInfo: userInfo,
            handlers: [
                { _, _ in true /* accepts, never fires */ }
            ],
            timeout: 0.1,
            completionQueue: .main
        ) { result in
            XCTAssertEqual(result, .failed)
            exp.fulfill()
        }
        wait(for: [exp], timeout: 0.5)
    }

    /// A handler that invokes its per-handler completion twice: only the
    /// first call counts. Second invocation is a no-op.
    func testPerHandlerDoubleCallIsNoop() {
        let exp = expectation(description: "completion")
        DriftPluginCoordinator.dispatchBackgroundFetch(
            userInfo: userInfo,
            handlers: [
                { _, completion in
                    completion(.newData)
                    completion(.failed) // would corrupt the merge if not guarded
                    return true
                }
            ],
            completionQueue: .main
        ) { result in
            XCTAssertEqual(result, .newData, "second completion call must not append .failed")
            exp.fulfill()
        }
        wait(for: [exp], timeout: 0.5)
    }

    /// Real completion fires exactly once, even when the timeout races
    /// group.notify. Wait beyond the timeout and assert the completion
    /// counter is exactly 1.
    func testCompletionFiresExactlyOnce() {
        let exp = expectation(description: "completion fired once")
        exp.expectedFulfillmentCount = 1
        exp.assertForOverFulfill = true

        var fireCount = 0
        let lock = NSLock()
        DriftPluginCoordinator.dispatchBackgroundFetch(
            userInfo: userInfo,
            handlers: [
                { _, completion in completion(.newData); return true }
            ],
            timeout: 0.05,
            completionQueue: .main
        ) { _ in
            lock.lock()
            fireCount += 1
            lock.unlock()
            exp.fulfill()
        }
        wait(for: [exp], timeout: 1.0)
        // Give the timeout's asyncAfter ample chance to race the
        // group.notify path.
        let settled = expectation(description: "post-timeout settle")
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) { settled.fulfill() }
        wait(for: [settled], timeout: 1.0)

        XCTAssertEqual(fireCount, 1, "real completion must fire exactly once across timeout + group.notify")
    }

    /// Every handler declines: same as no handlers, immediate .newData
    /// rather than waiting for the timeout.
    func testAllHandlersDeclineShortCircuits() {
        let exp = expectation(description: "completion")
        DriftPluginCoordinator.dispatchBackgroundFetch(
            userInfo: userInfo,
            handlers: [
                { _, _ in false },
                { _, _ in false }
            ],
            timeout: 10,
            completionQueue: .main
        ) { result in
            XCTAssertEqual(result, .newData)
            exp.fulfill()
        }
        wait(for: [exp], timeout: 0.5)
    }

    /// Declining handlers contribute no result: only the accepting one's
    /// .noData is merged, and completion does not wait for the timeout.
    func testDecliningHandlerDoesNotContribute() {
        let exp = expectation(description: "completion")
        DriftPluginCoordinator.dispatchBackgroundFetch(
            userInfo: userInfo,
            handlers: [
                { _, _ in false },
                { _, completion in completion(.noData); return true }
            ],
            timeout: 10,
            completionQueue: .main
        ) { result in
            XCTAssertEqual(result, .noData)
            exp.fulfill()
        }
        wait(for: [exp], timeout: 0.5)
    }
}
