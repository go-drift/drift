package skia

// TextSpanData describes a single styled text span for rich paragraph creation.
type TextSpanData struct {
	Text            string
	Family          string
	Size            float32
	Weight          int
	Style           int
	Color           uint32
	Decoration      int
	DecorationColor uint32
	DecorationStyle int
	LetterSpacing   float32
	WordSpacing     float32
	Height          float32
	HasBackground   bool
	BackgroundColor uint32
}
