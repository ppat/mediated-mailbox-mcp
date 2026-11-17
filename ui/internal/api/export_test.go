package api

// NewServer builds the read API's server from opts, as New builds it from the configuration and the
// process, so the package's tests can hand it a fixed clock, the provider fake's consents and a fault
// between a setup's writes, which no configuration carries. It exists only in this package's test
// binary.
var NewServer = newServer

// SetFault makes the server fail at the named point between a setup's writes, so a test proves the
// writes land together or not at all (ADR-0060). It exists only in this package's test binary.
func SetFault(s *Server, fault func(at string) error) { s.between = fault }
