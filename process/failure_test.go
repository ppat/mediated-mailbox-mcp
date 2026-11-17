package process

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
)

// A server that fails while the process serves stops the process in order, with an error naming the
// server, and a goroutine started through Go that runs until it is told to stop is told to stop and
// waited for before the run returns (ADR-0142).
func TestAServerThatFailsWhileServingStopsTheProcessInOrder(t *testing.T) {
	for _, failing := range []string{"the application server", "the operations server"} {
		t.Run(failing, func(t *testing.T) {
			p, err := newProcess("test", slog.New(slog.DiscardHandler))
			if err != nil {
				t.Fatal(err)
			}
			var stopped atomic.Bool
			p.goroutine("/", "steady", func(ctx context.Context) {
				<-ctx.Done()
				// A goroutine takes a moment to stop, which a process that did not wait for it would miss.
				time.Sleep(50 * time.Millisecond)
				stopped.Store(true)
			})
			var lc net.ListenConfig
			app, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			probes, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			// A failure the process missed would leave it serving until the context ends.
			ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() {
				done <- p.serveOn(ctx, app, probes, config.Serving{}, built{handlers: []mounted{{pattern: "/", handler: http.NotFoundHandler()}}})
			}()
			// The process serves once the health probe answers.
			for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+probes.Addr().String()+"/healthz", http.NoBody)
				if err != nil {
					t.Fatal(err)
				}
				if resp, err := http.DefaultClient.Do(req); err == nil {
					if err := resp.Body.Close(); err != nil {
						t.Error(err)
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("the process never served")
				}
			}
			ln := app
			if failing == "the operations server" {
				ln = probes
			}
			if err := ln.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				t.Fatal(err)
			}
			err = <-done
			if err == nil || !strings.Contains(err.Error(), failing+" failed") {
				t.Errorf("the run returned %v, want %s's failure", err, failing)
			}
			if ctx.Err() != nil {
				t.Error("the process served until its context ended rather than stopping on the failure")
			}
			if !stopped.Load() {
				t.Error("the run returned before the steady goroutine was told to stop and returned")
			}
		})
	}
}
