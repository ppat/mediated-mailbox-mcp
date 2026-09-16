//go:build banproof

package contract_test

// This file imports a kin-openapi package beside the one the lists name exactly, on purpose. It sorts
// directly after the exact entry, so dropping the entry's $ admits it.
import (
	_ "github.com/getkin/kin-openapi/openapi3filter" // want depguard "list 'ui'" depguard "list 'ordinary-tests'"
)
