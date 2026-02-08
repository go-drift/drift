/// DriftMediaSession.swift
/// Shared AVAudioSession lifecycle management for audio and video players.
///
/// Both NativeAudioPlayer and NativeVideoPlayer call activate() on init
/// and deactivate() on dispose. The underlying AVAudioSession is only
/// deactivated when no media players remain active.

import AVFoundation

enum DriftMediaSession {
    private static var activeCount = 0

    /// Activates the audio session if not already active.
    static func activate() {
        activeCount += 1
        if activeCount == 1 {
            do {
                try AVAudioSession.sharedInstance().setCategory(.playback)
                try AVAudioSession.sharedInstance().setActive(true)
            } catch {
                print("[drift] AVAudioSession setup failed: \(error)")
            }
        }
    }

    /// Decrements the active count and deactivates the audio session
    /// when no media players remain.
    static func deactivate() {
        activeCount = max(activeCount - 1, 0)
        if activeCount == 0 {
            try? AVAudioSession.sharedInstance().setActive(false, options: .notifyOthersOnDeactivation)
        }
    }
}
