//go:build banproof

package readiness

import (
	"net/http/httptest" // want depguard "list 'mediate-no-http'"
)

// This file reaches for a package under net/http from the readiness state, on purpose. The list
// admits the standard library package by package, so nothing under net/http is admitted.
var _ = httptest.NewRecorder
