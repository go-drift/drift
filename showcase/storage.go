package main

import (
	"context"

	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/layout"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildStoragePage creates a stateful widget for storage demos.
func buildStoragePage(ctx core.BuildContext) core.Widget {
	return core.NewStatefulWidget(func() *storageState { return &storageState{} })
}

type storageState struct {
	core.StateBase
	statusText   *core.ManagedState[string]
	selectedFile *core.ManagedState[*platform.PickedFile]
	selectedPath *core.ManagedState[string]
	appDirs      *core.ManagedState[map[string]string]
}

func (s *storageState) InitState() {
	s.statusText = core.NewManagedState(&s.StateBase, "Tap a button to pick files or directories.")
	s.selectedFile = core.NewManagedState[*platform.PickedFile](&s.StateBase, nil)
	s.selectedPath = core.NewManagedState(&s.StateBase, "")
	s.appDirs = core.NewManagedState(&s.StateBase, make(map[string]string))

	// Get app directories
	go func() {
		dirs := make(map[string]string)
		if path, err := platform.Storage.GetAppDirectory(platform.AppDirectoryDocuments); err == nil {
			dirs["Documents"] = path
		}
		if path, err := platform.Storage.GetAppDirectory(platform.AppDirectoryCache); err == nil {
			dirs["Cache"] = path
		}
		if path, err := platform.Storage.GetAppDirectory(platform.AppDirectoryTemp); err == nil {
			dirs["Temp"] = path
		}
		drift.Dispatch(func() {
			s.appDirs.Set(dirs)
		})
	}()
}

func (s *storageState) Build(ctx core.BuildContext) core.Widget {
	colors := theme.ColorsOf(ctx)

	return demoPage(ctx, "Storage",
		sectionTitle("File Picker", colors),
		widgets.VSpace(12),
		widgets.Text{Content: "Pick files and directories:", Style: labelStyle(colors)},
		widgets.VSpace(12),
		widgets.Row{
			MainAxisAlignment: widgets.MainAxisAlignmentStart,
			Children: []core.Widget{
				theme.ButtonOf(ctx, "Pick File", func() {
					s.pickFile()
				}),
				widgets.HSpace(8),
				theme.ButtonOf(ctx, "Pick Dir", func() {
					s.pickDirectory()
				}).WithColor(colors.Secondary, colors.OnSecondary),
				widgets.HSpace(8),
				theme.ButtonOf(ctx, "Save File", func() {
					s.saveFile()
				}).WithColor(colors.Tertiary, colors.OnTertiary),
			},
		},
		widgets.VSpace(24),

		sectionTitle("Selected Item", colors),
		widgets.VSpace(12),
		s.selectedItemCard(colors),
		widgets.VSpace(16),

		statusCard(s.statusText.Get(), colors),
		widgets.VSpace(24),

		sectionTitle("App Directories", colors),
		widgets.VSpace(12),
		s.appDirectoriesCard(colors),
		widgets.VSpace(40),
	)
}

func (s *storageState) selectedItemCard(colors theme.ColorScheme) core.Widget {
	file := s.selectedFile.Get()
	path := s.selectedPath.Get()

	if file == nil && path == "" {
		return widgets.Container{
			Color:        colors.SurfaceVariant,
			BorderRadius: 8,
			Padding:      layout.EdgeInsetsAll(16),
			Child: widgets.Text{Content: "No item selected", Style: graphics.TextStyle{
				Color:    colors.OnSurfaceVariant,
				FontSize: 14,
			}},
		}
	}

	if file != nil {
		return widgets.Container{
			Color:        colors.SurfaceVariant,
			BorderRadius: 8,
			Padding:      layout.EdgeInsetsAll(16),
			Child: widgets.Column{
				MainAxisAlignment:  widgets.MainAxisAlignmentStart,
				CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
				MainAxisSize:       widgets.MainAxisSizeMin,
				Children: []core.Widget{
					s.infoRow("Name", file.Name, colors),
					widgets.VSpace(8),
					s.infoRow("MIME Type", file.MimeType, colors),
					widgets.VSpace(8),
					s.infoRow("Size", formatSize(file.Size), colors),
					widgets.VSpace(8),
					widgets.Text{Content: "Path:", Style: graphics.TextStyle{
						Color:      colors.OnSurfaceVariant,
						FontSize:   12,
						FontWeight: graphics.FontWeightBold,
					}},
					widgets.VSpace(4),
					widgets.Text{Content: file.Path, Style: graphics.TextStyle{
						Color:    colors.OnSurface,
						FontSize: 12,
					}},
				},
			},
		}
	}

	return widgets.Container{
		Color:        colors.SurfaceVariant,
		BorderRadius: 8,
		Padding:      layout.EdgeInsetsAll(16),
		Child: widgets.Column{
			MainAxisAlignment:  widgets.MainAxisAlignmentStart,
			CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
			MainAxisSize:       widgets.MainAxisSizeMin,
			Children: []core.Widget{
				widgets.Text{Content: "Directory:", Style: graphics.TextStyle{
					Color:      colors.OnSurfaceVariant,
					FontSize:   12,
					FontWeight: graphics.FontWeightBold,
				}},
				widgets.VSpace(4),
				widgets.Text{Content: path, Style: graphics.TextStyle{
					Color:    colors.OnSurface,
					FontSize: 12,
				}},
			},
		},
	}
}

func (s *storageState) appDirectoriesCard(colors theme.ColorScheme) core.Widget {
	dirs := s.appDirs.Get()
	if len(dirs) == 0 {
		return statusCard("Loading directories...", colors)
	}

	var rows []core.Widget
	for name, path := range dirs {
		if len(rows) > 0 {
			rows = append(rows, widgets.VSpace(12))
		}
		rows = append(rows, dirEntry(name, path, colors))
	}

	return widgets.Container{
		Color:        colors.SurfaceVariant,
		BorderRadius: 8,
		Padding:      layout.EdgeInsetsAll(16),
		Child: widgets.Column{
			MainAxisAlignment:  widgets.MainAxisAlignmentStart,
			CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
			MainAxisSize:       widgets.MainAxisSizeMin,
			Children:           rows,
		},
	}
}

// dirEntry displays a directory name and path.
func dirEntry(name, path string, colors theme.ColorScheme) core.Widget {
	return widgets.Column{
		MainAxisAlignment:  widgets.MainAxisAlignmentStart,
		CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
		MainAxisSize:       widgets.MainAxisSizeMin,
		Children: []core.Widget{
			widgets.Text{Content: name + ":", Style: graphics.TextStyle{
				Color:      colors.OnSurfaceVariant,
				FontSize:   12,
				FontWeight: graphics.FontWeightBold,
			}},
			widgets.VSpace(2),
			widgets.Text{Content: path, Style: graphics.TextStyle{
				Color:    colors.OnSurface,
				FontSize: 11,
			}},
		},
	}
}

func (s *storageState) infoRow(label, value string, colors theme.ColorScheme) core.Widget {
	return widgets.Row{
		MainAxisAlignment:  widgets.MainAxisAlignmentSpaceBetween,
		CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
		Children: []core.Widget{
			widgets.Text{Content: label, Style: graphics.TextStyle{
				Color:    colors.OnSurfaceVariant,
				FontSize: 14,
			}},
			widgets.Text{Content: value, Style: graphics.TextStyle{
				Color:      colors.OnSurface,
				FontSize:   14,
				FontWeight: graphics.FontWeightSemibold,
			}},
		},
	}
}

func (s *storageState) pickFile() {
	s.statusText.Set("Opening file picker...")

	go func() {
		result, err := platform.Storage.PickFile(context.Background(), platform.PickFileOptions{
			AllowMultiple: false,
		})
		drift.Dispatch(func() {
			if err != nil {
				s.statusText.Set("Error: " + err.Error())
				return
			}
			if result.Cancelled {
				s.statusText.Set("Operation cancelled")
				return
			}
			if len(result.Files) > 0 {
				file := result.Files[0]
				s.selectedFile.Set(&file)
				s.selectedPath.Set("")
				s.statusText.Set("File selected: " + file.Name)
			}
		})
	}()
}

func (s *storageState) pickDirectory() {
	s.statusText.Set("Opening directory picker...")

	go func() {
		result, err := platform.Storage.PickDirectory(context.Background())
		drift.Dispatch(func() {
			if err != nil {
				s.statusText.Set("Error: " + err.Error())
				return
			}
			if result.Cancelled {
				s.statusText.Set("Operation cancelled")
				return
			}
			s.selectedFile.Set(nil)
			s.selectedPath.Set(result.Path)
			s.statusText.Set("Directory selected")
		})
	}()
}

func (s *storageState) saveFile() {
	s.statusText.Set("Opening save dialog...")

	go func() {
		data := []byte("Hello from Drift!\n\nThis file was saved using the Storage API.")
		result, err := platform.Storage.SaveFile(context.Background(), data, platform.SaveFileOptions{
			SuggestedName: "drift-demo.txt",
			MimeType:      "text/plain",
		})
		drift.Dispatch(func() {
			if err != nil {
				s.statusText.Set("Error: " + err.Error())
				return
			}
			if result.Cancelled {
				s.statusText.Set("Operation cancelled")
				return
			}
			s.selectedPath.Set(result.Path)
			s.statusText.Set("File saved")
		})
	}()
}
