//go:build devloop

package devloop

// init marks a binary built with the devloop build tag. The tag is positive, so a build without it
// compiles no file that sets the mark.
func init() {
	enabled = true
}
