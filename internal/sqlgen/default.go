package sqlgen

import (
	"encoding/json"
	"math/big"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"pdmview/internal/pdm"
)

type defaultKind uint8

const (
	defaultText defaultKind = iota + 1
	defaultNull
	defaultNumber
	defaultExpressionValue
)

type parsedDefault struct {
	kind       defaultKind
	text       string
	number     string
	expression defaultExpression
}

type defaultExpressionKind uint8

const (
	expressionCurrentDate defaultExpressionKind = iota + 1
	expressionCurrentTime
	expressionCurrentTimestamp
	expressionNow
	expressionLocalTime
	expressionLocalTimestamp
	expressionUTCTimestamp
	expressionUUID
	expressionJSONArray
	expressionJSONObject
	expressionPI
)

type defaultExpression struct {
	kind defaultExpressionKind
	fsp  int
}

type defaultProblem struct {
	code    string
	message string
}

func (compiled *compiledTable) defaultClause(column compiledColumn) (string, *Diagnostic) {
	if !column.source.DefaultValue.Present || !column.hasType {
		return "", nil
	}
	parsed, problem := parseDefaultValue(column.source.DefaultValue.Value)
	if problem != nil {
		return "", defaultDiagnostic(column.source, *problem)
	}
	clause, problem := renderDefaultValue(parsed, column)
	if problem != nil {
		return "", defaultDiagnostic(column.source, *problem)
	}
	return clause, nil
}

func defaultDiagnostic(column pdm.Column, problem defaultProblem) *Diagnostic {
	diagnostic := rawDiagnostic(
		problem.code, SeverityError, objectID(column.Metadata), problem.message, column.Raw)
	return &diagnostic
}

func parseDefaultValue(source string) (parsedDefault, *defaultProblem) {
	value := strings.TrimSpace(source)
	if value == "" {
		return parsedDefault{kind: defaultText, text: source}, nil
	}
	inner, wrapped, ok := unwrapDefaultParentheses(value)
	if !ok {
		return parsedDefault{}, invalidDefault("default expression has invalid parentheses")
	}
	if wrapped {
		value = strings.TrimSpace(inner)
		if value == "" || hasOuterParentheses(value) {
			return parsedDefault{}, ambiguousDefault("nested or empty default expression is ambiguous")
		}
	}
	if strings.EqualFold(value, "NULL") {
		return parsedDefault{kind: defaultNull}, nil
	}
	if value[0] == '\'' || value[0] == '"' {
		decoded, ok := parseSQLStringLiteral(value)
		if !ok {
			return parsedDefault{}, invalidDefault("quoted text default is not a complete MySQL string literal")
		}
		return parsedDefault{kind: defaultText, text: decoded}, nil
	}
	if strings.EqualFold(value, "TRUE") {
		return parsedDefault{kind: defaultNumber, number: "1", text: value}, nil
	}
	if strings.EqualFold(value, "FALSE") {
		return parsedDefault{kind: defaultNumber, number: "0", text: value}, nil
	}
	if validNumericLiteral(value) {
		return parsedDefault{kind: defaultNumber, number: value, text: value}, nil
	}
	if expression, ok := parseDefaultExpression(value); ok {
		return parsedDefault{
			kind: defaultExpressionValue, expression: expression,
		}, nil
	}
	if validUnquotedText(value) {
		return parsedDefault{kind: defaultText, text: value}, nil
	}
	if strings.ContainsAny(value, "()+-*/%=<>!,;") {
		return parsedDefault{}, unsupportedDefault("arbitrary default expression is not allowed")
	}
	return parsedDefault{}, ambiguousDefault("default is ambiguous between text and an expression")
}

func renderDefaultValue(parsed parsedDefault, column compiledColumn) (string, *defaultProblem) {
	switch parsed.kind {
	case defaultNull:
		if !column.nullable {
			return "", &defaultProblem{
				code: DiagnosticDefaultTypeMismatch, message: "NOT NULL column cannot default to NULL.",
			}
		}
		return "DEFAULT NULL", nil
	case defaultText:
		if !supportsTextDefault(column.typ.Base) {
			return "", &defaultProblem{
				code: DiagnosticDefaultTypeMismatch, message: "Text default is incompatible with column type.",
			}
		}
		return textDefault(parsed.text, column.typ)
	case defaultNumber:
		if column.typ.Family() == FamilyCharacter || column.typ.Family() == FamilyText {
			return textDefault(parsed.text, column.typ)
		}
		number, problem := numericDefault(parsed.number, column.typ)
		if problem != nil {
			return "", problem
		}
		return "DEFAULT " + maybeParenthesized(number, column.typ.Base), nil
	case defaultExpressionValue:
		expression, problem := expressionDefault(parsed.expression, column.typ)
		if problem != nil {
			return "", problem
		}
		return "DEFAULT " + expression, nil
	default:
		return "", &defaultProblem{code: DiagnosticInvalidDefault, message: "Default kind is unsupported."}
	}
}

func textDefault(value string, typ MySQLType) (string, *defaultProblem) {
	switch typ.Base {
	case TypeChar, TypeVarChar:
		if utf8.RuneCountInString(value) > typ.Length {
			return "", &defaultProblem{code: DiagnosticDefaultOutOfRange, message: "Text default exceeds the declared character length."}
		}
	case TypeBinary, TypeVarBinary:
		if len(value) > typ.Length {
			return "", &defaultProblem{code: DiagnosticDefaultOutOfRange, message: "Binary default exceeds the declared byte length."}
		}
	case TypeJSON:
		if !json.Valid([]byte(value)) {
			return "", invalidDefault("quoted JSON default does not contain valid JSON")
		}
	}
	return "DEFAULT " + maybeParenthesized(quoteSQLString(value), typ.Base), nil
}

func supportsTextDefault(base BaseType) bool {
	switch base {
	case TypeChar, TypeVarChar, TypeTinyText, TypeText, TypeMediumText, TypeLongText,
		TypeBinary, TypeVarBinary, TypeTinyBlob, TypeBlob, TypeMediumBlob, TypeLongBlob,
		TypeDate, TypeTime, TypeDateTime, TypeTimestamp, TypeYear, TypeJSON:
		return true
	default:
		return false
	}
}

func maybeParenthesized(value string, base BaseType) string {
	if requiresParenthesizedDefault(base) {
		return "(" + value + ")"
	}
	return value
}

func requiresParenthesizedDefault(base BaseType) bool {
	switch base {
	case TypeTinyText, TypeText, TypeMediumText, TypeLongText,
		TypeTinyBlob, TypeBlob, TypeMediumBlob, TypeLongBlob, TypeJSON:
		return true
	default:
		return false
	}
}

func numericDefault(value string, typ MySQLType) (string, *defaultProblem) {
	switch typ.Base {
	case TypeTinyInt, TypeSmallInt, TypeMediumInt, TypeInt, TypeBigInt:
		if !integerLiteral(value) {
			return "", &defaultProblem{
				code: DiagnosticDefaultTypeMismatch, message: "Integer column default is not an integer literal.",
			}
		}
		number := new(big.Int)
		if _, ok := number.SetString(strings.TrimPrefix(value, "+"), 10); !ok {
			return "", &defaultProblem{code: DiagnosticInvalidDefault, message: "Integer default is invalid."}
		}
		min, max := integerBounds(typ.Base, typ.Unsigned)
		if number.Cmp(min) < 0 || number.Cmp(max) > 0 {
			return "", &defaultProblem{
				code: DiagnosticDefaultOutOfRange, message: "Integer default is outside the column range.",
			}
		}
		return value, nil
	case TypeDecimal:
		if !validNumericLiteral(value) {
			return "", &defaultProblem{
				code: DiagnosticDefaultTypeMismatch, message: "Numeric column default is not a numeric literal.",
			}
		}
		if strings.ContainsAny(value, "eE") || !decimalFits(value, typ.Precision, typ.Scale) ||
			typ.Unsigned && negativeNonZero(value) {
			return "", &defaultProblem{
				code: DiagnosticDefaultOutOfRange, message: "DECIMAL default is outside the declared range.",
			}
		}
		return value, nil
	case TypeFloat, TypeDouble:
		if !validNumericLiteral(value) {
			return "", &defaultProblem{
				code: DiagnosticDefaultTypeMismatch, message: "Numeric column default is not a numeric literal.",
			}
		}
		if typ.Unsigned && negativeNonZero(value) {
			return "", &defaultProblem{
				code: DiagnosticDefaultOutOfRange, message: "UNSIGNED default must not be negative.",
			}
		}
		return value, nil
	case TypeBit:
		if !integerLiteral(value) || strings.HasPrefix(value, "-") {
			return "", &defaultProblem{
				code: DiagnosticDefaultTypeMismatch, message: "BIT default must be a non-negative integer.",
			}
		}
		number := new(big.Int)
		number.SetString(strings.TrimPrefix(value, "+"), 10)
		max := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(typ.Length)), big.NewInt(1))
		if number.Cmp(max) > 0 {
			return "", &defaultProblem{
				code: DiagnosticDefaultOutOfRange, message: "BIT default exceeds the declared width.",
			}
		}
		return value, nil
	case TypeYear:
		if !integerLiteral(value) {
			return "", &defaultProblem{
				code: DiagnosticDefaultTypeMismatch, message: "YEAR default must be an integer.",
			}
		}
		year, err := strconv.Atoi(strings.TrimPrefix(value, "+"))
		if err != nil || year != 0 && (year < 1901 || year > 2155) {
			return "", &defaultProblem{
				code: DiagnosticDefaultOutOfRange, message: "YEAR default must be 0 or 1901..2155.",
			}
		}
		return value, nil
	case TypeJSON:
		if !validNumericLiteral(value) {
			return "", &defaultProblem{
				code: DiagnosticDefaultTypeMismatch, message: "JSON numeric default is invalid.",
			}
		}
		return value, nil
	default:
		return "", &defaultProblem{
			code: DiagnosticDefaultTypeMismatch, message: "Numeric default is incompatible with column type.",
		}
	}
}

func expressionDefault(expression defaultExpression, typ MySQLType) (string, *defaultProblem) {
	switch expression.kind {
	case expressionCurrentDate:
		if typ.Base != TypeDate {
			return "", expressionTypeProblem("CURRENT_DATE requires a DATE column.")
		}
		return "(CURRENT_DATE)", nil
	case expressionCurrentTime:
		if typ.Base != TypeTime {
			return "", expressionTypeProblem("CURRENT_TIME requires a TIME column.")
		}
		if expression.fsp != typ.FractionalPrecision {
			return "", expressionFSPProblem("CURRENT_TIME precision must match the column precision.")
		}
		return "(" + currentTemporalSQL("CURRENT_TIME", expression.fsp) + ")", nil
	case expressionCurrentTimestamp:
		if typ.Base != TypeDateTime && typ.Base != TypeTimestamp {
			return "", expressionTypeProblem("CURRENT_TIMESTAMP requires a DATETIME or TIMESTAMP column.")
		}
		if expression.fsp != typ.FractionalPrecision {
			return "", expressionFSPProblem("CURRENT_TIMESTAMP precision must match the column precision.")
		}
		return currentTemporalSQL("CURRENT_TIMESTAMP", expression.fsp), nil
	case expressionNow:
		if typ.Base != TypeDateTime && typ.Base != TypeTimestamp {
			return "", expressionTypeProblem("NOW requires a DATETIME or TIMESTAMP column.")
		}
		if expression.fsp != typ.FractionalPrecision {
			return "", expressionFSPProblem("NOW precision must match the column precision.")
		}
		return "(NOW(" + strconv.Itoa(expression.fsp) + "))", nil
	case expressionLocalTime, expressionLocalTimestamp, expressionUTCTimestamp:
		if typ.Base != TypeDateTime && typ.Base != TypeTimestamp {
			return "", expressionTypeProblem("Temporal expression requires a DATETIME or TIMESTAMP column.")
		}
		if expression.fsp != typ.FractionalPrecision {
			return "", expressionFSPProblem("Temporal expression precision must match the column precision.")
		}
		name := map[defaultExpressionKind]string{
			expressionLocalTime:      "LOCALTIME",
			expressionLocalTimestamp: "LOCALTIMESTAMP",
			expressionUTCTimestamp:   "UTC_TIMESTAMP",
		}[expression.kind]
		return "(" + currentTemporalSQL(name, expression.fsp) + ")", nil
	case expressionUUID:
		if typ.Base != TypeChar && typ.Base != TypeVarChar {
			return "", expressionTypeProblem("UUID() requires a CHAR or VARCHAR column.")
		}
		return "(UUID())", nil
	case expressionJSONArray:
		if typ.Base != TypeJSON {
			return "", expressionTypeProblem("JSON_ARRAY() requires a JSON column.")
		}
		return "(JSON_ARRAY())", nil
	case expressionJSONObject:
		if typ.Base != TypeJSON {
			return "", expressionTypeProblem("JSON_OBJECT() requires a JSON column.")
		}
		return "(JSON_OBJECT())", nil
	case expressionPI:
		if typ.Base != TypeFloat && typ.Base != TypeDouble {
			return "", expressionTypeProblem("PI() requires a FLOAT or DOUBLE column.")
		}
		return "(PI())", nil
	default:
		return "", unsupportedDefault("Default expression is not recognized.")
	}
}

func parseDefaultExpression(value string) (defaultExpression, bool) {
	name := strings.ToUpper(value)
	args, call, ok := defaultCall(value)
	if !ok {
		return defaultExpression{}, false
	}
	if call {
		name = strings.ToUpper(value[:strings.IndexByte(value, '(')])
	}
	switch name {
	case "CURRENT_DATE":
		if call && args != "" {
			return defaultExpression{}, false
		}
		return defaultExpression{kind: expressionCurrentDate}, true
	case "CURDATE":
		if !call || args != "" {
			return defaultExpression{}, false
		}
		return defaultExpression{kind: expressionCurrentDate}, true
	case "CURRENT_TIME":
		return temporalExpression(expressionCurrentTime, args, call)
	case "CURTIME":
		if !call {
			return defaultExpression{}, false
		}
		return temporalExpression(expressionCurrentTime, args, call)
	case "CURRENT_TIMESTAMP":
		return temporalExpression(expressionCurrentTimestamp, args, call)
	case "NOW":
		if !call {
			return defaultExpression{}, false
		}
		return temporalExpression(expressionNow, args, call)
	case "LOCALTIME":
		return temporalExpression(expressionLocalTime, args, call)
	case "LOCALTIMESTAMP":
		return temporalExpression(expressionLocalTimestamp, args, call)
	case "UTC_TIMESTAMP":
		return temporalExpression(expressionUTCTimestamp, args, call)
	case "UUID":
		if call && args == "" {
			return defaultExpression{kind: expressionUUID}, true
		}
	case "JSON_ARRAY":
		if call && args == "" {
			return defaultExpression{kind: expressionJSONArray}, true
		}
	case "JSON_OBJECT":
		if call && args == "" {
			return defaultExpression{kind: expressionJSONObject}, true
		}
	case "PI":
		if call && args == "" {
			return defaultExpression{kind: expressionPI}, true
		}
	}
	return defaultExpression{}, false
}

func temporalExpression(
	kind defaultExpressionKind,
	args string,
	call bool,
) (defaultExpression, bool) {
	if !call && args != "" {
		return defaultExpression{}, false
	}
	expression := defaultExpression{kind: kind}
	if args == "" {
		return expression, true
	}
	fsp, err := strconv.Atoi(args)
	if err != nil || fsp < 0 || fsp > 6 {
		return defaultExpression{}, false
	}
	expression.fsp = fsp
	return expression, true
}

func defaultCall(value string) (args string, call bool, ok bool) {
	open := strings.IndexByte(value, '(')
	if open < 0 {
		for _, current := range value {
			if !(current == '_' || current >= 'A' && current <= 'Z' || current >= 'a' && current <= 'z') {
				return "", false, false
			}
		}
		return "", false, true
	}
	if !strings.HasSuffix(value, ")") || strings.Count(value, "(") != 1 || strings.Count(value, ")") != 1 {
		return "", false, false
	}
	args = value[open+1 : len(value)-1]
	if strings.TrimSpace(args) != args || strings.ContainsAny(args, "(),;") {
		return "", false, false
	}
	return args, true, true
}

func currentTemporalSQL(name string, fsp int) string {
	if fsp == 0 {
		return name
	}
	return name + "(" + strconv.Itoa(fsp) + ")"
}

func expressionTypeProblem(message string) *defaultProblem {
	return &defaultProblem{code: DiagnosticDefaultTypeMismatch, message: message}
}

func expressionFSPProblem(message string) *defaultProblem {
	return &defaultProblem{code: DiagnosticDefaultFractionalPrecision, message: message}
}

func invalidDefault(message string) *defaultProblem {
	return &defaultProblem{code: DiagnosticInvalidDefault, message: message}
}

func ambiguousDefault(message string) *defaultProblem {
	return &defaultProblem{code: DiagnosticAmbiguousDefault, message: message}
}

func unsupportedDefault(message string) *defaultProblem {
	return &defaultProblem{code: DiagnosticUnsupportedDefaultExpression, message: message}
}

func validNumericLiteral(value string) bool {
	if value == "" {
		return false
	}
	offset := 0
	if value[offset] == '+' || value[offset] == '-' {
		offset++
		if offset == len(value) {
			return false
		}
	}
	digits := 0
	for offset < len(value) && value[offset] >= '0' && value[offset] <= '9' {
		offset++
		digits++
	}
	if offset < len(value) && value[offset] == '.' {
		offset++
		for offset < len(value) && value[offset] >= '0' && value[offset] <= '9' {
			offset++
			digits++
		}
	}
	if digits == 0 {
		return false
	}
	if offset < len(value) && (value[offset] == 'e' || value[offset] == 'E') {
		offset++
		if offset < len(value) && (value[offset] == '+' || value[offset] == '-') {
			offset++
		}
		exponentDigits := 0
		for offset < len(value) && value[offset] >= '0' && value[offset] <= '9' {
			offset++
			exponentDigits++
		}
		if exponentDigits == 0 {
			return false
		}
	}
	return offset == len(value)
}

func decimalFits(value string, precision, scale int) bool {
	value = strings.TrimPrefix(strings.TrimPrefix(value, "+"), "-")
	integer, fraction, _ := strings.Cut(value, ".")
	integer = strings.TrimLeft(integer, "0")
	fraction = strings.TrimRight(fraction, "0")
	integerDigits := len(integer)
	return integerDigits <= precision-scale && len(fraction) <= scale
}

func negativeNonZero(value string) bool {
	if !strings.HasPrefix(value, "-") {
		return false
	}
	for _, current := range value[1:] {
		if current >= '1' && current <= '9' {
			return true
		}
	}
	return false
}

func integerLiteral(value string) bool {
	value = strings.TrimPrefix(value, "+")
	value = strings.TrimPrefix(value, "-")
	return value != "" && allDigits(value)
}

func allDigits(value string) bool {
	for offset := 0; offset < len(value); offset++ {
		if value[offset] < '0' || value[offset] > '9' {
			return false
		}
	}
	return true
}

func validUnquotedText(value string) bool {
	if value == "" {
		return true
	}
	for _, current := range value {
		if unicode.IsSpace(current) || unicode.IsControl(current) {
			return false
		}
		switch current {
		case '(', ')', '\'', '"', ',', ';', '+', '*', '/', '%', '=', '<', '>', '!', '\\':
			return false
		}
	}
	return true
}

func parseSQLStringLiteral(value string) (string, bool) {
	if len(value) < 2 || value[0] != '\'' && value[0] != '"' {
		return "", false
	}
	quote := value[0]
	var decoded strings.Builder
	for offset := 1; offset < len(value); offset++ {
		current := value[offset]
		if current == quote {
			if offset+1 < len(value) && value[offset+1] == quote {
				decoded.WriteByte(quote)
				offset++
				continue
			}
			return decoded.String(), offset == len(value)-1
		}
		if current != '\\' {
			decoded.WriteByte(current)
			continue
		}
		offset++
		if offset >= len(value) {
			return "", false
		}
		switch value[offset] {
		case '0':
			decoded.WriteByte(0)
		case 'b':
			decoded.WriteByte('\b')
		case 'n':
			decoded.WriteByte('\n')
		case 'r':
			decoded.WriteByte('\r')
		case 't':
			decoded.WriteByte('\t')
		case 'Z':
			decoded.WriteByte(0x1a)
		case '\\', '\'', '"':
			decoded.WriteByte(value[offset])
		case '%', '_':
			decoded.WriteByte('\\')
			decoded.WriteByte(value[offset])
		default:
			return "", false
		}
	}
	return "", false
}

func unwrapDefaultParentheses(value string) (string, bool, bool) {
	if !strings.HasPrefix(value, "(") || !strings.HasSuffix(value, ")") {
		return value, false, true
	}
	inner := strings.TrimSpace(value[1 : len(value)-1])
	if inner == "" {
		return "", false, false
	}
	return inner, true, true
}

func hasOuterParentheses(value string) bool {
	return len(value) >= 2 && value[0] == '(' && value[len(value)-1] == ')'
}
