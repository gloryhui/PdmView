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
	KeySearch            Key = "workspace.search"
	KeySearchResults     Key = "workspace.search_results"
	KeySearchNoResults   Key = "workspace.search_no_results"
	KeyDiagramsHeading   Key = "workspace.diagrams"
	KeyModelRoot         Key = "workspace.model_root"
	KeyDiagramFallback   Key = "workspace.diagram_fallback"
	KeyNotOnDiagram      Key = "workspace.not_on_diagram"
	KeyLoadingPDM        Key = "workspace.loading_pdm"
	KeyPreparingDiagram  Key = "workspace.preparing_diagram"
	KeyLoadPDMFailed     Key = "workspace.load_pdm_failed"
	KeyNoDiagramFallback Key = "workspace.no_diagram_fallback"
	KeyAutoLayoutStatus  Key = "workspace.auto_layout"
	KeySampleDiagram     Key = "canvas.sample"
	KeyFitSelection      Key = "canvas.fit_selection"
	KeyFitShort          Key = "canvas.fit_short"
	KeyShowAll           Key = "canvas.show_all"
	KeyShowAllShort      Key = "canvas.show_all_short"
	KeyResetView         Key = "canvas.reset"
	KeyResetShort        Key = "canvas.reset_short"
	KeyZoomIn            Key = "canvas.zoom_in"
	KeyZoomOut           Key = "canvas.zoom_out"
	KeyDetailCode        Key = "details.code"
	KeyDetailComment     Key = "details.comment"
	KeyDetailPackage     Key = "details.package"
	KeyDetailColumns     Key = "details.columns"
	KeyDetailKeys        Key = "details.keys"
	KeyDetailIndexes     Key = "details.indexes"
	KeyDetailForeignKeys Key = "details.foreign_keys"
	KeyDetailType        Key = "details.type"
	KeyDetailLength      Key = "details.length"
	KeyDetailPrecision   Key = "details.precision"
	KeyDetailScale       Key = "details.scale"
	KeyDetailNull        Key = "details.null"
	KeyDetailDefault     Key = "details.default"
	KeyDetailIdentity    Key = "details.identity"
	KeyDetailPK          Key = "details.pk"
	KeyDetailFK          Key = "details.fk"
	KeyDetailUnique      Key = "details.unique"
	KeyDetailSort        Key = "details.sort"
	KeyDetailPrimary     Key = "details.primary"
	KeyDetailParent      Key = "details.parent"
	KeyDetailChild       Key = "details.child"
	KeyDetailOnUpdate    Key = "details.on_update"
	KeyDetailOnDelete    Key = "details.on_delete"
	KeyDetailCardinality Key = "details.cardinality"
	KeyDetailYes         Key = "details.yes"
	KeyDetailNo          Key = "details.no"
	KeyDetailUnresolved  Key = "details.unresolved"
	KeyCopyTable         Key = "copy.table"
	KeyCopySucceeded     Key = "copy.succeeded"
	KeyCopyFailed        Key = "copy.failed"

	KeyTablePropertiesTitle     Key = "table_properties.title"
	KeyTablePropertiesGeneral   Key = "table_properties.general"
	KeyTablePropertiesPreview   Key = "table_properties.preview"
	KeyTablePropertiesClose     Key = "table_properties.close"
	KeyTablePropertiesName      Key = "table_properties.name"
	KeyTablePropertiesFlags     Key = "table_properties.flags"
	KeyTablePropertiesMandatory Key = "table_properties.mandatory"

	KeyDatabaseSQL              Key = "database.sql"
	KeyPreviewBack              Key = "preview.back"
	KeyPreviewLoading           Key = "preview.loading"
	KeyPreviewTables            Key = "preview.tables"
	KeyPreviewColumns           Key = "preview.columns"
	KeyPreviewKeys              Key = "preview.foreign_keys"
	KeyPreviewSummary           Key = "preview.summary"
	KeyPreviewDiagnostics       Key = "preview.diagnostics"
	KeyPreviewSQL               Key = "preview.sql"
	KeyPreviewError             Key = "preview.error"
	KeyPreviewWarning           Key = "preview.warning"
	KeyPreviewLine              Key = "preview.line"
	KeyPreviewDiagnosticHeading Key = "preview.diagnostic_heading"
	KeyPreviewSchemaIssue       Key = "preview.issue.schema"
	KeyPreviewColumnIssue       Key = "preview.issue.column"
	KeyPreviewDefaultIssue      Key = "preview.issue.default"
	KeyPreviewIndexIssue        Key = "preview.issue.index"
	KeyPreviewForeignKeyIssue   Key = "preview.issue.foreign_key"
	KeyPreviewParserIssue       Key = "preview.issue.parser"
	KeyPreviewDatabaseIssue     Key = "preview.issue.database"
	KeyPreviewCopySQL           Key = "preview.copy_sql"
	KeyPreviewExportSQL         Key = "preview.export_sql"
	KeyPreviewExporting         Key = "preview.exporting"
	KeyPreviewSQLCopied         Key = "preview.sql_copied"
	KeyPreviewSQLCopyFailed     Key = "preview.sql_copy_failed"
	KeyPreviewSQLExported       Key = "preview.sql_exported"
	KeyPreviewSQLExportFailed   Key = "preview.sql_export_failed"
	KeyPreviewSaveTitle         Key = "preview.save_title"
	KeyPreviewSaveFilter        Key = "preview.save_filter"
	KeyCopyCreateTable          Key = "copy.create_table"
	KeyCopyCreateTableFailed    Key = "copy.create_table_failed"
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
		KeySearch:            {text: "Search"},
		KeySearchResults:     {text: "Search results"},
		KeySearchNoResults:   {text: "No results"},
		KeyDiagramsHeading:   {text: "Diagrams"},
		KeyModelRoot:         {text: "Model"},
		KeyDiagramFallback:   {text: "Diagram {index}", params: []string{"index"}},
		KeyNotOnDiagram:      {text: "Not on a diagram"},
		KeyLoadingPDM:        {text: "Loading PDM…"},
		KeyPreparingDiagram:  {text: "Preparing diagram…"},
		KeyLoadPDMFailed:     {text: "Could not open PDM file"},
		KeyNoDiagramFallback: {text: "No diagrams · Auto layout"},
		KeyAutoLayoutStatus:  {text: "Includes automatically arranged tables"},
		KeySampleDiagram:     {text: "Sample diagram"},
		KeyFitSelection:      {text: "Fit selection"},
		KeyFitShort:          {text: "Fit"},
		KeyShowAll:           {text: "Show all"},
		KeyShowAllShort:      {text: "All"},
		KeyResetView:         {text: "Reset view"},
		KeyResetShort:        {text: "Reset"},
		KeyZoomIn:            {text: "Zoom in"},
		KeyZoomOut:           {text: "Zoom out"},
		KeyDetailCode:        {text: "Code"},
		KeyDetailComment:     {text: "Comment"},
		KeyDetailPackage:     {text: "Package"},
		KeyDetailColumns:     {text: "Columns"},
		KeyDetailKeys:        {text: "Keys"},
		KeyDetailIndexes:     {text: "Indexes"},
		KeyDetailForeignKeys: {text: "Foreign keys"},
		KeyDetailType:        {text: "Type"},
		KeyDetailLength:      {text: "Length"},
		KeyDetailPrecision:   {text: "Precision"},
		KeyDetailScale:       {text: "Scale"},
		KeyDetailNull:        {text: "NULL"},
		KeyDetailDefault:     {text: "Default"},
		KeyDetailIdentity:    {text: "Identity"},
		KeyDetailPK:          {text: "PK"},
		KeyDetailFK:          {text: "FK"},
		KeyDetailUnique:      {text: "Unique"},
		KeyDetailSort:        {text: "Sort"},
		KeyDetailPrimary:     {text: "Primary"},
		KeyDetailParent:      {text: "Parent"},
		KeyDetailChild:       {text: "Child"},
		KeyDetailOnUpdate:    {text: "On update"},
		KeyDetailOnDelete:    {text: "On delete"},
		KeyDetailCardinality: {text: "Cardinality"},
		KeyDetailYes:         {text: "Yes"},
		KeyDetailNo:          {text: "No"},
		KeyDetailUnresolved:  {text: "unresolved"},
		KeyCopyTable:         {text: "Copy table info"},
		KeyCopySucceeded:     {text: "Copied"},
		KeyCopyFailed:        {text: "Copy failed"},

		KeyTablePropertiesTitle:     {text: "Table properties"},
		KeyTablePropertiesGeneral:   {text: "General"},
		KeyTablePropertiesPreview:   {text: "Preview"},
		KeyTablePropertiesClose:     {text: "Close"},
		KeyTablePropertiesName:      {text: "Name"},
		KeyTablePropertiesFlags:     {text: "Flags"},
		KeyTablePropertiesMandatory: {text: "Mandatory"},

		KeyDatabaseSQL:              {text: "Database SQL"},
		KeyPreviewBack:              {text: "Back"},
		KeyPreviewLoading:           {text: "Generating database SQL…"},
		KeyPreviewTables:            {text: "Tables {count}", params: []string{"count"}},
		KeyPreviewColumns:           {text: "Columns {count}", params: []string{"count"}},
		KeyPreviewKeys:              {text: "Foreign keys {count}", params: []string{"count"}},
		KeyPreviewSummary:           {text: "Errors {errors} · Warnings {warnings}", params: []string{"errors", "warnings"}},
		KeyPreviewDiagnostics:       {text: "Diagnostics"},
		KeyPreviewSQL:               {text: "Full SQL"},
		KeyPreviewError:             {text: "Error"},
		KeyPreviewWarning:           {text: "Warning"},
		KeyPreviewLine:              {text: "line {line}", params: []string{"line"}},
		KeyPreviewDiagnosticHeading: {text: "{category} · {severity}", params: []string{"category", "severity"}},
		KeyPreviewSchemaIssue:       {text: "Schema"},
		KeyPreviewColumnIssue:       {text: "Columns and types"},
		KeyPreviewDefaultIssue:      {text: "Default values"},
		KeyPreviewIndexIssue:        {text: "Indexes and keys"},
		KeyPreviewForeignKeyIssue:   {text: "Foreign keys"},
		KeyPreviewParserIssue:       {text: "Source parsing"},
		KeyPreviewDatabaseIssue:     {text: "Database options"},
		KeyPreviewCopySQL:           {text: "Copy SQL"},
		KeyPreviewExportSQL:         {text: "Export .sql"},
		KeyPreviewExporting:         {text: "Exporting SQL…"},
		KeyPreviewSQLCopied:         {text: "SQL copied"},
		KeyPreviewSQLCopyFailed:     {text: "SQL copy failed"},
		KeyPreviewSQLExported:       {text: "SQL exported"},
		KeyPreviewSQLExportFailed:   {text: "SQL export failed"},
		KeyPreviewSaveTitle:         {text: "Export database SQL"},
		KeyPreviewSaveFilter:        {text: "SQL file"},
		KeyCopyCreateTable:          {text: "Copy CREATE TABLE"},
		KeyCopyCreateTableFailed:    {text: "CREATE TABLE copy failed: {category} ({code})", params: []string{"category", "code"}},
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
		KeySearch:            {text: "搜索"},
		KeySearchResults:     {text: "搜索结果"},
		KeySearchNoResults:   {text: "无结果"},
		KeyDiagramsHeading:   {text: "画板"},
		KeyModelRoot:         {text: "模型"},
		KeyDiagramFallback:   {text: "画板 {index}", params: []string{"index"}},
		KeyNotOnDiagram:      {text: "不在任何画板上"},
		KeyLoadingPDM:        {text: "正在加载 PDM…"},
		KeyPreparingDiagram:  {text: "正在准备画板…"},
		KeyLoadPDMFailed:     {text: "无法打开 PDM 文件"},
		KeyNoDiagramFallback: {text: "无画板 · 自动布局"},
		KeyAutoLayoutStatus:  {text: "含自动布局的表"},
		KeySampleDiagram:     {text: "模拟图表"},
		KeyFitSelection:      {text: "适应选中"},
		KeyFitShort:          {text: "适应"},
		KeyShowAll:           {text: "显示全部"},
		KeyShowAllShort:      {text: "全部"},
		KeyResetView:         {text: "重置视图"},
		KeyResetShort:        {text: "重置"},
		KeyZoomIn:            {text: "放大"},
		KeyZoomOut:           {text: "缩小"},
		KeyDetailCode:        {text: "编码"},
		KeyDetailComment:     {text: "注释"},
		KeyDetailPackage:     {text: "包"},
		KeyDetailColumns:     {text: "字段"},
		KeyDetailKeys:        {text: "键"},
		KeyDetailIndexes:     {text: "索引"},
		KeyDetailForeignKeys: {text: "外键"},
		KeyDetailType:        {text: "类型"},
		KeyDetailLength:      {text: "长度"},
		KeyDetailPrecision:   {text: "精度"},
		KeyDetailScale:       {text: "小数位"},
		KeyDetailNull:        {text: "NULL"},
		KeyDetailDefault:     {text: "默认值"},
		KeyDetailIdentity:    {text: "自增"},
		KeyDetailPK:          {text: "主键"},
		KeyDetailFK:          {text: "外键"},
		KeyDetailUnique:      {text: "唯一"},
		KeyDetailSort:        {text: "排序"},
		KeyDetailPrimary:     {text: "主键"},
		KeyDetailParent:      {text: "主表"},
		KeyDetailChild:       {text: "从表"},
		KeyDetailOnUpdate:    {text: "更新时"},
		KeyDetailOnDelete:    {text: "删除时"},
		KeyDetailCardinality: {text: "基数"},
		KeyDetailYes:         {text: "是"},
		KeyDetailNo:          {text: "否"},
		KeyDetailUnresolved:  {text: "未解析"},
		KeyCopyTable:         {text: "复制表信息"},
		KeyCopySucceeded:     {text: "已复制"},
		KeyCopyFailed:        {text: "复制失败"},

		KeyTablePropertiesTitle:     {text: "表属性"},
		KeyTablePropertiesGeneral:   {text: "常规"},
		KeyTablePropertiesPreview:   {text: "预览"},
		KeyTablePropertiesClose:     {text: "关闭"},
		KeyTablePropertiesName:      {text: "名称"},
		KeyTablePropertiesFlags:     {text: "标记"},
		KeyTablePropertiesMandatory: {text: "必填"},

		KeyDatabaseSQL:              {text: "数据库 SQL"},
		KeyPreviewBack:              {text: "返回"},
		KeyPreviewLoading:           {text: "正在生成数据库 SQL…"},
		KeyPreviewTables:            {text: "表 {count}", params: []string{"count"}},
		KeyPreviewColumns:           {text: "字段 {count}", params: []string{"count"}},
		KeyPreviewKeys:              {text: "外键 {count}", params: []string{"count"}},
		KeyPreviewSummary:           {text: "{errors} 个错误 · {warnings} 个警告", params: []string{"errors", "warnings"}},
		KeyPreviewDiagnostics:       {text: "诊断"},
		KeyPreviewSQL:               {text: "完整 SQL"},
		KeyPreviewError:             {text: "错误"},
		KeyPreviewWarning:           {text: "警告"},
		KeyPreviewLine:              {text: "行 {line}", params: []string{"line"}},
		KeyPreviewDiagnosticHeading: {text: "{category} · {severity}", params: []string{"category", "severity"}},
		KeyPreviewSchemaIssue:       {text: "结构"},
		KeyPreviewColumnIssue:       {text: "字段与类型"},
		KeyPreviewDefaultIssue:      {text: "默认值"},
		KeyPreviewIndexIssue:        {text: "索引与键"},
		KeyPreviewForeignKeyIssue:   {text: "外键"},
		KeyPreviewParserIssue:       {text: "源解析"},
		KeyPreviewDatabaseIssue:     {text: "数据库选项"},
		KeyPreviewCopySQL:           {text: "复制 SQL"},
		KeyPreviewExportSQL:         {text: "导出 .sql"},
		KeyPreviewExporting:         {text: "正在导出 SQL…"},
		KeyPreviewSQLCopied:         {text: "SQL 已复制"},
		KeyPreviewSQLCopyFailed:     {text: "SQL 复制失败"},
		KeyPreviewSQLExported:       {text: "SQL 已导出"},
		KeyPreviewSQLExportFailed:   {text: "SQL 导出失败"},
		KeyPreviewSaveTitle:         {text: "导出数据库 SQL"},
		KeyPreviewSaveFilter:        {text: "SQL 文件"},
		KeyCopyCreateTable:          {text: "复制建表 SQL"},
		KeyCopyCreateTableFailed:    {text: "建表 SQL 复制失败：{category}（{code}）", params: []string{"category", "code"}},
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
