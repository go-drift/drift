/**
 * NativeAudioPlayer.kt
 * Provides audio-only playback using ExoPlayer via a standalone platform channel.
 * Supports multiple concurrent player instances, each identified by a playerId.
 */
package {{.PackageName}}

import android.content.Context
import android.os.Handler
import android.os.Looper
import androidx.media3.common.AudioAttributes
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.exoplayer.ExoPlayer

/**
 * Per-instance audio player state.
 */
private class AudioPlayerInstance(
    val id: Long,
    context: Context,
    private val handler: Handler
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
                PlatformChannelManager.sendEvent(
                    "drift/audio_player/errors",
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
        PlatformChannelManager.sendEvent(
            "drift/audio_player/events",
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
                    PlatformChannelManager.sendEvent(
                        "drift/audio_player/events",
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

/**
 * Handles audio player platform channel methods from Go.
 * Manages multiple player instances keyed by playerId.
 */
object AudioPlayerHandler {
    private var context: Context? = null
    private val handler = Handler(Looper.getMainLooper())
    private val players = mutableMapOf<Long, AudioPlayerInstance>()

    fun handle(context: Context, method: String, args: Any?): Pair<Any?, Exception?> {
        if (this.context == null) {
            this.context = context.applicationContext
        }
        val argsMap = args as? Map<*, *>
        val playerId = (argsMap?.get("playerId") as? Number)?.toLong() ?: 0L

        return when (method) {
            "load" -> load(playerId, argsMap)
            "play" -> play(playerId)
            "pause" -> pause(playerId)
            "stop" -> stop(playerId)
            "seekTo" -> seekTo(playerId, argsMap)
            "setVolume" -> setVolume(playerId, argsMap)
            "setLooping" -> setLooping(playerId, argsMap)
            "setPlaybackSpeed" -> setPlaybackSpeed(playerId, argsMap)
            "dispose" -> dispose(playerId)
            else -> Pair(null, IllegalArgumentException("Unknown method: $method"))
        }
    }

    private fun ensurePlayer(playerId: Long): AudioPlayerInstance {
        return players.getOrPut(playerId) {
            val ctx = context ?: throw IllegalStateException("Context not initialized")
            AudioPlayerInstance(playerId, ctx, handler)
        }
    }

    private fun load(playerId: Long, args: Map<*, *>?): Pair<Any?, Exception?> {
        val url = args?.get("url") as? String
            ?: return Pair(null, IllegalArgumentException("Missing url"))

        handler.post {
            val instance = ensurePlayer(playerId)
            val mediaItem = MediaItem.fromUri(url)
            instance.player.setMediaItem(mediaItem)
            instance.player.prepare()
        }
        return Pair(null, null)
    }

    private fun play(playerId: Long): Pair<Any?, Exception?> {
        handler.post {
            val player = ensurePlayer(playerId).player
            if (player.playbackState == Player.STATE_IDLE) {
                player.prepare()
            }
            player.play()
        }
        return Pair(null, null)
    }

    private fun pause(playerId: Long): Pair<Any?, Exception?> {
        handler.post {
            players[playerId]?.player?.pause()
        }
        return Pair(null, null)
    }

    private fun stop(playerId: Long): Pair<Any?, Exception?> {
        handler.post {
            val instance = players[playerId] ?: return@post
            instance.player.stop()
            instance.player.seekTo(0)
            instance.sendStateEvent(0) // Idle with position reset to zero
        }
        return Pair(null, null)
    }

    private fun seekTo(playerId: Long, args: Map<*, *>?): Pair<Any?, Exception?> {
        val positionMs = (args?.get("positionMs") as? Number)?.toLong() ?: 0L
        handler.post {
            players[playerId]?.player?.seekTo(positionMs)
        }
        return Pair(null, null)
    }

    private fun setVolume(playerId: Long, args: Map<*, *>?): Pair<Any?, Exception?> {
        val volume = (args?.get("volume") as? Number)?.toFloat() ?: 1.0f
        handler.post {
            players[playerId]?.player?.volume = volume
        }
        return Pair(null, null)
    }

    private fun setLooping(playerId: Long, args: Map<*, *>?): Pair<Any?, Exception?> {
        val looping = args?.get("looping") as? Boolean ?: false
        handler.post {
            players[playerId]?.player?.repeatMode = if (looping) Player.REPEAT_MODE_ALL else Player.REPEAT_MODE_OFF
        }
        return Pair(null, null)
    }

    private fun setPlaybackSpeed(playerId: Long, args: Map<*, *>?): Pair<Any?, Exception?> {
        val rate = (args?.get("rate") as? Number)?.toFloat() ?: 1.0f
        handler.post {
            players[playerId]?.player?.setPlaybackSpeed(rate)
        }
        return Pair(null, null)
    }

    private fun dispose(playerId: Long): Pair<Any?, Exception?> {
        handler.post {
            players.remove(playerId)?.dispose()
        }
        return Pair(null, null)
    }
}
