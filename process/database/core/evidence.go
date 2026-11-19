package core

import "strings"

// Evidence is what one statement's or one connection attempt's outcome says about whether the
// database can serve the process (ADR-0142).
type Evidence int

// The kinds of evidence an outcome gives.
const (
	// None says nothing about the database, such as a statement its caller stopped waiting for or an
	// error the client raised reading a result.
	None Evidence = iota
	// Reachable shows the database answering, a success or an error the server itself returned about
	// the statement, such as a constraint it refused.
	Reachable
	// Fault shows the database unable to serve: a connection attempt that failed, a statement whose
	// failure lost its connection, or an error the server returned saying it cannot serve at all.
	Fault
)

// Outcome is what the database package observed when a statement ended or a connection attempt
// returned.
type Outcome struct {
	// Connect is set for a connection attempt, and unset for a statement.
	Connect bool
	// Holding is set for a connection attempt that ended while the pool held another connection,
	// idle or in use.
	Holding bool
	// Failed is set when it returned an error.
	Failed bool
	// CallerEnded is set when the context it ran under had ended, so the caller stopped waiting.
	CallerEnded bool
	// Code is the SQLSTATE the server returned, empty when the error did not come from the server.
	Code string
	// Lost is set when a statement's connection was closed once it ended.
	Lost bool
}

// Classify returns the evidence an outcome gives. A caller that stopped waiting proves nothing about
// the database, so its outcome is no evidence even when the connection was lost to it. A failed
// connection attempt is no evidence while the pool holds another connection, since the pool can still
// serve on what it holds, and a held connection that has lost the database shows it at its next
// statement. Any other failed connection attempt is a fault, the server's refusal of the credentials
// included, since a pool that holds no connection and cannot make one cannot serve. A statement the server answered with an error shows it
// reachable, unless the error's class says it cannot serve: a connection exception (08), insufficient
// resources (53), or an operator's intervention such as a shutdown (57P). A statement that failed
// with no answer from the server is a fault when its connection was lost, and no evidence otherwise.
func Classify(o Outcome) Evidence {
	switch {
	case !o.Failed:
		return Reachable
	case o.CallerEnded:
		return None
	case o.Connect && o.Holding:
		return None
	case o.Connect:
		return Fault
	case o.Code != "":
		if unableToServe(o.Code) {
			return Fault
		}
		return Reachable
	case o.Lost:
		return Fault
	default:
		return None
	}
}

// unableToServe reports whether a SQLSTATE says the server cannot serve, rather than refusing one
// statement.
func unableToServe(code string) bool {
	return strings.HasPrefix(code, "08") || strings.HasPrefix(code, "53") || strings.HasPrefix(code, "57P")
}
