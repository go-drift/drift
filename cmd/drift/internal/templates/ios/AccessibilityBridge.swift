/// AccessibilityBridge.swift
/// Provides accessibility support for Drift using iOS UIAccessibility.

import UIKit

/// Represents a semantics node received from the Go side.
struct SemanticsNode {
    let id: Int64
    let rect: CGRect
    let label: String?
    let value: String?
    let hint: String?
    let role: String
    let flags: UInt64
    let actions: UInt64
    let childIds: [Int64]
    let currentValue: Double?
    let minValue: Double?
    let maxValue: Double?
    let scrollPosition: Double?
    let scrollExtentMin: Double?
    let scrollExtentMax: Double?
    let headingLevel: Int
    let customActions: [CustomSemanticsAction]
}

struct CustomSemanticsAction {
    let id: Int64
    let label: String
}

/// AccessibilityBridge manages the accessibility elements for Drift's semantics tree.
final class AccessibilityBridge {

    // MARK: - Semantics Flags (must match Go side)
    static let flagHasCheckedState: UInt64 = 1 << 0
    static let flagIsChecked: UInt64 = 1 << 1
    static let flagHasSelectedState: UInt64 = 1 << 2
    static let flagIsSelected: UInt64 = 1 << 3
    static let flagHasEnabledState: UInt64 = 1 << 4
    static let flagIsEnabled: UInt64 = 1 << 5
    static let flagIsFocusable: UInt64 = 1 << 6
    static let flagIsFocused: UInt64 = 1 << 7
    static let flagIsButton: UInt64 = 1 << 8
    static let flagIsTextField: UInt64 = 1 << 9
    static let flagIsReadOnly: UInt64 = 1 << 10
    static let flagIsObscured: UInt64 = 1 << 11
    static let flagIsMultiline: UInt64 = 1 << 12
    static let flagIsSlider: UInt64 = 1 << 13
    static let flagIsLiveRegion: UInt64 = 1 << 14
    static let flagHasToggledState: UInt64 = 1 << 15
    static let flagIsToggled: UInt64 = 1 << 16
    static let flagHasImplicitScrolling: UInt64 = 1 << 17
    static let flagIsHidden: UInt64 = 1 << 18
    static let flagIsHeader: UInt64 = 1 << 19
    static let flagIsImage: UInt64 = 1 << 20
    static let flagNamesRoute: UInt64 = 1 << 21
    static let flagScopesRoute: UInt64 = 1 << 22
    static let flagIsInMutuallyExclusiveGroup: UInt64 = 1 << 23
    static let flagHasExpandedState: UInt64 = 1 << 24
    static let flagIsExpanded: UInt64 = 1 << 25

    // MARK: - Semantics Actions (must match Go side)
    static let actionTap: UInt64 = 1 << 0
    static let actionLongPress: UInt64 = 1 << 1
    static let actionScrollLeft: UInt64 = 1 << 2
    static let actionScrollRight: UInt64 = 1 << 3
    static let actionScrollUp: UInt64 = 1 << 4
    static let actionScrollDown: UInt64 = 1 << 5
    static let actionIncrease: UInt64 = 1 << 6
    static let actionDecrease: UInt64 = 1 << 7
    static let actionFocus: UInt64 = 1 << 18
    static let actionDismiss: UInt64 = 1 << 21

    // MARK: - Properties

    private weak var hostView: UIView?
    private var nodes: [Int64: SemanticsNode] = [:]
    private var elements: [Int64: DriftAccessibilityElement] = [:]
    private var parentIdByChildId: [Int64: Int64] = [:]  // O(1) parent lookup
    private var rootId: Int64 = -1
    private var cachedElements: [Any]?  // Cache to avoid flicker when root is temporarily nil
    private let lock = NSLock()  // Thread safety for node access

    // MARK: - Initialization

    init(hostView: UIView) {
        self.hostView = hostView
    }

    // MARK: - Public Methods

    /// Updates the semantics tree with new nodes and removals.
    func updateSemantics(updates: [[String: Any]], removals: [Int64]) {
        lock.lock()
        defer { lock.unlock() }

        // Process removals - O(children) per removal instead of O(n*m)
        for id in removals {
            // Clear parent mappings for this node's children before removing
            if let oldNode = nodes[id] {
                for childId in oldNode.childIds {
                    parentIdByChildId.removeValue(forKey: childId)
                }
            }
            nodes.removeValue(forKey: id)
            elements.removeValue(forKey: id)
        }

        // Process updates
        for update in updates {
            let node = parseNode(update)

            // Handle reparenting: clear parent mappings for children that were removed
            if let oldNode = nodes[node.id] {
                let oldChildIds = Set(oldNode.childIds)
                let newChildIds = Set(node.childIds)
                // Remove parent mappings for children that are no longer in this node
                for removedChildId in oldChildIds.subtracting(newChildIds) {
                    parentIdByChildId.removeValue(forKey: removedChildId)
                }
            }

            nodes[node.id] = node

            // Build/update parent lookup map for current children
            for childId in node.childIds {
                parentIdByChildId[childId] = node.id
            }

            // Update or create element
            if let element = elements[node.id] {
                element.update(with: node)
            } else {
                let element = DriftAccessibilityElement(
                    accessibilityContainer: hostView,
                    bridge: self,
                    node: node
                )
                elements[node.id] = element
            }
        }

        // Use the synthetic root (node 0) as the accessibility root so all its
        // children (e.g., barrier + sheet in overlays) are reachable by VoiceOver.
        if nodes[0] != nil {
            rootId = 0
        } else {
            rootId = nodes.keys.min() ?? -1
        }

        // Clear cached elements if tree is empty to avoid VoiceOver focusing stale elements
        if rootId == -1 || nodes.isEmpty {
            cachedElements = nil
        }

        // Post notification for accessibility changes
        DispatchQueue.main.async {
            UIAccessibility.post(notification: .layoutChanged, argument: nil)
        }
    }

    /// Announces a message to VoiceOver.
    func announce(message: String, politeness: String) {
        DispatchQueue.main.async {
            let notification: UIAccessibility.Notification = politeness == "assertive"
                ? .announcement
                : .announcement
            UIAccessibility.post(notification: notification, argument: message)
        }
    }

    /// Sets accessibility focus to a specific node.
    func setAccessibilityFocus(nodeId: Int64) {
        guard let element = elements[nodeId] else { return }
        DispatchQueue.main.async {
            UIAccessibility.post(notification: .screenChanged, argument: element)
        }
    }

    /// Clears the current accessibility focus.
    func clearAccessibilityFocus() {
        DispatchQueue.main.async {
            UIAccessibility.post(notification: .screenChanged, argument: nil)
        }
    }

    /// Returns the accessibility elements for the host view.
    /// Uses cached elements to avoid flicker when root is temporarily nil.
    func accessibilityElements() -> [Any]? {
        lock.lock()
        defer { lock.unlock() }

        guard rootId != -1, nodes[rootId] != nil else {
            // Return cached elements to avoid flicker during tree rebuilds
            return cachedElements
        }
        // Recompute and cache
        let elements = collectAccessibilityElements(nodeId: rootId)
        cachedElements = elements
        return elements
    }

    // MARK: - Internal Methods

    func performAction(_ action: UInt64, on nodeId: Int64, args: [String: Any]? = nil) {
        var payload: [String: Any] = [
            "nodeId": nodeId,
            "action": action
        ]
        if let args = args {
            payload["args"] = args
        }
        PlatformChannelManager.shared.sendEvent(
            channel: "drift/accessibility/actions",
            data: payload
        )
    }

    func node(for id: Int64) -> SemanticsNode? {
        lock.lock()
        defer { lock.unlock() }
        return nodes[id]
    }

    /// Finds the nearest scrollable ancestor for a given node using O(1) parent lookup.
    func findScrollableAncestor(for nodeId: Int64) -> SemanticsNode? {
        lock.lock()
        defer { lock.unlock() }

        var currentId = nodeId
        while let parentId = parentIdByChildId[currentId] {
            guard let parent = nodes[parentId] else {
                return nil
            }

            // Check if parent is scrollable
            let isScrollable = (parent.actions & AccessibilityBridge.actionScrollUp != 0) ||
                              (parent.actions & AccessibilityBridge.actionScrollDown != 0) ||
                              (parent.actions & AccessibilityBridge.actionScrollLeft != 0) ||
                              (parent.actions & AccessibilityBridge.actionScrollRight != 0)

            if isScrollable {
                return parent
            }

            currentId = parentId
        }
        return nil
    }

    // MARK: - Private Methods

    private func parseNode(_ data: [String: Any]) -> SemanticsNode {
        let childIds: [Int64] = (data["childIds"] as? [Any])?.compactMap { value in
            if let num = value as? NSNumber {
                return num.int64Value
            }
            return nil
        } ?? []

        let customActions: [CustomSemanticsAction] = (data["customActions"] as? [[String: Any]])?.compactMap { action in
            guard let id = (action["id"] as? NSNumber)?.int64Value,
                  let label = action["label"] as? String else {
                return nil
            }
            return CustomSemanticsAction(id: id, label: label)
        } ?? []

        let left = (data["left"] as? NSNumber)?.doubleValue ?? 0
        let top = (data["top"] as? NSNumber)?.doubleValue ?? 0
        let right = (data["right"] as? NSNumber)?.doubleValue ?? 0
        let bottom = (data["bottom"] as? NSNumber)?.doubleValue ?? 0

        return SemanticsNode(
            id: (data["id"] as? NSNumber)?.int64Value ?? 0,
            rect: CGRect(x: left, y: top, width: right - left, height: bottom - top),
            label: data["label"] as? String,
            value: data["value"] as? String,
            hint: data["hint"] as? String,
            role: data["role"] as? String ?? "none",
            flags: (data["flags"] as? NSNumber)?.uint64Value ?? 0,
            actions: (data["actions"] as? NSNumber)?.uint64Value ?? 0,
            childIds: childIds,
            currentValue: (data["currentValue"] as? NSNumber)?.doubleValue,
            minValue: (data["minValue"] as? NSNumber)?.doubleValue,
            maxValue: (data["maxValue"] as? NSNumber)?.doubleValue,
            scrollPosition: (data["scrollPosition"] as? NSNumber)?.doubleValue,
            scrollExtentMin: (data["scrollExtentMin"] as? NSNumber)?.doubleValue,
            scrollExtentMax: (data["scrollExtentMax"] as? NSNumber)?.doubleValue,
            headingLevel: (data["headingLevel"] as? NSNumber)?.intValue ?? 0,
            customActions: customActions
        )
    }

    /// Recursively collects accessibility elements from the node tree.
    /// IMPORTANT: Must be called while holding `lock` - not thread-safe on its own.
    private func collectAccessibilityElements(nodeId: Int64) -> [Any] {
        var result: [Any] = []

        guard let node = nodes[nodeId], let element = elements[nodeId] else {
            return result
        }

        // Check if this node is scrollable
        let isScrollable = (node.actions & AccessibilityBridge.actionScrollUp != 0) ||
                          (node.actions & AccessibilityBridge.actionScrollDown != 0) ||
                          (node.actions & AccessibilityBridge.actionScrollLeft != 0) ||
                          (node.actions & AccessibilityBridge.actionScrollRight != 0)

        // Collect children
        var childElements: [Any] = []
        for childId in node.childIds {
            childElements.append(contentsOf: collectAccessibilityElements(nodeId: childId))
        }

        // Determine if this node should be an accessibility element
        let hasContent = node.label != nil || node.value != nil || node.hint != nil
        let hasNonScrollActions = (node.actions & ~(AccessibilityBridge.actionScrollUp |
                                                    AccessibilityBridge.actionScrollDown |
                                                    AccessibilityBridge.actionScrollLeft |
                                                    AccessibilityBridge.actionScrollRight)) != 0
        let isFocusable = node.flags & AccessibilityBridge.flagIsFocusable != 0

        if !childElements.isEmpty {
            // Has accessible children
            if isScrollable {
                // Scrollable containers: only add children, skip the container itself.
                // Child elements will forward scroll commands to scrollable ancestor.
                result.append(contentsOf: childElements)
            } else if hasContent || hasNonScrollActions || isFocusable {
                // Non-scrollable parent with its own content/actions: add both
                // This handles labeled containers, focusable groups, etc.
                result.append(element)
                result.append(contentsOf: childElements)
            } else {
                // Just a grouping container with no content - only add children
                result.append(contentsOf: childElements)
            }
        } else {
            // No accessible children - add this node if it has content
            if hasContent || hasNonScrollActions || isFocusable || isScrollable {
                result.append(element)
            }
        }

        return result
    }

    private func updateHostAccessibilityElements() {
        // The host view (DriftMetalView) now uses accessibilityElementsProvider
        // to dynamically return elements, so we just need to notify VoiceOver
        // that the layout has changed
    }
}

/// Custom accessibility element for Drift semantics nodes.
final class DriftAccessibilityElement: UIAccessibilityElement {

    private weak var bridge: AccessibilityBridge?
    private var nodeId: Int64
    private var nodeRect: CGRect = .zero

    init(accessibilityContainer: Any?, bridge: AccessibilityBridge, node: SemanticsNode) {
        self.bridge = bridge
        self.nodeId = node.id
        super.init(accessibilityContainer: accessibilityContainer as Any)
        update(with: node)
    }

    // Compute accessibilityFrame dynamically to ensure correct screen coordinates
    override var accessibilityFrame: CGRect {
        get {
            guard let container = accessibilityContainer as? UIView else {
                return nodeRect
            }
            let scale = container.window?.screen.scale ?? UIScreen.main.scale
            // Convert from pixels to points
            let rectInPoints = CGRect(
                x: nodeRect.origin.x / scale,
                y: nodeRect.origin.y / scale,
                width: nodeRect.width / scale,
                height: nodeRect.height / scale
            )
            // Convert from view coordinates to screen coordinates
            return UIAccessibility.convertToScreenCoordinates(rectInPoints, in: container)
        }
        set {
            // Ignore direct sets, we compute this dynamically
        }
    }

    func update(with node: SemanticsNode) {
        nodeId = node.id
        nodeRect = node.rect

        // Set label and value
        accessibilityLabel = node.label
        accessibilityValue = node.value
        accessibilityHint = node.hint

        // Set traits
        accessibilityTraits = mapTraits(node: node)

        // Set custom actions
        if !node.customActions.isEmpty {
            accessibilityCustomActions = node.customActions.map { action in
                UIAccessibilityCustomAction(name: action.label) { [weak self] _ in
                    self?.bridge?.performAction(UInt64(action.id), on: node.id, args: ["actionId": action.id])
                    return true
                }
            }
        }
    }

    private func mapTraits(node: SemanticsNode) -> UIAccessibilityTraits {
        var traits: UIAccessibilityTraits = []

        // Role-based traits
        switch node.role {
        case "button":
            traits.insert(.button)
        case "link":
            traits.insert(.link)
        case "image":
            traits.insert(.image)
        case "header":
            traits.insert(.header)
        case "textField":
            // Text fields don't have a specific trait in iOS
            break
        default:
            break
        }

        // Flag-based traits
        if node.flags & AccessibilityBridge.flagIsButton != 0 {
            traits.insert(.button)
        }
        if node.flags & AccessibilityBridge.flagIsHeader != 0 {
            traits.insert(.header)
        }
        if node.flags & AccessibilityBridge.flagIsImage != 0 {
            traits.insert(.image)
        }
        if node.flags & AccessibilityBridge.flagIsSelected != 0 {
            traits.insert(.selected)
        }
        if node.flags & AccessibilityBridge.flagHasEnabledState != 0 &&
           node.flags & AccessibilityBridge.flagIsEnabled == 0 {
            traits.insert(.notEnabled)
        }
        if node.flags & AccessibilityBridge.flagIsSlider != 0 {
            traits.insert(.adjustable)
        }
        if node.flags & AccessibilityBridge.flagIsLiveRegion != 0 {
            traits.insert(.updatesFrequently)
        }

        return traits
    }

    // MARK: - Accessibility Actions

    override func accessibilityActivate() -> Bool {
        guard let node = bridge?.node(for: nodeId) else { return false }

        if node.actions & AccessibilityBridge.actionTap != 0 {
            bridge?.performAction(AccessibilityBridge.actionTap, on: nodeId)
            return true
        }
        return false
    }

    override func accessibilityIncrement() {
        guard let node = bridge?.node(for: nodeId) else { return }

        if node.actions & AccessibilityBridge.actionIncrease != 0 {
            bridge?.performAction(AccessibilityBridge.actionIncrease, on: nodeId)
        } else if node.actions & AccessibilityBridge.actionScrollDown != 0 {
            bridge?.performAction(AccessibilityBridge.actionScrollDown, on: nodeId)
        }
    }

    override func accessibilityDecrement() {
        guard let node = bridge?.node(for: nodeId) else { return }

        if node.actions & AccessibilityBridge.actionDecrease != 0 {
            bridge?.performAction(AccessibilityBridge.actionDecrease, on: nodeId)
        } else if node.actions & AccessibilityBridge.actionScrollUp != 0 {
            bridge?.performAction(AccessibilityBridge.actionScrollUp, on: nodeId)
        }
    }

    override func accessibilityScroll(_ direction: UIAccessibilityScrollDirection) -> Bool {
        guard let bridge = bridge else { return false }
        guard let node = bridge.node(for: nodeId) else { return false }

        // Try to scroll this node first
        if tryScroll(node: node, direction: direction, bridge: bridge) {
            return true
        }

        // If this node can't scroll, try to find a scrollable ancestor
        if let scrollableAncestor = bridge.findScrollableAncestor(for: nodeId) {
            return tryScroll(node: scrollableAncestor, direction: direction, bridge: bridge)
        }

        return false
    }

    private func tryScroll(node: SemanticsNode, direction: UIAccessibilityScrollDirection, bridge: AccessibilityBridge) -> Bool {
        switch direction {
        case .up:
            if node.actions & AccessibilityBridge.actionScrollUp != 0 {
                bridge.performAction(AccessibilityBridge.actionScrollUp, on: node.id)
                return true
            }
        case .down:
            if node.actions & AccessibilityBridge.actionScrollDown != 0 {
                bridge.performAction(AccessibilityBridge.actionScrollDown, on: node.id)
                return true
            }
        case .left:
            if node.actions & AccessibilityBridge.actionScrollLeft != 0 {
                bridge.performAction(AccessibilityBridge.actionScrollLeft, on: node.id)
                return true
            }
        case .right:
            if node.actions & AccessibilityBridge.actionScrollRight != 0 {
                bridge.performAction(AccessibilityBridge.actionScrollRight, on: node.id)
                return true
            }
        default:
            break
        }
        return false
    }

    override func accessibilityPerformEscape() -> Bool {
        guard let node = bridge?.node(for: nodeId) else { return false }

        if node.actions & AccessibilityBridge.actionDismiss != 0 {
            bridge?.performAction(AccessibilityBridge.actionDismiss, on: nodeId)
            return true
        }
        return false
    }
}
