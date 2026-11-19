//go:build banproof

package check

// This file imports the SQL parser into non-test code on purpose. The parser is admitted only in
// tests, so the data-access library's list over its non-test files and the list for non-test code both
// refuse it, and it never reaches a binary.
import (
	_ "github.com/wasilibs/go-pgquery" // want depguard "list 'db'" depguard "list 'non-test-code'"
)
