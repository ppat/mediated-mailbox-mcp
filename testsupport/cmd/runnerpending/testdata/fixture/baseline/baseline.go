// Package baseline holds a test that fails before any patch is applied, beside one that passes.
package baseline

// Double returns twice n.
func Double(n int) int {
	return n * 2
}
