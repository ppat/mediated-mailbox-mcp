//go:build banproof

package sessions

// This file attaches connections to the Session from outside the connections' package, on purpose,
// and banproof requires the finding below from go vet. Only process/connections attaches, when the
// connections are built, so code that holds the Session reaches a credential only through that
// package (ADR-0071, ADR-0142).
func attachesFromHere(s *Session, c Connections) error {
	return s.Attach(c) // want vetcheck "uses the Session's Attach outside process/connections"
}
