package pdm

import (
	"strconv"
	"strings"
	"unicode"
)

const (
	ColumnFilterAll     = "all"
	ColumnFilterPrimary = "primary"
	ColumnFilterKey     = "key"
)

type RGBColor struct {
	R, G, B uint8
}

type TableAppearance struct {
	Fill             *RGBColor
	Border           *RGBColor
	TitleColor       *RGBColor
	ColumnColor      *RGBColor
	PrimaryColor     *RGBColor
	PrimaryUnderline bool
	ForeignColor     *RGBColor
	GradientEnd      *RGBColor
	GradientMode     int
	Transparent      bool
	Display          *TableDisplay
}

type TableDisplay struct {
	ShowName             bool
	ShowColumns          bool
	ShowDataType         bool
	ShowKeyIndicator     bool
	ShowNullStatus       bool
	ShowStereotype       bool
	ShowColumnStereotype bool
	ShowComment          bool
	ColumnFilter         string
	ColumnLimit          int
}

func diagramAppearance(diagram *Element) TableAppearance {
	var appearance TableAppearance
	text, ok := diagram.Property("DisplayPreferences")
	if !ok {
		return appearance
	}
	sections := iniSections(text)
	style := sections["displaypreferences\\symbol\\tabl"]
	content := sections["displaypreferences\\object"]
	if color, ok := parseRGBFields(style["brush color"], false); ok {
		appearance.Fill = color
	}
	if color, ok := parsePenColor(style["pen"]); ok {
		appearance.Border = color
	}
	if filled, ok := parseYesNo(style["fill color"]); ok {
		appearance.Transparent = !filled
	}
	if mode, ok := parseInt(style["brush gradient mode"]); ok {
		appearance.GradientMode = mode
	}
	if color, ok := parseRGBFields(style["brush gradient color"], false); ok {
		appearance.GradientEnd = color
	}
	applyINIFontColors(&appearance, style)
	appearance.Display = displayFromINI(content)
	return appearance
}

func symbolAppearance(defaults TableAppearance, symbol *Element) TableAppearance {
	appearance := cloneTableAppearance(defaults)
	if value, ok := symbol.Property("FillColor"); ok {
		if color, ok := parseColorRef(value); ok {
			appearance.Fill = color
		}
	}
	if value, ok := symbol.Property("LineColor"); ok {
		if color, ok := parseColorRef(value); ok {
			appearance.Border = color
		}
	}
	if value, ok := symbol.Property("GradientEndColor"); ok {
		if color, ok := parseColorRef(value); ok {
			appearance.GradientEnd = color
		}
	}
	if value, ok := symbol.Property("GradientFillMode"); ok {
		if mode, ok := parseInt(value); ok {
			appearance.GradientMode = mode
		}
	}
	brush, _ := symbol.Property("BrushStyle")
	switch strings.TrimSpace(brush) {
	case "1":
		appearance.GradientMode = 0
		appearance.Transparent = false
	case "6":
		appearance.Transparent = false
	}
	if value, ok := symbol.Property("FontList"); ok {
		applyFontList(&appearance, value)
	}
	if value, ok := symbol.Property("SymbolContent"); ok {
		applySymbolContent(&appearance, value)
	}
	return appearance
}

func iniSections(text string) map[string]map[string]string {
	sections := make(map[string]map[string]string)
	current := ""
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			current = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			if sections[current] == nil {
				sections[current] = make(map[string]string)
			}
			continue
		}
		if current == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		sections[current][key] = strings.TrimSpace(value)
	}
	return sections
}

func displayFromINI(values map[string]string) *TableDisplay {
	display := defaultTableDisplay()
	present := false
	for _, setting := range []struct {
		key    string
		target *bool
	}{
		{"tablcoln", &display.ShowColumns},
		{"colndttp", &display.ShowDataType},
		{"colnkey", &display.ShowKeyIndicator},
		{"colnmand", &display.ShowNullStatus},
		{"colnstrn", &display.ShowColumnStereotype},
		{"tablstrn", &display.ShowStereotype},
		{"tablcmmt", &display.ShowComment},
		{"table.displayname", &display.ShowName},
		{"table.columns", &display.ShowColumns},
		{"table.stereotype", &display.ShowStereotype},
		{"table.comment", &display.ShowComment},
	} {
		if show, ok := parseYesNo(values[setting.key]); ok {
			*setting.target = show
			present = true
		}
	}
	if mode, ok := values["colnmode"]; ok {
		present = true
		if mode != "0" {
			display.ColumnFilter = ""
		}
	}
	if filter, ok := parseColumnFilter(values["table.columns._filter"]); ok {
		display.ColumnFilter = filter
		present = true
	}
	if value, ok := values["table.columns._columns"]; ok {
		present = true
		applyColumnFeatures(&display, value)
	}
	if limit, ok := parseInt(values["table.columns._limit"]); ok {
		present = true
		display.ColumnLimit = limit
	}
	if !present {
		return nil
	}
	return &display
}

func defaultTableDisplay() TableDisplay {
	return TableDisplay{
		ShowName:         true,
		ShowColumns:      true,
		ShowDataType:     true,
		ShowKeyIndicator: true,
		ColumnFilter:     ColumnFilterAll,
	}
}

func applyINIFontColors(appearance *TableAppearance, values map[string]string) {
	if color, ok := firstRGB(values, "*font color", "font color"); ok {
		appearance.TitleColor = cloneColor(color)
		appearance.ColumnColor = cloneColor(color)
		appearance.PrimaryColor = cloneColor(color)
		appearance.ForeignColor = cloneColor(color)
	}
	if color, ok := firstRGB(values, "dispnamefont color", "dispname font color"); ok {
		appearance.TitleColor = color
	}
	if color, ok := firstRGB(values, "columnsfont color", "columns font color"); ok {
		appearance.ColumnColor = color
	}
	if color, ok := firstRGB(values, "tablecolumnsfont color", "tablecolumns font color"); ok {
		appearance.ColumnColor = color
	}
	if color, ok := firstRGB(values, "tablepkcolumnsfont color", "tablepkcolumns font color"); ok {
		appearance.PrimaryColor = color
	}
	for _, key := range []string{"tablepkcolumnsfont", "tablepkcolumns font"} {
		if underline, ok := trailingUnderline(strings.Fields(values[key])); ok {
			appearance.PrimaryUnderline = underline
			break
		}
	}
	if color, ok := firstRGB(values, "tablefkcolumnsfont color", "tablefkcolumns font color"); ok {
		appearance.ForeignColor = color
	}
}

func applyFontList(appearance *TableAppearance, text string) {
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(strings.TrimSpace(strings.TrimSuffix(line, "\r")))
		if len(fields) < 2 {
			continue
		}
		color, ok := parseColorRef(fields[1])
		if !ok {
			continue
		}
		switch strings.ToUpper(fields[0]) {
		case "DISPNAME":
			appearance.TitleColor = color
		case "COLUMNS", "TABLECOLUMNS":
			appearance.ColumnColor = color
		case "TABLEPKCOLUMNS":
			appearance.PrimaryColor = color
			if underline, ok := trailingUnderline(fields[2:]); ok {
				appearance.PrimaryUnderline = underline
			}
		case "TABLEFKCOLUMNS":
			appearance.ForeignColor = color
		}
	}
}

func applySymbolContent(appearance *TableAppearance, text string) {
	hadDisplay := appearance.Display != nil
	if appearance.Display == nil {
		display := defaultTableDisplay()
		appearance.Display = &display
	}
	changed := false
	for _, line := range strings.Split(text, "\n") {
		fields := splitQuotedFields(strings.TrimSpace(strings.TrimSuffix(line, "\r")))
		if len(fields) < 2 {
			continue
		}
		switch fields[0] {
		case "Table.DisplayName":
			if show, ok := parseYesNo(fields[1]); ok {
				appearance.Display.ShowName = show
				changed = true
			}
		case "Table.Columns":
			show, ok := parseYesNo(fields[1])
			if !ok {
				continue
			}
			appearance.Display.ShowColumns = show
			changed = true
			if len(fields) > 2 {
				if limit, ok := parseInt(fields[2]); ok {
					appearance.Display.ColumnLimit = limit
				}
			}
			start := min(3, len(fields))
			for index := start; index < len(fields); index++ {
				if filter, ok := parseColumnFilter(fields[index]); ok {
					appearance.Display.ColumnFilter = filter
					start = index + 1
				}
			}
			if len(fields) > 3 {
				applyColumnFeatures(appearance.Display, strings.Join(fields[start:], " "))
			}
		case "Table.Stereotype":
			if show, ok := parseYesNo(fields[1]); ok {
				appearance.Display.ShowStereotype = show
				changed = true
			}
		case "Table.Comment":
			if show, ok := parseYesNo(fields[1]); ok {
				appearance.Display.ShowComment = show
				changed = true
			}
		}
	}
	if !changed && !hadDisplay {
		appearance.Display = nil
	}
}

func applyColumnFeatures(display *TableDisplay, text string) {
	display.ShowDataType = false
	display.ShowKeyIndicator = false
	display.ShowNullStatus = false
	display.ShowColumnStereotype = false
	tokens := strings.FieldsFunc(text, func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r)) && r != '_'
	})
	for _, token := range tokens {
		switch token {
		case "Stereotype", "ColumnStereotype":
			display.ShowColumnStereotype = true
		case "DataType", "SymbolDataType", "DomainOrDataType":
			display.ShowDataType = true
		case "KeyIndicator", "IdentifierIndicator":
			display.ShowKeyIndicator = true
		case "NullStatus", "NullIndicator":
			display.ShowNullStatus = true
		}
	}
}

func parseColumnFilter(text string) (string, bool) {
	upper := strings.ToUpper(text)
	switch {
	case strings.Contains(upper, "PDMCOLNPK"), strings.Contains(upper, "PRIMARY"):
		return ColumnFilterPrimary, true
	case strings.Contains(upper, "PDMCOLNKEY"):
		return ColumnFilterKey, true
	case strings.Contains(upper, "PDMCOLNALL"), strings.Contains(upper, "ALL COLUMNS"):
		return ColumnFilterAll, true
	case strings.TrimSpace(upper) == "ALL":
		return ColumnFilterAll, true
	}
	return "", false
}

func parseColorRef(text string) (*RGBColor, bool) {
	value, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
	if err != nil || value < 0 || value > 0xFFFFFF {
		return nil, false
	}
	bits := uint32(value)
	return &RGBColor{R: uint8(bits), G: uint8(bits >> 8), B: uint8(bits >> 16)}, true
}

func parseRGBFields(text string, last bool) (*RGBColor, bool) {
	fields := strings.FieldsFunc(strings.Trim(text, " \t\r\n\"'"), func(r rune) bool {
		return r == ',' || unicode.IsSpace(r)
	})
	if len(fields) < 3 {
		return nil, false
	}
	if last {
		fields = fields[len(fields)-3:]
	} else {
		fields = fields[:3]
	}
	var values [3]int
	for index, field := range fields {
		value, err := strconv.Atoi(field)
		if err != nil || value < 0 || value > 255 {
			return nil, false
		}
		values[index] = value
	}
	return &RGBColor{R: uint8(values[0]), G: uint8(values[1]), B: uint8(values[2])}, true
}

func parsePenColor(text string) (*RGBColor, bool) {
	return parseRGBFields(text, true)
}

func parseInt(text string) (int, bool) {
	value, err := strconv.Atoi(strings.TrimSpace(text))
	return value, err == nil
}

func parseYesNo(text string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "yes", "true", "1":
		return true, true
	case "no", "false", "0":
		return false, true
	}
	return false, false
}

func firstRGB(values map[string]string, keys ...string) (*RGBColor, bool) {
	for _, key := range keys {
		if color, ok := parseRGBFields(values[key], false); ok {
			return color, true
		}
	}
	return nil, false
}

func trailingUnderline(fields []string) (bool, bool) {
	for index := len(fields) - 1; index >= 0; index-- {
		parts := strings.Split(fields[index], ",")
		for part := len(parts) - 1; part >= 0; part-- {
			switch strings.ToUpper(strings.TrimSpace(parts[part])) {
			case "U":
				return true, true
			case "N":
				return false, true
			}
		}
	}
	return false, false
}

func splitQuotedFields(text string) []string {
	var fields []string
	var current strings.Builder
	quoted := false
	flush := func() {
		if current.Len() > 0 {
			fields = append(fields, current.String())
			current.Reset()
		}
	}
	for _, r := range text {
		switch {
		case r == '"':
			quoted = !quoted
		case unicode.IsSpace(r) && !quoted:
			flush()
		default:
			current.WriteRune(r)
		}
	}
	flush()
	return fields
}

func cloneTableAppearance(source TableAppearance) TableAppearance {
	appearance := source
	appearance.Fill = cloneColor(source.Fill)
	appearance.Border = cloneColor(source.Border)
	appearance.TitleColor = cloneColor(source.TitleColor)
	appearance.ColumnColor = cloneColor(source.ColumnColor)
	appearance.PrimaryColor = cloneColor(source.PrimaryColor)
	appearance.ForeignColor = cloneColor(source.ForeignColor)
	appearance.GradientEnd = cloneColor(source.GradientEnd)
	if source.Display != nil {
		display := *source.Display
		appearance.Display = &display
	}
	return appearance
}

func cloneColor(color *RGBColor) *RGBColor {
	if color == nil {
		return nil
	}
	copy := *color
	return &copy
}
