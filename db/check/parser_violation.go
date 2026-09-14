//go:build banproof

package check

// This file imports the SQL parser into non-test code on purpose. The parser is admitted only in
// tests, so it never reaches a binary.
import (
	_ "github.com/wasilibs/go-pgquery" // want depguard "list 'non-test-code'"
)
