// Package api stands for the UI's server, which holds the recording mux.
package api

import "net/http"

type recordingMux struct {
	mux      *http.ServeMux
	patterns []string
}

// Handle is the recording mux's own registration, the one place a route may reach a mux.
func (m *recordingMux) Handle(pattern string, h http.Handler) {
	m.patterns = append(m.patterns, pattern)
	m.mux.Handle(pattern, h)
}

// HandleFunc is another method of the recording mux, which records nothing.
func (m *recordingMux) HandleFunc(pattern string, h func(http.ResponseWriter, *http.Request)) {
	m.mux.HandleFunc(pattern, h) // want `\(\*http.ServeMux\).HandleFunc registers a route the UI's route check never sees`
}

// probesMux is another type with a Handle method of its own.
type probesMux struct{ mux *http.ServeMux }

func (p *probesMux) Handle(pattern string, h http.Handler) {
	p.mux.Handle(pattern, h) // want `\(\*http.ServeMux\).Handle registers a route`
}

func build(h http.Handler) {
	mux := &recordingMux{mux: http.NewServeMux()}
	mux.Handle("GET /api/lens", h)
	mux.mux.Handle("GET /api/hidden", h) // want `\(\*http.ServeMux\).Handle registers a route`

	second := http.NewServeMux()
	second.HandleFunc("GET /extra", func(http.ResponseWriter, *http.Request) {}) // want `\(\*http.ServeMux\).HandleFunc registers a route`
	register := second.Handle                                                    // want `\(\*http.ServeMux\).Handle registers a route`
	register("GET /value", h)

	http.Handle("/default", h)                                                    // want `http.Handle registers a route`
	http.HandleFunc("/default-func", func(http.ResponseWriter, *http.Request) {}) // want `http.HandleFunc registers a route`
}

// registrar is an interface a mux satisfies, so a mux held in it registers through it.
type registrar interface {
	Handle(pattern string, h http.Handler)
}

// lookalike has a Handle method whose parameters differ from the mux's.
type lookalike interface {
	Handle(pattern string)
}

func throughInterfaces(h http.Handler, l lookalike) {
	var r registrar = http.NewServeMux()
	r.Handle("GET /iface", h) // want `the interface method Handle registers a route`
	interface {
		HandleFunc(string, func(http.ResponseWriter, *http.Request))
	}(http.NewServeMux()).HandleFunc("GET /anon", nil) // want `the interface method HandleFunc registers a route`
	viaGeneric(http.NewServeMux(), h)
	l.Handle("GET /lookalike")
}

func viaGeneric[T registrar](t T, h http.Handler) {
	t.Handle("GET /generic", h) // want `the interface method Handle registers a route`
}

// The mux's parameters spelled through aliases are the mux's parameters.
type (
	P  = string
	H  = http.Handler
	HF = func(http.ResponseWriter, *http.Request)
	W  = http.ResponseWriter
)

type generic[T any] interface {
	Handle(pattern string, h T)
}

// results has a HandleFunc whose handler returns a value, which a mux does not satisfy.
type results interface {
	HandleFunc(pattern string, h func(http.ResponseWriter, *http.Request) error)
}

func throughAliases(h http.Handler, res results) {
	var p interface{ Handle(P, H) } = http.NewServeMux()
	p.Handle("GET /aliases", h) // want `the interface method Handle registers a route`
	var f interface{ HandleFunc(string, HF) } = http.NewServeMux()
	f.HandleFunc("GET /func-alias", nil) // want `the interface method HandleFunc registers a route`
	var g generic[H] = http.NewServeMux()
	g.Handle("GET /generic-alias", h) // want `the interface method Handle registers a route`
	var w interface {
		HandleFunc(string, func(W, *http.Request))
	} = http.NewServeMux()
	w.HandleFunc("GET /nested-alias", nil) // want `the interface method HandleFunc registers a route`
	res.HandleFunc("GET /results", nil)
}
