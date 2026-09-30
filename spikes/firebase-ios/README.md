# Firebase iOS packaging spike

Throwaway. Decides phase 1 item 6 of `docs/plugins-v1-plan.md`: can plugin
Swift compiled into the app target import SwiftPM products that reach the app
only through the `Drift/Plugins` sidecar (variant A), or do plugins need their
own SwiftPM targets (variant B)?

Run on macOS with Xcode 16+ and network access:

```sh
make cli
./spikes/firebase-ios/run.sh 2>&1 | tee /tmp/firebase-spike.log
cat spikes/firebase-ios/out/summary.txt
```

Send back `out/summary.txt` and any `out/*.errors.txt`. See `run.sh` for what
each variant does.
