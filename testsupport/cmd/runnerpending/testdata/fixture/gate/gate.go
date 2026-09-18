// Package gate holds a mechanism for the runner's tests to remove. It refuses flagged items.
package gate

// Allow reports whether an item may pass.
func Allow(flagged bool) bool {
	return !flagged
}
