/**
 * DriftMediaPlugin.kt
 *
 * The media plugin's native half on Android: the `drift/media/video`
 * platform view (VideoPlayerView) and the `drift/media/audio` channel
 * (AudioPlayers). When Drift warms up its platform views at launch, an
 * ExoPlayer is built and released so the first video opens faster.
 */
package com.drift.plugin.media

import android.util.Log
import androidx.media3.exoplayer.ExoPlayer
import com.drift.runner.DRIFT_PLATFORM_VIEW_WARM_UP_EVENT
import com.drift.runner.DriftPlugin
import com.drift.runner.DriftPluginHost

class DriftMediaPlugin : DriftPlugin {
    override fun onRegister(host: DriftPluginHost) {
        host.registerPlatformView("drift/media/video") { context, _, params, events ->
            VideoPlayerView(context, params, events)
        }
        val audio = AudioPlayers(host.context) { channel, data -> host.sendEvent(channel, data) }
        host.registerChannel(AudioPlayers.CHANNEL) { audio.declare(this) }
        host.observeEvent(DRIFT_PLATFORM_VIEW_WARM_UP_EVENT) {
            try {
                ExoPlayer.Builder(host.context).build().release()
            } catch (e: Exception) {
                Log.w("DriftMedia", "ExoPlayer warmup failed: ${e.message}")
            }
        }
    }
}
