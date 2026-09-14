// Command mutate applies a mutation patch, runs the tests, demands that they fail, records which
// tests failed, and restores the tree.
package main

import (
	"fmt"
	"os"
)

// main exits non-zero, so a pipeline step calling this program before it is written fails rather
// than passing while it did nothing.
func main() {
	fmt.Fprintln(os.Stderr, "mutate is not written yet")
	os.Exit(1)
}
