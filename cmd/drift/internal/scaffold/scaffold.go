package scaffold

// Settings describes the app metadata used for scaffolding.
type Settings struct {
	AppName     string
	AppID       string
	Bundle      string
	Orientation string
	AllowHTTP   bool
	Ejected     bool // If true, skip user-owned files (Swift/Kotlin, project files)
}
