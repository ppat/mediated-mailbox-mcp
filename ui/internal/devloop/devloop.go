package devloop

// enabled is false in every binary built without the devloop build tag, every image among them, and
// only on.go's init, compiled under the tag, sets it.
var enabled = false

// Enabled reports whether the binary was built with the devloop build tag.
func Enabled() bool { return enabled }
