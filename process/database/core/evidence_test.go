package core_test

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/process/database/core"
)

// Each outcome gives the evidence the classification states. A caller that stopped waiting proves
// nothing, a failed connection attempt is a fault however it failed unless the pool still holds a
// connection, a statement the server refused
// shows it reachable unless the refusal's class says it cannot serve, and a statement that lost its
// connection without an answer is a fault (ADR-0142).
func TestClassify(t *testing.T) {
	cases := []struct {
		name string
		in   core.Outcome
		want core.Evidence
	}{
		{"a statement that succeeded", core.Outcome{}, core.Reachable},
		{"a connection attempt that succeeded", core.Outcome{Connect: true}, core.Reachable},
		{"a statement its caller cancelled, its connection lost", core.Outcome{Failed: true, CallerEnded: true, Lost: true}, core.None},
		{"a connection attempt its caller cancelled", core.Outcome{Connect: true, Failed: true, CallerEnded: true}, core.None},
		{"a connection attempt the network refused", core.Outcome{Connect: true, Failed: true}, core.Fault},
		{"a connection attempt the server refused while the pool holds a connection", core.Outcome{Connect: true, Failed: true, Holding: true, Code: "55000"}, core.None},
		{"a connection attempt that succeeded while the pool holds a connection", core.Outcome{Connect: true, Holding: true}, core.Reachable},
		{"a connection attempt whose credentials the server refused", core.Outcome{Connect: true, Failed: true, Code: "28P01"}, core.Fault},
		{"a statement refused for a unique constraint", core.Outcome{Failed: true, Code: "23505"}, core.Reachable},
		{"a statement cancelled by the server's statement timeout", core.Outcome{Failed: true, Code: "57014"}, core.Reachable},
		{"a statement refused by row security", core.Outcome{Failed: true, Code: "42501"}, core.Reachable},
		{"a statement ended by an administrator's shutdown", core.Outcome{Failed: true, Code: "57P01", Lost: true}, core.Fault},
		{"a statement refused while the server cannot connect now", core.Outcome{Failed: true, Code: "57P03"}, core.Fault},
		{"a statement refused for a connection exception", core.Outcome{Failed: true, Code: "08006"}, core.Fault},
		{"a statement refused for a full disk", core.Outcome{Failed: true, Code: "53100"}, core.Fault},
		{"a statement whose connection was lost with no answer", core.Outcome{Failed: true, Lost: true}, core.Fault},
		{"a statement whose result the client could not read", core.Outcome{Failed: true}, core.None},
	}
	for _, c := range cases {
		if got := core.Classify(c.in); got != c.want {
			t.Errorf("%s: Classify(%+v) = %d, want %d", c.name, c.in, got, c.want)
		}
	}
}
