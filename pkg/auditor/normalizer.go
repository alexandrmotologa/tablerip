package auditor

import (
	"regexp"
	"strconv"
	"strings"
)

var (
	parenRegex         = regexp.MustCompile(`^\s*\(\s*([\d,]+\.?\d*)\s*\)\s*$`)
	trailingMinusRegex = regexp.MustCompile(`^\s*([\d,]+\.?\d*)-\s*$`)
	standardNumRegex   = regexp.MustCompile(`^(-?[\d,]+\.?\d*)$`)
)

// ParseFinancialNumber converts a financial table cell string into a numeric float64.
// Returns (value, isNumeric, currency).
func ParseFinancialNumber(raw string) (float64, bool, string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, false, ""
	}

	// Check for dash/em-dash representing zero or nil
	if trimmed == "-" || trimmed == "—" || trimmed == "–" {
		return 0.0, true, ""
	}

	// Detect and extract currency
	currency := ""
	if strings.Contains(trimmed, "$") {
		currency = "USD"
	} else if strings.Contains(trimmed, "€") {
		currency = "EUR"
	} else if strings.Contains(trimmed, "£") {
		currency = "GBP"
	} else if strings.Contains(trimmed, "¥") {
		currency = "JPY"
	}

	// Remove currency symbols and surrounding whitespace
	cleaned := strings.ReplaceAll(trimmed, "$", "")
	cleaned = strings.ReplaceAll(cleaned, "€", "")
	cleaned = strings.ReplaceAll(cleaned, "£", "")
	cleaned = strings.ReplaceAll(cleaned, "¥", "")
	cleaned = strings.TrimSpace(cleaned)

	// Case 1: Accounting Parentheses, e.g. (1,234) or ( 450 )
	if m := parenRegex.FindStringSubmatch(cleaned); len(m) > 1 {
		numStr := strings.ReplaceAll(m[1], ",", "")
		val, err := strconv.ParseFloat(numStr, 64)
		if err == nil {
			return -val, true, currency
		}
	}

	// Case 2: Trailing Minus, e.g. 1,234-
	if m := trailingMinusRegex.FindStringSubmatch(cleaned); len(m) > 1 {
		numStr := strings.ReplaceAll(m[1], ",", "")
		val, err := strconv.ParseFloat(numStr, 64)
		if err == nil {
			return -val, true, currency
		}
	}

	// Case 3: Standard numeric with optional leading minus
	if m := standardNumRegex.FindStringSubmatch(cleaned); len(m) > 1 {
		numStr := strings.ReplaceAll(m[1], ",", "")
		val, err := strconv.ParseFloat(numStr, 64)
		if err == nil {
			return val, true, currency
		}
	}

	return 0, false, ""
}
