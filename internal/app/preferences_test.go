package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"pdmview/internal/i18n"
)

func TestSavePreservesUnrelatedValuesAndReplacesInvalidLanguage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "PdmView", "preferences.json")
	store := NewStore(path)
	initial := []byte(`{
  "language": "fr",
  "window": {"width": 1200, "maximized": false},
  "recent": ["a.pdm", "b.pdm"],
  "count": 3
}
`)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, initial, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := store.Save(i18n.ZHCN); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(data, &values); err != nil {
		t.Fatal(err)
	}
	if got := string(values["language"]); got != `"zh-CN"` {
		t.Fatalf("language = %s", got)
	}
	var window struct {
		Width     int  `json:"width"`
		Maximized bool `json:"maximized"`
	}
	if err := json.Unmarshal(values["window"], &window); err != nil || window.Width != 1200 || window.Maximized {
		t.Fatalf("window value = %s, err=%v", values["window"], err)
	}
	var recent []string
	if err := json.Unmarshal(values["recent"], &recent); err != nil || !slices.Equal(recent, []string{"a.pdm", "b.pdm"}) {
		t.Fatalf("recent value = %s, err=%v", values["recent"], err)
	}
	var count int
	if err := json.Unmarshal(values["count"], &count); err != nil || count != 3 {
		t.Fatalf("count value = %s, err=%v", values["count"], err)
	}
}

func TestMalformedAndNonObjectPreferencesAreNotOverwritten(t *testing.T) {
	tests := []struct {
		name string
		data string
		err  error
	}{
		{name: "malformed", data: `{"language":"en"`, err: ErrMalformedPreferences},
		{name: "array", data: `["en"]`, err: ErrNotObject},
		{name: "null", data: `null`, err: ErrNotObject},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "PdmView", "preferences.json")
			store := NewStore(path)
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			before := []byte(tc.data)
			if err := os.WriteFile(path, before, 0o600); err != nil {
				t.Fatal(err)
			}

			if err := store.Save(i18n.EN); !errors.Is(err, tc.err) {
				t.Fatalf("Save error = %v, want %v", err, tc.err)
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(after, before) {
				t.Fatalf("preferences changed:\nbefore=%q\nafter=%q", before, after)
			}
			assertNoTemporaryPreferences(t, filepath.Dir(path))
		})
	}
}

func TestRenameFailurePreservesPreviousBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "PdmView", "preferences.json")
	store := NewStore(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	before := []byte("{\n  \"language\": \"en\",\n  \"keep\": true\n}\n")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	store.replace = func(string, string) error {
		return errors.New("injected rename failure")
	}

	if err := store.Save(i18n.ZHCN); err == nil {
		t.Fatal("Save succeeded after injected rename failure")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(after, before) {
		t.Fatalf("previous bytes changed:\nbefore=%q\nafter=%q", before, after)
	}
	assertNoTemporaryPreferences(t, filepath.Dir(path))
}

func assertNoTemporaryPreferences(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != preferencesFile {
			t.Fatalf("unexpected temporary file remains: %s", entry.Name())
		}
	}
}

func TestLoadLanguageValidationAndMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "PdmView", "preferences.json")
	store := NewStore(path)
	snapshot, err := store.Load()
	if err != nil || snapshot.HasLanguage {
		t.Fatalf("missing file: snapshot=%+v err=%v", snapshot, err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"language":"zh-CN","other":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.Load()
	if err != nil || !snapshot.HasLanguage || snapshot.Language != i18n.ZHCN {
		t.Fatalf("valid language: snapshot=%+v err=%v", snapshot, err)
	}

	if err := os.WriteFile(path, []byte(`{"language":"fr","other":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err = store.Load()
	if err != nil || snapshot.HasLanguage {
		t.Fatalf("invalid language: snapshot=%+v err=%v", snapshot, err)
	}
}
