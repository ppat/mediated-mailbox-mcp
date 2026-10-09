// Package probes serves the health probe and the metrics endpoint of a process that runs work rather
// than serving requests, on a listener of its own (ADR-0051). /healthz answers 200 while the process
// runs, and /metrics serves the process's registry, between units of work as during them. A process
// whose readiness changes while it runs, such as the mediator, serves its own probes, because its
// /readyz answers from state only it holds.
package probes

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ppat/mediated-mailbox-mcp/process/logging"
)

// Serve serves the health probe and the metrics endpoint on ln until the returned function stops
// them. The function shuts the server down within five seconds and returns what stopped it, or nil
// when the shutdown stopped it.
func Serve(ln net.Listener, registry *prometheus.Registry, logger *slog.Logger) func() error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		// A failed write means the prober went away, which leaves the operator nothing to act on.
		if _, err := fmt.Fprintln(w, "ok"); err != nil {
			logger.DebugContext(r.Context(), "writing a probe answer failed", "error", err)
		}
	})
	mux.Handle("GET /metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second, ErrorLog: logging.ServerErrorLog(logger)}
	served := make(chan error, 1)
	go func() {
		err := srv.Serve(ln)
		if errors.Is(err, http.ErrServerClosed) {
			err = nil
		}
		served <- err
	}()
	logger.Info("serving the probes", "listen", ln.Addr().String())
	return func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return errors.Join(srv.Shutdown(ctx), <-served)
	}
}
