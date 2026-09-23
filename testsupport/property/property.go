// Package property holds the test support property tests use alongside rapid (ADR-0069). Check runs
// a property with the failing-case store, and Report runs the generator report.
//
// It imports rapid, so only property test files, crash-sequence test files and the crash harness may
// import it.
//
// A property is written as two functions. The draw function draws the arguments a value's
// constructors take, one field at a time, and returns them as a plain struct with exported fields.
// The property function builds values from those arguments through the constructors and checks a
// rule. Keeping the arguments apart from the values is what lets the store keep a failing case as
// its arguments, which still build the same value after the draw function is edited.
//
// Both Check and Report refuse to run unless RAPID_SEED is set to a non-zero number and
// RAPID_NOFAILFILE is true, the gating run's settings ADR-0069 requires. Neither may be called from
// inside a property, which the go vet analyser in testsupport/analysis enforces.
package property

import (
	"errors"
	"os"
	"strconv"

	"pgregory.net/rapid"
)

// requireSettings fails the test unless the environment carries the settings every property run
// needs.
func requireSettings(t rapid.TB) {
	t.Helper()
	if err := settingsProblem(os.LookupEnv); err != nil {
		t.Fatal(err)
	}
}

// settingsProblem returns why the environment lookup describes a run that is not the fixed,
// repeatable run ADR-0055 requires, or nil.
func settingsProblem(lookup func(string) (string, bool)) error {
	var problems []error
	seed, ok := lookup("RAPID_SEED")
	if n, err := strconv.ParseUint(seed, 10, 64); !ok || err != nil || n == 0 {
		problems = append(problems, errors.New("RAPID_SEED must be set to a non-zero number, or rapid picks a different seed on every run"))
	}
	nofailfile, ok := lookup("RAPID_NOFAILFILE")
	if on, err := strconv.ParseBool(nofailfile); !ok || err != nil || !on {
		problems = append(problems, errors.New("RAPID_NOFAILFILE must be true, or rapid writes a fail file that silently stops reproducing after a generator edit"))
	}
	return errors.Join(problems...)
}
