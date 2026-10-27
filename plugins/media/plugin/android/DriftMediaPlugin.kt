/**
 * DriftMediaPlugin.kt
 *
 * The media plugin's native half on Android: the `drift/media/video`
 * platform view (VideoPlayerView) and the `drift/media/audio` channel
 * (AudioPlayers). When Drift warms up its platform views at launch, an
 * ExoPlayer is built and released so the first video opens faster.
 *
 * Playback stops when the app goes to the background: every player
 * pauses when the attached Activity stops, other than to be recreated
 * (dark mode, locale), and stays paused when the app returns.
 */
package com.drift.plugin.media

import android.util.Log
import androidx.lifecycle.DefaultLifecycleObserver
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleOwner
import androidx.media3.exoplayer.ExoPlayer
import com.drift.runner.DRIFT_PLATFORM_VIEW_WARM_UP_EVENT
import com.drift.runner.DriftActivityBinding
import com.drift.runner.DriftPlugin
import com.drift.runner.DriftPluginHost

class DriftMediaPlugin : DriftPlugin {
    private lateinit var audio: AudioPlayers
    /** The video views not yet disposed. */
    private val videos = mutableSetOf<VideoPlayerView>()
    /** The attached Activity's lifecycle and the observer pausing playback when it stops. */
    private var foreground: Pair<Lifecycle, DefaultLifecycleObserver>? = null

    override fun onRegister(host: DriftPluginHost) {
        host.registerPlatformView("drift/media/video") { context, _, params, events ->
            VideoPlayerView(context, params, events, onDispose = { videos -= it }).also { videos += it }
        }
        audio = AudioPlayers(host.context) { channel, data -> host.sendEvent(channel, data) }
        host.registerChannel(AudioPlayers.CHANNEL) { audio.declare(this) }
        host.observeEvent(DRIFT_PLATFORM_VIEW_WARM_UP_EVENT) {
            try {
                ExoPlayer.Builder(host.context).build().release()
            } catch (e: Exception) {
                Log.w("DriftMedia", "ExoPlayer warmup failed: ${e.message}")
            }
        }
    }

    override fun onAttach(binding: DriftActivityBinding) {
        val activity = binding.activity
        val observer = object : DefaultLifecycleObserver {
            override fun onStop(owner: LifecycleOwner) {
                // A configuration change stops the Activity only to recreate it.
                if (!activity.isChangingConfigurations) pauseAll()
            }
        }
        activity.lifecycle.addObserver(observer)
        foreground = activity.lifecycle to observer
    }

    override fun onDetach() {
        foreground?.let { (lifecycle, observer) -> lifecycle.removeObserver(observer) }
        foreground = null
    }

    /** Pauses every player; each reports Paused to Go if it was playing. */
    private fun pauseAll() {
        audio.pauseAll()
        videos.forEach { it.pause() }
    }
}
