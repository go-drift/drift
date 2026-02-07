package scaffold

// Settings describes the app metadata used for scaffolding.
type Settings struct {
	AppName string
	AppID   string
	Bundle  string
	Ejected bool // If true, skip user-owned files (Swift/Kotlin, project files)
}
