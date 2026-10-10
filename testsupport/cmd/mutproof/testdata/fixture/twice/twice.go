// Package twice holds two functions with the same body, so a hunk that changes one of them with too
// little context matches both.
package twice

// Allow reports whether an item may pass.
func Allow(flagged bool) bool {
	if flagged {
		return false
	}
	return true
}

// Admit reports whether an item may enter.
func Admit(flagged bool) bool {
	if flagged {
		return false
	}
	return true
}
