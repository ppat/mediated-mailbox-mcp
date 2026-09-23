//go:build banproof

package redact // want vetcheck "a pure core's file carries a line directive"

// This file gives a pure core package-level state on purpose, and banproof requires the wants below
// from go vet. A function variable set from outside would reach whatever the setter wraps, an error
// value holding a pointer could be changed through it, and a line directive could make a variable
// of the core pass for a test file's.

var Fetch func(messageID string) (string, error) // want vetcheck "a pure core declares no package-level variable other than an error value"

func init() {
	Fetch = func(string) (string, error) { return "", nil } // want vetcheck "writes Fetch, a pure core's package-level variable"
}

type limitError struct{ max int }

func (e *limitError) Error() string { return "limit" }

var ErrLimit error = &limitError{max: 5} // want vetcheck "a pure core's error value has no initial value or is made by errors.New from a constant message"

//line globals_violation_test.go:1
