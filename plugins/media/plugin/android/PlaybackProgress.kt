/**
 * PlaybackProgress.kt
 *
 * What audio players and video views share in reporting playback to Go:
 * the state as Go numbers it, and when progress (position, duration,
 * buffered) is reported. Progress goes to Go every 250ms while media is
 * loaded (buffering or ready, playing or paused) if it changed, with
 * every state change,
 * and at once when a seek moves the position, Stop's return to the start
 * included, so Go's Progress is right after Load, a seek while paused and
 * Stop (runtime/media.go). Main thread only.
 */
package com.drift.plugin.media

import android.os.Handler
import android.os.Looper
import androidx.media3.common.Player

/** The player's state as Go's PlaybackState (runtime/media.go). */
internal fun Player.driftState(): Int = when (playbackState) {
    Player.STATE_BUFFERING -> 1
    Player.STATE_READY -> if (isPlaying) 2 else 4 // Playing or Paused
    Player.STATE_ENDED -> 3
    else -> 0 // Idle
}

/**
 * Calls report every 250ms while player has media loaded, unless the
 * progress is as last reported (paused with the media buffered, say, or
 * the app in the background), and at once when a seek moves its position.
 * The owner reports with each state change itself.
 */
internal class ProgressTicker(private val player: Player, private val report: () -> Unit) {
    private val main = Handler(Looper.getMainLooper())
    /** The position, duration and buffered position this last reported. */
    private var reported: List<Long>? = null

    private val tick = object : Runnable {
        override fun run() {
            if (snapshot() != reported) reportNow()
            main.postDelayed(this, INTERVAL_MS)
        }
    }

    private fun snapshot() = listOf(player.currentPosition, player.duration, player.bufferedPosition)

    private fun reportNow() {
        reported = snapshot()
        report()
    }

    private val listener = object : Player.Listener {
        override fun onPlaybackStateChanged(playbackState: Int) {
            main.removeCallbacks(tick)
            if (playbackState == Player.STATE_BUFFERING || playbackState == Player.STATE_READY) {
                main.postDelayed(tick, INTERVAL_MS)
            }
        }

        override fun onPositionDiscontinuity(
            oldPosition: Player.PositionInfo,
            newPosition: Player.PositionInfo,
            reason: Int,
        ) {
            reportNow()
        }
    }

    init {
        player.addListener(listener)
    }

    /** Stops reporting, before the player is released. */
    fun stop() {
        main.removeCallbacks(tick)
        player.removeListener(listener)
    }

    private companion object {
        const val INTERVAL_MS = 250L
    }
}
