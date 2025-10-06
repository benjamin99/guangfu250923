package handlers

import "strconv"

// parsePositiveInt parses a query parameter into an int with bounds and default.
// If invalid or out of range it falls back to defaultValue.
func parsePositiveInt(raw string, defaultValue, min, max int) int {
	if raw == "" {
		return defaultValue
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return defaultValue
	}
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// checkStringPtrEqual checks if the following condition is met:
// - both are nil
// - both are not nil and are holding the same value
func checkStringPtrEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}
