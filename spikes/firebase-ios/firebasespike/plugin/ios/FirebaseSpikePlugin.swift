// Spike: compiled into the app target (variant A) or into its own SwiftPM
// target (variant B, moved there by run.sh). Referencing both Firebase types
// forces the linker to resolve them, not just the compiler.

import FirebaseCore
import FirebaseMessaging

public enum FirebaseSpikePlugin {
    public static func register(host: AnyObject) {
        let types: [Any.Type] = [FirebaseApp.self, Messaging.self]
        NSLog("firebase spike linked: %@", String(describing: types))
    }
}
