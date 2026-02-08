/**
 * NativeVideoPlayer.kt
 * Provides native ExoPlayer video playback embedded in Drift UI.
 */
package {{.PackageName}}

import android.content.Context
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
 * with a TextureView. TextureView integrates correctly with Drift's clipping
 * (View.clipBounds) and avoids z-ordering issues and black flashes on resize
 * that SurfaceView causes in a platform view context.
 */
class NativeVideoPlayerContainer(
    context: Context,
    override val viewId: Int,
    params: Map<String, Any?>
) : PlatformViewContainer {

    override val view: View
    private val playerView: PlayerView
    private val player: ExoPlayer

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

        playerView = PlayerView(context).apply {
            layoutParams = FrameLayout.LayoutParams(
                FrameLayout.LayoutParams.MATCH_PARENT,
                FrameLayout.LayoutParams.MATCH_PARENT
            )
            this.player = this@NativeVideoPlayerContainer.player
            controllerAutoShow = true
        }

        // Replace the default SurfaceView with a TextureView.
        // PlayerView uses SurfaceView by default, which does not respect
        // View.clipBounds and causes z-ordering issues in platform views.
        replaceWithTextureView(playerView)

        view = playerView

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
                PlatformChannelManager.sendEvent(
                    "drift/platform_views",
                    mapOf(
                        "method" to "onPlaybackStateChanged",
                        "viewId" to viewId,
                        "state" to state
                    )
                )
            }

            override fun onIsPlayingChanged(isPlaying: Boolean) {
                if (player.playbackState == Player.STATE_READY) {
                    val state = if (isPlaying) 2 else 4 // Playing or Paused
                    PlatformChannelManager.sendEvent(
                        "drift/platform_views",
                        mapOf(
                            "method" to "onPlaybackStateChanged",
                            "viewId" to viewId,
                            "state" to state
                        )
                    )
                    if (isPlaying) startPositionUpdates() else stopPositionUpdates()
                }
            }

            override fun onPlayerError(error: PlaybackException) {
                PlatformChannelManager.sendEvent(
                    "drift/platform_views",
                    mapOf(
                        "method" to "onVideoError",
                        "viewId" to viewId,
                        "code" to mediaErrorCodeString(error.errorCode),
                        "message" to (error.message ?: "Unknown playback error")
                    )
                )
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
                    PlatformChannelManager.sendEvent(
                        "drift/platform_views",
                        mapOf(
                            "method" to "onPositionChanged",
                            "viewId" to viewId,
                            "positionMs" to player.currentPosition,
                            "durationMs" to player.duration.coerceAtLeast(0),
                            "bufferedMs" to player.bufferedPosition
                        )
                    )
                }
                playerView.postDelayed(this, 250)
            }
        }
        playerView.post(positionRunnable!!)
    }

    private fun stopPositionUpdates() {
        positionRunnable?.let { playerView.removeCallbacks(it) }
        positionRunnable = null
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
        PlatformChannelManager.sendEvent(
            "drift/platform_views",
            mapOf(
                "method" to "onPlaybackStateChanged",
                "viewId" to viewId,
                "state" to 0 // Idle with position reset to zero
            )
        )
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

    fun load(url: String) {
        val mediaItem = MediaItem.fromUri(url)
        player.setMediaItem(mediaItem)
        player.prepare()
    }
}
