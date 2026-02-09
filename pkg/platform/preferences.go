package platform

// Preferences provides simple, unencrypted key-value storage using
// platform-native mechanisms (UserDefaults on iOS, SharedPreferences on Android).
// For sensitive data, use SecureStorage instead.
var Preferences = &PreferencesService{
	channel: NewMethodChannel("drift/preferences"),
}

// PreferencesService manages simple key-value preference storage.
type PreferencesService struct {
	channel *MethodChannel
}

// Get retrieves a string value for the given key.
// Returns empty string and nil error if the key doesn't exist.
// Use Contains to distinguish a missing key from a key set to "".
func (p *PreferencesService) Get(key string) (string, error) {
	result, err := p.channel.Invoke("get", map[string]any{
		"key": key,
	})
	if err != nil {
		return "", err
	}

	if result == nil {
		return "", nil
	}

	if m, ok := result.(map[string]any); ok {
		if value, ok := m["value"].(string); ok {
			return value, nil
		}
	}

	return "", nil
}

// Set stores a string value for the given key.
func (p *PreferencesService) Set(key, value string) error {
	_, err := p.channel.Invoke("set", map[string]any{
		"key":   key,
		"value": value,
	})
	return err
}

// Delete removes the value for the given key.
func (p *PreferencesService) Delete(key string) error {
	_, err := p.channel.Invoke("delete", map[string]any{
		"key": key,
	})
	return err
}

// Contains checks if a key exists in preferences.
func (p *PreferencesService) Contains(key string) (bool, error) {
	result, err := p.channel.Invoke("contains", map[string]any{
		"key": key,
	})
	if err != nil {
		return false, err
	}

	if m, ok := result.(map[string]any); ok {
		if exists, ok := m["exists"].(bool); ok {
			return exists, nil
		}
	}

	return false, nil
}

// GetAllKeys returns all keys stored in preferences.
func (p *PreferencesService) GetAllKeys() ([]string, error) {
	result, err := p.channel.Invoke("getAllKeys", nil)
	if err != nil {
		return nil, err
	}

	if m, ok := result.(map[string]any); ok {
		if keys, ok := m["keys"].([]any); ok {
			strKeys := make([]string, 0, len(keys))
			for _, k := range keys {
				if str, ok := k.(string); ok {
					strKeys = append(strKeys, str)
				}
			}
			return strKeys, nil
		}
	}

	return []string{}, nil
}

// DeleteAll removes all values from preferences.
func (p *PreferencesService) DeleteAll() error {
	_, err := p.channel.Invoke("deleteAll", nil)
	return err
}
