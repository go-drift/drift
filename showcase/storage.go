package main

import (
	"github.com/go-drift/drift/pkg/core"
	"github.com/go-drift/drift/pkg/drift"
	"github.com/go-drift/drift/pkg/platform"
	"github.com/go-drift/drift/pkg/graphics"
	"github.com/go-drift/drift/pkg/theme"
	"github.com/go-drift/drift/pkg/widgets"
)

// buildStoragePage creates a stateful widget for storage demos.
func buildStoragePage(ctx core.BuildContext) core.Widget {
	return storagePage{}
}

type storagePage struct{}

func (p storagePage) CreateElement() core.Element {
	return core.NewStatefulElement(p, nil)
}

func (p storagePage) Key() any {
	return nil
}

func (p storagePage) CreateState() core.State {
	return &storageState{}
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
		if path, err := platform.GetAppDirectory(platform.AppDirectoryDocuments); err == nil {
			dirs["Documents"] = path
		}
		if path, err := platform.GetAppDirectory(platform.AppDirectoryCache); err == nil {
			dirs["Cache"] = path
		}
		if path, err := platform.GetAppDirectory(platform.AppDirectoryTemp); err == nil {
			dirs["Temp"] = path
		}
		drift.Dispatch(func() {
			s.appDirs.Set(dirs)
		})
	}()

	// Listen for storage results
	go func() {
		for result := range platform.StorageResults() {
			drift.Dispatch(func() {
				if result.Cancelled {
					s.statusText.Set("Operation cancelled")
					return
				}
				if result.Error != "" {
					s.statusText.Set("Error: " + result.Error)
					return
				}

				switch result.Type {
				case "pickFile":
					if len(result.Files) > 0 {
						file := result.Files[0]
						s.selectedFile.Set(&file)
						s.selectedPath.Set("")
						s.statusText.Set("File selected: " + file.Name)
					}
				case "pickDirectory":
					s.selectedFile.Set(nil)
					s.selectedPath.Set(result.Path)
					s.statusText.Set("Directory selected")
				case "saveFile":
					s.selectedPath.Set(result.Path)
					s.statusText.Set("File saved")
				}
			})
		}
	}()
}

func (s *storageState) Build(ctx core.BuildContext) core.Widget {
	_, colors, _ := theme.UseTheme(ctx)

	return demoPage(ctx, "Storage",
		sectionTitle("File Picker", colors),
		widgets.VSpace(12),
		widgets.TextOf("Pick files and directories:", labelStyle(colors)),
		widgets.VSpace(16),

		widgets.ButtonOf("Pick File", func() {
			s.pickFile()
		}).WithColor(colors.Primary, colors.OnPrimary),
		widgets.VSpace(12),

		widgets.ButtonOf("Pick Directory", func() {
			s.pickDirectory()
		}).WithColor(colors.Secondary, colors.OnSecondary),
		widgets.VSpace(12),

		widgets.ButtonOf("Save File", func() {
			s.saveFile()
		}).WithColor(colors.Tertiary, colors.OnTertiary),
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
			Color: colors.SurfaceVariant,
			ChildWidget: widgets.PaddingAll(16,
				widgets.TextOf("No item selected", graphics.TextStyle{
					Color:    colors.OnSurfaceVariant,
					FontSize: 14,
				}),
			),
		}
	}

	if file != nil {
		return widgets.Container{
			Color: colors.SurfaceVariant,
			ChildWidget: widgets.PaddingAll(16,
				widgets.Column{
					MainAxisAlignment:  widgets.MainAxisAlignmentStart,
					CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
					MainAxisSize:       widgets.MainAxisSizeMin,
					ChildrenWidgets: []core.Widget{
						s.infoRow("Name", file.Name, colors),
						widgets.VSpace(8),
						s.infoRow("MIME Type", file.MimeType, colors),
						widgets.VSpace(8),
						s.infoRow("Size", formatSize(file.Size), colors),
						widgets.VSpace(8),
						widgets.TextOf("Path:", graphics.TextStyle{
							Color:      colors.OnSurfaceVariant,
							FontSize:   12,
							FontWeight: graphics.FontWeightBold,
						}),
						widgets.VSpace(4),
						widgets.TextOf(file.Path, graphics.TextStyle{
							Color:    colors.OnSurface,
							FontSize: 12,
						}),
					},
				},
			),
		}
	}

	return widgets.Container{
		Color: colors.SurfaceVariant,
		ChildWidget: widgets.PaddingAll(16,
			widgets.Column{
				MainAxisAlignment:  widgets.MainAxisAlignmentStart,
				CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
				MainAxisSize:       widgets.MainAxisSizeMin,
				ChildrenWidgets: []core.Widget{
					widgets.TextOf("Directory:", graphics.TextStyle{
						Color:      colors.OnSurfaceVariant,
						FontSize:   12,
						FontWeight: graphics.FontWeightBold,
					}),
					widgets.VSpace(4),
					widgets.TextOf(path, graphics.TextStyle{
						Color:    colors.OnSurface,
						FontSize: 12,
					}),
				},
			},
		),
	}
}

func (s *storageState) appDirectoriesCard(colors theme.ColorScheme) core.Widget {
	dirs := s.appDirs.Get()

	if len(dirs) == 0 {
		return widgets.Container{
			Color: colors.SurfaceVariant,
			ChildWidget: widgets.PaddingAll(16,
				widgets.TextOf("Loading directories...", graphics.TextStyle{
					Color:    colors.OnSurfaceVariant,
					FontSize: 14,
				}),
			),
		}
	}

	rows := make([]core.Widget, 0)
	for name, path := range dirs {
		if len(rows) > 0 {
			rows = append(rows, widgets.VSpace(12))
		}
		rows = append(rows,
			widgets.Column{
				MainAxisAlignment:  widgets.MainAxisAlignmentStart,
				CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
				MainAxisSize:       widgets.MainAxisSizeMin,
				ChildrenWidgets: []core.Widget{
					widgets.TextOf(name+":", graphics.TextStyle{
						Color:      colors.OnSurfaceVariant,
						FontSize:   12,
						FontWeight: graphics.FontWeightBold,
					}),
					widgets.VSpace(2),
					widgets.TextOf(path, graphics.TextStyle{
						Color:    colors.OnSurface,
						FontSize: 11,
					}),
				},
			},
		)
	}

	return widgets.Container{
		Color: colors.SurfaceVariant,
		ChildWidget: widgets.PaddingAll(16,
			widgets.Column{
				MainAxisAlignment:  widgets.MainAxisAlignmentStart,
				CrossAxisAlignment: widgets.CrossAxisAlignmentStart,
				MainAxisSize:       widgets.MainAxisSizeMin,
				ChildrenWidgets:    rows,
			},
		),
	}
}

func (s *storageState) infoRow(label, value string, colors theme.ColorScheme) core.Widget {
	return widgets.Row{
		MainAxisAlignment:  widgets.MainAxisAlignmentSpaceBetween,
		CrossAxisAlignment: widgets.CrossAxisAlignmentCenter,
		ChildrenWidgets: []core.Widget{
			widgets.TextOf(label, graphics.TextStyle{
				Color:    colors.OnSurfaceVariant,
				FontSize: 14,
			}),
			widgets.TextOf(value, graphics.TextStyle{
				Color:      colors.OnSurface,
				FontSize:   14,
				FontWeight: graphics.FontWeightSemibold,
			}),
		},
	}
}

func (s *storageState) pickFile() {
	s.statusText.Set("Opening file picker...")

	err := platform.PickFile(platform.PickFileOptions{
		AllowMultiple: false,
	})
	if err != nil {
		s.statusText.Set("Error: " + err.Error())
	}
}

func (s *storageState) pickDirectory() {
	s.statusText.Set("Opening directory picker...")

	err := platform.PickDirectory()
	if err != nil {
		s.statusText.Set("Error: " + err.Error())
	}
}

func (s *storageState) saveFile() {
	s.statusText.Set("Opening save dialog...")

	data := []byte("Hello from Drift!\n\nThis file was saved using the Storage API.")
	err := platform.SaveFile(data, platform.SaveFileOptions{
		SuggestedName: "drift-demo.txt",
		MimeType:      "text/plain",
	})
	if err != nil {
		s.statusText.Set("Error: " + err.Error())
	}
}
