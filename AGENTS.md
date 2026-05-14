## Build Commands
```bash
make cli                # Build drift CLI
go mod tidy             # Sync dependencies
gofmt -w .              # Format code
go vet ./...            # Lint
```

## Testing
```bash
go test ./...                              
DRIFT_UPDATE_SNAPSHOTS=1 go test ./pkg/...  # Update snapshot tests
```

## Documentation
```bash
go run cmd/docgen/main.go # Generate Docusaurus docs
```

## Project Structure
- `cmd/drift/` - CLI tool (build, run, clean, init, devices, log)
- `cmd/docgen/` - Documentation generator
- `pkg/` - Core framework packages
- `showcase/` - Demo application
- `scripts/` - Skia build scripts
- `third_party/` - Skia source and prebuilt binaries
- `website-docs/guides/` - Website guides source files
- `website/` - Docusaurus website config

## CGO and Platform
- CGO bridges Go to Skia (C++)
- Android: Kotlin embedder, JNI bridge
- iOS: Swift embedder, Metal rendering
- Keep bridge functions thin; delegate logic to Go

## Toolchain Requirements
- **Xcode 16+** for iOS builds: the iOS template ships in project format 77 (`PBXFileSystemSynchronizedRootGroup`). `drift build ios` preflight aborts on older Xcode with `Drift requires Xcode 16+`. xtool builds are unaffected.
- Plugin runner support files live under `app/src/main/java/com/drift/runner/` (Android) and `Runner/` (iOS). The package id `com.drift.runner` is reserved; `validateAppID` rejects user app ids that collide.

## Platform Native Code
When adding iOS or Android native code, ensure these files in `cmd/drift/internal/templates/` are updated:
- **iOS**: Info.plist, xcodeproj, and xtool project templates
- **Android**: AndroidManifest.xml and project templates
When editing iOS templates, update both `ios/Info.plist.tmpl` and `xtool/Info.plist.tmpl` (and corresponding xcodeproj/xtool templates) in the same change.
