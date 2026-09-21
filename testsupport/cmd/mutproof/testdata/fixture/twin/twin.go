// Package twin holds a test with the same name as one in package gate, which no gate patch reaches.
package twin

// Refuse reports whether a flagged item is refused.
func Refuse(flagged bool) bool {
	return flagged
}
