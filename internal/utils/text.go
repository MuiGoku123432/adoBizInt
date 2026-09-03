package utils

// Truncate shortens s to at most maxLen characters, replacing the tail with an
// ellipsis when there is room for one. It cuts on rune boundaries so multi-byte
// characters are never split.
func Truncate(s string, maxLen int) string {
	// Byte length bounds rune count, so this is a safe fast path.
	if len(s) <= maxLen {
		return s
	}
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(r[:maxLen])
	}
	return string(r[:maxLen-3]) + "..."
}
