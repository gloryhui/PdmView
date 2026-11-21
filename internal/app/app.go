package app

import (
	"pdmview/internal/i18n"

	"github.com/egoist/mygo"
)

const (
	MenuIDApp          = "pdmview.menu.app"
	MenuIDQuit         = "pdmview.menu.quit"
	MenuIDSettings     = "pdmview.menu.settings"
	MenuIDLanguage     = "pdmview.menu.language"
	MenuIDLanguageZHCN = "pdmview.language.zh-CN"
	MenuIDLanguageEN   = "pdmview.language.en"
)

type Status uint8

const (
	StatusLanguage Status = iota
	StatusSaved
	StatusSaveFailed
	StatusLoadFailed
)

type State struct {
	store  *Store
	locale i18n.Locale
	status Status
}

func New(store *Store, systemTag string) *State {
	if store == nil {
		store = NewStore("")
	}
	state := &State{
		store:  store,
		locale: i18n.Resolve("", systemTag),
		status: StatusLanguage,
	}
	snapshot, err := store.Load()
	if err != nil {
		state.status = StatusLoadFailed
		return state
	}
	saved := ""
	if snapshot.HasLanguage {
		saved = string(snapshot.Language)
	}
	state.locale = i18n.Resolve(saved, systemTag)
	return state
}

func (s *State) Locale() i18n.Locale {
	return s.locale
}

func (s *State) Status() Status {
	return s.status
}

func (s *State) Text(key i18n.Key, params map[string]string) string {
	return i18n.Translate(s.locale, key, params)
}

func (s *State) LanguageName() string {
	return i18n.LanguageName(s.locale)
}

func (s *State) StatusText() string {
	params := map[string]string{"language": s.LanguageName()}
	switch s.status {
	case StatusSaved:
		return s.Text(i18n.KeyStatusSaved, params)
	case StatusSaveFailed:
		return s.Text(i18n.KeyStatusSaveFailed, params)
	case StatusLoadFailed:
		return s.Text(i18n.KeyStatusLoadFailed, params)
	default:
		return s.Text(i18n.KeyStatusLanguage, params)
	}
}

func (s *State) SelectLanguage(locale i18n.Locale) {
	if !locale.Valid() {
		return
	}
	s.locale = locale
	if err := s.store.Save(locale); err != nil {
		s.status = StatusSaveFailed
		return
	}
	s.status = StatusSaved
}

func (s *State) Menu(onChanged func()) *mygo.Menu {
	brand := s.Text(i18n.KeyAppBrand, nil)
	selectLanguage := func(locale i18n.Locale) func(*mygo.MenuItem, *mygo.Window) {
		return func(*mygo.MenuItem, *mygo.Window) {
			s.SelectLanguage(locale)
			if onChanged != nil {
				onChanged()
			}
		}
	}

	return mygo.NewMenu([]*mygo.MenuItem{
		{
			ID:    MenuIDApp,
			Label: brand,
			Submenu: []*mygo.MenuItem{
				{
					ID:    MenuIDQuit,
					Role:  mygo.RoleQuit,
					Label: s.Text(i18n.KeyMenuQuit, map[string]string{"app": brand}),
				},
			},
		},
		{
			ID:    MenuIDSettings,
			Label: s.Text(i18n.KeyMenuSettings, nil),
			Submenu: []*mygo.MenuItem{
				{
					ID:    MenuIDLanguage,
					Label: s.Text(i18n.KeyMenuLanguage, nil),
					Submenu: []*mygo.MenuItem{
						{
							ID:      MenuIDLanguageZHCN,
							Label:   s.Text(i18n.KeyLanguageZHCN, nil),
							Type:    mygo.MenuItemRadio,
							Checked: s.locale == i18n.ZHCN,
							Click:   selectLanguage(i18n.ZHCN),
						},
						{
							ID:      MenuIDLanguageEN,
							Label:   s.Text(i18n.KeyLanguageEN, nil),
							Type:    mygo.MenuItemRadio,
							Checked: s.locale == i18n.EN,
							Click:   selectLanguage(i18n.EN),
						},
					},
				},
			},
		},
	})
}
