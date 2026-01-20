package platform

// ClipboardService provides access to the system clipboard.
var Clipboard = &ClipboardService{
	channel: NewMethodChannel("drift/clipboard"),
}

// ClipboardService manages clipboard operations.
type ClipboardService struct {
	channel *MethodChannel
}

// ClipboardData represents data on the clipboard.
type ClipboardData struct {
	Text string `json:"text,omitempty"`
}

// GetText retrieves text from the clipboard.
// Returns empty string if clipboard is empty or contains non-text data.
func (c *ClipboardService) GetText() (string, error) {
	result, err := c.channel.Invoke("getText", nil)
	if err != nil {
		return "", err
	}

	if result == nil {
		return "", nil
	}

	// Result should be a map with "text" key
	if m, ok := result.(map[string]any); ok {
		if text, ok := m["text"].(string); ok {
			return text, nil
		}
	}

	// Or directly a string
	if text, ok := result.(string); ok {
		return text, nil
	}

	return "", nil
}

// SetText copies text to the clipboard.
func (c *ClipboardService) SetText(text string) error {
	_, err := c.channel.Invoke("setText", map[string]any{
		"text": text,
	})
	return err
}

// HasText returns true if the clipboard contains text.
func (c *ClipboardService) HasText() (bool, error) {
	result, err := c.channel.Invoke("hasText", nil)
	if err != nil {
		return false, err
	}

	if b, ok := result.(bool); ok {
		return b, nil
	}

	return false, nil
}

// Clear removes all data from the clipboard.
func (c *ClipboardService) Clear() error {
	_, err := c.channel.Invoke("clear", nil)
	return err
}
