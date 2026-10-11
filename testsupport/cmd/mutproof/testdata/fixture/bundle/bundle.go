// Package bundle holds a mechanism for the runner's tests to remove, whose test reads the built
// browser bundle, which git ignores. It names the bundle's entry module.
package bundle

// Entry is the file name of the bundle's entry module.
func Entry() string {
	return "main.js"
}
