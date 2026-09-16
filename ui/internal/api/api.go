package api

import "io/fs"

// Server serves the UI. Everything it serves arrives through New, so nothing it reads is ambient.
type Server struct {
	bundle fs.FS
}

// New returns a server over the browser bundle, whose root holds the files bun build wrote.
func New(bundle fs.FS) *Server {
	return &Server{bundle: bundle}
}
