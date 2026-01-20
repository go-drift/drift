/**
 * NotificationBridge.kt
 * Hooks for remote notification providers.
 */
package {{.PackageName}}

import android.content.Context

object NotificationBridge {
    fun handleRemoteMessage(
        context: Context,
        title: String?,
        body: String?,
        data: Map<String, Any?>? = null
    ) {
        NotificationHandler.handleRemoteMessage(context, title, body, data)
    }

    fun handleNewToken(token: String, isRefresh: Boolean = true) {
        NotificationHandler.handleNewToken(token, isRefresh)
    }
}
