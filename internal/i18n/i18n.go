package i18n

import "strings"

type Locale string

const (
	ZHCN Locale = "zh-CN"
	EN   Locale = "en"
)

type Key string

const (
	KeyAppBrand         Key = "app.brand"
	KeyMenuSettings     Key = "menu.settings"
	KeyMenuLanguage     Key = "menu.language"
	KeyMenuQuit         Key = "menu.quit"
	KeyLanguageZHCN     Key = "language.zh-CN"
	KeyLanguageEN       Key = "language.en"
	KeyStatusLanguage   Key = "status.language"
	KeyStatusSaved      Key = "status.saved"
	KeyStatusSaveFailed Key = "status.save_failed"
	KeyStatusLoadFailed Key = "status.load_failed"
)

type message struct {
	text   string
	params []string
}

var resources = map[Locale]map[Key]message{
	EN: {
		KeyAppBrand:         {text: "PdmView"},
		KeyMenuSettings:     {text: "Settings"},
		KeyMenuLanguage:     {text: "Language"},
		KeyMenuQuit:         {text: "Quit {app}", params: []string{"app"}},
		KeyLanguageZHCN:     {text: "简体中文"},
		KeyLanguageEN:       {text: "English"},
		KeyStatusLanguage:   {text: "Interface language: {language}", params: []string{"language"}},
		KeyStatusSaved:      {text: "Interface language: {language}. Language preference saved.", params: []string{"language"}},
		KeyStatusSaveFailed: {text: "Interface language: {language}. Could not save the language preference; this session continues with the selected language.", params: []string{"language"}},
		KeyStatusLoadFailed: {text: "Interface language: {language}. Preferences could not be read; the default language is in use.", params: []string{"language"}},
	},
	ZHCN: {
		KeyAppBrand:         {text: "PdmView"},
		KeyMenuSettings:     {text: "设置"},
		KeyMenuLanguage:     {text: "语言"},
		KeyMenuQuit:         {text: "退出 {app}", params: []string{"app"}},
		KeyLanguageZHCN:     {text: "简体中文"},
		KeyLanguageEN:       {text: "English"},
		KeyStatusLanguage:   {text: "当前界面语言：{language}", params: []string{"language"}},
		KeyStatusSaved:      {text: "当前界面语言：{language}。语言偏好已保存。", params: []string{"language"}},
		KeyStatusSaveFailed: {text: "当前界面语言：{language}。语言偏好保存失败；本次会话继续使用所选语言。", params: []string{"language"}},
		KeyStatusLoadFailed: {text: "当前界面语言：{language}。偏好文件无法读取，已使用默认语言。", params: []string{"language"}},
	},
}

func (l Locale) Valid() bool {
	return l == ZHCN || l == EN
}

func ParseSaved(value string) (Locale, bool) {
	switch Locale(value) {
	case ZHCN:
		return ZHCN, true
	case EN:
		return EN, true
	default:
		return "", false
	}
}

func Resolve(saved, systemTag string) Locale {
	if locale, ok := ParseSaved(saved); ok {
		return locale
	}
	return FromSystemTag(systemTag)
}

func FromSystemTag(tag string) Locale {
	if primarySubtag(tag) == "zh" {
		return ZHCN
	}
	return EN
}

func LanguageName(locale Locale) string {
	switch locale {
	case ZHCN:
		return Translate(locale, KeyLanguageZHCN, nil)
	default:
		return Translate(EN, KeyLanguageEN, nil)
	}
}

func Translate(locale Locale, key Key, params map[string]string) string {
	if !locale.Valid() {
		locale = EN
	}
	msg, ok := resources[locale][key]
	if !ok {
		msg, ok = resources[EN][key]
	}
	if !ok {
		return string(key)
	}
	text := msg.text
	for _, name := range msg.params {
		text = strings.ReplaceAll(text, "{"+name+"}", params[name])
	}
	return text
}

func primarySubtag(tag string) string {
	tag = strings.TrimSpace(tag)
	if i := strings.IndexAny(tag, "-_"); i >= 0 {
		tag = tag[:i]
	}
	return strings.ToLower(tag)
}
