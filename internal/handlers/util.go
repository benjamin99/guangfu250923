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
