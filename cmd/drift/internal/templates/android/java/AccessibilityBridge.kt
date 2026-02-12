/**
 * AccessibilityBridge.kt
 * Provides accessibility support for Drift using Android's AccessibilityNodeProvider.
 */
package {{.PackageName}}

import android.graphics.Rect
import android.os.Bundle
import android.view.View
import android.view.accessibility.AccessibilityEvent
import android.view.accessibility.AccessibilityNodeInfo
import android.view.accessibility.AccessibilityNodeProvider
import androidx.core.view.ViewCompat
import androidx.core.view.accessibility.AccessibilityNodeInfoCompat
import kotlin.math.roundToInt

/**
 * Represents a semantics node received from the Go side.
 */
data class SemanticsNode(
    val id: Long,
    val left: Float,
    val top: Float,
    val right: Float,
    val bottom: Float,
    val label: String?,
    val value: String?,
    val hint: String?,
    val role: String,
    val flags: Long,
    val actions: Long,
    val childIds: List<Long>,
    val currentValue: Double?,
    val minValue: Double?,
    val maxValue: Double?,
    val scrollPosition: Double?,
    val scrollExtentMin: Double?,
    val scrollExtentMax: Double?,
    val headingLevel: Int,
    val customActions: List<CustomAction>
)

data class CustomAction(
    val id: Long,
    val label: String
)

/**
 * AccessibilityBridge extends AccessibilityNodeProvider to expose Drift's semantics
 * tree to Android's accessibility services (TalkBack, Switch Access, etc.).
 */
class AccessibilityBridge(private val hostView: View) : AccessibilityNodeProvider() {

    private val nodes = mutableMapOf<Long, SemanticsNode>()
    private var rootId: Long = -1
    private var accessibilityFocusedNodeId: Long = View.NO_ID.toLong()

    companion object {
        // Semantics flags (must match Go side)
        const val FLAG_HAS_CHECKED_STATE = 1L shl 0
        const val FLAG_IS_CHECKED = 1L shl 1
        const val FLAG_HAS_SELECTED_STATE = 1L shl 2
        const val FLAG_IS_SELECTED = 1L shl 3
        const val FLAG_HAS_ENABLED_STATE = 1L shl 4
        const val FLAG_IS_ENABLED = 1L shl 5
        const val FLAG_IS_FOCUSABLE = 1L shl 6
        const val FLAG_IS_FOCUSED = 1L shl 7
        const val FLAG_IS_BUTTON = 1L shl 8
        const val FLAG_IS_TEXT_FIELD = 1L shl 9
        const val FLAG_IS_READ_ONLY = 1L shl 10
        const val FLAG_IS_OBSCURED = 1L shl 11
        const val FLAG_IS_MULTILINE = 1L shl 12
        const val FLAG_IS_SLIDER = 1L shl 13
        const val FLAG_IS_LIVE_REGION = 1L shl 14
        const val FLAG_HAS_TOGGLED_STATE = 1L shl 15
        const val FLAG_IS_TOGGLED = 1L shl 16
        const val FLAG_HAS_IMPLICIT_SCROLLING = 1L shl 17
        const val FLAG_IS_HIDDEN = 1L shl 18
        const val FLAG_IS_HEADER = 1L shl 19
        const val FLAG_IS_IMAGE = 1L shl 20
        const val FLAG_NAMES_ROUTE = 1L shl 21
        const val FLAG_SCOPES_ROUTE = 1L shl 22
        const val FLAG_IS_IN_MUTUALLY_EXCLUSIVE_GROUP = 1L shl 23
        const val FLAG_HAS_EXPANDED_STATE = 1L shl 24
        const val FLAG_IS_EXPANDED = 1L shl 25

        // Semantics actions (must match Go side)
        const val ACTION_TAP = 1L shl 0
        const val ACTION_LONG_PRESS = 1L shl 1
        const val ACTION_SCROLL_LEFT = 1L shl 2
        const val ACTION_SCROLL_RIGHT = 1L shl 3
        const val ACTION_SCROLL_UP = 1L shl 4
        const val ACTION_SCROLL_DOWN = 1L shl 5
        const val ACTION_INCREASE = 1L shl 6
        const val ACTION_DECREASE = 1L shl 7
        const val ACTION_FOCUS = 1L shl 18
        const val ACTION_DISMISS = 1L shl 21
    }

    /**
     * Updates the semantics tree with new nodes and removals.
     */
    fun updateSemantics(updates: List<Map<String, Any?>>, removals: List<Long>) {
        // Process removals
        for (id in removals) {
            nodes.remove(id)
        }

        // Process updates
        for (update in updates) {
            val node = parseNode(update)
            nodes[node.id] = node
        }

        // Use the synthetic root (node 0) as the accessibility root so all its
        // children (e.g., barrier + sheet in overlays) are reachable by TalkBack.
        val newRootId = if (nodes.containsKey(0L)) {
            0L
        } else {
            nodes.keys.minOrNull() ?: -1L
        }

        if (newRootId != rootId) {
            rootId = newRootId
        }

        // Notify accessibility services of changes
        hostView.post {
            val event = AccessibilityEvent.obtain(AccessibilityEvent.TYPE_WINDOW_CONTENT_CHANGED)
            event.packageName = hostView.context.packageName
            event.setSource(hostView, View.NO_ID)
            hostView.parent?.requestSendAccessibilityEvent(hostView, event)
        }
    }

    private fun parseNode(data: Map<String, Any?>): SemanticsNode {
        val childIds = (data["childIds"] as? List<*>)?.mapNotNull {
            when (it) {
                is Number -> it.toLong()
                else -> null
            }
        } ?: emptyList()

        val customActions = (data["customActions"] as? List<*>)?.mapNotNull { action ->
            (action as? Map<*, *>)?.let {
                CustomAction(
                    id = (it["id"] as? Number)?.toLong() ?: return@let null,
                    label = it["label"] as? String ?: ""
                )
            }
        } ?: emptyList()

        return SemanticsNode(
            id = (data["id"] as? Number)?.toLong() ?: 0L,
            left = (data["left"] as? Number)?.toFloat() ?: 0f,
            top = (data["top"] as? Number)?.toFloat() ?: 0f,
            right = (data["right"] as? Number)?.toFloat() ?: 0f,
            bottom = (data["bottom"] as? Number)?.toFloat() ?: 0f,
            label = data["label"] as? String,
            value = data["value"] as? String,
            hint = data["hint"] as? String,
            role = data["role"] as? String ?: "none",
            flags = (data["flags"] as? Number)?.toLong() ?: 0L,
            actions = (data["actions"] as? Number)?.toLong() ?: 0L,
            childIds = childIds,
            currentValue = (data["currentValue"] as? Number)?.toDouble(),
            minValue = (data["minValue"] as? Number)?.toDouble(),
            maxValue = (data["maxValue"] as? Number)?.toDouble(),
            scrollPosition = (data["scrollPosition"] as? Number)?.toDouble(),
            scrollExtentMin = (data["scrollExtentMin"] as? Number)?.toDouble(),
            scrollExtentMax = (data["scrollExtentMax"] as? Number)?.toDouble(),
            headingLevel = (data["headingLevel"] as? Number)?.toInt() ?: 0,
            customActions = customActions
        )
    }

    override fun createAccessibilityNodeInfo(virtualViewId: Int): AccessibilityNodeInfo? {
        if (virtualViewId == View.NO_ID) {
            // Create info for the host view itself
            val info = AccessibilityNodeInfo.obtain(hostView)
            hostView.onInitializeAccessibilityNodeInfo(info)

            // Add root semantic node as child if present
            if (rootId != -1L) {
                info.addChild(hostView, rootId.toInt())
            }
            return info
        }

        val node = nodes[virtualViewId.toLong()] ?: return null

        return createNodeInfo(node)
    }

    private fun createNodeInfo(node: SemanticsNode): AccessibilityNodeInfo {
        val info = AccessibilityNodeInfo.obtain(hostView, node.id.toInt())
        val compat = AccessibilityNodeInfoCompat.wrap(info)

        // Set parent
        val parent = findParentNode(node.id)
        if (parent != null) {
            info.setParent(hostView, parent.id.toInt())
        } else {
            info.setParent(hostView)
        }

        // Set bounds
        val bounds = Rect(
            node.left.roundToInt(),
            node.top.roundToInt(),
            node.right.roundToInt(),
            node.bottom.roundToInt()
        )
        info.setBoundsInParent(bounds)

        // Convert to screen coordinates
        val location = IntArray(2)
        hostView.getLocationOnScreen(location)
        bounds.offset(location[0], location[1])

        // Get the visible area on screen
        val visibleRect = Rect()
        hostView.getGlobalVisibleRect(visibleRect)

        // Clip bounds to visible area and track if node is visible
        val clippedBounds = Rect(bounds)
        val isOnScreen = clippedBounds.intersect(visibleRect)
        info.setBoundsInScreen(if (isOnScreen) clippedBounds else bounds)

        // Set text content
        node.label?.let { info.contentDescription = it }
        node.value?.let { info.text = it }
        node.hint?.let { compat.hintText = it }

        // Set class name based on role
        info.className = mapRoleToClassName(node.role)

        // Set state flags
        if (node.flags and FLAG_HAS_CHECKED_STATE != 0L) {
            info.isCheckable = true
            info.isChecked = node.flags and FLAG_IS_CHECKED != 0L
        }

        if (node.flags and FLAG_HAS_SELECTED_STATE != 0L) {
            info.isSelected = node.flags and FLAG_IS_SELECTED != 0L
        }

        if (node.flags and FLAG_HAS_ENABLED_STATE != 0L) {
            info.isEnabled = node.flags and FLAG_IS_ENABLED != 0L
        } else {
            info.isEnabled = true
        }

        val hasContent = node.label != null || node.value != null || node.hint != null
        val hasActions = node.actions != 0L
        info.isFocusable = node.flags and FLAG_IS_FOCUSABLE != 0L || hasContent || hasActions
        info.isFocused = node.flags and FLAG_IS_FOCUSED != 0L
        info.isAccessibilityFocused = node.id == accessibilityFocusedNodeId
        // Node is visible if not hidden AND at least partially on screen
        info.isVisibleToUser = (node.flags and FLAG_IS_HIDDEN == 0L) && isOnScreen

        info.isClickable = node.actions and ACTION_TAP != 0L
        info.isLongClickable = node.actions and ACTION_LONG_PRESS != 0L

        if (node.flags and FLAG_IS_TEXT_FIELD != 0L) {
            compat.isEditable = node.flags and FLAG_IS_READ_ONLY == 0L
            info.isPassword = node.flags and FLAG_IS_OBSCURED != 0L
            compat.isMultiLine = node.flags and FLAG_IS_MULTILINE != 0L
        }

        if (node.flags and FLAG_IS_HEADER != 0L) {
            compat.isHeading = true
        }

        if (node.flags and FLAG_IS_LIVE_REGION != 0L) {
            info.liveRegion = View.ACCESSIBILITY_LIVE_REGION_POLITE
        }

        if (node.flags and FLAG_HAS_EXPANDED_STATE != 0L) {
            val extras = info.extras
            extras.putBoolean("AccessibilityNodeInfo.isExpanded", node.flags and FLAG_IS_EXPANDED != 0L)
        }

        // Set range info for sliders
        if (node.currentValue != null && node.minValue != null && node.maxValue != null) {
            val rangeType = AccessibilityNodeInfoCompat.RangeInfoCompat.RANGE_TYPE_FLOAT
            val rangeInfo = AccessibilityNodeInfoCompat.RangeInfoCompat.obtain(
                rangeType,
                node.minValue.toFloat(),
                node.maxValue.toFloat(),
                node.currentValue.toFloat()
            )
            compat.rangeInfo = rangeInfo
        }

        // Add actions
        if (node.actions and ACTION_TAP != 0L) {
            info.addAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_CLICK)
        }
        if (node.actions and ACTION_LONG_PRESS != 0L) {
            info.addAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_LONG_CLICK)
        }
        if (node.actions and ACTION_SCROLL_UP != 0L || node.actions and ACTION_SCROLL_DOWN != 0L) {
            info.addAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_FORWARD)
            info.addAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_BACKWARD)
            info.isScrollable = true
        }
        if (node.actions and ACTION_SCROLL_LEFT != 0L || node.actions and ACTION_SCROLL_RIGHT != 0L) {
            info.addAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_LEFT)
            info.addAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_RIGHT)
            info.isScrollable = true
        }
        if (node.actions and ACTION_INCREASE != 0L) {
            compat.addAction(AccessibilityNodeInfoCompat.AccessibilityActionCompat.ACTION_SET_PROGRESS)
        }
        if (node.actions and ACTION_FOCUS != 0L) {
            info.addAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_ACCESSIBILITY_FOCUS)
        }
        if (node.actions and ACTION_DISMISS != 0L) {
            info.addAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_DISMISS)
        }

        // Always allow focus/unfocus for accessibility navigation
        info.addAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_ACCESSIBILITY_FOCUS)
        info.addAction(AccessibilityNodeInfo.AccessibilityAction.ACTION_CLEAR_ACCESSIBILITY_FOCUS)

        // Add custom actions
        for (action in node.customActions) {
            val customAction = AccessibilityNodeInfo.AccessibilityAction(
                action.id.toInt(),
                action.label
            )
            info.addAction(customAction)
        }

        // Add children
        for (childId in node.childIds) {
            info.addChild(hostView, childId.toInt())
        }

        return info
    }

    private fun mapRoleToClassName(role: String): String {
        return when (role) {
            "button" -> "android.widget.Button"
            "checkbox" -> "android.widget.CheckBox"
            "radio" -> "android.widget.RadioButton"
            "switch" -> "android.widget.Switch"
            "textField" -> "android.widget.EditText"
            "slider" -> "android.widget.SeekBar"
            "progressIndicator" -> "android.widget.ProgressBar"
            "image" -> "android.widget.ImageView"
            "tab" -> "android.widget.TabWidget"
            "list" -> "android.widget.ListView"
            "scrollView" -> "android.widget.ScrollView"
            "header" -> "android.widget.TextView"
            else -> "android.view.View"
        }
    }

    private fun findParentNode(childId: Long): SemanticsNode? {
        for (node in nodes.values) {
            if (node.childIds.contains(childId)) {
                return node
            }
        }
        return null
    }

    override fun performAction(virtualViewId: Int, action: Int, arguments: Bundle?): Boolean {
        if (virtualViewId == View.NO_ID) {
            // Handle host view actions directly
            when (action) {
                AccessibilityNodeInfo.ACTION_ACCESSIBILITY_FOCUS -> {
                    // Focus the first accessible child (the root of our semantic tree)
                    if (rootId != -1L) {
                        setAccessibilityFocus(rootId)
                    }
                    return true
                }
                AccessibilityNodeInfo.ACTION_CLEAR_ACCESSIBILITY_FOCUS -> {
                    clearAccessibilityFocus()
                    return true
                }
                else -> {
                    return hostView.performAccessibilityAction(action, arguments)
                }
            }
        }

        val node = nodes[virtualViewId.toLong()] ?: return false

        when (action) {
            AccessibilityNodeInfo.ACTION_CLICK -> {
                if (node.actions and ACTION_TAP != 0L) {
                    sendActionToGo(node.id, ACTION_TAP, null)
                    return true
                }
            }
            AccessibilityNodeInfo.ACTION_LONG_CLICK -> {
                if (node.actions and ACTION_LONG_PRESS != 0L) {
                    sendActionToGo(node.id, ACTION_LONG_PRESS, null)
                    return true
                }
            }
            AccessibilityNodeInfo.ACTION_SCROLL_FORWARD -> {
                if (node.actions and ACTION_SCROLL_DOWN != 0L) {
                    sendActionToGo(node.id, ACTION_SCROLL_DOWN, null)
                    return true
                }
                if (node.actions and ACTION_INCREASE != 0L) {
                    sendActionToGo(node.id, ACTION_INCREASE, null)
                    return true
                }
            }
            AccessibilityNodeInfo.ACTION_SCROLL_BACKWARD -> {
                if (node.actions and ACTION_SCROLL_UP != 0L) {
                    sendActionToGo(node.id, ACTION_SCROLL_UP, null)
                    return true
                }
                if (node.actions and ACTION_DECREASE != 0L) {
                    sendActionToGo(node.id, ACTION_DECREASE, null)
                    return true
                }
            }
            AccessibilityNodeInfo.ACTION_ACCESSIBILITY_FOCUS -> {
                if (accessibilityFocusedNodeId != node.id) {
                    accessibilityFocusedNodeId = node.id
                    sendAccessibilityEvent(node.id, AccessibilityEvent.TYPE_VIEW_ACCESSIBILITY_FOCUSED)
                    return true
                }
            }
            AccessibilityNodeInfo.ACTION_CLEAR_ACCESSIBILITY_FOCUS -> {
                if (accessibilityFocusedNodeId == node.id) {
                    accessibilityFocusedNodeId = View.NO_ID.toLong()
                    sendAccessibilityEvent(node.id, AccessibilityEvent.TYPE_VIEW_ACCESSIBILITY_FOCUS_CLEARED)
                    return true
                }
            }
            AccessibilityNodeInfo.ACTION_DISMISS -> {
                if (node.actions and ACTION_DISMISS != 0L) {
                    sendActionToGo(node.id, ACTION_DISMISS, null)
                    return true
                }
            }
            AccessibilityNodeInfoCompat.AccessibilityActionCompat.ACTION_SET_PROGRESS.id -> {
                // Handle slider progress changes from TalkBack
                val progress = arguments?.getFloat(AccessibilityNodeInfoCompat.ACTION_ARGUMENT_PROGRESS_VALUE)
                if (progress != null && node.minValue != null && node.maxValue != null) {
                    sendActionToGo(node.id, ACTION_INCREASE, mapOf("value" to progress.toDouble()))
                    return true
                }
            }
            AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_LEFT.id -> {
                if (node.actions and ACTION_SCROLL_LEFT != 0L) {
                    sendActionToGo(node.id, ACTION_SCROLL_LEFT, null)
                    return true
                }
            }
            AccessibilityNodeInfo.AccessibilityAction.ACTION_SCROLL_RIGHT.id -> {
                if (node.actions and ACTION_SCROLL_RIGHT != 0L) {
                    sendActionToGo(node.id, ACTION_SCROLL_RIGHT, null)
                    return true
                }
            }
            else -> {
                // Check for custom action
                val customAction = node.customActions.find { it.id.toInt() == action }
                if (customAction != null) {
                    sendActionToGo(node.id, action.toLong(), mapOf("actionId" to customAction.id))
                    return true
                }
            }
        }

        return false
    }

    private fun sendActionToGo(nodeId: Long, action: Long, args: Map<String, Any>?) {
        val payload = mutableMapOf<String, Any?>(
            "nodeId" to nodeId,
            "action" to action
        )
        if (args != null) {
            payload["args"] = args
        }
        PlatformChannelManager.sendEvent("drift/accessibility/actions", payload)
    }

    private fun sendAccessibilityEvent(nodeId: Long, eventType: Int) {
        hostView.post {
            val event = AccessibilityEvent.obtain(eventType)
            event.packageName = hostView.context.packageName
            event.setSource(hostView, nodeId.toInt())
            hostView.parent?.requestSendAccessibilityEvent(hostView, event)
        }
    }

    /**
     * Finds the virtual view that has the specified focus type.
     * Required for TalkBack's explore-by-touch to work properly.
     */
    override fun findFocus(focus: Int): AccessibilityNodeInfo? {
        when (focus) {
            AccessibilityNodeInfo.FOCUS_ACCESSIBILITY -> {
                if (accessibilityFocusedNodeId != View.NO_ID.toLong()) {
                    val node = nodes[accessibilityFocusedNodeId]
                    if (node != null) {
                        return createNodeInfo(node)
                    }
                }
            }
            AccessibilityNodeInfo.FOCUS_INPUT -> {
                // Find node with input focus (text field being edited)
                for (node in nodes.values) {
                    if (node.flags and FLAG_IS_FOCUSED != 0L) {
                        return createNodeInfo(node)
                    }
                }
            }
        }
        return null
    }

    /**
     * Finds the virtual view at the given screen coordinates.
     * Used by explore-by-touch to determine which element to focus.
     */
    fun findNodeAtPoint(x: Float, y: Float): SemanticsNode? {
        // The coordinates from touch events are already in view coordinates
        // Our node bounds are also in view coordinates (we add screen offset only for setBoundsInScreen)
        return findNodeAtPointRecursive(rootId, x, y)
    }

    private fun findNodeAtPointRecursive(nodeId: Long, x: Float, y: Float): SemanticsNode? {
        val node = nodes[nodeId] ?: return null

        // Skip hidden nodes and their descendants
        if (node.flags and FLAG_IS_HIDDEN != 0L) {
            return null
        }

        // Check if point is within this node's bounds
        val inBounds = x >= node.left && x <= node.right && y >= node.top && y <= node.bottom
        if (!inBounds) {
            return null
        }

        // Check children first (depth-first, so we find the deepest/topmost node)
        for (childId in node.childIds.reversed()) {
            val childResult = findNodeAtPointRecursive(childId, x, y)
            if (childResult != null) {
                return childResult
            }
        }

        // If this node has content or actions, return it
        val hasContent = node.label != null || node.value != null || node.hint != null
        val hasActions = node.actions != 0L
        val isFocusable = node.flags and FLAG_IS_FOCUSABLE != 0L

        if (hasContent || hasActions || isFocusable) {
            return node
        }

        return null
    }

    /**
     * Handles hover events for explore-by-touch.
     * Call this from the host view's dispatchHoverEvent.
     * Returns true if the event was handled.
     */
    fun onHoverEvent(x: Float, y: Float, action: Int): Boolean {
        val node = findNodeAtPoint(x, y)
        val nodeId = node?.id ?: View.NO_ID.toLong()

        when (action) {
            android.view.MotionEvent.ACTION_HOVER_ENTER,
            android.view.MotionEvent.ACTION_HOVER_MOVE -> {
                if (nodeId != accessibilityFocusedNodeId && nodeId != View.NO_ID.toLong()) {
                    // Clear old focus
                    if (accessibilityFocusedNodeId != View.NO_ID.toLong()) {
                        sendAccessibilityEvent(accessibilityFocusedNodeId, AccessibilityEvent.TYPE_VIEW_HOVER_EXIT)
                    }
                    // Set new focus
                    accessibilityFocusedNodeId = nodeId
                    sendAccessibilityEvent(nodeId, AccessibilityEvent.TYPE_VIEW_HOVER_ENTER)
                    sendAccessibilityEvent(nodeId, AccessibilityEvent.TYPE_VIEW_ACCESSIBILITY_FOCUSED)
                    return true
                }
            }
            android.view.MotionEvent.ACTION_HOVER_EXIT -> {
                if (accessibilityFocusedNodeId != View.NO_ID.toLong()) {
                    sendAccessibilityEvent(accessibilityFocusedNodeId, AccessibilityEvent.TYPE_VIEW_HOVER_EXIT)
                    // Don't clear focus on exit - keep the last focused item
                    return true
                }
            }
        }
        return false
    }

    /**
     * Announces a message to accessibility services.
     */
    fun announce(message: String, politeness: String) {
        hostView.post {
            hostView.announceForAccessibility(message)
        }
    }

    /**
     * Sets accessibility focus to a specific node.
     */
    fun setAccessibilityFocus(nodeId: Long) {
        if (nodes.containsKey(nodeId) && accessibilityFocusedNodeId != nodeId) {
            // Clear previous focus first
            val oldId = accessibilityFocusedNodeId
            if (oldId != View.NO_ID.toLong()) {
                sendAccessibilityEvent(oldId, AccessibilityEvent.TYPE_VIEW_ACCESSIBILITY_FOCUS_CLEARED)
            }
            // Set new focus
            accessibilityFocusedNodeId = nodeId
            sendAccessibilityEvent(nodeId, AccessibilityEvent.TYPE_VIEW_ACCESSIBILITY_FOCUSED)
        }
    }

    /**
     * Clears the current accessibility focus.
     */
    fun clearAccessibilityFocus() {
        if (accessibilityFocusedNodeId != View.NO_ID.toLong()) {
            val oldId = accessibilityFocusedNodeId
            accessibilityFocusedNodeId = View.NO_ID.toLong()
            sendAccessibilityEvent(oldId, AccessibilityEvent.TYPE_VIEW_ACCESSIBILITY_FOCUS_CLEARED)
        }
    }
}
