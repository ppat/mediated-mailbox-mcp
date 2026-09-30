package api

import (
	"net/http"
	"testing"
)

// A test's mux is never served, so its registrations are not reported.
func TestServe(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle("/", http.NotFoundHandler())
}
