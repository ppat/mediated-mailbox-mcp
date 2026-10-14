package host

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// Both of a process's servers, the application server and the operations server, give a request's
// headers 10 seconds and the whole request 30, close a keep-alive connection idle for two minutes, and
// set no deadline on writing an answer, which may wait for leases and provider calls, or be an event
// stream (ADR-0130).
func TestTheConnectionLimits(t *testing.T) {
	type timeouts struct{ ReadHeader, Read, Idle, Write time.Duration }
	srv := newServer(http.NotFoundHandler(), slog.New(slog.DiscardHandler))
	want := timeouts{ReadHeader: 10 * time.Second, Read: 30 * time.Second, Idle: 2 * time.Minute}
	got := timeouts{ReadHeader: srv.ReadHeaderTimeout, Read: srv.ReadTimeout, Idle: srv.IdleTimeout, Write: srv.WriteTimeout}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the servers' timeouts (-want +got):\n%s", diff)
	}
}

// shortLimits runs the test's servers with limits short enough to watch, the idle limit below the read
// limit, since a server with no idle limit of its own closes an idle connection at its read limit.
func shortLimits(t *testing.T) limits {
	t.Helper()
	kept := connectionLimits
	connectionLimits = limits{Header: 2 * time.Second, Read: 1500 * time.Millisecond, Idle: 300 * time.Millisecond}
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
	// A server that never answers fails the test rather than holding it until the run times out.
	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			t.Error(err)
		}
	})
	return conn
}

// Each server closes a keep-alive connection once it has been idle for the idle limit, and cuts off a
// request whose body has not arrived within the read limit, so an idle or a trickling client holds no
// connection for longer, while an answer that takes longer than the read limit to write, such as an
// event stream, is never cut (ADR-0130).
func TestTheServersCloseIdleAndSlowConnectionsAndNeverCutAnAnswer(t *testing.T) {
	l := shortLimits(t)
	h, err := New(slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	h.Add("comp", []string{"/"}, Parts{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/long" {
			w.WriteHeader(http.StatusOK)
			for range 4 {
				if _, err := fmt.Fprintln(w, "part"); err != nil {
					return
				}
				if err := http.NewResponseController(w).Flush(); err != nil {
					return
				}
				time.Sleep(l.Read / 2)
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})})
	var lc net.ListenConfig
	app, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	probes, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- h.Serve(ctx, Listeners{App: app, Probes: probes}, Serving{}) }()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})

	for _, addr := range []string{app.Addr().String(), probes.Addr().String()} {
		idle := dial(t, addr)
		path := "/"
		if addr == probes.Addr().String() {
			path = "/healthz"
		}
		if _, err := fmt.Fprintf(idle, "GET %s HTTP/1.1\r\nHost: host\r\n\r\n", path); err != nil {
			t.Fatal(err)
		}
		resp, err := http.ReadResponse(bufio.NewReader(idle), nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
		if resp.Close {
			t.Fatalf("%s: the first request's connection was not kept open", addr)
		}
		if !closedWithin(t, idle, l.Idle+700*time.Millisecond) {
			t.Errorf("%s: an idle connection was still open %s after its last answer, past the idle limit of %s", addr, l.Idle+700*time.Millisecond, l.Idle)
		}

		slow := dial(t, addr)
		if _, err := fmt.Fprintf(slow, "POST %s HTTP/1.1\r\nHost: host\r\nContent-Length: 200\r\n\r\n{", path); err != nil {
			t.Fatal(err)
		}
		if !closedWithin(t, slow, l.Read+2*time.Second) {
			t.Errorf("%s: a request whose body stopped arriving was still held %s after it started, past the read limit of %s", addr, l.Read+2*time.Second, l.Read)
		}
	}

	long := dial(t, app.Addr().String())
	if _, err := fmt.Fprintf(long, "GET /long HTTP/1.1\r\nHost: host\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(long), nil)
	if err != nil {
		t.Fatal(err)
	}
	body := bufio.NewScanner(resp.Body)
	parts := 0
	for body.Scan() {
		parts++
	}
	if err := resp.Body.Close(); err != nil {
		t.Error(err)
	}
	if parts != 4 {
		t.Errorf("an answer written over twice the read limit was cut after %d of 4 parts", parts)
	}
}
