/**
 * VideoPlayerView.kt
 *
 * The `drift/media/video` platform view: ExoPlayer playback in a media3
 * PlayerView, with its native controls. Events go to the Go view
 * (runtime/video.go).
 */
package com.drift.plugin.media

import com.drift.runner.DriftPlatformView
import com.drift.runner.DriftCallError
import com.drift.runner.DriftPlatformViewEvents
import android.content.Context
import android.os.Handler
import android.os.Looper
import android.view.TextureView
import android.view.View
import android.view.ViewGroup
import android.widget.FrameLayout
import androidx.media3.common.AudioAttributes
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.ui.PlayerView

/**
 * Platform view container for native video player using ExoPlayer.
 *
 * Wraps a PlayerView inside a FrameLayout and replaces the default SurfaceView
 * with a TextureView. The player, and so playback, lives as long as the
 * view; the PlayerView, the controls' UI, is rebuilt when Drift moves the
 * view to another Activity (onHostChanged), since its controls keep a popup
 * window tied to the Activity they were created in. TextureView integrates correctly with Drift's clipping
 * (View.clipBounds) and avoids z-ordering issues and black flashes on resize
 * that SurfaceView causes in a platform view context.
 */
class VideoPlayerView(
    context: Context,
    params: Map<String, Any?>,
    private val events: DriftPlatformViewEvents,
) : DriftPlatformView {

    private val container = FrameLayout(context)
    override val view: View get() = container
    override val supportsRegionMask: Boolean get() = true
    private lateinit var playerView: PlayerView
    private val player: ExoPlayer
    private val main = Handler(Looper.getMainLooper())
    private var showControls = true

    init {
        player = ExoPlayer.Builder(context).build().also {
            it.setAudioAttributes(
                AudioAttributes.Builder()
                    .setUsage(C.USAGE_MEDIA)
                    .setContentType(C.AUDIO_CONTENT_TYPE_MUSIC)
                    .build(),
                /* handleAudioFocus= */ true
            )
        }

        buildPlayerView()

        // Configure player from params
        val url = params["url"] as? String
        val autoPlay = params["autoPlay"] as? Boolean ?: false
        val looping = params["looping"] as? Boolean ?: false
        val volume = (params["volume"] as? Number)?.toFloat() ?: 1.0f

        player.repeatMode = if (looping) Player.REPEAT_MODE_ALL else Player.REPEAT_MODE_OFF
        player.volume = volume

        // Add listener for state and position events
        player.addListener(object : Player.Listener {
            override fun onPlaybackStateChanged(playbackState: Int) {
                val state = when (playbackState) {
                    Player.STATE_IDLE -> 0
                    Player.STATE_BUFFERING -> 1
                    Player.STATE_READY -> if (player.isPlaying) 2 else 4 // Playing or Paused
                    Player.STATE_ENDED -> 3
                    else -> 0
                }
                if (playbackState == Player.STATE_IDLE || playbackState == Player.STATE_ENDED) {
                    stopPositionUpdates()
                }
                events.send("onPlaybackStateChanged", mapOf("state" to state))
            }

            override fun onIsPlayingChanged(isPlaying: Boolean) {
                if (player.playbackState == Player.STATE_READY) {
                    val state = if (isPlaying) 2 else 4 // Playing or Paused
                    events.send("onPlaybackStateChanged", mapOf("state" to state))
                    if (isPlaying) startPositionUpdates() else stopPositionUpdates()
                }
            }

            override fun onPlayerError(error: PlaybackException) {
                events.send("onVideoError", mapOf(
                    "code" to mediaErrorCodeString(error.errorCode),
                    "message" to (error.message ?: "Unknown playback error"),
                ))
            }
        })

        // Load media if URL provided
        if (url != null && url.isNotEmpty()) {
            val mediaItem = MediaItem.fromUri(url)
            player.setMediaItem(mediaItem)
            player.prepare()
            if (autoPlay) {
                player.playWhenReady = true
            }
        }

    }

    /**
     * Builds the PlayerView in the container's context (the hosting
     * Activity), replacing any earlier one.
     */
    private fun buildPlayerView() {
        if (::playerView.isInitialized) {
            playerView.player = null
            container.removeView(playerView)
        }
        playerView = PlayerView(container.context).apply {
            layoutParams = FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.MATCH_PARENT,
                FrameLayout.LayoutParams.MATCH_PARENT
            )
            this.player = this@VideoPlayerView.player
            controllerAutoShow = true
            useController = showControls
        }
        // Replace the default SurfaceView with a TextureView.
        // PlayerView uses SurfaceView by default, which does not respect
        // View.clipBounds and causes z-ordering issues in platform views.
        replaceWithTextureView(playerView)
        container.addView(playerView)
    }

    override fun onHostChanged() {
        buildPlayerView()
    }

    /**
     * Replaces the default SurfaceView inside PlayerView with a TextureView.
     * PlayerView renders video into the first SurfaceView/TextureView child it finds
     * in its subtree. By swapping the SurfaceView for a TextureView at the same
     * position and size, the player renders through TextureView instead.
     *
     * Searches recursively since some PlayerView versions nest the SurfaceView
     * inside child ViewGroups.
     */
    private fun replaceWithTextureView(playerView: PlayerView) {
        val surfaceView = findSurfaceView(playerView) ?: return
        val parent = surfaceView.parent as? ViewGroup ?: return
        val index = parent.indexOfChild(surfaceView)
        val params = surfaceView.layoutParams
        parent.removeViewAt(index)
        val textureView = TextureView(playerView.context)
        textureView.layoutParams = params
        parent.addView(textureView, index)

        // Connect the TextureView to the player via the PlayerView's
        // video output mechanism
        player.setVideoTextureView(textureView)
    }

    /**
     * Recursively searches a ViewGroup for the first SurfaceView child.
     */
    private fun findSurfaceView(group: ViewGroup): android.view.SurfaceView? {
        for (i in 0 until group.childCount) {
            val child = group.getChildAt(i)
            if (child is android.view.SurfaceView) return child
            if (child is ViewGroup) {
                val found = findSurfaceView(child)
                if (found != null) return found
            }
        }
        return null
    }

    private var positionRunnable: Runnable? = null

    private fun startPositionUpdates() {
        stopPositionUpdates()
        positionRunnable = object : Runnable {
            override fun run() {
                if (player.playbackState != Player.STATE_IDLE) {
                    events.send("onPositionChanged", mapOf(
                        "positionMs" to player.currentPosition,
                        "durationMs" to player.duration.coerceAtLeast(0),
                        "bufferedMs" to player.bufferedPosition,
                    ))
                }
                main.postDelayed(this, 250)
            }
        }
        main.post(positionRunnable!!)
    }

    private fun stopPositionUpdates() {
        positionRunnable?.let { main.removeCallbacks(it) }
        positionRunnable = null
    }

    override fun handle(method: String, args: Map<String, Any?>) {
        when (method) {
            "play" -> play()
            "pause" -> pause()
            "stop" -> stop()
            "seekTo" -> seekTo((args["positionMs"] as? Number)?.toLong() ?: 0L)
            "setVolume" -> setVolume((args["volume"] as? Number)?.toFloat() ?: 1.0f)
            "setLooping" -> setLooping(args["looping"] as? Boolean ?: false)
            "setPlaybackSpeed" -> setPlaybackSpeed((args["rate"] as? Number)?.toFloat() ?: 1.0f)
            "setShowControls" -> setShowControls(args["show"] as? Boolean ?: true)
            "load" -> (args["url"] as? String)?.let { load(it) }
            else -> throw DriftCallError("method_not_found", "unknown video method $method")
        }
    }

    override fun dispose() {
        stopPositionUpdates()
        player.release()
    }

    fun play() {
        if (player.playbackState == Player.STATE_IDLE) {
            player.prepare()
        }
        player.play()
    }

    fun pause() {
        player.pause()
    }

    fun stop() {
        player.stop()
        player.seekTo(0)
        events.send("onPlaybackStateChanged", mapOf("state" to 0)) // Idle, position reset to zero
    }

    fun seekTo(positionMs: Long) {
        player.seekTo(positionMs)
    }

    fun setVolume(volume: Float) {
        player.volume = volume
    }

    fun setLooping(looping: Boolean) {
        player.repeatMode = if (looping) Player.REPEAT_MODE_ALL else Player.REPEAT_MODE_OFF
    }

    fun setPlaybackSpeed(rate: Float) {
        player.setPlaybackSpeed(rate)
    }

    fun setShowControls(show: Boolean) {
        showControls = show
        playerView.useController = show
    }

    fun load(url: String) {
        val mediaItem = MediaItem.fromUri(url)
        player.setMediaItem(mediaItem)
        player.prepare()
    }
}
