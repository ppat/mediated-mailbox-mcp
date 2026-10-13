package app

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// Both of the mediator's HTTP servers, the client surface's and the probes', give a request's headers
// 10 seconds and the whole request 30, close a keep-alive connection idle for two minutes, and set no
// deadline on writing an answer, which may wait for leases and provider calls (ADR-0130).
func TestTheConnectionLimits(t *testing.T) {
	type timeouts struct{ ReadHeader, Read, Idle, Write time.Duration }
	of := func(s *http.Server) timeouts {
		return timeouts{ReadHeader: s.ReadHeaderTimeout, Read: s.ReadTimeout, Idle: s.IdleTimeout, Write: s.WriteTimeout}
	}
	surface, probes := servers(Configuration{}, http.NotFoundHandler(), http.NotFoundHandler(), discard())
	limited := timeouts{ReadHeader: 10 * time.Second, Read: 30 * time.Second, Idle: 2 * time.Minute}
	want := map[string]timeouts{"client surface": limited, "probes": limited}
	got := map[string]timeouts{"client surface": of(surface), "probes": of(probes)}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the servers' timeouts (-want +got):\n%s", diff)
	}
}

// shortLimits runs the test's mediator with limits short enough to watch, the idle limit below the
// read limit, since a server with no idle limit of its own closes an idle connection at its read
// limit.
func shortLimits(t *testing.T) limits {
	t.Helper()
	kept := connectionLimits
	connectionLimits = limits{header: 2 * time.Second, read: 1500 * time.Millisecond, idle: 300 * time.Millisecond}
	t.Cleanup(func() { connectionLimits = kept })
	return connectionLimits
}

// closedWithin reports whether conn's server closes it within d, reading and discarding whatever the
// server writes until then.
func closedWithin(t *testing.T, conn net.Conn, d time.Duration) bool {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(d)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4096)
	for {
		_, err := conn.Read(buf)
		var timeout net.Error
		switch {
		case err == nil:
		case errors.As(err, &timeout) && timeout.Timeout():
			return false
		default:
			return true
		}
	}
}

func dial(t *testing.T, addr string) net.Conn {
	t.Helper()
	conn, err := (&net.Dialer{}).DialContext(t.Context(), "tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Error(err)
		}
	})
	return conn
}

// The client surface closes a keep-alive connection once it has been idle for the idle limit, and
// cuts off a request whose body has not arrived within the read limit, so an idle or a trickling
// client holds no connection for longer (ADR-0130).
func TestTheSurfaceClosesIdleAndSlowConnections(t *testing.T) {
	l := shortLimits(t)
	addr, _, _ := started(t, Configuration{})

	idle := dial(t, addr)
	if _, err := fmt.Fprintf(idle, "GET /api/accounts/acct-a/echo HTTP/1.1\r\nHost: mediator\r\nAuthorization: Bearer s3cret\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(idle), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := resp.Body.Close(); err != nil {
		t.Error(err)
	}
	if resp.StatusCode != http.StatusOK || resp.Close {
		t.Fatalf("the request on the kept-alive connection answered %d, close %t, want 200 on a connection kept open", resp.StatusCode, resp.Close)
	}
	if !closedWithin(t, idle, l.idle+700*time.Millisecond) {
		t.Errorf("an idle connection was still open %s after its last answer, past the idle limit of %s", l.idle+700*time.Millisecond, l.idle)
	}

	slow := dial(t, addr)
	if _, err := fmt.Fprintf(slow, "POST /mcp HTTP/1.1\r\nHost: mediator\r\nAuthorization: Bearer s3cret\r\nContent-Type: application/json\r\n"+
		"Accept: application/json, text/event-stream\r\nContent-Length: 200\r\n\r\n{"); err != nil {
		t.Fatal(err)
	}
	if !closedWithin(t, slow, l.read+2*time.Second) {
		t.Errorf("a request whose body stopped arriving was still held %s after it started, past the read limit of %s", l.read+2*time.Second, l.read)
	}
}
