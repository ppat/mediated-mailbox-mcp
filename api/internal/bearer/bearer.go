// Package bearer is the API server's bearer-token check and its no-store header, which wrap the
// handler of each of its endpoints, and the readiness report on its token file (ADR-0087, ADR-0127,
// ADR-0142). The token is a mounted file read again on each request, so a rotated file takes effect
// without a restart (ADR-0079, ADR-0080). It is the one package of the API server apart from its two
// roots that imports net/http, as a whole package, and it registers no operation.
package bearer

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
)

// readInterval is how often the readiness report reads the token file again.
const readInterval = 5 * time.Second

// The reasons the report gives while the API server is not ready. They carry no error's detail, which
// the report's loop logs at debug.
const (
	notReadYet = "the bearer token file has not been read yet"
	unreadable = "the bearer token file cannot be read"
	noToken    = "the bearer token file holds no token"
)

// held is the slot the process keeps the one readiness report in, so the two endpoints that ask
// share one loop.
var held = process.NewOnePerProcess[*report]("bearer token")

// report is the readiness report on one token file.
type report struct {
	tokenFile string
}

// Wrap returns next behind the bearer check and the no-store header, for the endpoint p was handed to.
// The first endpoint that asks reads c's token file, so a file that cannot be read or holds no token
// fails the build and with it the start, and starts the readiness report, which reads the file again
// every five seconds on a goroutine the process starts and waits for, and reports the API server not
// ready while it cannot be read or holds no token. Every later endpoint shares that report. The check
// and the endpoint's failures to read the file log through p's logger.
func Wrap(ctx context.Context, p process.Capabilities, c config.API, next http.Handler) (http.Handler, error) {
	if _, err := held.Get(ctx, p, func(_ context.Context, shared process.Capabilities) (*report, func(), error) {
		if err := readable(c.TokenFile); err != nil {
			return nil, nil, err
		}
		r := &report{tokenFile: c.TokenFile}
		shared.Report(process.NotReady, notReadYet)
		shared.Go("bearer token read", func(ctx context.Context) { r.run(ctx, shared) })
		return r, nil, nil
	}); err != nil {
		return nil, err
	}
	return noStore(p.Logger(), check(c.TokenFile, p.Logger(), next)), nil
}

// run reads the token file at once and then every readInterval until ctx ends, and reports each
// read. A read has no deadline of its own, since cancelling a file read would take a goroutine of its
// own, so a read that never returns holds both the next report and the process's stop, which waits
// for this loop. A mounted Secret's file is in memory and never blocks.
func (r *report) run(ctx context.Context, reporter process.Capabilities) {
	ticker := time.NewTicker(readInterval)
	defer ticker.Stop()
	for {
		err := readable(r.tokenFile)
		switch {
		case ctx.Err() != nil:
			return
		case errors.Is(err, errNoToken):
			reporter.Report(process.NotReady, noToken)
		case err != nil:
			reporter.Logger().DebugContext(ctx, "the bearer token file could not be read", "error", err)
			reporter.Report(process.NotReady, unreadable)
		default:
			reporter.Report(process.Ready, "")
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// errNoToken is a token file that holds no token, which admits no client.
var errNoToken = errors.New("the bearer token file holds no token")

// readable refuses a token file that cannot be read or holds no token.
func readable(tokenFile string) error {
	token, err := os.ReadFile(tokenFile) //nolint:gosec // the operator names the mounted file the token is read from
	if err != nil {
		return fmt.Errorf("reading the bearer token: %w", err)
	}
	if strings.TrimSpace(string(token)) == "" {
		return errNoToken
	}
	return nil
}

// noStore marks every response uncacheable, before its handler runs, so a response its handler
// never writes carries the mark too, and again as its header is written, removing any ETag, so a
// handler's own caching headers do not survive (ADR-0087).
func noStore(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(&noStoreWriter{ResponseWriter: w, logger: logger}, r)
	})
}

// noStoreWriter sets the caching headers when the response's header is written, so a handler
// setting its own cannot override them.
type noStoreWriter struct {
	http.ResponseWriter
	logger      *slog.Logger
	wroteHeader bool
}

func (w *noStoreWriter) WriteHeader(status int) {
	if !w.wroteHeader {
		w.wroteHeader = true
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Del("ETag")
	}
	w.ResponseWriter.WriteHeader(status)
}

func (w *noStoreWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// Flush passes a flush through, for the MCP transport's streamed responses. A failed flush means the
// client went away, which leaves the operator nothing to act on, so it is detail.
func (w *noStoreWriter) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if err := http.NewResponseController(w.ResponseWriter).Flush(); err != nil {
		w.logger.Debug("flushing a response failed", "error", err)
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *noStoreWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// check admits a request to next only when it carries the bearer token held in tokenFile. The file
// is read on each request. A file that cannot be read or holds no token admits nothing.
func check(tokenFile string, logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want, err := os.ReadFile(tokenFile) //nolint:gosec // the operator names the mounted file the token is read from
		if err != nil {
			logger.ErrorContext(r.Context(), "reading the bearer token failed", "error", err)
		}
		want = []byte(strings.TrimSpace(string(want)))
		scheme, got, _ := strings.Cut(r.Header.Get("Authorization"), " ")
		if len(want) == 0 || !strings.EqualFold(scheme, "Bearer") || subtle.ConstantTimeCompare([]byte(got), want) != 1 {
			w.Header().Set("WWW-Authenticate", "Bearer")
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
