// Package config holds every program's configuration type, with each part's defaults and checks
// (ADR-0078). The configuration library in process/settings layers the values into these types, and
// a program's check runs on what it returns before anything else starts (ADR-0040).
//
// A shared part is a section every program that has it reads under the same keys. They are Process,
// Operations, Serving, Database, Keys, Accounts and Scanner. A program's type, API, UI, Worker or
// Migrator, holds the shared parts it has and its own keys. A part embedded inline sits beside the
// type's own keys at the top of the tree. Each part and each program's type has a Check method, and a
// program's Check checks every part it has, in the order the refusals have always been made.
//
// A length of time is a Duration, a number of nanoseconds, which the configuration library fills
// from text such as 5m and writes back as such text. Its reader converts it where it is used, so
// this package imports no clock.
//
// Each type is pinned field by field by the tests beside it, so a new value, one meant to hold a
// secret included, is a visible change.
package config

// Duration is a length of time in nanoseconds, the unit of a time.Duration, which a reader converts
// it to where it is used.
type Duration int64

// The lengths of time the defaults are written in.
const (
	millisecond Duration = 1_000_000
	second               = 1000 * millisecond
	minute               = 60 * second
	hour                 = 60 * minute
)
