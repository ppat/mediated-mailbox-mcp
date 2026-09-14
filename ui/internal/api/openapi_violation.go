//go:build banproof

package api

// This file imports the contract generator's document library into shipping code, on purpose. The
// generator is test code, so the library never reaches the binary.
import (
	_ "github.com/getkin/kin-openapi/openapi3" // want depguard "list 'non-test-code'"
)
