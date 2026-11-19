//go:build banproof

package markdown

// This file imports the CommonMark parser from non-test code on purpose. The family's list over its test
// files admits the parser for the property test that reads the output, so the family's list over its
// non-test files and the list for non-test code report it.
import (
	_ "github.com/yuin/goldmark" // want depguard "list 'content'" depguard "list 'non-test-code'"
)
