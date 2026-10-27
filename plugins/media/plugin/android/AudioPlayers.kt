/**
 * AudioPlayers.kt
 *
 * Audio-only ExoPlayer playback for the `drift/media/audio` channel:
 * several players at once, each identified by the playerId Go assigns
 * (runtime/audio.go). Main thread only.
 */
package com.drift.plugin.media

import android.content.Context
import androidx.media3.common.AudioAttributes
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.exoplayer.ExoPlayer
import com.drift.runner.DriftCallError
import com.drift.runner.DriftChannel

/**
 * Per-instance audio player state. Every event carries the state and the
 * progress (PlaybackProgress.kt says when they are sent).
 */
private class AudioPlayerInstance(
    val id: Long,
    context: Context,
    /** Sends an event to Go on a channel. */
    private val send: (channel: String, data: Map<String, Any?>) -> Unit,
) {
    val player: ExoPlayer = ExoPlayer.Builder(context).build().also {
        it.setAudioAttributes(
            AudioAttributes.Builder()
                .setUsage(C.USAGE_MEDIA)
                .setContentType(C.AUDIO_CONTENT_TYPE_MUSIC)
                .build(),
            /* handleAudioFocus= */ true
        )
    }
    private val progress: ProgressTicker

    init {
        player.addListener(object : Player.Listener {
            override fun onPlaybackStateChanged(playbackState: Int) {
                sendStateEvent()
            }

            override fun onIsPlayingChanged(isPlaying: Boolean) {
                if (player.playbackState == Player.STATE_READY) sendStateEvent()
            }

            override fun onPlayerError(error: PlaybackException) {
                send(
                    AudioPlayers.ERRORS_CHANNEL,
                    mapOf(
                        "playerId" to id,
                        "code" to mediaErrorCodeString(error.errorCode),
                        "message" to (error.message ?: "Unknown playback error")
                    )
                )
            }
        })
        progress = ProgressTicker(player) { sendStateEvent() }
    }

    fun sendStateEvent() {
        send(
            AudioPlayers.EVENTS_CHANNEL,
            mapOf(
                "playerId" to id,
                "playbackState" to player.driftState(),
                "positionMs" to player.currentPosition,
                "durationMs" to player.duration.coerceAtLeast(0),
                "bufferedMs" to player.bufferedPosition
            )
        )
    }

    fun dispose() {
        progress.stop()
        player.release()
    }
}

/** The `drift/media/audio` channel's players, keyed by playerId. */
internal class AudioPlayers(
    private val context: Context,
    private val send: (channel: String, data: Map<String, Any?>) -> Unit,
) {
    private val players = mutableMapOf<Long, AudioPlayerInstance>()

    /** Declares the channel's methods. */
    fun declare(channel: DriftChannel) = with(channel) {
        method("load") { args ->
            val url = (args as? Map<*, *>)?.get("url") as? String
                ?: throw DriftCallError("invalid_arguments", "load needs a url")
            val player = player(args).player
            player.setMediaItem(MediaItem.fromUri(url))
            player.prepare()
            null
        }
        method("play") { args ->
            val player = player(args).player
            if (player.playbackState == Player.STATE_IDLE) player.prepare()
            player.play()
            null
        }
        method("pause") { args -> player(args).player.pause(); null }
        method("stop") { args ->
            // Idle, then the seek reports the position back at zero.
            val player = player(args).player
            player.stop()
            player.seekTo(0)
            null
        }
        method("seekTo") { args ->
            player(args).player.seekTo(number(args, "positionMs")?.toLong() ?: 0L)
            null
        }
        method("setVolume") { args ->
            player(args).player.volume = number(args, "volume")?.toFloat() ?: 1.0f
            null
        }
        method("setLooping") { args ->
            val looping = (args as? Map<*, *>)?.get("looping") as? Boolean ?: false
            player(args).player.repeatMode = if (looping) Player.REPEAT_MODE_ALL else Player.REPEAT_MODE_OFF
            null
        }
        method("setPlaybackSpeed") { args ->
            player(args).player.setPlaybackSpeed(number(args, "rate")?.toFloat() ?: 1.0f)
            null
        }
        method("dispose") { args ->
            players.remove(number(args, "playerId")?.toLong() ?: 0L)?.dispose()
            null
        }
    }

    /** Pauses every player. */
    fun pauseAll() {
        players.values.forEach { it.player.pause() }
    }

    private fun number(args: Any?, key: String) = (args as? Map<*, *>)?.get(key) as? Number

    /**
     * The player args' playerId names, created on first use by any method
     * but dispose, so settings made before load apply to what it loads.
     */
    private fun player(args: Any?): AudioPlayerInstance {
        val id = number(args, "playerId")?.toLong() ?: 0L
        return players.getOrPut(id) { AudioPlayerInstance(id, context, send) }
    }

    companion object {
        const val CHANNEL = "drift/media/audio"
        const val EVENTS_CHANNEL = "drift/media/audio/events"
        const val ERRORS_CHANNEL = "drift/media/audio/errors"
    }
}
