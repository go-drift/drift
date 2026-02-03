package scaffold

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-drift/drift/cmd/drift/internal/templates"
)

// WriteAndroid writes the Android project files to root.
// If settings.Ejected is true, this returns early without writing anything.
// For ejected builds, bridge files and jniLibs are handled separately by
// workspace.Prepare and the compile command.
func WriteAndroid(root string, settings Settings) error {
	if settings.Ejected {
		return nil
	}

	androidDir := filepath.Join(root, "android")
	appDir := filepath.Join(androidDir, "app")
	srcDir := filepath.Join(appDir, "src", "main")
	cppDir := filepath.Join(srcDir, "cpp")
	resDir := filepath.Join(srcDir, "res")

	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		return fmt.Errorf("failed to create android source directory: %w", err)
	}

	// Create template data
	tmplData := templates.NewTemplateData(templates.TemplateInput{
		AppName:        settings.AppName,
		AndroidPackage: settings.AppID,
		IOSBundleID:    settings.Bundle,
		Orientation:    settings.Orientation,
		AllowHTTP:      settings.AllowHTTP,
	})

	writeTemplateFile := func(templatePath, destPath string, perm os.FileMode) error {
		content, err := templates.ReadFile(templatePath)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", templatePath, err)
		}

		processed, err := templates.ProcessTemplate(string(content), tmplData)
		if err != nil {
			return fmt.Errorf("failed to process template %s: %w", templatePath, err)
		}

		if err := os.WriteFile(destPath, []byte(processed), perm); err != nil {
			return fmt.Errorf("failed to write %s: %w", destPath, err)
		}

		return nil
	}

	// settings.gradle
	if err := writeTemplateFile("android/settings.gradle.tmpl", filepath.Join(androidDir, "settings.gradle"), 0o644); err != nil {
		return err
	}

	// build.gradle (root)
	if err := writeTemplateFile("android/build.gradle", filepath.Join(androidDir, "build.gradle"), 0o644); err != nil {
		return err
	}

	// gradle.properties
	if err := writeTemplateFile("android/gradle.properties", filepath.Join(androidDir, "gradle.properties"), 0o644); err != nil {
		return err
	}

	// app/build.gradle
	if err := writeTemplateFile("android/app.build.gradle.tmpl", filepath.Join(appDir, "build.gradle"), 0o644); err != nil {
		return err
	}

	// AndroidManifest.xml
	if err := writeTemplateFile("android/AndroidManifest.xml.tmpl", filepath.Join(srcDir, "AndroidManifest.xml"), 0o644); err != nil {
		return err
	}

	// styles.xml
	stylesDir := filepath.Join(resDir, "values")
	if err := os.MkdirAll(stylesDir, 0o755); err != nil {
		return fmt.Errorf("failed to create res/values: %w", err)
	}

	if err := writeTemplateFile("android/styles.xml", filepath.Join(stylesDir, "styles.xml"), 0o644); err != nil {
		return err
	}

	// FileProvider paths
	xmlDir := filepath.Join(resDir, "xml")
	if err := os.MkdirAll(xmlDir, 0o755); err != nil {
		return fmt.Errorf("failed to create res/xml: %w", err)
	}

	if err := writeTemplateFile("android/res/xml/file_paths.xml.tmpl", filepath.Join(xmlDir, "file_paths.xml"), 0o644); err != nil {
		return err
	}

	// Write Kotlin files from templates
	kotlinPkg := strings.ReplaceAll(settings.AppID, ".", "/")
	kotlinDir := filepath.Join(srcDir, "java", kotlinPkg)
	if err := os.MkdirAll(kotlinDir, 0o755); err != nil {
		return fmt.Errorf("failed to create kotlin directory: %w", err)
	}

	javaFiles, err := templates.GetAndroidJavaFiles()
	if err != nil {
		return fmt.Errorf("failed to list java templates: %w", err)
	}

	for _, file := range javaFiles {
		content, err := templates.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", file, err)
		}

		processed, err := templates.ProcessTemplate(string(content), tmplData)
		if err != nil {
			return fmt.Errorf("failed to process template %s: %w", file, err)
		}

		destFile := filepath.Join(kotlinDir, templates.FileName(file))
		if err := os.WriteFile(destFile, []byte(processed), 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", destFile, err)
		}
	}

	// Write C/C++ files from templates
	if err := os.MkdirAll(cppDir, 0o755); err != nil {
		return fmt.Errorf("failed to create cpp directory: %w", err)
	}

	cppFiles, err := templates.GetAndroidCPPFiles()
	if err != nil {
		return fmt.Errorf("failed to list cpp templates: %w", err)
	}

	for _, file := range cppFiles {
		content, err := templates.ReadFile(file)
		if err != nil {
			return fmt.Errorf("failed to read template %s: %w", file, err)
		}

		processed, err := templates.ProcessTemplate(string(content), tmplData)
		if err != nil {
			return fmt.Errorf("failed to process template %s: %w", file, err)
		}

		destFile := filepath.Join(cppDir, templates.FileName(file))
		if err := os.WriteFile(destFile, []byte(processed), 0o644); err != nil {
			return fmt.Errorf("failed to write %s: %w", destFile, err)
		}
	}

	// Create jniLibs directory structure
	jniLibsDir := filepath.Join(srcDir, "jniLibs")
	for _, abi := range []string{"arm64-v8a", "armeabi-v7a", "x86_64"} {
		if err := os.MkdirAll(filepath.Join(jniLibsDir, abi), 0o755); err != nil {
			return fmt.Errorf("failed to create jniLibs/%s: %w", abi, err)
		}
	}

	// Create gradle wrapper directory
	gradleWrapperDir := filepath.Join(androidDir, "gradle", "wrapper")
	if err := os.MkdirAll(gradleWrapperDir, 0o755); err != nil {
		return fmt.Errorf("failed to create gradle/wrapper: %w", err)
	}

	if err := writeTemplateFile("android/gradle/wrapper/gradle-wrapper.properties", filepath.Join(gradleWrapperDir, "gradle-wrapper.properties"), 0o644); err != nil {
		return err
	}

	gradlew, err := templates.ReadFile("android/gradlew")
	if err != nil {
		return fmt.Errorf("failed to read gradlew template: %w", err)
	}
	gradlewPath := filepath.Join(androidDir, "gradlew")
	if err := os.WriteFile(gradlewPath, gradlew, 0o755); err != nil {
		return fmt.Errorf("failed to write gradlew: %w", err)
	}

	gradlewBat, err := templates.ReadFile("android/gradlew.bat")
	if err != nil {
		return fmt.Errorf("failed to read gradlew.bat template: %w", err)
	}
	if err := os.WriteFile(filepath.Join(androidDir, "gradlew.bat"), gradlewBat, 0o644); err != nil {
		return fmt.Errorf("failed to write gradlew.bat: %w", err)
	}

	wrapperJar, err := templates.ReadFile("android/gradle/wrapper/gradle-wrapper.jar")
	if err != nil {
		return fmt.Errorf("failed to read gradle-wrapper.jar: %w", err)
	}
	if err := os.WriteFile(filepath.Join(gradleWrapperDir, "gradle-wrapper.jar"), wrapperJar, 0o644); err != nil {
		return fmt.Errorf("failed to write gradle-wrapper.jar: %w", err)
	}

	wrapperSharedJar, err := templates.ReadFile("android/gradle/wrapper/gradle-wrapper-shared.jar")
	if err != nil {
		return fmt.Errorf("failed to read gradle-wrapper-shared.jar: %w", err)
	}
	if err := os.WriteFile(filepath.Join(gradleWrapperDir, "gradle-wrapper-shared.jar"), wrapperSharedJar, 0o644); err != nil {
		return fmt.Errorf("failed to write gradle-wrapper-shared.jar: %w", err)
	}

	wrapperCliJar, err := templates.ReadFile("android/gradle/wrapper/gradle-cli-8.2.jar")
	if err != nil {
		return fmt.Errorf("failed to read gradle-cli-8.2.jar: %w", err)
	}
	if err := os.WriteFile(filepath.Join(gradleWrapperDir, "gradle-cli-8.2.jar"), wrapperCliJar, 0o644); err != nil {
		return fmt.Errorf("failed to write gradle-cli-8.2.jar: %w", err)
	}

	return nil
}
