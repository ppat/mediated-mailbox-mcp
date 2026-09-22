// Package count holds a mechanism whose removal a property test catches only after many cases.
package count

// Holds reports whether the mechanism holds for the nth generated case.
func Holds(n int) bool {
	return n > 0
}
