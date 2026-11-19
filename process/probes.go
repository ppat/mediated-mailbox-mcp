package process

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// probesHandler serves the operations endpoints of every process that runs until it is told to stop,
// the health probe, the readiness probe and the metrics endpoint, on a listener of their own
// (ADR-0051). /healthz answers 200 only while live holds, /readyz only while ready holds, and when
// either does not, 503 with the line it gives saying why, and /metrics serves the process's one
// registry, between units of work as during them. live and ready read what the process recorded and
// run nothing (ADR-0142). Every answer is plain text, for a platform's healthcheck and scraper, which
// carry no credential and read no page.
func probesHandler(gatherer prometheus.Gatherer, live, ready func(*http.Request) (bool, string), logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		if ok, why := live(r); !ok {
			answerProbe(w, r, logger, http.StatusServiceUnavailable, why)
			return
		}
		answerProbe(w, r, logger, http.StatusOK, "ok")
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if ok, why := ready(r); !ok {
			answerProbe(w, r, logger, http.StatusServiceUnavailable, why)
			return
		}
		answerProbe(w, r, logger, http.StatusOK, "ready")
	})
	mux.Handle("GET /metrics", promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}))
	return mux
}

// answerProbe writes a plain-text probe answer. A failed write means the prober went away, which leaves
// the operator nothing to act on, so it is detail.
func answerProbe(w http.ResponseWriter, r *http.Request, logger *slog.Logger, status int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	if _, err := fmt.Fprintln(w, text); err != nil {
		logger.DebugContext(r.Context(), "writing a probe answer failed", "error", err)
	}
}
