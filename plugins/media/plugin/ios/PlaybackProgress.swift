/// PlaybackProgress.swift
///
/// A player's progress (the runtime package's Progress) and the ticker that
/// reports it to Go while media is loaded, for audio players and video
/// views alike. Main thread only.

import AVFoundation

/// Position, duration and buffered length, in milliseconds.
struct PlaybackProgress: Equatable {
    let positionMs: Int64
    let durationMs: Int64
    let bufferedMs: Int64

    /// player's progress, at position instead of its current time when
    /// given (a seek just requested).
    init(_ player: AVPlayer, at position: CMTime? = nil) {
        let time = position ?? player.currentTime()
        positionMs = Self.ms(time)
        var durationMs: Int64 = 0
        var bufferedMs: Int64 = 0
        if let item = player.currentItem {
            durationMs = Self.ms(item.duration)
            if let range = item.loadedTimeRanges.last?.timeRangeValue {
                bufferedMs = Self.ms(CMTimeAdd(range.start, range.duration))
            }
        }
        self.durationMs = durationMs
        self.bufferedMs = bufferedMs
    }

    /// The event fields Go reads (runtime/audio.go, runtime/video.go).
    var fields: [String: Any] {
        ["positionMs": positionMs, "durationMs": durationMs, "bufferedMs": bufferedMs]
    }

    /// time in milliseconds; 0 when it is not a number (an unknown
    /// duration) or negative.
    private static func ms(_ time: CMTime) -> Int64 {
        time.isNumeric ? max(Int64(CMTimeGetSeconds(time) * 1000), 0) : 0
    }
}

/// Reports a player's progress about every 250ms while media is loaded,
/// playing or not, skipping ticks that change nothing, and at once on
/// request (a state change, a seek).
final class ProgressTicker {
    private let player: AVPlayer
    private let report: (PlaybackProgress) -> Void
    private var timer: Timer?
    private var last: PlaybackProgress?

    init(_ player: AVPlayer, report: @escaping (PlaybackProgress) -> Void) {
        self.player = player
        self.report = report
    }

    /// Starts the ticks, for media just loaded; idempotent.
    func start() {
        guard timer == nil else { return }
        let timer = Timer(timeInterval: 0.25, repeats: true) { [weak self] _ in
            guard let self else { return }
            let progress = PlaybackProgress(self.player)
            if progress != self.last {
                self.send(progress)
            }
        }
        // .common: ticks go on while the user scrolls.
        RunLoop.main.add(timer, forMode: .common)
        self.timer = timer
    }

    /// Stops the ticks, for a player disposed.
    func stop() {
        timer?.invalidate()
        timer = nil
    }

    /// Reports progress now, at position instead of the current time when
    /// given (a seek just requested), changed or not.
    func reportNow(at position: CMTime? = nil) {
        send(PlaybackProgress(player, at: position))
    }

    private func send(_ progress: PlaybackProgress) {
        last = progress
        report(progress)
    }
}
