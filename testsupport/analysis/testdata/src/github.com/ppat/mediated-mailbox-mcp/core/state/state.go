// Package state is a pure core holding every kind of package-level declaration the globals rule
// sees.
package state

import "errors"

const message = "constant"

// ErrRefused is an error value made by errors.New from a constant message, which a pure core may
// declare.
var ErrRefused = errors.New("refused")

// ErrConstant takes its message from a named constant.
var ErrConstant = errors.New(message)

// ErrTyped is declared with the error type and no value.
var ErrTyped error

var _ = ErrRefused

var _ = 1

var Fetch func(id string) (string, error) // want `a pure core declares no package-level variable other than an error value`

var (
	cache = map[string]int{} // want `a pure core declares no package-level variable other than an error value`
	count int                // want `a pure core declares no package-level variable other than an error value`
)

var first, ErrSecond = 1, errors.New("second") // want `a pure core declares no package-level variable other than an error value`

var Box *int // want `a pure core declares no package-level variable other than an error value`

var Pair struct{ A int } // want `a pure core declares no package-level variable other than an error value`

type limitError struct{ max int }

func (e *limitError) Error() string { return "limit" }

var ErrLimit error = &limitError{max: 5} // want `a pure core's error value has no initial value or is made by errors.New from a constant message`

func text() string { return "computed" }

var ErrComputed = errors.New(text()) // want `a pure core's error value has no initial value or is made by errors.New from a constant message`

// Refuse writes the package's own variables, each a write only a declaration may make.
func Refuse(ids []string) error {
	ErrRefused = errors.New("replaced") // want `writes ErrRefused, a pure core's package-level variable`
	cache["k"] = 1                      // want `writes cache, a pure core's package-level variable`
	count++                             // want `writes count, a pure core's package-level variable`
	for _, first = range []int{1} {     // want `writes first, a pure core's package-level variable`
	}
	for count = range 3 { // want `writes count, a pure core's package-level variable`
	}
	ErrSpoofed = nil // want `writes ErrSpoofed, a pure core's package-level variable`
	*Box = 1         // want `writes Box, a pure core's package-level variable`
	Pair.A = 1 // want `writes Pair, a pure core's package-level variable`
	p := &ErrTyped // want `writes ErrTyped, a pure core's package-level variable`
	*p = nil
	ErrRefused := errors.New("a local of the same name")
	_ = ids
	return ErrRefused
}
