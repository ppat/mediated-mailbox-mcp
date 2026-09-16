//go:build integration && banproof

package tx_test

// This file imports rapid into an ordinary integration test on purpose. It is linted only while the
// configuration lists the integration tag.
import (
	_ "pgregory.net/rapid" // want depguard "list 'ordinary-tests'"
)
