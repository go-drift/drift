package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-drift/drift/cmd/drift/internal/config"
	"github.com/go-drift/drift/cmd/drift/internal/templates"
	"github.com/go-drift/drift/cmd/drift/internal/workspace"
)

func init() {
	RegisterCommand(&Command{
		Name:  "eject",
		Short: "Eject platform for customization",
		Long: `Eject a platform's native project for full customization.

After ejecting, you can open the project in Xcode (iOS) or Android Studio
(Android) and make changes that persist across builds.

Platforms:
  ios       Eject iOS project to ./platform/ios/
  android   Eject Android project to ./platform/android/
  all       Eject both platforms

Flags:
  --force   Overwrite existing platform directory (creates backup)

The ejected project is a real, fully-functioning project with all template
values substituted. You can edit Swift/Kotlin code, modify project settings,
add dependencies, etc.

Note: Changes to drift.yaml will NOT affect ejected platforms. To incorporate
drift.yaml changes, delete the platform directory and re-eject.`,
		Usage: "drift eject <ios|android|all> [--force]",
		Run:   runEject,
	})
}

type ejectOptions struct {
	force bool
}

func runEject(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("platform is required\n\nUsage: drift eject <ios|android|all> [--force]")
	}

	var platforms []string
	opts := ejectOptions{}

	for _, arg := range args {
		switch arg {
		case "--force":
			opts.force = true
		case "ios", "android":
			platforms = append(platforms, arg)
		case "all":
			platforms = []string{"ios", "android"}
		case "xtool":
			return fmt.Errorf("xtool eject is not yet supported (xtool uses SwiftPM, not Xcode projects)")
		default:
			return fmt.Errorf("unknown argument %q\n\nUsage: drift eject <ios|android|all> [--force]", arg)
		}
	}

	if len(platforms) == 0 {
		return fmt.Errorf("platform is required\n\nUsage: drift eject <ios|android|all> [--force]")
	}

	root, err := config.FindProjectRoot()
	if err != nil {
		return err
	}

	cfg, err := config.Resolve(root)
	if err != nil {
		return err
	}

	// Check for existing platform directories first (fail fast)
	var existing []string
	for _, platform := range platforms {
		platformDir := filepath.Join(root, "platform", platform)
		if _, err := os.Stat(platformDir); err == nil {
			existing = append(existing, platformDir)
		}
	}

	if len(existing) > 0 && !opts.force {
		if len(existing) == 1 {
			return fmt.Errorf("%s already exists. Use --force to overwrite (creates backup)", existing[0])
		}
		return fmt.Errorf("cannot eject all platforms. Existing directories:\n  - %s\nUse --force to backup and overwrite, or eject platforms individually",
			strings.Join(existing, "\n  - "))
	}

	// Eject each platform
	for _, platform := range platforms {
		if err := ejectPlatform(root, cfg, platform, opts); err != nil {
			return err
		}
	}

	return nil
}

func ejectPlatform(root string, cfg *config.Resolved, platform string, opts ejectOptions) error {
	platformDir := filepath.Join(root, "platform", platform)

	// Handle existing directory
	if _, err := os.Stat(platformDir); err == nil {
		if opts.force {
			backupDir, err := createBackup(platformDir)
			if err != nil {
				return fmt.Errorf("failed to backup %s: %w", platformDir, err)
			}
			fmt.Printf("Backed up %s to %s\n", platformDir, backupDir)
		}
	}

	// Create platform directory
	if err := os.MkdirAll(platformDir, 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", platformDir, err)
	}

	tmplData := templates.NewTemplateData(templates.TemplateInput{
		AppName:        cfg.AppName,
		AndroidPackage: cfg.AppID,
		IOSBundleID:    cfg.AppID,
		Orientation:    cfg.Orientation,
		AllowHTTP:      cfg.AllowHTTP,
	})

	// Write platform files
	switch platform {
	case "ios":
		if err := ejectIOS(platformDir, tmplData); err != nil {
			return err
		}
	case "android":
		if err := ejectAndroid(platformDir, tmplData); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown platform %q", platform)
	}

	// Write driftw wrapper script (bash for macOS/Linux)
	driftwPath := filepath.Join(platformDir, "driftw")
	driftwContent, err := templates.ReadFile("driftw")
	if err != nil {
		return fmt.Errorf("failed to read driftw template: %w", err)
	}
	if err := os.WriteFile(driftwPath, driftwContent, 0o755); err != nil {
		return fmt.Errorf("failed to write driftw: %w", err)
	}

	// Write driftw.bat for Android only (Windows Android Studio builds)
	if platform == "android" {
		driftwBatPath := filepath.Join(platformDir, "driftw.bat")
		driftwBatContent, err := templates.ReadFile("driftw.bat")
		if err != nil {
			return fmt.Errorf("failed to read driftw.bat template: %w", err)
		}
		if err := os.WriteFile(driftwBatPath, driftwBatContent, 0o644); err != nil {
			return fmt.Errorf("failed to write driftw.bat: %w", err)
		}
	}

	// Write .drift.env with absolute path to drift binary (for IDE builds)
	if err := workspace.WriteDriftEnv(platformDir); err != nil {
		return fmt.Errorf("failed to write .drift.env: %w", err)
	}

	// Print success message
	fmt.Printf("\nEjected %s to %s\n\n", platform, platformDir)

	switch platform {
	case "ios":
		fmt.Printf("Open in Xcode:\n  open %s/Runner.xcodeproj\n\n", platformDir)
	case "android":
		fmt.Printf("Open in Android Studio:\n  studio %s\n\n", platformDir)
	}

	fmt.Println("Note: Changes to drift.yaml will NOT affect this ejected project.")
	fmt.Println("To incorporate drift.yaml changes, delete the platform directory and re-eject.")
	fmt.Println()
	fmt.Println("Suggested .gitignore additions:")
	switch platform {
	case "ios":
		fmt.Println("  platform/ios/.drift.env")
		fmt.Println("  platform/ios/Runner/libdrift.a")
		fmt.Println("  platform/ios/Runner/libdrift_skia.a")
		fmt.Println("  platform/ios/Runner/.drift-skia-version")
		fmt.Println("  platform/ios/bridge/")
	case "android":
		fmt.Println("  platform/android/.drift.env")
		fmt.Println("  platform/android/app/src/main/jniLibs/")
		fmt.Println("  platform/android/bridge/")
	}

	return nil
}

func createBackup(dir string) (string, error) {
	now := time.Now()
	base := dir + ".backup." + now.Format("20060102-150405")

	// Try the unsuffixed name first, then add a counter on collision
	if _, err := os.Stat(base); os.IsNotExist(err) {
		if err := os.Rename(dir, base); err != nil {
			return "", err
		}
		return base, nil
	}
	for i := 2; i <= 999; i++ {
		backupDir := fmt.Sprintf("%s-%03d", base, i)
		if _, err := os.Stat(backupDir); os.IsNotExist(err) {
			if err := os.Rename(dir, backupDir); err != nil {
				return "", err
			}
			return backupDir, nil
		}
	}

	return "", fmt.Errorf("too many backups exist for %s", dir)
}

func ejectIOS(platformDir string, data *templates.TemplateData) error {
	runnerDir := filepath.Join(platformDir, "Runner")
	xcodeprojDir := filepath.Join(platformDir, "Runner.xcodeproj")

	if err := os.MkdirAll(runnerDir, 0o755); err != nil {
		return fmt.Errorf("failed to create Runner directory: %w", err)
	}
	if err := os.MkdirAll(xcodeprojDir, 0o755); err != nil {
		return fmt.Errorf("failed to create xcodeproj directory: %w", err)
	}

	// Write Info.plist
	if err := writeTemplateFile("ios/Info.plist.tmpl", filepath.Join(runnerDir, "Info.plist"), data, 0o644); err != nil {
		return err
	}

	// Write Swift files
	iosFiles, err := templates.GetIOSFiles()
	if err != nil {
		return fmt.Errorf("failed to list ios templates: %w", err)
	}

	for _, file := range iosFiles {
		baseName := templates.FileName(file)
		if strings.HasSuffix(baseName, ".swift") || strings.HasSuffix(baseName, ".swift.tmpl") {
			destName := strings.TrimSuffix(baseName, ".tmpl")
			if err := writeTemplateFile(file, filepath.Join(runnerDir, destName), data, 0o644); err != nil {
				return err
			}
		}
	}

	// Write LaunchScreen.storyboard
	if err := writeTemplateFile("ios/LaunchScreen.storyboard", filepath.Join(runnerDir, "LaunchScreen.storyboard"), data, 0o644); err != nil {
		return err
	}

	// Write Xcode project files
	xcodeFiles, err := templates.GetXcodeProjectFiles()
	if err != nil {
		return fmt.Errorf("failed to list xcode templates: %w", err)
	}

	for _, file := range xcodeFiles {
		baseName := templates.FileName(file)
		destName := strings.TrimSuffix(baseName, ".tmpl")
		if err := writeTemplateFile(file, filepath.Join(xcodeprojDir, destName), data, 0o644); err != nil {
			return err
		}
	}

	return nil
}

func ejectAndroid(platformDir string, data *templates.TemplateData) error {
	appDir := filepath.Join(platformDir, "app")
	srcDir := filepath.Join(appDir, "src", "main")
	cppDir := filepath.Join(srcDir, "cpp")
	resDir := filepath.Join(srcDir, "res")
	kotlinDir := filepath.Join(srcDir, "java", data.PackagePath)

	// Create directory structure
	dirs := []string{
		srcDir,
		cppDir,
		filepath.Join(resDir, "values"),
		filepath.Join(resDir, "xml"),
		kotlinDir,
		filepath.Join(platformDir, "gradle", "wrapper"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	// Write root gradle files
	if err := writeTemplateFile("android/settings.gradle.tmpl", filepath.Join(platformDir, "settings.gradle"), data, 0o644); err != nil {
		return err
	}
	if err := writeTemplateFile("android/build.gradle", filepath.Join(platformDir, "build.gradle"), data, 0o644); err != nil {
		return err
	}
	if err := writeTemplateFile("android/gradle.properties", filepath.Join(platformDir, "gradle.properties"), data, 0o644); err != nil {
		return err
	}

	// Write app/build.gradle
	if err := writeTemplateFile("android/app.build.gradle.tmpl", filepath.Join(appDir, "build.gradle"), data, 0o644); err != nil {
		return err
	}

	// Write AndroidManifest.xml
	if err := writeTemplateFile("android/AndroidManifest.xml.tmpl", filepath.Join(srcDir, "AndroidManifest.xml"), data, 0o644); err != nil {
		return err
	}

	// Write styles.xml
	if err := writeTemplateFile("android/styles.xml", filepath.Join(resDir, "values", "styles.xml"), data, 0o644); err != nil {
		return err
	}

	// Write file_paths.xml
	if err := writeTemplateFile("android/res/xml/file_paths.xml.tmpl", filepath.Join(resDir, "xml", "file_paths.xml"), data, 0o644); err != nil {
		return err
	}

	// Write Kotlin files
	javaFiles, err := templates.GetAndroidJavaFiles()
	if err != nil {
		return fmt.Errorf("failed to list java templates: %w", err)
	}

	for _, file := range javaFiles {
		baseName := templates.FileName(file)
		if err := writeTemplateFile(file, filepath.Join(kotlinDir, baseName), data, 0o644); err != nil {
			return err
		}
	}

	// Write C++ files
	cppFiles, err := templates.GetAndroidCPPFiles()
	if err != nil {
		return fmt.Errorf("failed to list cpp templates: %w", err)
	}

	for _, file := range cppFiles {
		baseName := templates.FileName(file)
		if err := writeTemplateFile(file, filepath.Join(cppDir, baseName), data, 0o644); err != nil {
			return err
		}
	}

	// Write gradle wrapper files
	wrapperDir := filepath.Join(platformDir, "gradle", "wrapper")
	if err := writeTemplateFile("android/gradle/wrapper/gradle-wrapper.properties", filepath.Join(wrapperDir, "gradle-wrapper.properties"), data, 0o644); err != nil {
		return err
	}

	// Copy binary files directly (not templates)
	binaryFiles := []struct {
		src  string
		dest string
		perm os.FileMode
	}{
		{"android/gradlew", filepath.Join(platformDir, "gradlew"), 0o755},
		{"android/gradlew.bat", filepath.Join(platformDir, "gradlew.bat"), 0o644},
		{"android/gradle/wrapper/gradle-wrapper.jar", filepath.Join(wrapperDir, "gradle-wrapper.jar"), 0o644},
		{"android/gradle/wrapper/gradle-wrapper-shared.jar", filepath.Join(wrapperDir, "gradle-wrapper-shared.jar"), 0o644},
		{"android/gradle/wrapper/gradle-cli-8.2.jar", filepath.Join(wrapperDir, "gradle-cli-8.2.jar"), 0o644},
	}

	for _, bf := range binaryFiles {
		content, err := templates.ReadFile(bf.src)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", bf.src, err)
		}
		if err := os.WriteFile(bf.dest, content, bf.perm); err != nil {
			return fmt.Errorf("failed to write %s: %w", bf.dest, err)
		}
	}

	// Create jniLibs directory structure
	jniLibsDir := filepath.Join(srcDir, "jniLibs")
	for _, abi := range []string{"arm64-v8a", "armeabi-v7a", "x86_64"} {
		if err := os.MkdirAll(filepath.Join(jniLibsDir, abi), 0o755); err != nil {
			return fmt.Errorf("failed to create jniLibs/%s: %w", abi, err)
		}
	}

	return nil
}

func writeTemplateFile(templatePath, destPath string, data *templates.TemplateData, perm os.FileMode) error {
	content, err := templates.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("failed to read template %s: %w", templatePath, err)
	}

	processed, err := templates.ProcessTemplate(string(content), data)
	if err != nil {
		return fmt.Errorf("failed to process template %s: %w", templatePath, err)
	}

	if err := os.WriteFile(destPath, []byte(processed), perm); err != nil {
		return fmt.Errorf("failed to write %s: %w", destPath, err)
	}

	return nil
}
