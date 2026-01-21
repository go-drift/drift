/**
 * AccessibilityHandler.kt
 * Handles accessibility platform channel messages for Drift.
 */
package {{.PackageName}}

import android.content.Context
import android.view.accessibility.AccessibilityManager
import androidx.core.view.ViewCompat
import androidx.core.view.accessibility.AccessibilityNodeInfoCompat

object AccessibilityHandler {
    private var bridge: AccessibilityBridge? = null
    private var accessibilityManager: AccessibilityManager? = null

    /**
     * Initializes the accessibility handler with the host view.
     */
    fun initialize(context: Context, hostView: android.view.View) {
        bridge = AccessibilityBridge(hostView)
        accessibilityManager = context.getSystemService(Context.ACCESSIBILITY_SERVICE) as? AccessibilityManager

        // Make the view important for accessibility
        hostView.importantForAccessibility = android.view.View.IMPORTANT_FOR_ACCESSIBILITY_YES
        hostView.isFocusable = true
        hostView.isFocusableInTouchMode = true

        // Set hover listener for explore-by-touch
        hostView.setOnHoverListener { _, event ->
            onHoverEvent(event.x, event.y, event.actionMasked)
        }

        // Use ViewCompat to set the accessibility delegate for better compatibility
        ViewCompat.setAccessibilityDelegate(hostView, object : androidx.core.view.AccessibilityDelegateCompat() {
            override fun getAccessibilityNodeProvider(host: android.view.View): androidx.core.view.accessibility.AccessibilityNodeProviderCompat? {
                // Wrap our AccessibilityNodeProvider in a compat wrapper
                return bridge?.let { androidx.core.view.accessibility.AccessibilityNodeProviderCompat(it) }
            }

            override fun onInitializeAccessibilityNodeInfo(host: android.view.View, info: AccessibilityNodeInfoCompat) {
                super.onInitializeAccessibilityNodeInfo(host, info)
                // Mark that this view has virtual children and supports exploration
                info.className = "android.view.View"
                info.isScreenReaderFocusable = true
                info.isVisibleToUser = true
                info.isEnabled = true
                info.isFocusable = true
                // Set content description to help TalkBack understand this is an interactive view
                if (info.contentDescription == null) {
                    info.contentDescription = "Drift application content"
                }
            }

            override fun dispatchPopulateAccessibilityEvent(host: android.view.View, event: android.view.accessibility.AccessibilityEvent): Boolean {
                return bridge?.let { true } ?: super.dispatchPopulateAccessibilityEvent(host, event)
            }
        })

        // Notify Go side of initial accessibility state
        notifyAccessibilityState()

        // Listen for accessibility state changes
        accessibilityManager?.addAccessibilityStateChangeListener { enabled ->
            PlatformChannelManager.sendEvent(
                "drift/accessibility/state",
                mapOf("enabled" to enabled)
            )
        }
    }

    /**
     * Handles platform channel method calls.
     */
    fun handle(context: Context, method: String, args: Any?): Pair<Any?, Exception?> {
        return when (method) {
            "updateSemantics" -> updateSemantics(args)
            "announce" -> announce(args)
            "setAccessibilityFocus" -> setAccessibilityFocus(args)
            "clearAccessibilityFocus" -> clearAccessibilityFocus()
            "isAccessibilityEnabled" -> isAccessibilityEnabled()
            else -> Pair(null, IllegalArgumentException("Unknown method: $method"))
        }
    }

    private fun updateSemantics(args: Any?): Pair<Any?, Exception?> {
        val map = args as? Map<*, *> ?: return Pair(null, IllegalArgumentException("Invalid arguments"))

        @Suppress("UNCHECKED_CAST")
        val updates = (map["updates"] as? List<Map<String, Any?>>) ?: emptyList()
        val removals = (map["removals"] as? List<*>)?.mapNotNull {
            when (it) {
                is Number -> it.toLong()
                else -> null
            }
        } ?: emptyList()

        bridge?.updateSemantics(updates, removals)
        return Pair(null, null)
    }

    private fun announce(args: Any?): Pair<Any?, Exception?> {
        val map = args as? Map<*, *> ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val message = map["message"] as? String ?: return Pair(null, IllegalArgumentException("Missing message"))
        val politeness = map["politeness"] as? String ?: "polite"

        bridge?.announce(message, politeness)
        return Pair(null, null)
    }

    private fun setAccessibilityFocus(args: Any?): Pair<Any?, Exception?> {
        val map = args as? Map<*, *> ?: return Pair(null, IllegalArgumentException("Invalid arguments"))
        val nodeId = (map["nodeId"] as? Number)?.toLong() ?: return Pair(null, IllegalArgumentException("Missing nodeId"))

        bridge?.setAccessibilityFocus(nodeId)
        return Pair(null, null)
    }

    private fun clearAccessibilityFocus(): Pair<Any?, Exception?> {
        bridge?.clearAccessibilityFocus()
        return Pair(null, null)
    }

    private fun isAccessibilityEnabled(): Pair<Any?, Exception?> {
        val enabled = accessibilityManager?.isEnabled ?: false
        return Pair(mapOf("enabled" to enabled), null)
    }

    private fun notifyAccessibilityState() {
        val enabled = accessibilityManager?.isEnabled ?: false
        PlatformChannelManager.sendEvent(
            "drift/accessibility/state",
            mapOf("enabled" to enabled)
        )
    }

    /**
     * Handles hover events for explore-by-touch accessibility.
     * Returns true if the event was handled.
     */
    fun onHoverEvent(x: Float, y: Float, action: Int): Boolean {
        val touchExploration = accessibilityManager?.isTouchExplorationEnabled ?: false
        if (!touchExploration) {
            return false
        }
        return bridge?.onHoverEvent(x, y, action) ?: false
    }

    /**
     * Handles explore-by-touch when the user taps to find an element.
     * This is called from dispatchTouchEvent when touch exploration is enabled.
     * Returns true if accessibility handled the touch (element was found and focused).
     */
    fun handleExploreByTouch(x: Float, y: Float): Boolean {
        val touchExploration = accessibilityManager?.isTouchExplorationEnabled ?: false
        if (!touchExploration) {
            return false
        }

        val node = bridge?.findNodeAtPoint(x, y)
        if (node != null) {
            bridge?.setAccessibilityFocus(node.id)
            return true
        }

        return false
    }
}
