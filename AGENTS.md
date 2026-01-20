# AGENTS.md

## Scope
- This guidance applies to the entire repo.
- Sources: `Makefile`, `scripts/*.sh`, `STYLE.md`, `go.mod`, and current Go code layout.

## Environment and Tooling
- Go version: `1.24.0` (from `go.mod`).
- Module name: `drift`.
- CGO is used for mobile bridges; keep toolchains installed.
- Android builds rely on the Gradle wrapper in `showcase/android`.
- Skia scripts expect `python3`, `gn`, and `ninja` in `PATH`.

## Cursor/Copilot Rules
- No `.cursor/rules/`, `.cursorrules`, or `.github/copilot-instructions.md` found.
- If any appear later, mirror their requirements here.

## Build, Lint, and Test Commands
### Core Go builds
- Build the Drift CLI: `make cli` (alias for `go build -o bin/drift ./cmd/drift`).
- Build all Go packages: `go build ./...`.
- Sync modules: `go mod tidy`.
- Clean build artifacts: `make clean`.

### Android build pipeline
- Build Go shared libs: `make android-libs`.
- Build the showcase APK: `make android-build`.
- Install to device: `make android-install`.
- Build + install + run: `make android-run`.
- View logs: `make android-log`.
- Start emulator (requires `AVD`): `make android-emulator`.
- Required env vars: `ANDROID_HOME`, `ANDROID_NDK_HOME`, and optional `HOST_TAG`.

### Skia toolchain
- Fetch Skia: `scripts/fetch_skia.sh`.
- Build Skia Android: `scripts/build_skia_android.sh`.
- Build Skia iOS: `scripts/build_skia_ios.sh`.
- Build release artifacts: `scripts/build_skia_release.sh`.

### Formatting and linting
- Format Go code: `gofmt -w <file-or-dir>`.
- Vet packages: `go vet ./...`.
- No `golangci-lint` config present; stick to `gofmt` + `go vet` unless added.

### Tests
- No `*_test.go` files currently in repo.
- When tests exist, run all: `go test ./...`.
- Run a single test: `go test ./path/to/pkg -run '^TestName$' -count=1`.
- Run a single subtest: `go test ./path/to/pkg -run '^TestName$/Subtest$' -count=1`.

### Dependency management
- Add dependencies with `go get` and follow with `go mod tidy`.
- Keep `go.sum` updated alongside `go.mod` changes.
- Prefer minimal version bumps unless required.

## Code Style (Go)
### Formatting and layout
- Always `gofmt` Go sources.
- Keep functions short and focused; extract helpers when logic grows.
- Prefer early returns for error handling over nested `if` chains.

### Imports
- Group standard library imports first, then a blank line, then module imports.
- Avoid dot or blank identifier imports unless absolutely required.

### Naming
- Use `CamelCase` for exported identifiers, `lowerCamelCase` for unexported.
- Prefer descriptive names over abbreviations; short names ok for tight scopes.
- Use `NewType` for constructors, `DefaultX` for defaults.
- Package names are short, lowercase, singular.

### Comments and docs
- Start public packages with a package comment where possible.
- Use full sentences for exported type/function doc comments.
- Keep inline comments short; prefer self-explanatory code.

### Types and API shape
- Favor structs for configuration (see `STYLE.md`), avoid long parameter lists.
- Prefer concrete types over `any` unless a generic interface is required.
- Use pointers when mutation is expected; value types for immutable configs.
- Keep interfaces small and focused; define them where used.

### Error handling
- Always check and return errors; avoid panics in library code.
- Wrap errors with context using `fmt.Errorf("...: %w", err)`.
- Use sentinel errors sparingly; prefer contextual wrapping.
- CLI code uses `errors.PrintError` and `errors.RecoverPanic` patterns.
- When wrapping build failures, capture command/output context.

### CLI and tooling
- Commands return errors; top-level `main` prints errors and exits non-zero.
- Prefer formatted user-facing errors over raw panics.
- Keep CLI output concise and consistent (see `cmd/drift/internal/errors`).

### Concurrency
- Keep UI/thread-affine work on the UI goroutine; background work uses goroutines.
- Avoid data races by passing data via channels or immutable structs.

### CGO and platform bridges
- Keep `// #include` and `//export` blocks together at the top of cgo files.
- Keep bridge functions thin; delegate logic to Go packages.
- Ensure Android/iOS toolchains are installed before running builds.
- Avoid introducing CGO into pure Go packages unless needed.

### Documentation and examples
- Update `STYLE.md` if new widget patterns emerge.
- Keep CLI usage text in sync with command behavior.
- Prefer small example snippets over large sample apps.

## Drift UI Style (from `STYLE.md`)
### Widget construction
- Prefer struct literals for simple, self-documenting widgets.
- Use helper constructors when defaults matter (e.g. `widgets.NewButton`).
- Use builder patterns for customization; call `.Build()` when required.

### Composition and layout
- Compose UIs by nesting widgets; avoid deep inheritance patterns.
- Use `widgets.ColumnOf` / `widgets.RowOf` with spacing helpers.
- Prefer `widgets.VSpace` / `widgets.HSpace` for consistent gaps.

### State management
- Stateless widgets are configuration-only; stateful widgets hold mutable state.
- Only mutate state inside `SetState` so the UI rebuilds.
- Do not store `BuildContext` beyond the `Build` call.
- Avoid creating widgets in `InitState`; build them in `Build`.

### Theme usage
- Prefer `theme.UseTheme(ctx)` or `theme.UseThemeData` at the start of `Build`.
- Use `theme.ColorsOf` / `theme.TextThemeOf` for targeted access.

### Navigation
- Use `navigation.NavigatorOf(ctx)` and guard against `nil`.
- Prefer named routes in `OnGenerateRoute` and use `PushNamed` / `Pop`.

## Project Structure Notes
- CLI entrypoint: `cmd/drift/main.go` and `cmd/drift/cmd/*`.
- Core framework packages live under `pkg/`.
- Mobile showcase app is in `showcase/`.
- Skia vendoring/build lives in `third_party/skia` and `scripts/`.

## When Updating or Adding Code
- Follow `STYLE.md` for widget patterns and state usage.
- Keep Go code idiomatic; match existing formatting and error style.
- Prefer minimal diffs and avoid unrelated refactors.
- Update `AGENTS.md` if new commands, rules, or style conventions appear.
