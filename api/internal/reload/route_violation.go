//go:build banproof

package reload

import (
	"net/http/httptest" // want depguard "list 'api-no-http'"
)

// This file reaches for a package under net/http from the reload, on purpose. The list
// admits the standard library package by package, so nothing under net/http is admitted.
var _ = httptest.NewRecorder
