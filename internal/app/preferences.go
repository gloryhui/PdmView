package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"pdmview/internal/i18n"
)

const (
	configDirectory = "PdmView"
	preferencesFile = "preferences.json"
	languageField   = "language"
)

var (
	ErrMalformedPreferences = errors.New("preferences: malformed JSON")
	ErrNotObject            = errors.New("preferences: root is not an object")
	ErrPreferencesPath      = errors.New("preferences: path is unavailable")
)

type Snapshot struct {
	Values      map[string]json.RawMessage
	Language    i18n.Locale
	HasLanguage bool
}

type Store struct {
	path    string
	replace func(string, string) error
}

func NewStore(path string) *Store {
	return &Store{
		path:    path,
		replace: os.Rename,
	}
}

func PreferencesPath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("%w: %w", ErrPreferencesPath, err)
	}
	return filepath.Join(root, configDirectory, preferencesFile), nil
}

func (s *Store) Load() (Snapshot, error) {
	if s.path == "" {
		return Snapshot{}, ErrPreferencesPath
	}
	raw, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Snapshot{}, nil
	}
	if err != nil {
		return Snapshot{}, fmt.Errorf("preferences: read %s: %w", s.path, err)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return Snapshot{}, fmt.Errorf("%w: empty file", ErrMalformedPreferences)
	}
	if trimmed[0] != '{' {
		return Snapshot{}, ErrNotObject
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &values); err != nil {
		return Snapshot{}, fmt.Errorf("%w: %v", ErrMalformedPreferences, err)
	}
	if values == nil {
		values = map[string]json.RawMessage{}
	}
	snapshot := Snapshot{Values: values}
	if rawLanguage, ok := values[languageField]; ok {
		var value string
		if json.Unmarshal(rawLanguage, &value) == nil {
			if locale, valid := i18n.ParseSaved(value); valid {
				snapshot.Language = locale
				snapshot.HasLanguage = true
			}
		}
	}
	return snapshot, nil
}

func (s *Store) Save(locale i18n.Locale) error {
	if !locale.Valid() {
		return fmt.Errorf("preferences: invalid language %q", locale)
	}
	if s.path == "" {
		return ErrPreferencesPath
	}
	snapshot, err := s.Load()
	if err != nil {
		return fmt.Errorf("preferences: refusing unsafe overwrite: %w", err)
	}
	values := make(map[string]json.RawMessage, len(snapshot.Values)+1)
	for key, value := range snapshot.Values {
		values[key] = value
	}
	encodedLanguage, err := json.Marshal(string(locale))
	if err != nil {
		return fmt.Errorf("preferences: encode language: %w", err)
	}
	values[languageField] = encodedLanguage
	data, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return fmt.Errorf("preferences: encode document: %w", err)
	}
	data = append(data, '\n')

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("preferences: create %s: %w", dir, err)
	}

	temp, err := os.CreateTemp(dir, ".preferences-*")
	if err != nil {
		return fmt.Errorf("preferences: create temporary file: %w", err)
	}
	tempPath := temp.Name()
	closed := false
	defer func() {
		if !closed {
			_ = temp.Close()
		}
		_ = os.Remove(tempPath)
	}()
	if _, err := temp.Write(data); err != nil {
		return fmt.Errorf("preferences: write temporary file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("preferences: sync temporary file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("preferences: close temporary file: %w", err)
	}
	closed = true
	if err := s.replace(tempPath, s.path); err != nil {
		return fmt.Errorf("preferences: replace %s: %w", s.path, err)
	}
	return nil
}
