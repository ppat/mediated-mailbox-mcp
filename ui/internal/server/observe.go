package server

import (
	"context"
	"crypto/rand"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

// RequestLog is what one request's log line reports. The handlers fill the route, the account and
// the error, and the error contract fills the origin and the code.
type RequestLog struct {
	// Route is the route pattern that answered the request, or the name of what answered it.
	Route string
	// Account is the account the request names.
	Account string
	// Err is what failed, which the line names and no answer does.
	Err error

	id     string
	origin string
	code   string
}

type requestLogKey struct{}

// RequestLogOf returns the request's log line. Every request passes through observe first, so it is
// always present. A request reaching a handler another way gets a fresh one that is never logged.
func RequestLogOf(ctx context.Context) *RequestLog {
	if l, ok := ctx.Value(requestLogKey{}).(*RequestLog); ok {
		return l
	}
	return &RequestLog{}
}

// DatabaseFailed records err as the database's failure of a request whose answer has already begun, so
// the line is logged at error, as an event stream's poll failing after its 200 is.
func (l *RequestLog) DatabaseFailed(err error) {
	l.Err, l.origin = err, originDatabase
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
		i := &RequestLog{id: newRequestID()}
		w.Header().Set("X-Request-Id", i.id)
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r.WithContext(context.WithValue(r.Context(), requestLogKey{}, i)))
		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		outcome := i.origin
		if outcome == "" {
			outcome = "ok"
		}
		attrs := []any{
			"request_id", i.id, "route", i.Route, "account", i.Account, "status", status,
			"outcome", outcome, "code", i.code, "duration_ms", strconv.FormatInt(time.Since(start).Milliseconds(), 10),
		}
		if i.Err != nil {
			attrs = append(attrs, "error", i.Err.Error())
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
