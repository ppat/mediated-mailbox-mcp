//go:build banproof

package api

import "net/http"

// This file registers a route on the mux beneath the recording mux, on purpose. The route check reads
// only what the recording mux's Handle records, so a route registered this way would be served
// unchecked (ADR-0057, ADR-0071).
func unrecorded(m *recordingMux) {
	m.mux.Handle("GET /api/unchecked", http.NotFoundHandler()) // want vetcheck "\\(\\*http.ServeMux\\).Handle registers a route the UI's route check never sees"
}
