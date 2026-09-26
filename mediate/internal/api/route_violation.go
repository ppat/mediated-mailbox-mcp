//go:build banproof

package api

import (
	"net/http" // want depguard "list 'mediate-api-root'"
)

// This file hand-registers an API operation beside the API root's generator, on purpose. Only the
// generator, api.go, may serve HTTP, so an operation the registry does not carry cannot be added to
// the root (ADR-0053).
func handRegistered(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/approve_plan", func(http.ResponseWriter, *http.Request) {})
}
