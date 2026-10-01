# Firebase demo

Push notifications through `plugins/firebase`. The app shows its FCM token
(also logged as `FCM token: ...`, and copyable), asks for notification
permission, and lists the messages it receives and the notifications the user
taps. Every event is also logged (`FCM received ...`, `FCM tapped ...`).

## Firebase setup (once)

1. In the [Firebase console](https://console.firebase.google.com), add a
   project (Google Analytics is not needed).
2. Add an iOS app with bundle ID `com.example.driftfirebasedemo` (drift.yaml
   `app.id`). Download `GoogleService-Info.plist` into `firebase/`.
3. Add an Android app with package name `com.example.driftfirebasedemo`.
   Download `google-services.json` into `firebase/`.
4. iOS push: in the Apple Developer portal, Certificates, IDs & Profiles,
   Keys, add a key with Apple Push Notifications service (APNs). Download the
   `.p8` file and note its Key ID and your Team ID. In Firebase, Project
   settings, Cloud Messaging, Apple app configuration, upload it.
5. Sending test messages: in Firebase, Project settings, Service accounts,
   Generate new private key. Keep the JSON outside the repository, e.g.
   `~/.config/drift/fcm-sa.json`.

`firebase/` is gitignored: the files name your Firebase project.

## Run

```sh
../../bin/drift run android   # or: drift build ios, then run from Xcode
```

On iOS, pick your team under Runner, Signing & Capabilities; the Push
Notifications capability comes from the entitlements the plugin adds.

## Send test messages

```sh
go run ./tools/fcmsend -key ~/.config/drift/fcm-sa.json -token <FCM token> \
    -title Hello -body World -data route=/inbox      # notification message
go run ./tools/fcmsend -key ~/.config/drift/fcm-sa.json -token <FCM token> \
    -data-only -data k=v                             # data message
```

- App in the foreground: both arrive under "Messages and taps" as received
  messages; nothing is shown by the system.
- App in the background: the notification message appears in the
  notification shade; tapping it opens the app and lists it as tapped. A data
  message is listed as received (foreground=false) while the process lives.
- App not running: swipe it away from Recents (on Android, not Force stop,
  which blocks FCM until the next launch), send a notification message, tap
  it. The app starts and lists the tap.
