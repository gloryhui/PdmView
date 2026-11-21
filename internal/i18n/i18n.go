package i18n

import "strings"

type Locale string

const (
	ZHCN Locale = "zh-CN"
	EN   Locale = "en"
)

type Key string

const (
	KeyAppBrand          Key = "app.brand"
	KeyMenuSettings      Key = "menu.settings"
	KeyMenuLanguage      Key = "menu.language"
	KeyMenuQuit          Key = "menu.quit"
	KeyLanguageZHCN      Key = "language.zh-CN"
	KeyLanguageEN        Key = "language.en"
	KeyStatusLanguage    Key = "status.language"
	KeyStatusSaved       Key = "status.saved"
	KeyStatusSaveFailed  Key = "status.save_failed"
	KeyStatusLoadFailed  Key = "status.load_failed"
	KeyNavigationHeading Key = "workspace.navigation"
	KeyCanvasHeading     Key = "workspace.canvas"
	KeyDetailsHeading    Key = "workspace.details"
	KeyOpenPDM           Key = "workspace.open_pdm"
	KeyDiagramsHeading   Key = "workspace.diagrams"
	KeyDiagramFallback   Key = "workspace.diagram_fallback"
	KeyLoadingPDM        Key = "workspace.loading_pdm"
	KeyPreparingDiagram  Key = "workspace.preparing_diagram"
	KeyLoadPDMFailed     Key = "workspace.load_pdm_failed"
	KeyNoDiagramFallback Key = "workspace.no_diagram_fallback"
	KeyAutoLayoutStatus  Key = "workspace.auto_layout"
	KeySampleDiagram     Key = "canvas.sample"
	KeyFitSelection      Key = "canvas.fit_selection"
	KeyShowAll           Key = "canvas.show_all"
	KeyResetView         Key = "canvas.reset"
	KeyZoomIn            Key = "canvas.zoom_in"
	KeyZoomOut           Key = "canvas.zoom_out"
)

type message struct {
	text   string
	params []string
}

var resources = map[Locale]map[Key]message{
	EN: {
		KeyAppBrand:          {text: "PdmView"},
		KeyMenuSettings:      {text: "Settings"},
		KeyMenuLanguage:      {text: "Language"},
		KeyMenuQuit:          {text: "Quit {app}", params: []string{"app"}},
		KeyLanguageZHCN:      {text: "简体中文"},
		KeyLanguageEN:        {text: "English"},
		KeyStatusLanguage:    {text: "Interface language: {language}", params: []string{"language"}},
		KeyStatusSaved:       {text: "Interface language: {language}. Language preference saved.", params: []string{"language"}},
		KeyStatusSaveFailed:  {text: "Interface language: {language}. Could not save the language preference; this session continues with the selected language.", params: []string{"language"}},
		KeyStatusLoadFailed:  {text: "Interface language: {language}. Preferences could not be read; the default language is in use.", params: []string{"language"}},
		KeyNavigationHeading: {text: "Navigation"},
		KeyCanvasHeading:     {text: "Diagram canvas"},
		KeyDetailsHeading:    {text: "Details"},
		KeyOpenPDM:           {text: "Open PDM…"},
		KeyDiagramsHeading:   {text: "Diagrams"},
		KeyDiagramFallback:   {text: "Diagram {index}", params: []string{"index"}},
		KeyLoadingPDM:        {text: "Loading PDM…"},
		KeyPreparingDiagram:  {text: "Preparing diagram…"},
		KeyLoadPDMFailed:     {text: "Could not open PDM file"},
		KeyNoDiagramFallback: {text: "No diagrams · Auto layout"},
		KeyAutoLayoutStatus:  {text: "Includes automatically arranged tables"},
		KeySampleDiagram:     {text: "Sample diagram"},
		KeyFitSelection:      {text: "Fit selection"},
		KeyShowAll:           {text: "Show all"},
		KeyResetView:         {text: "Reset view"},
		KeyZoomIn:            {text: "Zoom in"},
		KeyZoomOut:           {text: "Zoom out"},
	},
	ZHCN: {
		KeyAppBrand:          {text: "PdmView"},
		KeyMenuSettings:      {text: "设置"},
		KeyMenuLanguage:      {text: "语言"},
		KeyMenuQuit:          {text: "退出 {app}", params: []string{"app"}},
		KeyLanguageZHCN:      {text: "简体中文"},
		KeyLanguageEN:        {text: "English"},
		KeyStatusLanguage:    {text: "当前界面语言：{language}", params: []string{"language"}},
		KeyStatusSaved:       {text: "当前界面语言：{language}。语言偏好已保存。", params: []string{"language"}},
		KeyStatusSaveFailed:  {text: "当前界面语言：{language}。语言偏好保存失败；本次会话继续使用所选语言。", params: []string{"language"}},
		KeyStatusLoadFailed:  {text: "当前界面语言：{language}。偏好文件无法读取，已使用默认语言。", params: []string{"language"}},
		KeyNavigationHeading: {text: "导航"},
		KeyCanvasHeading:     {text: "图表画布"},
		KeyDetailsHeading:    {text: "详情"},
		KeyOpenPDM:           {text: "打开 PDM…"},
		KeyDiagramsHeading:   {text: "画板"},
		KeyDiagramFallback:   {text: "画板 {index}", params: []string{"index"}},
		KeyLoadingPDM:        {text: "正在加载 PDM…"},
		KeyPreparingDiagram:  {text: "正在准备画板…"},
		KeyLoadPDMFailed:     {text: "无法打开 PDM 文件"},
		KeyNoDiagramFallback: {text: "无画板 · 自动布局"},
		KeyAutoLayoutStatus:  {text: "含自动布局的表"},
		KeySampleDiagram:     {text: "模拟图表"},
		KeyFitSelection:      {text: "适应选中"},
		KeyShowAll:           {text: "显示全部"},
		KeyResetView:         {text: "重置视图"},
		KeyZoomIn:            {text: "放大"},
		KeyZoomOut:           {text: "缩小"},
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
