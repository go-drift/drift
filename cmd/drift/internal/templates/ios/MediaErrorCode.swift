/// MediaErrorCode.swift
/// Shared AVPlayer error-code mapping used by both audio and video players.

import AVFoundation

/// Maps an AVPlayer error to a canonical Drift error code string.
/// Aligns with the Android ExoPlayer mapping so that both platforms
/// produce the same set of codes: "source_error", "decoder_error",
/// "playback_failed".
func mediaErrorCode(for error: Error?) -> String {
    guard let error = error else { return "playback_failed" }

    if let avError = error as? AVError {
        switch avError.code {
        case .decoderNotFound, .decoderTemporarilyUnavailable,
             .contentIsNotAuthorized:
            return "decoder_error"
        case .fileFormatNotRecognized, .failedToParse:
            return "source_error"
        default:
            break
        }
    }

    if (error as NSError).domain == NSURLErrorDomain {
        return "source_error"
    }

    return "playback_failed"
}
