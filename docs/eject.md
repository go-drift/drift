# Plan: Template Customization for Drift

## Overview

Allow users to "eject" iOS/Android projects to their repository for full customization. After ejecting, the platform directory contains a **real project** (not templates) that can be opened in Xcode/Android Studio.

## Design Decisions

### 1. Command: `drift eject <platform>`
```bash
drift eject ios        # Eject iOS project
drift eject android    # Eject Android project
drift eject all        # Eject both platforms
```

**Idempotency and overwrite behavior:**
- If `platform/<platform>/` already exists, `drift eject` **errors** with a message:
  ```
  Error: platform/ios/ already exists. Use --force to overwrite (creates backup).
  ```
- `drift eject --force <platform>` moves existing directory to `platform/<platform>.backup.YYYYMMDD-HHMMSS-NNN/` before ejecting
- Timestamp format uses `YYYYMMDD-HHMMSS-NNN` where NNN is a counter (e.g., `platform/ios.backup.20260202-143052-001/`)
- The counter prevents collisions if multiple `--force` runs happen within the same second
- Counter starts at 001 and increments until an unused name is found

**`eject all` behavior when one platform exists:**
- `drift eject all` **fails fast** if any platform directory exists, listing all conflicts:
  ```
  Error: Cannot eject all platforms. Existing directories:
    - platform/ios/
    - platform/android/
  Use --force to backup and overwrite, or eject platforms individually.
  ```
  (If only one exists, only that one is listed.)
- `drift eject --force all` backs up existing platforms and ejects both:
  - If `platform/ios/` exists: back it up, then eject iOS
  - If `platform/ios/` doesn't exist: eject iOS directly (no backup needed)
  - Same logic for Android
  - Success message clarifies what happened for each platform:
    ```
    Ejected iOS (backed up existing to platform/ios.backup.20260202-143052-001/)
    Ejected Android (created new)
    ```
- Rationale: Partial eject (ejecting only the non-existing platform) would be confusing and inconsistent. Users who want that can run `drift eject android` explicitly.

### 2. Ejected Structure: Real projects, not templates
```
myapp/
├── drift.yaml
├── main.go
└── platform/
    ├── ios/                      # Real Xcode project
    │   ├── Runner/
    │   │   ├── Info.plist        # Values substituted (no {{.BundleID}})
    │   │   ├── AppDelegate.swift
    │   │   └── ...
    │   ├── Runner.xcodeproj/
    │   │   └── project.pbxproj
    │   ├── bridge/               # Drift-owned, regenerated on build
    │   │   └── DriftBridge.swift
    │   └── driftw                # Wrapper script for IDE builds (chmod 0755)
    └── android/                  # Real Android project
        ├── app/
        │   ├── build.gradle
        │   └── src/main/
        │       ├── AndroidManifest.xml
        │       ├── jniLibs/      # Drift-owned native libraries
        │       │   ├── arm64-v8a/
        │       │   │   ├── libdrift.so
        │       │   │   └── libc++_shared.so
        │       │   └── ...
        │       └── java/         # Package path from bundle ID
        │           └── com/example/myapp/
        │               └── MainActivity.kt
        ├── settings.gradle
        ├── gradle.properties
        ├── gradle/
        │   └── wrapper/
        │       ├── gradle-wrapper.jar
        │       └── gradle-wrapper.properties
        ├── bridge/               # Drift-owned, regenerated on build
        │   └── DriftBridge.kt
        └── driftw                # Wrapper script for IDE builds (chmod 0755)
```

**Android package structure:** The Kotlin source path is derived from the bundle ID in `drift.yaml`. For example, `bundle_id: com.example.myapp` creates `kotlin/com/example/myapp/`. All Android code uses Kotlin (not Java).

### 3. Build behavior when ejected
- **No `./platform/<platform>/`**: Build in `~/.drift/build/` (current behavior, with hash-based caching)
- **Has `./platform/<platform>/`**: Build IN PLACE in `./platform/<platform>/`
  - Drift generates bridge files into `./platform/<platform>/bridge/`
  - Drift compiles Go code and copies libraries
  - User's project files are NOT overwritten
  - Xcodebuild/Gradle runs in the ejected directory

**Cache invalidation for ejected builds:** When building in an ejected directory, there is no hash-based cache directory. Instead:
- Go compilation uses standard Go build caching (`GOCACHE`)
- Libraries are always written to the ejected directory (overwrites previous)
- Bridge files are always regenerated (they're cheap to generate)
- This is intentional: ejected projects are "live" and should always reflect current Go code

### 4. What Drift owns vs what user owns

#### iOS Ownership

| Path | Owned by | On build |
|------|----------|----------|
| `platform/ios/Runner/` (except libs) | User | Not touched |
| `platform/ios/Runner.xcodeproj/` | User | Not touched |
| `platform/ios/bridge/` | Drift | Regenerated |
| `platform/ios/Runner/libdrift.a` | Drift | Regenerated |
| `platform/ios/Runner/libdrift_skia.a` | Drift | Copied if missing or version mismatch |
| `platform/ios/Runner/.drift-skia-version` | Drift | Updated with Drift CLI version |
| `platform/ios/driftw` | Drift | Created at eject, not modified after |

#### Android Ownership

| Path | Owned by | On build |
|------|----------|----------|
| `platform/android/app/` (except jniLibs) | User | Not touched |
| `platform/android/settings.gradle` | User | Not touched |
| `platform/android/gradle.properties` | User | Not touched |
| `platform/android/gradle/` | User | Not touched |
| `platform/android/bridge/` | Drift | Regenerated |
| `platform/android/app/src/main/jniLibs/` | Drift | Created/overwritten |
| `platform/android/driftw` | Drift | Created at eject, not modified after |

**Note on jniLibs:** Although `platform/android/app/` is user-owned, the `app/src/main/jniLibs/` subdirectory is an exception—Drift creates and fully manages this directory. On each build, Drift may overwrite files in `jniLibs/`. Do not place custom native libraries here; use a separate directory and configure Gradle's `jniLibs.srcDirs` if needed.

#### Skia Library Copy Policy

Skia libraries are large (~50MB each) and change infrequently. The copy policy:
- **Copy if missing:** First build after eject copies Skia libs
- **Overwrite on version mismatch:** Drift uses its CLI version to tag prebuilt Skia libraries. On build, if the version in `.drift-skia-version` doesn't match the current CLI version, the library is replaced. This prevents stale binary issues when users upgrade Drift.
- **Never copy if version matches:** Avoids unnecessary I/O

**Marker file location:** The `.drift-skia-version` marker is colocated with the Skia library:
- iOS: `platform/ios/Runner/.drift-skia-version` (alongside `libdrift_skia.a`)

This colocation ensures version tracking is directly tied to the library it describes.

**Note on Android Skia:** Android uses static Skia linking—the Skia library is compiled into `libdrift.so` at build time. There is no separate `libskia.so` or version marker in `jniLibs/`.

### 5. Running from IDE: Build hooks

Both Xcode and Android Studio can automatically compile Go before building.

**Platform support:** The `driftw` wrapper script and IDE build hooks are **macOS and Linux only**. Windows users can still use all Drift CLI commands (`drift build`, `drift run`, `drift compile`, etc.) from the command line—only the IDE integration (Xcode build phases, Gradle hooks) is unsupported. For ejected Android projects on Windows, run `drift compile android` manually before building in Android Studio, or use `drift build android` / `drift run android` directly from the terminal.

**Wrapper script (`driftw`):** To avoid PATH issues in IDE environments, ejected projects include a `driftw` bash wrapper script that locates and runs `drift`. Written with mode `0755` (executable).

**Note:** The script requires Bash (uses arrays and `[[ ]]` syntax). It will not work with `/bin/sh` on minimal distros or systems where `/bin/sh` is not Bash. The `#!/usr/bin/env bash` shebang ensures Bash is used when available.

```bash
#!/usr/bin/env bash
# driftw - Drift wrapper for IDE builds (macOS/Linux only)
# Locates drift CLI and runs it with provided arguments

set -e

# Build candidate list (only include GOBIN if set)
candidates=("drift")
[[ -n "$GOBIN" ]] && candidates+=("$GOBIN/drift")
candidates+=(
    "$HOME/go/bin/drift"
    "$HOME/.local/bin/drift"
    "/usr/local/bin/drift"
    "/opt/homebrew/bin/drift"
)

# Try each candidate
for candidate in "${candidates[@]}"; do
    # For bare "drift", use command -v to search PATH
    # For absolute paths, check existence and executable bit explicitly
    if [[ "$candidate" == /* ]]; then
        [[ -x "$candidate" ]] && exec "$candidate" "$@"
    else
        resolved=$(command -v "$candidate" 2>/dev/null) && [[ -x "$resolved" ]] && exec "$resolved" "$@"
    fi
done

echo "Error: drift not found in PATH or common locations." >&2
echo "Install with: go install github.com/go-drift/drift/cmd/drift@latest" >&2
echo "Or ensure drift is in one of: PATH, \$GOBIN, ~/go/bin, ~/.local/bin, /usr/local/bin" >&2
exit 1
```

**iOS (Xcode Build Phase Script):**

The ejected Xcode project includes a **Run Script build phase**:
```
Build Phases:
  1. [Run Script] "Compile Drift"  →  "$PROJECT_DIR/driftw" compile ios
  2. [Compile Sources]             →  Swift files
  3. [Link Binary]                 →  includes libdrift.a
```

Press ⌘R in Xcode → Go compiles → Swift compiles → app runs.

**Android (Gradle Task):**

The ejected `app/build.gradle` includes a custom task:
```groovy
tasks.register("compileDrift", Exec) {
    workingDir = rootProject.projectDir.parentFile.parentFile  // project root
    commandLine "${rootProject.projectDir}/driftw", "compile", "android"
    // Only run if driftw exists (ejected project) and not on Windows
    onlyIf {
        def driftw = file("${rootProject.projectDir}/driftw")
        def isWindows = System.getProperty("os.name").toLowerCase().contains("windows")
        driftw.exists() && driftw.canExecute() && !isWindows
    }
}

tasks.named("preBuild") {
    dependsOn "compileDrift"
}
```

Press Run in Android Studio (macOS/Linux) → Gradle runs `driftw compile android` → builds Go → then normal Android build.

**New command:** `drift compile <platform>`
- Compiles Go code only (no scaffold, no xcodebuild/gradle)
- Always regenerates bridge files
- iOS: outputs `libdrift.a`, copies `libdrift_skia.a` if missing or version mismatch
- Android: outputs `libdrift.so` for each ABI (Skia is statically linked, no separate copy)
- Fast for iterative development
- **Warning:** For Android, this command writes to `jniLibs/`. Do not place custom native libraries there—they will be overwritten. See the jniLibs note in the ownership section.

**Relationship with `drift build`:** `drift compile` produces the same output layout as the compile phase of `drift build`. Running `drift compile` followed by `drift build` will not recompile Go code (Go's build cache handles this). This means:
- IDE hooks can use `drift compile` for fast iteration
- `drift build` can be used afterward without redundant work
- Both commands write to the same locations (ejected dir or `~/.drift/build/`)

**Output layout (consistent for both ejected and managed builds):**

| Platform | Output | Ejected location | Managed location |
|----------|--------|------------------|------------------|
| iOS | `libdrift.a` | `./platform/ios/Runner/libdrift.a` | `~/.drift/build/.../Runner/libdrift.a` |
| iOS | `libdrift_skia.a` | `./platform/ios/Runner/libdrift_skia.a` | `~/.drift/build/.../Runner/libdrift_skia.a` |
| iOS | `.drift-skia-version` | `./platform/ios/Runner/.drift-skia-version` | `~/.drift/build/.../Runner/.drift-skia-version` |
| iOS | Bridge files | `./platform/ios/bridge/` | `~/.drift/build/.../bridge/` |
| Android | `libdrift.so` | `./platform/android/app/src/main/jniLibs/<abi>/` | `~/.drift/build/.../android/app/src/main/jniLibs/<abi>/` |
| Android | `libc++_shared.so` | `./platform/android/app/src/main/jniLibs/<abi>/` | `~/.drift/build/.../android/app/src/main/jniLibs/<abi>/` |
| Android | Bridge files | `./platform/android/bridge/` | `~/.drift/build/.../bridge/` |

**Note on Android Skia:** Skia is statically linked into `libdrift.so` at compile time. There is no separate Skia library or version marker in the Android output.

**Implementation note:** Both ejected and managed builds use consistent paths. For managed builds, the full project structure is created under `~/.drift/build/.../` with the same subdirectory layout as ejected projects. All code paths must use the workspace helper functions (`JniLibsDir()`, `BridgeDir()`, etc.) to get correct locations—never hardcode paths directly.

**Note:** `drift compile all` is intentionally not supported. Compile targets a single platform because:
1. IDE build hooks call it for one platform at a time
2. Compiling both when you only need one wastes time
3. Users can run `drift compile ios && drift compile android` if needed

### 6. `drift.yaml` relationship
- Values from `drift.yaml` are substituted **at eject time only**
- After eject, user edits files directly (no more placeholders)
- **Changing `drift.yaml` won't affect already-ejected platforms** (this is called out in eject success message)
- Non-ejected platforms still use `drift.yaml` values

### 7. Eject status

**Command:** `drift status`

Shows which platforms are ejected and where builds go:
```
$ drift status
Project: myapp (com.example.myapp)

Platforms:
  ios:     ejected → ./platform/ios/
  android: managed → ~/.drift/build/myapp/android/<hash>/
```

This helps users understand the current state and build locations.

---

## What Changes After Eject

| Aspect | Before eject | After eject |
|--------|--------------|-------------|
| Build location | `~/.drift/build/<hash>/` | `./platform/<platform>/` |
| Project files | Generated fresh each build | User-owned, never overwritten |
| `drift.yaml` | Used for all values | Only affects non-ejected platforms |
| Bridge files | Generated to build dir | Generated to `./platform/<platform>/bridge/` |
| Libraries | Generated to build dir | Generated to ejected directory |
| IDE usage | Not practical | Full Xcode/Android Studio support (macOS/Linux) |
| Version control | Nothing to commit | Commit `./platform/` to repo |

**Skia libraries:** Copied to ejected directory on first build and updated when Drift's Skia version changes. These are large binaries (~50MB each) and should be added to `.gitignore`.

**`.gitignore` configuration:**

```gitignore
# Drift build artifacts (add these lines)
platform/*/.drift.env
platform/ios/Runner/libdrift.a
platform/ios/Runner/libdrift_skia.a
platform/ios/Runner/.drift-skia-version
platform/android/app/src/main/jniLibs/
platform/*/bridge/
```

**Important:** Some projects may already have `platform/` in `.gitignore` (e.g., from other frameworks). If so, you'll need to modify or remove that rule to commit your ejected project files. For example:
```gitignore
# Before (ignores all of platform/)
platform/

# After (ignore only build artifacts, not project files)
platform/ios/Runner/libdrift.a
platform/ios/Runner/libdrift_skia.a
# ... etc
```

---

## Undoing Eject

To return to managed (non-ejected) mode, delete the platform directory:
```bash
rm -rf ./platform/ios      # Return iOS to managed mode
rm -rf ./platform/android  # Return Android to managed mode
rm -rf ./platform          # Return both to managed mode
```

After deletion, `drift build` and `drift run` will use `~/.drift/build/` again and regenerate everything from templates + `drift.yaml`.

---

## Implementation

### Phase 1: Add `drift eject` Command

**New file: `cmd/drift/cmd/eject.go`**

```go
func ejectCmd(args []string) error {
    // 1. Parse platform argument (ios, android, all) and --force flag
    // 2. Find project root, load drift.yaml
    // 3. For "all": check BOTH platforms first, fail fast if either exists (unless --force)
    // 4. For each platform to eject:
    //    a. Check if platform dir exists; error or backup based on --force
    //    b. Backup format: platform/<platform>.backup.YYYYMMDD-HHMMSS-NNN/
    //    c. Increment NNN counter until unused name found
    // 5. Create TemplateData from config
    // 6. Create ./platform/<platform>/ directory
    // 7. Process templates and write as real files (not .tmpl)
    // 8. Write driftw wrapper script with mode 0755
    // 9. Print success message with:
    //    - Xcode/Studio open instructions
    //    - Warning that drift.yaml changes won't affect ejected platform
    //    - Suggested .gitignore additions
    //    - Note about existing platform/ gitignore rules if detected
}
```

Uses existing `templates.ReadFile()` + `templates.ProcessTemplate()` to generate processed output.

### Phase 2: Detect ejected platforms in workspace

**File: `cmd/drift/internal/workspace/workspace.go`**

Add detection with strict validation (requires multiple expected files):
```go
// isEjected returns true if the platform has been ejected.
// Validates by checking for multiple expected project files to avoid
// false positives from stray or partial directories.
func (w *Workspace) isEjected(platform string) bool {
    platformDir := filepath.Join(w.projectRoot, "platform", platform)

    switch platform {
    case "ios":
        // Require Runner/ to be a directory and project.pbxproj to be a file
        runner := filepath.Join(platformDir, "Runner")
        pbxproj := filepath.Join(platformDir, "Runner.xcodeproj", "project.pbxproj")
        runnerInfo, err1 := os.Stat(runner)
        pbxprojInfo, err2 := os.Stat(pbxproj)
        return err1 == nil && runnerInfo.IsDir() &&
               err2 == nil && !pbxprojInfo.IsDir()
    case "android":
        // Require settings.gradle and app/build.gradle to be files
        settings := filepath.Join(platformDir, "settings.gradle")
        buildGradle := filepath.Join(platformDir, "app", "build.gradle")
        settingsInfo, err1 := os.Stat(settings)
        buildGradleInfo, err2 := os.Stat(buildGradle)
        return err1 == nil && !settingsInfo.IsDir() &&
               err2 == nil && !buildGradleInfo.IsDir()
    default:
        return false
    }
}

func (w *Workspace) buildDir(platform string) string {
    if w.isEjected(platform) {
        return filepath.Join(w.projectRoot, "platform", platform)
    }
    return filepath.Join(cacheDir, "build", w.moduleSlug, platform, w.hash)
}

// bridgeDir returns where bridge files should be written.
func (w *Workspace) bridgeDir(platform string) string {
    return filepath.Join(w.buildDir(platform), "bridge")
}

// jniLibsDir returns where Android native libraries should be written.
// For ejected builds: ./platform/android/app/src/main/jniLibs
// For managed builds: ~/.drift/build/.../jniLibs (scaffold copies to app/src/main/jniLibs)
//
// IMPORTANT: Always use this function for native lib paths. Never hardcode
// app/src/main/jniLibs directly, as managed builds use a different structure.
func (w *Workspace) jniLibsDir(platform string) string {
    if platform != "android" {
        return ""
    }
    if w.isEjected(platform) {
        return filepath.Join(w.buildDir(platform), "app", "src", "main", "jniLibs")
    }
    // Managed builds: libs go directly under build dir, scaffold copies them
    return filepath.Join(w.buildDir(platform), "jniLibs")
}
```

### Phase 3: Update scaffold to skip user-owned files

**Files: `cmd/drift/internal/scaffold/{ios,android,xtool}.go`**

When building in ejected mode, scaffold should:
- Skip writing Swift/Kotlin files (user owns them)
- Skip writing project files (user owns them)
- Still write bridge files to `bridge/` subdirectory
- Still copy compiled libraries to correct locations

For managed Android builds, scaffold must copy `jniLibs/` from the build directory to `app/src/main/jniLibs/` in the assembled project.

```go
func WriteIOS(root string, settings Settings) error {
    if settings.Ejected {
        // Only write bridge files and copy libraries
        if err := writeBridgeFiles(filepath.Join(root, "bridge"), settings); err != nil {
            return err
        }
        return copyLibraries(root, settings)
    }
    // Full scaffold (current behavior)
    ...
}

func WriteAndroid(root string, settings Settings) error {
    if settings.Ejected {
        // Only write bridge files; jniLibs already in correct location
        return writeBridgeFiles(filepath.Join(root, "bridge"), settings)
    }
    // Full scaffold: generate project files, then copy jniLibs to app/src/main/jniLibs
    ...
    // Copy from settings.JniLibsDir to assembled project's app/src/main/jniLibs
    ...
}
```

### Phase 4: Add `drift compile` command

**New file: `cmd/drift/cmd/compile.go`**

Compiles Go code only, without scaffolding or running xcodebuild/gradle:
```go
func compileCmd(args []string) error {
    // 1. Parse platform (ios, android) - "all" not supported, error if specified
    // 2. Find project root, determine output directory (ejected or build dir)
    // 3. Compile Go → libdrift.a / libdrift.so (to buildDir or jniLibsDir)
    // 4. Generate bridge files to bridgeDir()
    // 5. Copy Skia libraries (iOS only):
    //    Check .drift-skia-version, copy libdrift_skia.a if missing or version mismatch
    //    (Android uses static Skia linking - no separate library to copy)
}
```

This is called by:
- The Xcode build phase script (for iOS)
- The Gradle build script (for Android)
- Users manually if they prefer

### Phase 5: Add `drift status` command

**New file: `cmd/drift/cmd/status.go`**

```go
func statusCmd(args []string) error {
    // 1. Load workspace
    // 2. For each platform (ios, android):
    //    - Check if ejected
    //    - Show build location
    // 3. Print formatted status
}
```

### Phase 6: Update build commands

**File: `cmd/drift/cmd/build.go`**

Pass ejected flag to scaffold, use correct build directory.

### Phase 7: Add build phase to ejected projects

**File: `cmd/drift/internal/templates/xcodeproj/project.pbxproj.tmpl`**

Add Run Script build phase that calls `"$PROJECT_DIR/driftw" compile ios`.

**File: `cmd/drift/internal/templates/xtool/project.pbxproj.tmpl`**

Add the same Run Script build phase. Per project rules, iOS template changes must update both `xcodeproj/` and `xtool/` templates in the same change.

**File: `cmd/drift/internal/templates/android/app.build.gradle.tmpl`**

Add compileDrift Gradle task that calls `driftw compile android`.

---

## Files to Create/Modify

| File | Action |
|------|--------|
| `cmd/drift/cmd/eject.go` | **New** - eject command |
| `cmd/drift/cmd/compile.go` | **New** - compile command (Go only, no xcodebuild) |
| `cmd/drift/cmd/status.go` | **New** - status command |
| `cmd/drift/cmd/root.go` | Register eject, compile, and status commands |
| `cmd/drift/internal/workspace/workspace.go` | Add `isEjected()`, `buildDir()`, `bridgeDir()`, `jniLibsDir()` |
| `cmd/drift/internal/scaffold/ios.go` | Skip user files when ejected |
| `cmd/drift/internal/scaffold/android.go` | Skip user files when ejected; copy jniLibs for managed builds |
| `cmd/drift/internal/scaffold/xtool.go` | Skip user files when ejected |
| `cmd/drift/internal/scaffold/settings.go` | Add `Ejected bool` field |
| `cmd/drift/internal/templates/driftw.tmpl` | **New** - wrapper script template |
| `cmd/drift/internal/templates/xcodeproj/project.pbxproj.tmpl` | Add Run Script build phase |
| `cmd/drift/internal/templates/xtool/project.pbxproj.tmpl` | Add Run Script build phase (must match xcodeproj) |
| `cmd/drift/internal/templates/android/app.build.gradle.tmpl` | Add compileDrift Gradle task |

---

## User Workflow

```bash
# 1. Create app normally
drift init myapp
cd myapp
drift run ios  # Works, builds in ~/.drift/build/

# 2. Check current status
drift status
# → ios: managed, android: managed

# 3. Need to customize iOS? Eject it.
drift eject ios
# → Created ./platform/ios/
# → Note: Changes to drift.yaml will not affect ejected iOS project
# → Open ./platform/ios/Runner.xcodeproj in Xcode
# → Add to .gitignore: platform/ios/Runner/libdrift.a, platform/ios/Runner/libdrift_skia.a, ...

# 4. Check status again
drift status
# → ios: ejected → ./platform/ios/
# → android: managed → ~/.drift/build/...

# 5. Customize in Xcode
#    - Add Firebase SDK
#    - Edit Info.plist for permissions
#    - Modify AppDelegate for push notifications

# 6. Two ways to run after ejecting:

#    Option A: From CLI (still works!)
drift run ios
# → Compiles Go, runs xcodebuild in ./platform/ios/
# → Deploys to simulator/device
# → Your customizations preserved

#    Option B: From Xcode (macOS)
#    - Open ./platform/ios/Runner.xcodeproj
#    - Press ⌘R
#    - Xcode calls driftw compile via build phase
#    - Your customizations preserved

# 7. To undo eject and return to managed mode:
rm -rf ./platform/ios
drift status
# → ios: managed, android: managed
```

**Key point:** `drift run` and `drift build` work identically before and after ejecting. The only difference is WHERE the build happens (ejected directory vs ~/.drift/build/) and that user files aren't overwritten.

---

## Verification

**Note:** Verification steps using `open` are macOS-specific. On Linux, use `xdg-open` or open the project directory in your IDE manually.

1. **Default behavior unchanged:**
   ```bash
   drift init testapp && cd testapp
   drift build ios
   # Should build in ~/.drift/build/
   drift status
   # Should show: ios: managed
   ```

2. **Eject creates real project (macOS):**
   ```bash
   drift eject ios
   ls ./platform/ios/Runner/Info.plist  # Should exist, no {{}} placeholders
   ls -l ./platform/ios/driftw          # Should be executable (rwxr-xr-x)
   open ./platform/ios/Runner.xcodeproj  # macOS: opens in Xcode
   drift status
   # Should show: ios: ejected → ./platform/ios/
   ```

3. **Eject idempotency:**
   ```bash
   drift eject ios
   drift eject ios  # Should error: "platform/ios/ already exists"
   drift eject --force ios  # Should backup and re-eject
   ls ./platform/ios.backup.20260202-143052-001/  # Backup should exist (with actual timestamp)
   ```

4. **Eject all with existing platform:**
   ```bash
   drift eject ios
   drift eject all  # Should error: "platform/ios/ already exists. Cannot eject all platforms."
   drift eject --force all  # Should backup ios and eject both
   ```

5. **Build uses ejected directory:**
   ```bash
   drift build ios
   # Should build in ./platform/ios/, not ~/.drift/build/
   ls ./platform/ios/bridge/  # Bridge files should be here
   ```

6. **User changes preserved:**
   ```bash
   echo "// custom" >> ./platform/ios/Runner/AppDelegate.swift
   drift build ios
   grep "// custom" ./platform/ios/Runner/AppDelegate.swift  # Should still be there
   ```

7. **Run from Xcode works (macOS):**
   ```bash
   drift eject ios
   open ./platform/ios/Runner.xcodeproj  # macOS-specific
   # Press ⌘R in Xcode
   # Should compile Go (via build phase), then Swift, then run
   ```

8. **`drift compile` works standalone:**
   ```bash
   drift compile ios
   ls ./platform/ios/Runner/libdrift.a  # Should exist
   ls ./platform/ios/bridge/            # Bridge files should exist

   drift compile android
   ls ./platform/android/app/src/main/jniLibs/arm64-v8a/libdrift.so  # Should exist
   ```

9. **`drift compile` works for non-ejected builds:**
   ```bash
   rm -rf ./platform/ios
   drift compile ios
   # Should compile to ~/.drift/build/.../
   # Subsequent drift build ios should not recompile (Go cache hit)
   ```

10. **Run from Android Studio works (macOS/Linux):**
    ```bash
    drift eject android
    # Open ./platform/android/ in Android Studio
    # Press Run (Shift+F10)
    # Should compile Go (via Gradle task), then build APK, then run
    ```

11. **Stray directory doesn't trigger ejected mode:**
    ```bash
    mkdir -p ./platform/ios  # Empty directory
    drift status
    # Should show: ios: managed (missing Runner/ and project.pbxproj)

    mkdir -p ./platform/ios/Runner.xcodeproj
    drift status
    # Should show: ios: managed (missing Runner/ and project.pbxproj content)
    ```

12. **Undo eject works:**
    ```bash
    drift eject ios
    drift status  # ios: ejected
    rm -rf ./platform/ios
    drift status  # ios: managed
    drift build ios  # Should work, builds in ~/.drift/build/
    ```

13. **Skia version update:**
    ```bash
    drift eject ios
    drift build ios
    cat ./platform/ios/Runner/.drift-skia-version  # Should show current version
    # Simulate drift upgrade with new Skia
    drift build ios  # Should replace libdrift_skia.a if version changed
    ```

14. **Managed Android jniLibs path:**
    ```bash
    rm -rf ./platform/android
    drift compile android
    # Libs should be at ~/.drift/build/.../android/app/src/main/jniLibs/<abi>/
    drift build android
    # Full build uses the same path structure
    ```

---

## Future Enhancements

- `drift eject --diff <platform>` - Compare ejected files with current templates
- `drift sync <platform>` - Update ejected project with new drift changes (interactive merge)
- Selective eject (only certain files)
- `drift eject --dry-run` - Show what would be created without writing files
- Windows support for ejected Android builds (`.bat`/`.ps1` wrapper scripts)
