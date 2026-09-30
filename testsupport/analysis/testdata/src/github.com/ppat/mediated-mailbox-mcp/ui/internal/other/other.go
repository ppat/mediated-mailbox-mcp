// Package other declares a type named like the recording mux in another package, which is not exempt.
package other

import "net/http"

type recordingMux struct{ mux *http.ServeMux }

func (m *recordingMux) Handle(pattern string, h http.Handler) {
	m.mux.Handle(pattern, h) // want `\(\*http.ServeMux\).Handle registers a route`
}
