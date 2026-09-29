package api

import (
	"context"
	"crypto/rand"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// The UI's own metrics (docs/UI.md section 18.2). No label ever carries message-derived text. The
// dataset and level labels take only values the registry declares, because the dataset endpoint
// refuses every other before it reads.
const (
	readDurationName = "mediated_mailbox_ui_read_duration_seconds"
	subscribersName  = "mediated_mailbox_ui_stream_subscribers"
)

type metrics struct {
	readDuration *prometheus.HistogramVec
	subscribers  prometheus.Gauge
}

func newMetrics(reg prometheus.Registerer) (*metrics, error) {
	m := &metrics{
		readDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    readDurationName,
			Help:    "Seconds the dataset endpoint took to answer a read, by dataset and level.",
			Buckets: prometheus.DefBuckets,
		}, []string{"dataset", "level"}),
		subscribers: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: subscribersName,
			Help: "Event streams open now.",
		}),
	}
	for _, c := range []prometheus.Collector{m.readDuration, m.subscribers} {
		if err := reg.Register(c); err != nil {
			return nil, err
		}
	}
	return m, nil
}

// info is what one request's log line reports. Handlers fill the account, the route and the outcome.
type info struct {
	id      string
	route   string
	account string
	origin  string
	code    string
	err     error
}

type infoKey struct{}

// requestInfo returns the request's info. Every request passes through observe first, so it is
// always present. A request reaching a handler another way gets a fresh one that is never logged.
func requestInfo(ctx context.Context) *info {
	if i, ok := ctx.Value(infoKey{}).(*info); ok {
		return i
	}
	return &info{}
}

// statusRecorder keeps the status a handler wrote, for the log line, and passes flushes through for
// the event stream.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(status int) {
	if s.status == 0 {
		s.status = status
	}
	s.ResponseWriter.WriteHeader(status)
}

func (s *statusRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	return s.ResponseWriter.Write(b)
}

func (s *statusRecorder) Flush() {
	if f, ok := s.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// observe gives every request an id, returned in X-Request-Id and in any error body, and logs one line
// when it ends with its id, route, account, status and the error contract's origin
// (docs/UI.md section 18.2). The path is never logged, because a filter value in it can be
// message-derived.
func observe(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		i := &info{id: newRequestID()}
		w.Header().Set("X-Request-Id", i.id)
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r.WithContext(context.WithValue(r.Context(), infoKey{}, i)))
		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		outcome := i.origin
		if outcome == "" {
			outcome = "ok"
		}
		attrs := []any{
			"request_id", i.id, "route", i.route, "account", i.account, "status", status,
			"outcome", outcome, "code", i.code, "duration_ms", strconv.FormatInt(time.Since(start).Milliseconds(), 10),
		}
		if i.err != nil {
			attrs = append(attrs, "error", i.err.Error())
		}
		level := slog.LevelInfo
		// An event stream answers 200 before its first poll, so a poll that fails later is known by its
		// origin rather than by the status.
		if status >= http.StatusInternalServerError || i.origin == originUI || i.origin == originDatabase {
			level = slog.LevelError
		}
		logger.Log(r.Context(), level, "request", attrs...)
	})
}

// newRequestID is a random identifier of 130 bits.
func newRequestID() string {
	return rand.Text()
}
