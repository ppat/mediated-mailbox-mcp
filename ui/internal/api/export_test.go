package api

// SetFault makes the server fail at the named point between a setup's writes, so a test proves the
// writes land together or not at all (ADR-0060). It exists only in this package's test binary.
func SetFault(s *Server, fault func(at string) error) { s.between = fault }
