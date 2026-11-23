package app

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"pdmview/internal/i18n"

	"github.com/egoist/mygo"
)

func TestStateLanguageResolutionAndPreferencePriority(t *testing.T) {
	path := filepath.Join(t.TempDir(), "PdmView", "preferences.json")
	store := NewStore(path)

	if got := New(store, "zh-Hans-CN").Locale(); got != i18n.ZHCN {
		t.Fatalf("missing prefs with Chinese system locale = %q", got)
	}
	if got := New(store, "ja-JP").Locale(); got != i18n.EN {
		t.Fatalf("missing prefs with non-Chinese system locale = %q", got)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"language":"zh-CN"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := New(store, "en-US").Locale(); got != i18n.ZHCN {
		t.Fatalf("saved language did not win: %q", got)
	}

	if err := os.WriteFile(path, []byte(`{"language":"fr"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := New(store, "en-US").Locale(); got != i18n.EN {
		t.Fatalf("invalid saved language did not recover to system default: %q", got)
	}
}

func TestStateRecoversMalformedPreferencesIntoRetryableFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "PdmView", "preferences.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	before := []byte(`{"language":`)
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)
	state := New(store, "en-US")
	if state.Status() != StatusLoadFailed {
		t.Fatalf("startup status = %v, want load failure", state.Status())
	}
	if state.Locale() != i18n.EN {
		t.Fatalf("startup locale = %q", state.Locale())
	}
	if got, want := state.StatusText(), "Interface language: English. Preferences could not be read; the default language is in use."; got != want {
		t.Fatalf("startup status text = %q, want %q", got, want)
	}

	state.SelectLanguage(i18n.ZHCN)
	if state.Locale() != i18n.ZHCN {
		t.Fatalf("session locale = %q, want selected zh-CN", state.Locale())
	}
	if state.Status() != StatusSaveFailed {
		t.Fatalf("save status = %v, want failure", state.Status())
	}
	if got, want := state.StatusText(), "当前界面语言：简体中文。语言偏好保存失败；本次会话继续使用所选语言。"; got != want {
		t.Fatalf("save failure status text = %q, want %q", got, want)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(after, before) {
		t.Fatalf("malformed preferences changed:\nbefore=%q\nafter=%q", before, after)
	}
}

func TestSameLanguageSelectionIsPersisted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "PdmView", "preferences.json")
	state := New(NewStore(path), "en-US")
	if state.Locale() != i18n.EN {
		t.Fatalf("initial locale = %q", state.Locale())
	}

	state.SelectLanguage(i18n.EN)
	if state.Status() != StatusSaved {
		t.Fatalf("status = %v, want saved", state.Status())
	}
	snapshot, err := NewStore(path).Load()
	if err != nil || !snapshot.HasLanguage || snapshot.Language != i18n.EN {
		t.Fatalf("saved same language: snapshot=%+v err=%v", snapshot, err)
	}
}

func TestSaveFailureKeepsSessionAndCanRetry(t *testing.T) {
	path := filepath.Join(t.TempDir(), "PdmView", "preferences.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	before := []byte("{\n  \"language\": \"en\",\n  \"keep\": \"value\"\n}\n")
	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}
	store := NewStore(path)
	state := New(store, "en-US")
	failRename := true
	store.replace = func(oldpath, newpath string) error {
		if failRename {
			return errors.New("injected rename failure")
		}
		return os.Rename(oldpath, newpath)
	}

	state.SelectLanguage(i18n.ZHCN)
	if state.Locale() != i18n.ZHCN || state.Status() != StatusSaveFailed {
		t.Fatalf("after failure: locale=%q status=%v", state.Locale(), state.Status())
	}
	afterFailure, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(afterFailure, before) {
		t.Fatalf("failed save changed the file: %q", afterFailure)
	}

	failRename = false
	state.SelectLanguage(i18n.ZHCN)
	if state.Status() != StatusSaved {
		t.Fatalf("retry status = %v, want saved", state.Status())
	}
	snapshot, err := NewStore(path).Load()
	if err != nil || !snapshot.HasLanguage || snapshot.Language != i18n.ZHCN {
		t.Fatalf("retry snapshot=%+v err=%v", snapshot, err)
	}
	if _, ok := snapshot.Values["keep"]; !ok {
		t.Fatal("unrelated setting was not preserved on retry")
	}
}

func TestMenuLabelsIDsAndCheckedState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "PdmView", "preferences.json")
	state := New(NewStore(path), "en-US")
	changed := 0
	menu := state.Menu(func() { changed++ })

	if got := menu.Items()[0].Label; got != "PdmView" {
		t.Fatalf("app menu label = %q", got)
	}
	quit := menu.ItemByID(MenuIDQuit)
	if quit == nil || quit.Role != mygo.RoleQuit || quit.Label != "Quit PdmView" {
		t.Fatalf("quit item = %+v", quit)
	}
	if got := menu.ItemByID(MenuIDSettings).Label; got != "Settings" {
		t.Fatalf("settings label = %q", got)
	}
	if got := menu.ItemByID(MenuIDLanguage).Label; got != "Language" {
		t.Fatalf("language label = %q", got)
	}
	if zh := menu.ItemByID(MenuIDLanguageZHCN); zh == nil || zh.Label != "简体中文" || zh.Checked {
		t.Fatalf("zh-CN item = %+v", zh)
	}
	if en := menu.ItemByID(MenuIDLanguageEN); en == nil || en.Label != "English" || !en.Checked {
		t.Fatalf("en item = %+v", en)
	}

	menu.ItemByID(MenuIDLanguageZHCN).Click(nil, nil)
	if changed != 1 || state.Locale() != i18n.ZHCN || state.Status() != StatusSaved {
		t.Fatalf("selection: changed=%d locale=%q status=%v", changed, state.Locale(), state.Status())
	}

	zhMenu := state.Menu(nil)
	if got := zhMenu.ItemByID(MenuIDSettings).Label; got != "设置" {
		t.Fatalf("Chinese settings label = %q", got)
	}
	if got := zhMenu.ItemByID(MenuIDLanguage).Label; got != "语言" {
		t.Fatalf("Chinese language label = %q", got)
	}
	if got := zhMenu.ItemByID(MenuIDQuit).Label; got != "退出 PdmView" {
		t.Fatalf("Chinese quit label = %q", got)
	}
	if item := zhMenu.ItemByID(MenuIDLanguageZHCN); item == nil || !item.Checked {
		t.Fatalf("Chinese checked state = %+v", item)
	}
}

func TestThemeMenuRestoresPreferenceAndPreservesLanguage(t *testing.T) {
	store := NewStore(filepath.Join(t.TempDir(), "preferences.json"))
	state := New(store, "zh-Hans")
	state.SelectLanguage(i18n.EN)
	changed := 0
	menu := state.Menu(func() { changed++ })
	if !menu.ItemByID(MenuIDThemeSystem).Checked {
		t.Fatal("expected the system appearance to be selected by default")
	}
	menu.ItemByID(MenuIDThemeLight).Click(nil, nil)
	if changed != 1 || state.Theme() != mygo.ThemeLight {
		t.Fatalf("selection: refreshes=%d theme=%q", changed, state.Theme())
	}
	reloaded := New(store, "zh-Hans")
	menu = reloaded.Menu(nil)
	if reloaded.Locale() != i18n.EN || !menu.ItemByID(MenuIDThemeLight).Checked || menu.ItemByID(MenuIDThemeSystem).Checked {
		t.Fatalf("reloaded choices: locale=%q theme=%q", reloaded.Locale(), reloaded.Theme())
	}
	menu.ItemByID(MenuIDThemeDark).Click(nil, nil)
	reloaded.SelectLanguage(i18n.ZHCN)
	if got := New(store, "en").Theme(); got != mygo.ThemeDark {
		t.Fatalf("language change dropped the selected theme: %q", got)
	}
}
