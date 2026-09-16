// Package db is the data-access library, published as mediated-mailbox-db. It is a narrow, named
// exception to the rule that shared code is pure.
//
// It holds the whole database concern in one directory. The goose migration chain, the superuser
// bootstrap, the subsections, the shared transaction helper, and the checks over statement and
// migration files each have a subdirectory. A subsection is one concern's statement files beside the
// package sqlc generates from them, and other components import only the subsections their import
// lists admit. Nothing belongs in this package itself.
package db
