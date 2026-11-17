//go:build banproof

package registry

import (
	"net/http/httptest" // want depguard "list 'api-no-http'"
)

// This file reaches for a package under net/http from the registry, on purpose. The list
// admits the standard library package by package, so nothing under net/http is admitted.
var _ = httptest.NewRecorder
