//go:build banproof

package markdown

// This file imports the CommonMark parser from non-test code on purpose. The library's own list
// admits the parser for the property test that reads the output, so only the list for non-test code
// reports it.
import (
	_ "github.com/yuin/goldmark" // want depguard "list 'non-test-code'"
)
