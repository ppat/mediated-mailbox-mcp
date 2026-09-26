// Command mediate is the mediator, published as mediated-mailbox-mediate.
//
// This file is the composition root. It constructs the object graph by hand in ordinary code, and
// nothing else in this component is package main. Every other package of this deployable sits under
// internal, so the compiler refuses an import of it from any other component.
//
// The mediator listens twice. The client surface, the API root under /api/ and the MCP root at
// /mcp, is served over TLS unless the settings declare that an ingress in front terminates TLS, and
// every request to it passes the bearer check before it reaches either root (ADR-0030, ADR-0087).
// The mediator is ready once its policy has loaded, both listeners are up, and the TLS key pair and
// the bearer token have loaded. The probes and the metrics endpoint are served over plain HTTP on a
// listener of their own, because a platform's healthcheck and scraper carry no bearer token
// (ADR-0051). They are operational endpoints outside the operation registry, and the client
// surface's listener never serves them.
//
// It reads its settings through the standard flag package, and #86 moves it onto the configuration
// library (ADR-0078). The database connection is configured by the standard PostgreSQL client
// environment variables, with the password in the mounted file PGPASSFILE names, and the mediator
// refuses to start while PGPASSWORD or PGSSLPASSWORD is set, so no password arrives by the
// environment (ADR-0078, ADR-0079). The accounts it serves reach the registry and the policy loader
// through one seam, servedAccounts. Accounts live in the database and are read from it, never from
// configuration (ADR-0080). The registry refuses every call naming an account the seam does not
// return. The TLS key pair and the bearer token are mounted files, read again on each handshake and
// each request, so a rotated file takes effect without a restart (ADR-0079, ADR-0080). The mediator
// also refuses to start while MCPGODEBUG is set, because the MCP SDK reads it to switch its
// behaviour (ADR-0086).
package main

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/api"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/mcp"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/readiness"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
	"github.com/ppat/mediated-mailbox-mcp/policyload"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:])
	stop()
	if err != nil {
		slog.Error("the mediator stopped", "error", err)
		os.Exit(1)
	}
}

// settings are the mediator's command-line settings.
type settings struct {
	listen       string
	probeListen  string
	tlsAtIngress bool
	tlsCert      string
	tlsKey       string
	tokenFile    string
}

// parse reads the settings from args.
func parse(args []string) (settings, error) {
	var s settings
	fs := flag.NewFlagSet("mediate", flag.ContinueOnError)
	fs.StringVar(&s.listen, "listen", ":8443", "the address the client surface listens on, over TLS")
	fs.StringVar(&s.probeListen, "probe-listen", ":8080", "the address the probes and the metrics endpoint listen on")
	fs.BoolVar(&s.tlsAtIngress, "tls-at-ingress", false, "an ingress in front of the mediator terminates TLS, so the client surface listens over plain HTTP")
	fs.StringVar(&s.tlsCert, "tls-cert", "", "the mounted file holding the client surface's TLS certificate chain")
	fs.StringVar(&s.tlsKey, "tls-key", "", "the mounted file holding the client surface's TLS private key")
	fs.StringVar(&s.tokenFile, "token-file", "", "the mounted file holding the bearer token clients present")
	if err := fs.Parse(args); err != nil {
		return settings{}, err
	}
	if fs.NArg() > 0 {
		return settings{}, fmt.Errorf("the mediator takes no arguments, and was given %q", fs.Args())
	}
	required := []struct{ name, value string }{{"token-file", s.tokenFile}}
	if s.tlsAtIngress {
		if s.tlsCert != "" || s.tlsKey != "" {
			return settings{}, errors.New("-tls-at-ingress declares that the mediator serves no TLS, so it takes no -tls-cert or -tls-key")
		}
	} else {
		required = append([]struct{ name, value string }{{"tls-cert", s.tlsCert}, {"tls-key", s.tlsKey}}, required...)
	}
	var missing []string
	for _, f := range required {
		if f.value == "" {
			missing = append(missing, "-"+f.name)
		}
	}
	if len(missing) > 0 {
		return settings{}, fmt.Errorf("the mediator needs %s", strings.Join(missing, ", "))
	}
	return s, nil
}

// run builds the mediator's object graph, loads its policy and serves until ctx ends.
func run(ctx context.Context, args []string) error {
	if err := refuseEnvironment(); err != nil {
		return err
	}
	s, err := parse(args)
	if err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, "")
	if err != nil {
		return fmt.Errorf("configuring the database connection: %w", err)
	}
	defer pool.Close()
	metrics := prometheus.NewRegistry()
	accounts := servedAccounts()
	if err := loadPolicy(ctx, pool, accounts, metrics); err != nil {
		return err
	}
	registry, err := service.NewRegistry(accounts, service.Operations()...)
	if err != nil {
		return err
	}
	var ready readiness.State

	surfaceListener, err := net.Listen("tcp", s.listen)
	if err != nil {
		return err
	}
	probeListener, err := net.Listen("tcp", s.probeListen)
	if err != nil {
		return errors.Join(err, surfaceListener.Close())
	}
	surfaceHandler, err := surface(registry, s.tokenFile)
	if err != nil {
		return errors.Join(err, surfaceListener.Close(), probeListener.Close())
	}
	surfaceServer := &http.Server{
		Handler:           surfaceHandler,
		TLSConfig:         tlsConfig(s.tlsCert, s.tlsKey, s.tlsAtIngress),
		ReadHeaderTimeout: 10 * time.Second,
	}
	probeServer := &http.Server{
		Handler:           probes(&ready, metrics),
		ReadHeaderTimeout: 10 * time.Second,
	}
	// One slot for each server and one for the readiness check, so no send blocks.
	errs := make(chan error, 3)
	go func() { errs <- serveSurface(surfaceServer, surfaceListener) }()
	go func() { errs <- serveProbes(probeServer, probeListener) }()
	if err := markReadyWhenServable(&ready, s); err != nil {
		errs <- err
	}
	slog.Info("serving", "surface", s.listen, "probes", s.probeListen)

	select {
	case err = <-errs:
	case <-ctx.Done():
	}
	ready.MarkNotReady()
	shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return errors.Join(err, surfaceServer.Shutdown(shutdown), probeServer.Shutdown(shutdown))
}

// servedAccounts is the seam the accounts the mediator serves pass through, to the registry and to
// the policy loader. It returns none, so the registry refuses every call naming an account.
func servedAccounts() []string { return nil }

// loadPolicy loads the policy of accounts through the shared policy loader, which reads each
// account's policy in a transaction of that account. With no account there is no policy to load.
func loadPolicy(ctx context.Context, pool *pgxpool.Pool, accounts []string, metrics *prometheus.Registry) error {
	if len(accounts) == 0 {
		slog.Info("no account is served, so no policy is loaded")
		return nil
	}
	policies, err := policyload.New(pool, accounts, metrics)
	if err != nil {
		return err
	}
	if err := policies.Reload(ctx); err != nil {
		return fmt.Errorf("loading the policy: %w", err)
	}
	slog.Info("policy loaded", "accounts", len(accounts))
	return nil
}

// refuseEnvironment returns an error while MCPGODEBUG, PGPASSWORD or PGSSLPASSWORD is set, even to
// an empty value. The MCP SDK reads MCPGODEBUG to switch behaviour this root relies on, sessions in
// a stateless server among them. pgx reads the two passwords on every parse, and either would win
// over the mounted password file.
func refuseEnvironment() error {
	if _, set := os.LookupEnv("MCPGODEBUG"); set {
		return errors.New("MCPGODEBUG is set, and the mediator runs the MCP SDK only as it ships (ADR-0086)")
	}
	for _, name := range []string{"PGPASSWORD", "PGSSLPASSWORD"} {
		if _, set := os.LookupEnv(name); set {
			return fmt.Errorf("%s is set, and the database password arrives only as a mounted file (ADR-0078)", name)
		}
	}
	return nil
}

// markReadyWhenServable marks the mediator ready once the TLS key pair, unless an ingress in front
// terminates TLS, and the bearer token load, so a pod whose mounts cannot serve a client never
// reports ready. They are read again on each handshake and each request.
func markReadyWhenServable(ready *readiness.State, s settings) error {
	if !s.tlsAtIngress {
		if _, err := tls.LoadX509KeyPair(s.tlsCert, s.tlsKey); err != nil {
			return fmt.Errorf("loading the TLS key pair: %w", err)
		}
	}
	token, err := os.ReadFile(s.tokenFile)
	if err != nil {
		return fmt.Errorf("reading the bearer token: %w", err)
	}
	if strings.TrimSpace(string(token)) == "" {
		return errors.New("the bearer token file holds no token")
	}
	ready.MarkReady()
	return nil
}

// serveSurface serves the client surface on ln until it is shut down, over TLS unless srv has no TLS
// configuration, which only a declared ingress in front leaves it without (ADR-0087).
func serveSurface(srv *http.Server, ln net.Listener) error {
	if srv.TLSConfig == nil {
		return stopped(srv.Serve(ln))
	}
	return stopped(srv.ServeTLS(ln, "", ""))
}

// serveProbes serves the probes and the metrics endpoint on ln until it is shut down.
func serveProbes(srv *http.Server, ln net.Listener) error {
	return stopped(srv.Serve(ln))
}

// stopped returns nil for a server that stopped because it was shut down.
func stopped(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// surface returns the client surface, the API root and the MCP root generated from registry, behind
// the bearer check. Anything else the surface is asked for is not found, after the bearer check.
// Every response carries Cache-Control: no-store and no ETag, so no response a gate decided outlives
// a change in its decision (ADR-0087).
func surface(registry service.Registry, tokenFile string) (http.Handler, error) {
	apiRoot, err := api.Handler(registry)
	if err != nil {
		return nil, err
	}
	roots := http.NewServeMux()
	roots.Handle("/api/", apiRoot)
	roots.Handle("/mcp", mcp.Handler(registry, version()))
	return noStore(bearer(tokenFile, roots)), nil
}

// noStore marks every response uncacheable, before its handler runs, so a response its handler
// never writes carries the mark too, and again as its header is written, removing any ETag, so a
// handler's own caching headers do not survive.
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(&noStoreWriter{ResponseWriter: w}, r)
	})
}

// noStoreWriter sets the caching headers when the response's header is written, so a handler
// setting its own cannot override them.
type noStoreWriter struct {
	http.ResponseWriter
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

// Flush passes a flush through, for the MCP transport's streamed responses.
func (w *noStoreWriter) Flush() {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	if err := http.NewResponseController(w.ResponseWriter).Flush(); err != nil {
		slog.Warn("flushing a response failed", "error", err)
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (w *noStoreWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// bearer admits a request to next only when it carries the bearer token held in tokenFile. The file
// is read on each request. A file that cannot be read or holds no token admits nothing.
func bearer(tokenFile string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		want, err := os.ReadFile(tokenFile) //nolint:gosec // the operator names the mounted file the token is read from
		if err != nil {
			slog.ErrorContext(r.Context(), "reading the bearer token failed", "error", err)
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

// tlsConfig returns the client surface's TLS configuration, which reads the key pair from its
// mounted files on each handshake, or none when an ingress in front terminates TLS.
func tlsConfig(certFile, keyFile string, atIngress bool) *tls.Config {
	if atIngress {
		return nil
	}
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			pair, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				return nil, fmt.Errorf("loading the TLS key pair: %w", err)
			}
			return &pair, nil
		},
	}
}

// probes returns the operational endpoints. /healthz answers 200 while the process serves, /readyz
// answers 200 only while ready reports ready and 503 otherwise, and /metrics serves the process's
// registry.
func probes(ready *readiness.State, metrics *prometheus.Registry) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		answer(w, r, http.StatusOK, "ok")
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if !ready.Ready() {
			answer(w, r, http.StatusServiceUnavailable, "not ready")
			return
		}
		answer(w, r, http.StatusOK, "ready")
	})
	mux.Handle("GET /metrics", promhttp.HandlerFor(metrics, promhttp.HandlerOpts{}))
	return mux
}

// answer writes a plain-text probe answer.
func answer(w http.ResponseWriter, r *http.Request, status int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	if _, err := fmt.Fprintln(w, text); err != nil {
		slog.WarnContext(r.Context(), "writing a probe answer failed", "error", err)
	}
}

// version returns the mediator's module version, as the build recorded it.
func version() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "unknown"
}
