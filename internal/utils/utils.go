package utils

// Utility functions for the application
import (
	"strconv"
)

// StringToInt converts a string to an integer
func StringToInt(s string) (int, error) {
	return strconv.Atoi(s)
}

// IntToString converts an integer to a string
func IntToString(i int) string {
	return strconv.Itoa(i)
}
