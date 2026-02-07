/**
 * NativeAudioPlayer.kt
 * Provides audio-only playback using ExoPlayer via a standalone platform channel.
 */
package {{.PackageName}}

import android.content.Context
import android.os.Handler
import android.os.Looper
import androidx.media3.common.MediaItem
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.exoplayer.ExoPlayer

/**
 * Handles audio player platform channel methods from Go.
 */
object AudioPlayerHandler {
    private var context: Context? = null
    private var player: ExoPlayer? = null
    private val handler = Handler(Looper.getMainLooper())
    private var positionRunnable: Runnable? = null

    fun handle(context: Context, method: String, args: Any?): Pair<Any?, Exception?> {
        if (this.context == null) {
            this.context = context.applicationContext
        }
        val argsMap = args as? Map<*, *>

        return when (method) {
            "load" -> load(argsMap)
            "play" -> play()
            "pause" -> pause()
            "stop" -> stop()
            "seekTo" -> seekTo(argsMap)
            "setVolume" -> setVolume(argsMap)
            "setLooping" -> setLooping(argsMap)
            "setPlaybackSpeed" -> setPlaybackSpeed(argsMap)
            "dispose" -> dispose()
            else -> Pair(null, IllegalArgumentException("Unknown method: $method"))
        }
    }

    private fun ensurePlayer(): ExoPlayer {
        if (player == null) {
            val ctx = context ?: throw IllegalStateException("Context not initialized")
            val newPlayer = ExoPlayer.Builder(ctx).build()

            newPlayer.addListener(object : Player.Listener {
                override fun onPlaybackStateChanged(playbackState: Int) {
                    val state = when (playbackState) {
                        Player.STATE_IDLE -> 0
                        Player.STATE_BUFFERING -> 2
                        Player.STATE_READY -> if (newPlayer.isPlaying) 3 else 6
                        Player.STATE_ENDED -> {
                            stopPositionUpdates()
                            4
                        }
                        else -> 0
                    }
                    sendStateEvent(state)
                }

                override fun onIsPlayingChanged(isPlaying: Boolean) {
                    if (newPlayer.playbackState == Player.STATE_READY) {
                        val state = if (isPlaying) 3 else 6 // Playing or Paused
                        sendStateEvent(state)
                        if (isPlaying) startPositionUpdates() else stopPositionUpdates()
                    }
                }

                override fun onPlayerError(error: PlaybackException) {
                    PlatformChannelManager.sendEvent(
                        "drift/audio_player/errors",
                        mapOf(
                            "code" to error.errorCode.toString(),
                            "message" to (error.message ?: "Unknown playback error")
                        )
                    )
                }
            })

            player = newPlayer
        }
        return player!!
    }

    private fun sendStateEvent(state: Int) {
        val p = player ?: return
        PlatformChannelManager.sendEvent(
            "drift/audio_player/events",
            mapOf(
                "playbackState" to state,
                "positionMs" to p.currentPosition,
                "durationMs" to p.duration.coerceAtLeast(0),
                "bufferedMs" to p.bufferedPosition
            )
        )
    }

    private fun startPositionUpdates() {
        stopPositionUpdates()
        positionRunnable = object : Runnable {
            override fun run() {
                val p = player ?: return
                if (p.playbackState != Player.STATE_IDLE) {
                    PlatformChannelManager.sendEvent(
                        "drift/audio_player/events",
                        mapOf(
                            "playbackState" to when (p.playbackState) {
                                Player.STATE_IDLE -> 0
                                Player.STATE_BUFFERING -> 2
                                Player.STATE_READY -> if (p.isPlaying) 3 else 6
                                Player.STATE_ENDED -> 4
                                else -> 0
                            },
                            "positionMs" to p.currentPosition,
                            "durationMs" to p.duration.coerceAtLeast(0),
                            "bufferedMs" to p.bufferedPosition
                        )
                    )
                }
                handler.postDelayed(this, 250)
            }
        }
        handler.post(positionRunnable!!)
    }

    private fun stopPositionUpdates() {
        positionRunnable?.let { handler.removeCallbacks(it) }
        positionRunnable = null
    }

    private fun load(args: Map<*, *>?): Pair<Any?, Exception?> {
        val url = args?.get("url") as? String
            ?: return Pair(null, IllegalArgumentException("Missing url"))

        handler.post {
            val p = ensurePlayer()
            val mediaItem = MediaItem.fromUri(url)
            p.setMediaItem(mediaItem)
            p.prepare()
        }
        return Pair(null, null)
    }

    private fun play(): Pair<Any?, Exception?> {
        handler.post {
            ensurePlayer().play()
        }
        return Pair(null, null)
    }

    private fun pause(): Pair<Any?, Exception?> {
        handler.post {
            player?.pause()
        }
        return Pair(null, null)
    }

    private fun stop(): Pair<Any?, Exception?> {
        handler.post {
            player?.stop()
        }
        return Pair(null, null)
    }

    private fun seekTo(args: Map<*, *>?): Pair<Any?, Exception?> {
        val positionMs = (args?.get("positionMs") as? Number)?.toLong() ?: 0L
        handler.post {
            player?.seekTo(positionMs)
        }
        return Pair(null, null)
    }

    private fun setVolume(args: Map<*, *>?): Pair<Any?, Exception?> {
        val volume = (args?.get("volume") as? Number)?.toFloat() ?: 1.0f
        handler.post {
            player?.volume = volume
        }
        return Pair(null, null)
    }

    private fun setLooping(args: Map<*, *>?): Pair<Any?, Exception?> {
        val looping = args?.get("looping") as? Boolean ?: false
        handler.post {
            player?.repeatMode = if (looping) Player.REPEAT_MODE_ALL else Player.REPEAT_MODE_OFF
        }
        return Pair(null, null)
    }

    private fun setPlaybackSpeed(args: Map<*, *>?): Pair<Any?, Exception?> {
        val rate = (args?.get("rate") as? Number)?.toFloat() ?: 1.0f
        handler.post {
            player?.setPlaybackSpeed(rate)
        }
        return Pair(null, null)
    }

    private fun dispose(): Pair<Any?, Exception?> {
        handler.post {
            stopPositionUpdates()
            player?.release()
            player = null
        }
        return Pair(null, null)
    }
}
