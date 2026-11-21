package sqlgen

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"pdmview/internal/pdm"
)

var (
	ErrUnknownType     = errors.New("sqlgen: unknown MySQL type")
	ErrInvalidType     = errors.New("sqlgen: invalid MySQL type")
	ErrUnsupportedType = errors.New("sqlgen: unsupported MySQL type")
	ErrConflictingType = errors.New("sqlgen: conflicting MySQL type metadata")
)

// BaseType is a canonical MySQL 8.0 column type.
type BaseType string

const (
	TypeTinyInt   BaseType = "TINYINT"
	TypeSmallInt  BaseType = "SMALLINT"
	TypeMediumInt BaseType = "MEDIUMINT"
	TypeInt       BaseType = "INT"
	TypeBigInt    BaseType = "BIGINT"
	TypeDecimal   BaseType = "DECIMAL"
	TypeFloat     BaseType = "FLOAT"
	TypeDouble    BaseType = "DOUBLE"
	TypeBit       BaseType = "BIT"

	TypeChar       BaseType = "CHAR"
	TypeVarChar    BaseType = "VARCHAR"
	TypeBinary     BaseType = "BINARY"
	TypeVarBinary  BaseType = "VARBINARY"
	TypeTinyText   BaseType = "TINYTEXT"
	TypeText       BaseType = "TEXT"
	TypeMediumText BaseType = "MEDIUMTEXT"
	TypeLongText   BaseType = "LONGTEXT"
	TypeTinyBlob   BaseType = "TINYBLOB"
	TypeBlob       BaseType = "BLOB"
	TypeMediumBlob BaseType = "MEDIUMBLOB"
	TypeLongBlob   BaseType = "LONGBLOB"

	TypeDate      BaseType = "DATE"
	TypeTime      BaseType = "TIME"
	TypeDateTime  BaseType = "DATETIME"
	TypeTimestamp BaseType = "TIMESTAMP"
	TypeYear      BaseType = "YEAR"
	TypeJSON      BaseType = "JSON"
)

// TypeFamily groups types that share downstream compatibility rules.
type TypeFamily uint8

const (
	FamilyNumeric TypeFamily = iota + 1
	FamilyCharacter
	FamilyBinary
	FamilyTemporal
	FamilyText
	FamilyBlob
	FamilyJSON
)

// MySQLType is a validated, canonical MySQL column type. Length is the
// character or binary length, Precision and Scale describe DECIMAL, and
// Precision also records a source FLOAT(p) choice; FractionalPrecision describes TIME, DATETIME, and TIMESTAMP. Integer and
// BOOL display widths are retained only as LegacyDisplayWidth and are not
// emitted in SQL.
type MySQLType struct {
	Base                BaseType
	Length              int
	Precision           int
	Scale               int
	FractionalPrecision int
	LegacyDisplayWidth  int
	Unsigned            bool
	CharacterSet        string
	Collation           string
}

// SQL returns the canonical MySQL 8.0 spelling.
func (t MySQLType) SQL() string {
	var text strings.Builder
	text.WriteString(string(t.Base))
	switch t.Base {
	case TypeChar, TypeVarChar, TypeBinary, TypeVarBinary:
		text.WriteByte('(')
		text.WriteString(strconv.Itoa(t.Length))
		text.WriteByte(')')
	case TypeDecimal:
		text.WriteByte('(')
		text.WriteString(strconv.Itoa(t.Precision))
		text.WriteByte(',')
		text.WriteString(strconv.Itoa(t.Scale))
		text.WriteByte(')')
	case TypeBit:
		text.WriteByte('(')
		text.WriteString(strconv.Itoa(t.Length))
		text.WriteByte(')')
	case TypeTime, TypeDateTime, TypeTimestamp:
		if t.FractionalPrecision > 0 {
			text.WriteByte('(')
			text.WriteString(strconv.Itoa(t.FractionalPrecision))
			text.WriteByte(')')
		}
	}
	if t.Unsigned {
		text.WriteString(" UNSIGNED")
	}
	if t.CharacterSet != "" {
		text.WriteString(" CHARACTER SET ")
		text.WriteString(t.CharacterSet)
	}
	if t.Collation != "" {
		text.WriteString(" COLLATE ")
		text.WriteString(t.Collation)
	}
	return text.String()
}

// Family returns the downstream compatibility family of the canonical type.
func (t MySQLType) Family() TypeFamily {
	switch t.Base {
	case TypeTinyInt, TypeSmallInt, TypeMediumInt, TypeInt, TypeBigInt,
		TypeDecimal, TypeFloat, TypeDouble, TypeBit:
		return FamilyNumeric
	case TypeChar, TypeVarChar:
		return FamilyCharacter
	case TypeBinary, TypeVarBinary:
		return FamilyBinary
	case TypeTinyText, TypeText, TypeMediumText, TypeLongText:
		return FamilyText
	case TypeTinyBlob, TypeBlob, TypeMediumBlob, TypeLongBlob:
		return FamilyBlob
	case TypeDate, TypeTime, TypeDateTime, TypeTimestamp, TypeYear:
		return FamilyTemporal
	case TypeJSON:
		return FamilyJSON
	default:
		return 0
	}
}

// MapType validates one PDM column type and maps it to a canonical MySQL
// 8.0 type. It never passes through the source spelling.
func MapType(column pdm.Column) (MySQLType, error) {
	if !column.DataType.Present {
		return MySQLType{}, invalidType("data type is absent")
	}
	spec, err := parseTypeSpec(column.DataType.Value)
	if err != nil {
		return MySQLType{}, err
	}
	rawModifiers, err := parseRawTypeModifiers(column.Raw)
	if err != nil {
		return MySQLType{}, err
	}
	modifiers, err := mergeTypeModifiers(spec.modifiers, rawModifiers)
	if err != nil {
		return MySQLType{}, err
	}
	length, hasLength, err := optionalDimension(column.Length, "Length")
	if err != nil {
		return MySQLType{}, err
	}
	precision, hasPrecision, err := optionalDimension(column.Precision, "Precision")
	if err != nil {
		return MySQLType{}, err
	}
	scale, hasScale, err := optionalDimension(column.Scale, "Scale")
	if err != nil {
		return MySQLType{}, err
	}
	result := MySQLType{
		Base:               spec.base,
		Unsigned:           modifiers.unsigned != nil && *modifiers.unsigned,
		CharacterSet:       modifierString(modifiers.characterSet),
		Collation:          modifierString(modifiers.collation),
		LegacyDisplayWidth: spec.legacyDisplayWidth,
	}
	if err := validateModifiers(
		result,
		modifiers.unsigned != nil,
		modifiers.characterSet != nil,
		modifiers.collation != nil,
	); err != nil {
		return MySQLType{}, err
	}
	if err := applyDimensions(&result, spec, length, hasLength, precision, hasPrecision, scale, hasScale); err != nil {
		return MySQLType{}, err
	}
	return result, nil
}

type typeModifiers struct {
	unsigned     *bool
	characterSet *string
	collation    *string
}

type typeSpec struct {
	base               BaseType
	args               []int
	hasArgs            bool
	legacyDisplayWidth int
	modifiers          typeModifiers
}

func applyDimensions(
	result *MySQLType,
	spec typeSpec,
	length int,
	hasLength bool,
	precision int,
	hasPrecision bool,
	scale int,
	hasScale bool,
) error {
	rejectPrecisionScale := func() error {
		if hasPrecision || hasScale {
			return unsupportedType("Precision or Scale is not valid for " + string(result.Base))
		}
		return nil
	}
	switch result.Base {
	case TypeTinyInt, TypeSmallInt, TypeMediumInt, TypeInt, TypeBigInt:
		if err := rejectPrecisionScale(); err != nil {
			return err
		}
		width, err := singleDimension(spec, hasLength, length, 0, false, "integer display width")
		if err != nil {
			return err
		}
		if width > 0 && (width < 1 || width > 255) {
			return invalidType("integer display width is outside 1..255")
		}
		if width > 0 {
			result.LegacyDisplayWidth = width
		}
	case TypeDecimal:
		if len(spec.args) > 2 {
			return invalidType("DECIMAL takes at most two dimensions")
		}
		if hasPrecision {
			switch {
			case !hasScale:
				scale, hasScale = precision, true
			case !hasLength:
				length, hasLength = precision, true
			case precision != scale && precision != length:
				return conflictingType("Precision conflicts with Length and Scale")
			}
		}
		result.Precision = 10
		if spec.hasArgs {
			result.Precision = spec.args[0]
			if len(spec.args) == 2 {
				result.Scale = spec.args[1]
			}
			if (hasLength && length != result.Precision) || (hasScale && scale != result.Scale) {
				return conflictingType("embedded DECIMAL dimensions disagree with column attributes")
			}
		} else {
			if hasLength {
				result.Precision = length
			}
			if hasScale {
				result.Scale = scale
			}
		}
		if result.Precision < 1 || result.Precision > 65 {
			return invalidType("DECIMAL precision is outside 1..65")
		}
		if result.Scale < 0 || result.Scale > 30 || result.Scale > result.Precision {
			return invalidType("DECIMAL scale is outside 0..min(30, precision)")
		}
	case TypeFloat:
		if hasScale {
			return unsupportedType("deprecated FLOAT(M,D) syntax is unsupported")
		}
		if len(spec.args) > 1 {
			return unsupportedType("deprecated FLOAT(M,D) syntax is unsupported")
		}
		if hasLength && hasPrecision && length != precision {
			return unsupportedType("two FLOAT dimensions are unsupported")
		}
		bits := 0
		switch {
		case len(spec.args) == 1:
			bits = spec.args[0]
			if (hasLength && length != bits) || (hasPrecision && precision != bits) {
				return conflictingType("embedded precision disagrees with column attributes")
			}
		case hasLength:
			bits = length
			if hasPrecision && precision != bits {
				return conflictingType("Length and Precision disagree")
			}
		case hasPrecision:
			bits = precision
		}
		if bits < 0 || bits > 53 {
			return invalidType("FLOAT precision is outside 0..53")
		}
		result.Precision = bits
		if bits >= 25 {
			result.Base = TypeDouble
		}
	case TypeDouble:
		if spec.hasArgs || hasLength || hasPrecision || hasScale {
			return unsupportedType("DOUBLE dimensions are unsupported")
		}
	case TypeBit:
		if err := rejectPrecisionScale(); err != nil {
			return err
		}
		bits, err := singleDimension(spec, hasLength, length, 1, true, "BIT length")
		if err != nil {
			return err
		}
		if bits < 1 || bits > 64 {
			return invalidType("BIT length is outside 1..64")
		}
		result.Length = bits
	case TypeChar, TypeBinary:
		if err := rejectPrecisionScale(); err != nil {
			return err
		}
		size, err := singleDimension(spec, hasLength, length, 1, false, "character or binary length")
		if err != nil {
			return err
		}
		if size > 255 {
			return invalidType(string(result.Base) + " length is outside 0..255")
		}
		result.Length = size
	case TypeVarChar, TypeVarBinary:
		if err := rejectPrecisionScale(); err != nil {
			return err
		}
		size, err := singleDimension(spec, hasLength, length, 0, true, "variable length")
		if err != nil {
			return err
		}
		if size > 65535 {
			return invalidType(string(result.Base) + " length is outside 0..65535")
		}
		result.Length = size
	case TypeTime, TypeDateTime, TypeTimestamp:
		if err := rejectPrecisionScale(); err != nil {
			return err
		}
		fsp, err := singleDimension(spec, hasLength, length, 0, false, "fractional precision")
		if err != nil {
			return err
		}
		if fsp > 6 {
			return invalidType(string(result.Base) + " fractional precision is outside 0..6")
		}
		result.FractionalPrecision = fsp
	case TypeYear:
		if err := rejectPrecisionScale(); err != nil {
			return err
		}
		yearWidth, err := singleDimension(spec, hasLength, length, 4, true, "YEAR width")
		if err != nil {
			return err
		}
		if yearWidth != 4 {
			return unsupportedType("YEAR(2) and other YEAR widths are unsupported")
		}
	case TypeDate:
		if spec.hasArgs || hasLength || hasPrecision || hasScale {
			return unsupportedType("DATE dimensions are unsupported")
		}
	case TypeTinyText, TypeText, TypeMediumText, TypeLongText,
		TypeTinyBlob, TypeBlob, TypeMediumBlob, TypeLongBlob, TypeJSON:
		if spec.hasArgs || hasLength || hasPrecision || hasScale {
			return unsupportedType(string(result.Base) + " dimensions are unsupported")
		}
	default:
		return unsupportedType("type family is unsupported")
	}
	return nil
}

func singleDimension(
	spec typeSpec,
	hasLength bool,
	length int,
	defaultValue int,
	required bool,
	label string,
) (int, error) {
	if spec.hasArgs && len(spec.args) != 1 {
		return 0, invalidType(label + " takes one dimension")
	}
	value, found := defaultValue, defaultValue != 0 || !required
	if defaultValue == 0 && !required {
		found = hasLength || spec.hasArgs
	}
	switch {
	case spec.hasArgs:
		value, found = spec.args[0], true
		if hasLength && length != value {
			return 0, conflictingType("embedded " + label + " disagrees with Length")
		}
	case hasLength:
		value, found = length, true
	}
	if !found && required {
		return 0, invalidType(label + " is required")
	}
	return value, nil
}

func validateModifiers(
	t MySQLType,
	unsignedSpecified bool,
	characterSetSpecified bool,
	collationSpecified bool,
) error {
	unsignedValid := t.Base == TypeTinyInt || t.Base == TypeSmallInt ||
		t.Base == TypeMediumInt || t.Base == TypeInt || t.Base == TypeBigInt ||
		t.Base == TypeDecimal || t.Base == TypeFloat || t.Base == TypeDouble
	character := t.Base == TypeChar || t.Base == TypeVarChar ||
		t.Base == TypeTinyText || t.Base == TypeText ||
		t.Base == TypeMediumText || t.Base == TypeLongText
	if unsignedSpecified && !unsignedValid {
		return unsupportedType("UNSIGNED is not valid for " + string(t.Base))
	}
	if (characterSetSpecified || collationSpecified) && !character {
		return unsupportedType("character set or collation is not valid for " + string(t.Base))
	}
	if strings.EqualFold(t.CharacterSet, "binary") || strings.EqualFold(t.Collation, "binary") {
		return unsupportedType("binary character set changes the column type; specify the binary type directly")
	}
	return nil
}

func optionalDimension(value pdm.OptionalText, name string) (int, bool, error) {
	if !value.Present {
		return 0, false, nil
	}
	text := strings.TrimSpace(value.Value)
	if !digits(text) {
		return 0, false, invalidType(name + " is not an unsigned decimal integer")
	}
	number, err := strconv.ParseUint(text, 10, 31)
	if err != nil {
		return 0, false, invalidType(name + " is outside the supported range")
	}
	return int(number), true, nil
}

func parseTypeSpec(source string) (typeSpec, error) {
	if word, ok := firstTypeWord(source); ok {
		if _, unsupported := unsupportedAliases[strings.ToUpper(word)]; unsupported {
			return typeSpec{}, unsupportedType("type is recognized but not supported")
		}
	}
	tokens, err := lexTypeSpec(source)
	if err != nil {
		return typeSpec{}, err
	}
	base, consumed, recognized, unsupported := resolveBase(tokens)
	if unsupported {
		return typeSpec{}, unsupportedType("type is recognized but not supported")
	}
	if !recognized {
		return typeSpec{}, unknownType("type is not a supported MySQL 8.0 type")
	}
	spec := typeSpec{base: base}
	switch strings.ToUpper(strings.Join(tokens[:consumed], " ")) {
	case "BOOL", "BOOLEAN":
		spec.legacyDisplayWidth = 1
	}
	if consumed < len(tokens) && strings.HasPrefix(tokens[consumed], "(") {
		args, err := parseArguments(tokens[consumed])
		if err != nil {
			return typeSpec{}, err
		}
		spec.args, spec.hasArgs = args, true
		consumed++
	}
	for consumed < len(tokens) {
		switch strings.ToUpper(tokens[consumed]) {
		case "UNSIGNED":
			if spec.modifiers.unsigned != nil {
				return typeSpec{}, conflictingType("UNSIGNED is repeated")
			}
			value := true
			spec.modifiers.unsigned = &value
			consumed++
		case "SIGNED":
			if spec.modifiers.unsigned != nil {
				return typeSpec{}, conflictingType("SIGNED conflicts with UNSIGNED")
			}
			value := false
			spec.modifiers.unsigned = &value
			consumed++
		case "ZEROFILL":
			return typeSpec{}, unsupportedType("ZEROFILL is deprecated")
		case "BINARY", "ASCII", "UNICODE", "BYTE":
			return typeSpec{}, unsupportedType(strings.ToUpper(tokens[consumed]) + " type modifier is unsupported")
		case "CHARACTER":
			consumed++
			if consumed >= len(tokens) || !strings.EqualFold(tokens[consumed], "SET") {
				return typeSpec{}, invalidType("CHARACTER must be followed by SET")
			}
			consumed++
			value, err := modifierIdentifier(tokens, &consumed, "character set")
			if err != nil {
				return typeSpec{}, err
			}
			if spec.modifiers.characterSet != nil {
				return typeSpec{}, conflictingType("character set is repeated")
			}
			spec.modifiers.characterSet = &value
		case "COLLATE":
			consumed++
			value, err := modifierIdentifier(tokens, &consumed, "collation")
			if err != nil {
				return typeSpec{}, err
			}
			if spec.modifiers.collation != nil {
				return typeSpec{}, conflictingType("collation is repeated")
			}
			spec.modifiers.collation = &value
		default:
			return typeSpec{}, invalidType("type modifier is not supported")
		}
	}
	return spec, nil
}

func lexTypeSpec(source string) ([]string, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil, invalidType("data type is blank")
	}
	var tokens []string
	for offset := 0; offset < len(source); {
		if asciiSpace(source[offset]) {
			offset++
			continue
		}
		if source[offset] == '(' {
			end := strings.IndexByte(source[offset:], ')')
			if end < 0 {
				return nil, invalidType("unterminated type arguments")
			}
			end += offset
			inner := strings.TrimSpace(source[offset+1 : end])
			if inner == "" || !argumentText(inner) {
				return nil, invalidType("type arguments contain invalid characters")
			}
			tokens = append(tokens, "("+inner+")")
			offset = end + 1
			continue
		}
		if !asciiIdentifierStart(source[offset]) {
			return nil, invalidType("data type contains invalid characters")
		}
		end := offset + 1
		for end < len(source) && asciiIdentifierPart(source[end]) {
			end++
		}
		tokens = append(tokens, source[offset:end])
		offset = end
	}
	return tokens, nil
}

func parseArguments(token string) ([]int, error) {
	inner := strings.TrimSuffix(strings.TrimPrefix(token, "("), ")")
	parts := strings.Split(inner, ",")
	if len(parts) > 2 {
		return nil, invalidType("too many type arguments")
	}
	values := make([]int, len(parts))
	for offset, part := range parts {
		part = strings.TrimSpace(part)
		if !digits(part) {
			return nil, invalidType("type argument is not an unsigned decimal integer")
		}
		value, err := strconv.ParseUint(part, 10, 31)
		if err != nil {
			return nil, invalidType("type argument is outside the supported range")
		}
		values[offset] = int(value)
	}
	return values, nil
}

func resolveBase(tokens []string) (BaseType, int, bool, bool) {
	for words := min(3, len(tokens)); words > 0; words-- {
		key := strings.ToUpper(strings.Join(tokens[:words], " "))
		if base, ok := typeAliases[key]; ok {
			return base, words, true, false
		}
		if _, ok := unsupportedAliases[key]; ok {
			return "", words, false, true
		}
	}
	return "", 0, false, false
}

func parseRawTypeModifiers(raw *pdm.Element) (typeModifiers, error) {
	var modifiers typeModifiers
	unsigned, ok, err := rawProperty(raw, "Column.Unsigned", "Unsigned")
	if err != nil {
		return modifiers, err
	}
	if ok {
		switch strings.TrimSpace(unsigned) {
		case "1":
			value := true
			modifiers.unsigned = &value
		case "0":
			value := false
			modifiers.unsigned = &value
		default:
			return modifiers, invalidType("raw Unsigned property must be 0 or 1")
		}
	}
	characterSet, ok, err := rawProperty(raw,
		"Column.CharacterSet", "CharacterSet", "Column.Charset", "Charset")
	if err != nil {
		return modifiers, err
	}
	if ok {
		if !validIdentifier(characterSet) {
			return modifiers, invalidType("raw character set property is not a valid identifier")
		}
		modifiers.characterSet = &characterSet
	}
	collation, ok, err := rawProperty(raw,
		"Column.Collation", "Collation", "Column.Collate", "Collate")
	if err != nil {
		return modifiers, err
	}
	if ok {
		if !validIdentifier(collation) {
			return modifiers, invalidType("raw collation property is not a valid identifier")
		}
		modifiers.collation = &collation
	}
	return modifiers, nil
}

func rawProperty(raw *pdm.Element, names ...string) (string, bool, error) {
	value, found := "", false
	if raw == nil {
		return "", false, nil
	}
	for _, child := range raw.Children {
		if child == nil || child.Name.Space != pdm.NamespaceAttribute {
			continue
		}
		matched := false
		for _, name := range names {
			if child.Name.Local == name {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		current := strings.TrimSpace(child.Text)
		if found && current != value {
			return "", false, conflictingType("raw type modifier properties disagree")
		}
		value, found = current, true
	}
	return value, found, nil
}

func mergeTypeModifiers(suffix, raw typeModifiers) (typeModifiers, error) {
	var merged typeModifiers
	unsigned, conflict := mergeBool(suffix.unsigned, raw.unsigned)
	if conflict {
		return typeModifiers{}, conflictingType("UNSIGNED sources disagree")
	}
	merged.unsigned = unsigned
	var err error
	merged.characterSet, err = mergeString(suffix.characterSet, raw.characterSet, "character set")
	if err != nil {
		return typeModifiers{}, err
	}
	merged.collation, err = mergeString(suffix.collation, raw.collation, "collation")
	if err != nil {
		return typeModifiers{}, err
	}
	return merged, nil
}

func mergeBool(left, right *bool) (*bool, bool) {
	switch {
	case left == nil:
		return right, false
	case right == nil:
		return left, false
	case *left != *right:
		return nil, true
	default:
		return left, false
	}
}

func mergeString(left, right *string, label string) (*string, error) {
	switch {
	case left == nil:
		return right, nil
	case right == nil:
		return left, nil
	case !strings.EqualFold(*left, *right):
		return nil, conflictingType(label + " sources disagree")
	default:
		return left, nil
	}
}

func modifierString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func modifierIdentifier(tokens []string, offset *int, label string) (string, error) {
	if *offset >= len(tokens) || !validIdentifier(tokens[*offset]) {
		return "", invalidType(label + " must be an ASCII identifier")
	}
	value := tokens[*offset]
	(*offset)++
	return value, nil
}

func firstTypeWord(source string) (string, bool) {
	source = strings.TrimSpace(source)
	if source == "" || !asciiIdentifierStart(source[0]) {
		return "", false
	}
	end := 1
	for end < len(source) && asciiIdentifierPart(source[end]) {
		end++
	}
	return source[:end], true
}

func argumentText(value string) bool {
	digit := false
	for offset := 0; offset < len(value); offset++ {
		switch {
		case value[offset] >= '0' && value[offset] <= '9':
			digit = true
		case value[offset] == ',' || asciiSpace(value[offset]):
		default:
			return false
		}
	}
	return digit
}

func digits(value string) bool {
	if value == "" {
		return false
	}
	for offset := 0; offset < len(value); offset++ {
		if value[offset] < '0' || value[offset] > '9' {
			return false
		}
	}
	return true
}

func validIdentifier(value string) bool {
	if value == "" || !asciiIdentifierStart(value[0]) {
		return false
	}
	for offset := 1; offset < len(value); offset++ {
		if !asciiIdentifierPart(value[offset]) {
			return false
		}
	}
	return true
}

func asciiIdentifierStart(value byte) bool {
	return value == '_' || value >= 'A' && value <= 'Z' || value >= 'a' && value <= 'z'
}

func asciiIdentifierPart(value byte) bool {
	return asciiIdentifierStart(value) || value >= '0' && value <= '9'
}

func asciiSpace(value byte) bool {
	switch value {
	case ' ', '\t', '\n', '\r':
		return true
	default:
		return false
	}
}

func unknownType(reason string) error {
	return fmt.Errorf("%w: %s", ErrUnknownType, reason)
}

func invalidType(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidType, reason)
}

func unsupportedType(reason string) error {
	return fmt.Errorf("%w: %s", ErrUnsupportedType, reason)
}

func conflictingType(reason string) error {
	return fmt.Errorf("%w: %s", ErrConflictingType, reason)
}

var typeAliases = map[string]BaseType{
	"TINYINT": TypeTinyInt, "INT1": TypeTinyInt,
	"SMALLINT": TypeSmallInt, "INT2": TypeSmallInt,
	"MEDIUMINT": TypeMediumInt, "INT3": TypeMediumInt, "MIDDLEINT": TypeMediumInt,
	"INT": TypeInt, "INTEGER": TypeInt, "INT4": TypeInt,
	"BIGINT": TypeBigInt, "INT8": TypeBigInt,
	"BOOL": TypeTinyInt, "BOOLEAN": TypeTinyInt,
	"DECIMAL": TypeDecimal, "DEC": TypeDecimal, "NUMERIC": TypeDecimal, "FIXED": TypeDecimal,
	"FLOAT":  TypeFloat,
	"DOUBLE": TypeDouble, "DOUBLE PRECISION": TypeDouble, "REAL": TypeDouble,
	"BIT":  TypeBit,
	"CHAR": TypeChar, "CHARACTER": TypeChar,
	"VARCHAR": TypeVarChar, "CHARACTER VARYING": TypeVarChar, "CHAR VARYING": TypeVarChar,
	"BINARY": TypeBinary, "VARBINARY": TypeVarBinary,
	"TINYTEXT": TypeTinyText, "TEXT": TypeText, "MEDIUMTEXT": TypeMediumText, "LONGTEXT": TypeLongText,
	"TINYBLOB": TypeTinyBlob, "BLOB": TypeBlob, "MEDIUMBLOB": TypeMediumBlob, "LONGBLOB": TypeLongBlob,
	"DATE": TypeDate, "TIME": TypeTime, "DATETIME": TypeDateTime,
	"TIMESTAMP": TypeTimestamp, "YEAR": TypeYear, "JSON": TypeJSON,
}

var unsupportedAliases = map[string]struct{}{
	"ENUM": {}, "SET": {}, "SERIAL": {},
	"GEOMETRY": {}, "POINT": {}, "LINESTRING": {}, "POLYGON": {},
	"MULTIPOINT": {}, "MULTILINESTRING": {}, "MULTIPOLYGON": {}, "GEOMETRYCOLLECTION": {},
}
