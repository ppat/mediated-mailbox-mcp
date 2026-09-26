//go:build banproof

package service

import (
	"net/http" // want depguard "list 'mediate-no-http'"
)

// This file hand-registers an API operation from the service layer, on purpose. The service layer
// serves no HTTP, so an operation the registry does not carry cannot be added to the client surface
// from here (ADR-0053).
func routeRegistered(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/approve_plan", func(http.ResponseWriter, *http.Request) {})
}
