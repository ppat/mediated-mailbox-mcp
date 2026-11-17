//go:build banproof

package main

// This file imports two of the driver's packages the UI does not use, on purpose. The component's
// lists name the driver's packages they admit exactly, and a prefix entry for the driver would admit
// both. The list for non-test code admits the whole driver, so only the component's lists report them.
import (
	_ "github.com/jackc/pgx/v5/pgproto3" // want depguard "list 'ui'" depguard "list 'ui-outside-setup'"
	_ "github.com/jackc/pgx/v5/stdlib"   // want depguard "list 'ui'" depguard "list 'ui-outside-setup'"
)
