package migrations

import (
	"errors"
	"fmt"
	"io/fs"
	"strconv"
	"strings"
)

// ErrBelowFloor is the refusal of a schema whose compatibility floor is above this build's chain, so a
// migration this build's binaries cannot run on has been applied (ADR-0140).
var ErrBelowFloor = errors.New("the schema's compatibility floor is above this binary's migration chain")

// ErrNotMigrated is the refusal of a schema that has not yet applied every migration of this build's
// chain, so the migration step has not run for this release (ADR-0140).
var ErrNotMigrated = errors.New("the database has not applied this binary's migration chain")

// Version is the version of the newest migration of the chain this build carries, read from the
// embedded files' names, each of which starts with its version and an underscore, as goose reads them.
// It panics on a chain holding a file named otherwise, which the embedded chain's own test refuses.
func Version() int64 {
	names, err := fs.Glob(chain, "*.sql")
	if err != nil {
		panic(err)
	}
	var newest int64
	for _, name := range names {
		prefix, _, found := strings.Cut(name, "_")
		v, err := strconv.ParseInt(prefix, 10, 64)
		if !found || err != nil {
			panic(fmt.Sprintf("migration %s is not named for its version", name))
		}
		newest = max(newest, v)
	}
	return newest
}

// Refuse decides whether a binary built with this chain may run on a schema whose newest applied
// migration is applied and whose compatibility floor is floor. It refuses a floor above the chain's
// version, and a newest applied migration older than it, and accepts every other schema, a newer one
// whose floor the chain meets among them (ADR-0140).
func Refuse(applied, floor int64) error {
	k := Version()
	if k < floor {
		return fmt.Errorf("%w: the floor is %d and the chain's newest migration is %d", ErrBelowFloor, floor, k)
	}
	if applied < k {
		return fmt.Errorf("%w: the newest applied migration is %d and the chain's newest is %d", ErrNotMigrated, applied, k)
	}
	return nil
}
