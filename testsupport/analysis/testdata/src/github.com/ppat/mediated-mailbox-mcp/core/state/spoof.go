package state // want `a pure core's file carries a line directive`

import "errors"

// A line directive naming a test file does not make the variable below a test's.
//
//line spoof_test.go:1
var ErrSpoofed = errors.New("spoofed")
