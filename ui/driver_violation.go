//go:build banproof

package main

// This file imports two of the driver's packages the UI does not use, on purpose. The component's
// list names the driver's root, pgtype and pgxpool exactly, and a prefix entry for the driver would
// admit both. The list for non-test code admits the whole driver, so only the component's list
// reports them.
import (
	_ "github.com/jackc/pgx/v5/pgconn" // want depguard "list 'ui'"
	_ "github.com/jackc/pgx/v5/stdlib" // want depguard "list 'ui'"
)
