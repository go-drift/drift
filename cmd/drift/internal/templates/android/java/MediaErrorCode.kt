/**
 * MediaErrorCode.kt
 * Shared ExoPlayer error-code mapping used by both audio and video players.
 */
package {{.PackageName}}

/**
 * Maps an ExoPlayer error code to a canonical Drift error code string.
 * Source/IO/parsing errors (2000-3999) become "source_error",
 * decoder/audio-track/DRM errors (4000-6999) become "decoder_error",
 * and everything else becomes "playback_failed".
 */
internal fun mediaErrorCodeString(code: Int): String = when {
    code in 2000..3999 -> "source_error"
    code in 4000..6999 -> "decoder_error"
    else -> "playback_failed"
}
