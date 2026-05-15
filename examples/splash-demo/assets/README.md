# Splash demo assets

Drop a real splash image at `splash.png` (recommended 1024×1024) before
running `drift build android` or `drift build ios`.

The build pipeline reads this file via `ctx.ResolveAsset` and bundles it
as an iOS asset-catalogue entry and an Android drawable. A missing file
fails the build with a clear error.

For dark-mode support, add a sibling `splash_dark.png` and update
`drift.yaml` with a `dark:` block:

```yaml
dark:
  image: assets/splash_dark.png
  background_color: "#000000"
```
