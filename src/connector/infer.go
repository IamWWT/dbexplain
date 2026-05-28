package connector

import (
	"strconv"
	"strings"
	"time"
)

// inferColumnType samples string values and returns the most specific type.
// Detection order: INTEGER → FLOAT → DATE → TEXT
func inferColumnType(samples []string) string {
	if len(samples) == 0 {
		return "TEXT"
	}
	allInt := true
	allFloat := true
	allDate := true
	for _, s := range samples {
		if s == "" || s == "NULL" || s == "null" {
			continue
		}
		if allInt {
			if _, err := strconv.ParseInt(s, 10, 64); err != nil {
				allInt = false
			}
		}
		if allFloat {
			if _, err := strconv.ParseFloat(s, 64); err != nil {
				allFloat = false
			}
		}
		if allDate {
			if !isDateLike(s) {
				allDate = false
			}
		}
	}
	switch {
	case allInt:
		return "INTEGER"
	case allFloat:
		return "FLOAT"
	case allDate:
		return "DATE"
	default:
		return "TEXT"
	}
}

// isDateLike checks if a string matches common date/datetime formats.
func isDateLike(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 8 {
		return false
	}
	// Common date/datetime formats
	formats := []string{
		"2006-01-02",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006/01/02",
		"2006/01/02 15:04:05",
		"01/02/2006",
		"01/02/2006 15:04:05",
		"2006.01.02",
		"02-Jan-2006",
		"January 02, 2006",
		"Jan 02, 2006",
		"20060102",
	}
	for _, f := range formats {
		if _, err := time.Parse(f, s); err == nil {
			return true
		}
	}
	return false
}
