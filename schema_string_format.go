package jmespath

import "fmt"

func parseSchemaStringFormat(raw interface{}, path string, hasType bool, kind schemaKind) (stringFormat, bool, error) {
	if raw == nil {
		return stringFormatNone, false, nil
	}
	value, ok := raw.(string)
	if !ok {
		return stringFormatNone, true, unsupportedSchemaError(path, "format must be a string")
	}
	var format stringFormat
	switch value {
	case "date":
		format = stringFormatDate
	default:
		return stringFormatNone, true, unsupportedSchemaError(path, fmt.Sprintf("unsupported format %q", value))
	}
	if hasType && kind != schemaKindString {
		return stringFormatNone, true, unsupportedSchemaError(path, "format is supported only for type \"string\"")
	}
	return format, true, nil
}

func validateDateString(value string) error {
	// Keep this parser allocation-free because every schema-aware date comparison
	// goes through it for runtime operands.
	if len(value) != 10 || value[4] != '-' || value[7] != '-' {
		return fmt.Errorf("date must match YYYY-MM-DD")
	}

	year, ok := parseFourDigits(value[0], value[1], value[2], value[3])
	if !ok {
		return fmt.Errorf("date must match YYYY-MM-DD")
	}
	month, ok := parseTwoDigits(value[5], value[6])
	if !ok {
		return fmt.Errorf("date must match YYYY-MM-DD")
	}
	day, ok := parseTwoDigits(value[8], value[9])
	if !ok {
		return fmt.Errorf("date must match YYYY-MM-DD")
	}
	if month < 1 || month > 12 {
		return fmt.Errorf("month out of range")
	}
	if day < 1 || day > daysInMonth(year, month) {
		return fmt.Errorf("day out of range")
	}
	return nil
}

func isValidDateString(value string) bool {
	return validateDateString(value) == nil
}

func parseTwoDigits(first, second byte) (int, bool) {
	firstDigit, ok := parseDigit(first)
	if !ok {
		return 0, false
	}
	secondDigit, ok := parseDigit(second)
	if !ok {
		return 0, false
	}
	return firstDigit*10 + secondDigit, true
}

func parseFourDigits(a, b, c, d byte) (int, bool) {
	first, ok := parseDigit(a)
	if !ok {
		return 0, false
	}
	second, ok := parseDigit(b)
	if !ok {
		return 0, false
	}
	third, ok := parseDigit(c)
	if !ok {
		return 0, false
	}
	fourth, ok := parseDigit(d)
	if !ok {
		return 0, false
	}
	return ((first*10+second)*10+third)*10 + fourth, true
}

func parseDigit(value byte) (int, bool) {
	if value < '0' || value > '9' {
		return 0, false
	}
	return int(value - '0'), true
}

func daysInMonth(year, month int) int {
	switch month {
	case 2:
		if isLeapYear(year) {
			return 29
		}
		return 28
	case 4, 6, 9, 11:
		return 30
	default:
		return 31
	}
}

func isLeapYear(year int) bool {
	return year%4 == 0 && (year%100 != 0 || year%400 == 0)
}
