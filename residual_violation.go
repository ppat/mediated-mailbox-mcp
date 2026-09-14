//go:build banproof

package mediatedmailbox

// This file sits outside every component on purpose, where only the residual import list matches.
import (
	_ "pgregory.net/rapid" // want depguard "list 'residual'" depguard "list 'non-test-code'"
)
