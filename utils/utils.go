package utils

import "strconv"

// StringToInt64 converts a string to int64, 0 is returned on failure
func StringToInt64(s string) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		n = 0
	}
	return n
}
