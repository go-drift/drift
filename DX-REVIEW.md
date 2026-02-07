# DX Review: `feat/media-player` Branch

## 1. API Asymmetry: `Play(url)` vs `LoadURL` + `Play()`

Audio's `Play(url string)` combines loading and playback in one call, with an internal URL cache so repeat calls with the same URL just resume. Video splits this into `LoadURL(url)` and `Play()`.

A developer who learns the audio API will reach for `controller.Play(url)` on video and find it doesn't exist. The two APIs teach different mental models for the same underlying operation.

### Decision: Adopt `Load(url)` + `Play()` on both

Both audio and video should use a consistent two-step model:

- **`Load(url)`**: prepare the source, buffer, resolve metadata. No playback starts.
- **`Play()`**: start or resume playback. Parameterless.
- **`Pause()`**: pause playback.
- **`Stop()`**: stop playback, reset to idle.

Rationale:
- `Load` is unambiguous: "prepare but don't start." `Play(url)` misleadingly implies playback begins.
- Parameterless `Play()` works for both initial start and resume after pause, so no `Resume()` verb is needed.
- Transport controls (`Play`/`Pause`/`Stop`) form a clean group with no parameters.
- Enables pre-buffering (e.g. loading the next track while the current one plays).
- Removes the implicit URL cache and the surprising `Stop()` side-effect of clearing it.
- Matches Flutter's official `video_player` pattern (source set via constructor/initialize, parameterless `play()`).

### Implementation plan

**Audio (`AudioPlayerController`)**
1. Rename `Play(url string)` to `Load(url string)`. This method sends the native `load` command only (no `play`).
2. Add `Play()` (parameterless) that sends the native `play` command.
3. Remove the `loadedURL` cache field and the conditional-load logic in the current `Play`.
4. Update `Stop()` to no longer clear `loadedURL` (field is removed).
5. Update tests, showcase, and documentation.

**Video (`VideoPlayerView`)**
1. Rename `LoadURL(url string)` to `Load(url string)` for consistency with audio.
2. `Play()` already exists and is parameterless; no change needed.
3. Update tests, showcase, and documentation.

**Documentation**
1. Fix the example bug (item 2 below) at the same time.
2. Update the guide to show the `Load` / `Play` two-step pattern for both audio and video.
3. Add a note explaining that `Play()` after `Stop()` resumes the previously loaded source (native player retains the media item).

## 2. Documentation Bug (media-player.md:165)

The `OnPositionChanged` example references an out-of-scope variable:

```go
s.controller.OnPositionChanged = func(position, duration, buffered time.Duration) {
    s.status.Set(state.String() + " " + position.String() + " / " + duration.String())
}
```

`state` is the parameter name from the `OnPlaybackStateChanged` closure above; it's not in scope here. This won't compile. Likely should be:

```go
s.status.Set(position.String() + " / " + duration.String())
```

## 3. `Stop()` Resets the URL Cache on Audio, but Not Clearly on Video

`AudioPlayerController.Stop()` clears `loadedURL`, so a subsequent `Play(url)` re-sends the `load` command. This is documented in the godoc: "A subsequent call to Play will reload the URL."

On the video side, `Stop()` just sends `"stop"` to native. There's no URL cache to clear because loading is explicit via `LoadURL`. The behavioral difference is subtle: after `Stop()`, an audio controller needs a URL to play again, while a video view can just call `Play()` (the native player still has the media item). Worth a line in the guide calling out the distinction.

**Note:** Resolved by item 1. The `Load`/`Play` split removes the URL cache entirely, making `Stop()` behaviour symmetric across audio and video.

## 4. No `Dispose()` on `VideoPlayerView`

`VideoPlayerView.Dispose()` is a no-op, with cleanup delegated to the platform view registry. By contrast, `AudioPlayerController.Dispose()` is explicit and mandatory. This is architecturally correct (the widget state lifecycle handles video cleanup), but it means audio controllers are a resource leak risk if developers forget `Dispose()`.

Consider whether `AudioPlayerController` could integrate with the widget lifecycle more directly, or whether a finalizer/leak-detection log in debug mode would be worthwhile.

## 5. Error Code Granularity Differs Between Platforms

The canonical error codes are `source_error`, `decoder_error`, and `playback_failed`. Android maps ExoPlayer error ranges to all three, but iOS only maps `NSURLErrorDomain` to `source_error` and everything else to `playback_failed`. iOS never produces `decoder_error`.

This means error handling code like the following will silently behave differently per platform:

```go
switch code {
case "decoder_error":
    // never fires on iOS
}
```

Either document this, or improve the iOS mapping to classify `AVError` decoder-related codes.

## 6. Callback Timing and Missed Events

Callbacks are optional fields set after construction. There's a window between `NewAudioPlayerController()` and assigning `OnPlaybackStateChanged` where events could arrive and be dropped. In practice this probably never happens since `Play()` hasn't been called yet, but the godoc should note that callbacks should be set before calling any playback methods.

## 7. Minor Nits

- **`PlaybackState.String()`** returns `"Unknown"` for out-of-range values. Since native code controls the int values, an unknown state likely signals a protocol mismatch. Logging/reporting this instead of silently returning "Unknown" could help debug version skew issues.

- **`parseString` / `toInt64` / `toFloat64` helpers** silently return zero values on type assertion failure. A malformed native event will be silently ignored. This is fine for production robustness, but a debug-mode log would help catch integration issues.

- **Showcase readability**: the showcase file at 286 lines combines both audio and video demos with full transport controls. Splitting it into two separate showcase entries would make each one a clearer standalone example.
