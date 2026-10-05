/**
 * AudioPlayers.kt
 *
 * Audio-only ExoPlayer playback for the `drift/media/audio` channel:
 * several players at once, each identified by the playerId Go assigns
 * (runtime/audio.go). Main thread only.
 */
package com.drift.plugin.media

import android.content.Context
import android.os.Handler
import android.os.Looper
import androidx.media3.common.AudioAttributes
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.exoplayer.ExoPlayer
import com.drift.runner.DriftCallError
import com.drift.runner.DriftChannel

/**
 * Per-instance audio player state.
 */
private class AudioPlayerInstance(
    val id: Long,
    context: Context,
    private val handler: Handler,
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
    private var positionRunnable: Runnable? = null

    init {
        player.addListener(object : Player.Listener {
            override fun onPlaybackStateChanged(playbackState: Int) {
                val state = when (playbackState) {
                    Player.STATE_IDLE -> 0
                    Player.STATE_BUFFERING -> 1
                    Player.STATE_READY -> if (player.isPlaying) 2 else 4
                    Player.STATE_ENDED -> {
                        stopPositionUpdates()
                        3
                    }
                    else -> 0
                }
                sendStateEvent(state)
            }

            override fun onIsPlayingChanged(isPlaying: Boolean) {
                if (player.playbackState == Player.STATE_READY) {
                    val state = if (isPlaying) 2 else 4 // Playing or Paused
                    sendStateEvent(state)
                    if (isPlaying) startPositionUpdates() else stopPositionUpdates()
                }
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
    }

    fun sendStateEvent(state: Int) {
        send(
            AudioPlayers.EVENTS_CHANNEL,
            mapOf(
                "playerId" to id,
                "playbackState" to state,
                "positionMs" to player.currentPosition,
                "durationMs" to player.duration.coerceAtLeast(0),
                "bufferedMs" to player.bufferedPosition
            )
        )
    }

    fun startPositionUpdates() {
        stopPositionUpdates()
        positionRunnable = object : Runnable {
            override fun run() {
                if (player.playbackState != Player.STATE_IDLE) {
                    send(
                        AudioPlayers.EVENTS_CHANNEL,
                        mapOf(
                            "playerId" to id,
                            "playbackState" to when (player.playbackState) {
                                Player.STATE_IDLE -> 0
                                Player.STATE_BUFFERING -> 1
                                Player.STATE_READY -> if (player.isPlaying) 2 else 4
                                Player.STATE_ENDED -> 3
                                else -> 0
                            },
                            "positionMs" to player.currentPosition,
                            "durationMs" to player.duration.coerceAtLeast(0),
                            "bufferedMs" to player.bufferedPosition
                        )
                    )
                }
                handler.postDelayed(this, 250)
            }
        }
        handler.post(positionRunnable!!)
    }

    fun stopPositionUpdates() {
        positionRunnable?.let { handler.removeCallbacks(it) }
        positionRunnable = null
    }

    fun dispose() {
        stopPositionUpdates()
        player.release()
    }
}

/** The `drift/media/audio` channel's players, keyed by playerId. */
internal class AudioPlayers(
    private val context: Context,
    private val send: (channel: String, data: Map<String, Any?>) -> Unit,
) {
    private val handler = Handler(Looper.getMainLooper())
    private val players = mutableMapOf<Long, AudioPlayerInstance>()

    /** Declares the channel's methods. */
    fun declare(channel: DriftChannel) = with(channel) {
        method("load") { args ->
            val url = (args as? Map<*, *>)?.get("url") as? String
                ?: throw DriftCallError("invalid_arguments", "load needs a url")
            val player = player(args, create = true)!!.player
            player.setMediaItem(MediaItem.fromUri(url))
            player.prepare()
            null
        }
        method("play") { args ->
            val player = player(args, create = true)!!.player
            if (player.playbackState == Player.STATE_IDLE) player.prepare()
            player.play()
            null
        }
        method("pause") { args -> player(args)?.player?.pause(); null }
        method("stop") { args ->
            player(args)?.let {
                it.player.stop()
                it.player.seekTo(0)
                it.sendStateEvent(0) // Idle with position reset to zero
            }
            null
        }
        method("seekTo") { args ->
            player(args)?.player?.seekTo(number(args, "positionMs")?.toLong() ?: 0L)
            null
        }
        method("setVolume") { args ->
            player(args)?.player?.volume = number(args, "volume")?.toFloat() ?: 1.0f
            null
        }
        method("setLooping") { args ->
            val looping = (args as? Map<*, *>)?.get("looping") as? Boolean ?: false
            player(args)?.player?.repeatMode = if (looping) Player.REPEAT_MODE_ALL else Player.REPEAT_MODE_OFF
            null
        }
        method("setPlaybackSpeed") { args ->
            player(args)?.player?.setPlaybackSpeed(number(args, "rate")?.toFloat() ?: 1.0f)
            null
        }
        method("dispose") { args ->
            players.remove(number(args, "playerId")?.toLong() ?: 0L)?.dispose()
            null
        }
    }

    private fun number(args: Any?, key: String) = (args as? Map<*, *>)?.get(key) as? Number

    /** The player args' playerId names, created on first use when create. */
    private fun player(args: Any?, create: Boolean = false): AudioPlayerInstance? {
        val id = number(args, "playerId")?.toLong() ?: 0L
        players[id]?.let { return it }
        if (!create) return null
        return AudioPlayerInstance(id, context, handler, send).also { players[id] = it }
    }

    companion object {
        const val CHANNEL = "drift/media/audio"
        const val EVENTS_CHANNEL = "drift/media/audio/events"
        const val ERRORS_CHANNEL = "drift/media/audio/errors"
    }
}
