// Package probes is the operations endpoints of every process, the health probe, the readiness
// probe and the metrics endpoint, served on a listener of their own (ADR-0051). /healthz answers 200
// while the process runs, /readyz answers 200 only while the readiness it is handed holds, and
// /metrics serves the process's one registry, between units of work as during them. Every answer is
// plain text, for a platform's healthcheck and scraper, which carry no credential and read no page.
package probes

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Handler serves the three endpoints. ready answers whether the process is ready, and when it is not,
// a line saying why, which the readiness probe answers with 503.
func Handler(gatherer prometheus.Gatherer, ready func(*http.Request) (bool, string), logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		answer(w, r, logger, http.StatusOK, "ok")
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if ok, why := ready(r); !ok {
			answer(w, r, logger, http.StatusServiceUnavailable, why)
			return
		}
		answer(w, r, logger, http.StatusOK, "ready")
	})
	mux.Handle("GET /metrics", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))
	return mux
}

// answer writes a plain-text probe answer. A failed write means the prober went away, which leaves
// the operator nothing to act on, so it is detail.
func answer(w http.ResponseWriter, r *http.Request, logger *slog.Logger, status int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	if _, err := fmt.Fprintln(w, text); err != nil {
		logger.DebugContext(r.Context(), "writing a probe answer failed", "error", err)
	}
}
