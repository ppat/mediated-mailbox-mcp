package state // want `a pure core's file carries a line directive`

import "errors"

// A line directive written as a block comment is refused as well.
/*line block_test.go:1*/
var ErrBlock = errors.New("block")
