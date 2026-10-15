package app

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/api/internal/service"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/process/host"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// counted returns a registry holding one operation, echo, and the count of calls that reached it.
func counted(t *testing.T) (service.Registry, *atomic.Int64) {
	t.Helper()
	var calls atomic.Int64
	reg, err := service.NewRegistry([]string{"acct-a"}, service.Operation{
		Name:        "echo",
		Description: "Returns its arguments.",
		Effect:      service.Read,
		Path:        "/api/accounts/{account_id}/echo",
		Input:       json.RawMessage(`{"type":"object","properties":{"account_id":{"type":"string"}},"required":["account_id"]}`),
		Output:      json.RawMessage(`{"type":"object"}`),
		Handle: func(_ context.Context, _ string, in json.RawMessage) (json.RawMessage, error) {
			calls.Add(1)
			return in, nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return reg, &calls
}

// mustSurface returns the client surface over reg behind the token in tokenFile.
func mustSurface(t *testing.T, reg service.Registry, tokenFile string) http.Handler {
	t.Helper()
	h, err := surface(reg, tokenFile, discard())
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// tokenFile writes token to a file standing in for the mounted token and returns its path.
func tokenFile(t *testing.T, token string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte(token), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// request is one request to the client surface.
type request struct {
	method, path, authorization, body string
}

// apiCall and mcpCall reach the operation on each root.
var (
	apiCall = request{method: http.MethodGet, path: "/api/accounts/acct-a/echo"}
	mcpCall = request{method: http.MethodPost, path: "/mcp", body: `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","arguments":{"account_id":"acct-a"}}}`}
)

func (r request) with(authorization string) request {
	r.authorization = authorization
	return r
}

// status sends r to h and returns the response's status.
func status(t *testing.T, h http.Handler, r request) int {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), r.method, r.path, strings.NewReader(r.body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	if r.authorization != "" {
		req.Header.Set("Authorization", r.authorization)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

// Every request to the client surface passes the bearer check before it reaches either root, so a
// request without the token never reaches the service layer (ADR-0030). A token file that cannot be
// read or holds no token admits nothing, whatever the request carries.
func TestEveryRequestPassesTheBearerCheckFirst(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent")
	cases := []struct {
		name    string
		token   string
		request request
		want    int
	}{
		{"the API root without a token", tokenFile(t, "s3cret\n"), apiCall, 401},
		{"the MCP root without a token", tokenFile(t, "s3cret\n"), mcpCall, 401},
		{"the API root with a wrong token", tokenFile(t, "s3cret\n"), apiCall.with("Bearer wrong"), 401},
		{"the MCP root with a wrong token", tokenFile(t, "s3cret\n"), mcpCall.with("Bearer wrong"), 401},
		{"the token as a basic credential", tokenFile(t, "s3cret\n"), apiCall.with("Basic s3cret"), 401},
		{"the token without a scheme", tokenFile(t, "s3cret\n"), apiCall.with("s3cret"), 401},
		{"a token with the right one as its prefix", tokenFile(t, "s3cret\n"), apiCall.with("Bearer s3cret2"), 401},
		{"an empty token file", tokenFile(t, "\n"), apiCall.with("Bearer "), 401},
		{"an empty token file and a token", tokenFile(t, ""), mcpCall.with("Bearer s3cret"), 401},
		{"a token file that is absent", missing, apiCall.with("Bearer "), 401},
		{"a path outside both roots reaching the surface's handler, unauthenticated", tokenFile(t, "s3cret\n"), request{method: http.MethodGet, path: "/healthz"}, 401},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reg, calls := counted(t)
			if got := status(t, mustSurface(t, reg, c.token), c.request); got != c.want {
				t.Errorf("status %d, want %d", got, c.want)
			}
			if n := calls.Load(); n != 0 {
				t.Errorf("the operation ran %d times behind a refused bearer check", n)
			}
		})
	}
}

// With the token, both roots reach the operation. The surface serves the two roots and nothing else,
// so the probes and the metrics endpoint are not on it.
func TestTheTokenReachesBothRoots(t *testing.T) {
	reg, calls := counted(t)
	h := mustSurface(t, reg, tokenFile(t, "s3cret\n"))
	got := map[string]int{
		"api":     status(t, h, apiCall.with("Bearer s3cret")),
		"mcp":     status(t, h, mcpCall.with("bearer s3cret")),
		"healthz": status(t, h, request{method: http.MethodGet, path: "/healthz", authorization: "Bearer s3cret"}),
		"metrics": status(t, h, request{method: http.MethodGet, path: "/metrics", authorization: "Bearer s3cret"}),
	}
	want := map[string]int{"api": 200, "mcp": 200, "healthz": 404, "metrics": 404}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("statuses (-want +got):\n%s", diff)
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("the operation ran %d times, want once per root", n)
	}
}

// writeKeyPair writes a new self-signed certificate for 127.0.0.1 and its key to certFile and keyFile,
// and returns the certificate.
func writeKeyPair(t *testing.T, certFile, keyFile string, serial int64) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: "api test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		// Self-signed, so the test's client trusts it as its own authority.
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

// Given a key pair, the API server's start serves the client surface over TLS and logs that it does
// (ADR-0127). A plain HTTP request never reaches a root, a TLS request without the token is refused
// by the bearer check, a TLS request with the token reaches its root, and a key pair rotated in its
// mounted files serves the next handshake.
func TestTheSurfaceIsServedOverTLSWhenGivenAKeyPair(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	first := writeKeyPair(t, certFile, keyFile, 1)
	addr, calls, log := started(t, host.Serving{TLSCert: certFile, TLSKey: keyFile})

	request := func(client *http.Client, scheme, token string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, scheme+"://"+addr+"/api/accounts/acct-a/echo", http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		if token != "" {
			req.Header.Set("Authorization", token)
		}
		return client.Do(req)
	}
	post := func(client *http.Client, scheme string) (*http.Response, error) {
		return request(client, scheme, "Bearer s3cret")
	}
	trusting := func(cert *x509.Certificate) *http.Client {
		pool := x509.NewCertPool()
		pool.AddCert(cert)
		return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}, DisableKeepAlives: true}}
	}

	if resp, err := post(&http.Client{}, "http"); err == nil {
		body, readErr := io.ReadAll(resp.Body)
		if err := errors.Join(readErr, resp.Body.Close()); err != nil {
			t.Error(err)
		}
		if resp.StatusCode == http.StatusOK {
			t.Errorf("a plain HTTP request was served: %s", body)
		}
	}
	if n := calls.Load(); n != 0 {
		t.Fatalf("a plain HTTP request reached the operation %d times", n)
	}

	unauthenticated, err := request(trusting(first), "https", "")
	if err != nil {
		t.Fatalf("a TLS request without the token: %v", err)
	}
	if err := unauthenticated.Body.Close(); err != nil {
		t.Error(err)
	}
	if unauthenticated.StatusCode != http.StatusUnauthorized || calls.Load() != 0 {
		t.Fatalf("a TLS request without the token answered %d, and the operation ran %d times", unauthenticated.StatusCode, calls.Load())
	}

	for i := range 2 {
		cert := first
		if i == 1 {
			cert = writeKeyPair(t, certFile, keyFile, 2)
		}
		resp, err := post(trusting(cert), "https")
		if err != nil {
			t.Fatalf("handshake %d with the key pair then in the files: %v", i+1, err)
		}
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
		if resp.StatusCode != http.StatusOK || resp.TLS == nil || resp.TLS.PeerCertificates[0].SerialNumber.Int64() != cert.SerialNumber.Int64() {
			t.Errorf("handshake %d: status %d with certificate serial %v, want 200 with serial %v", i+1, resp.StatusCode, resp.TLS.PeerCertificates[0].SerialNumber, cert.SerialNumber)
		}
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("the operation ran %d times over TLS, want 2", n)
	}
	if got := servingRecord(t, log); !strings.Contains(got, " tls=true") {
		t.Errorf("the start's serving record does not say it serves TLS: %s", got)
	}
}

// lockedBuffer is a log destination the start and the test read and write from different goroutines.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// started serves the API server's parts over the counted registry, behind the bearer token s3cret, on
// two loopback listeners through the host, as the API server's process serves them, with s, until the
// test ends. It returns once the process reports ready, with the client surface's address, the
// operation's call count and the process's log, and fails the test when the process stops or never
// reports ready.
func started(t *testing.T, s host.Serving) (string, *atomic.Int64, *lockedBuffer) {
	t.Helper()
	surfaceListener, probeListener := loopback(t), loopback(t)
	reg, calls := counted(t)
	log := &lockedBuffer{}
	h, err := host.New(slog.New(slog.NewTextHandler(log, nil)))
	if err != nil {
		t.Fatal(err)
	}
	p, err := parts(&serving{registry: reg, logger: discard()}, Section{TokenFile: tokenFile(t, "s3cret"), AccountReloadInterval: time.Hour}, discard())
	if err != nil {
		t.Fatal(err)
	}
	h.Add("api", Mount, p)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- h.Serve(ctx, host.Listeners{App: surfaceListener, Probes: probeListener}, s)
	}()
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	})
	probe := "http://" + probeListener.Addr().String() + "/readyz"
	client := &http.Client{Timeout: 200 * time.Millisecond}
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		select {
		case err := <-done:
			done <- err
			t.Fatalf("the start stopped before it reported ready: %v", err)
		default:
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, probe, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
		if resp.StatusCode == http.StatusOK {
			return surfaceListener.Addr().String(), calls, log
		}
	}
	t.Fatal("the start never reported ready")
	return "", nil, nil
}

// servingRecord returns the start's serving record from log, waiting for it, since the start writes
// it after it reports ready.
func servingRecord(t *testing.T, log *lockedBuffer) string {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		for line := range strings.SplitSeq(log.String(), "\n") {
			if strings.Contains(line, " msg=serving ") {
				return line
			}
		}
	}
	t.Fatalf("the start wrote no serving record:\n%s", log.String())
	return ""
}

// Given no key pair, or two TLS paths of spaces, the API server's start serves the client surface over
// plain HTTP, behind the bearer check, and logs that it does not serve TLS (ADR-0127,
// ADR-0087).
func TestTheSurfaceGivenNoKeyPairIsPlain(t *testing.T) {
	for name, s := range map[string]host.Serving{
		"neither file": {},
		"both blank":   {TLSCert: " ", TLSKey: "\t"},
	} {
		t.Run(name, func(t *testing.T) {
			addr, calls, log := started(t, s)
			statuses := map[string]int{}
			for name, token := range map[string]string{"with the token": "Bearer s3cret", "without": ""} {
				req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+addr+"/api/accounts/acct-a/echo", http.NoBody)
				if err != nil {
					t.Fatal(err)
				}
				if token != "" {
					req.Header.Set("Authorization", token)
				}
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				if err := resp.Body.Close(); err != nil {
					t.Error(err)
				}
				statuses[name] = resp.StatusCode
			}
			if diff := cmp.Diff(map[string]int{"with the token": 200, "without": 401}, statuses, compare.Options); diff != "" {
				t.Errorf("statuses (-want +got):\n%s", diff)
			}
			if n := calls.Load(); n != 1 {
				t.Errorf("the operation ran %d times, want once", n)
			}
			if got := servingRecord(t, log); !strings.Contains(got, " tls=false") {
				t.Errorf("the start's serving record does not say it serves plain HTTP: %s", got)
			}
		})
	}
}

// In the API server's process a path outside the API root and the MCP root is not found, answered
// by the application server before any bearer check, since the host mounts the client surface at
// those two patterns alone. A path under either still meets the bearer check (ADR-0138, ADR-0030).
func TestAPathOutsideBothRootsIsNotFound(t *testing.T) {
	addr, calls, _ := started(t, host.Serving{})
	statuses := map[string]int{}
	for _, path := range []string{"/healthz", "/readyz", "/metrics", "/", "/apix", "/mcp/x", "/api/accounts/acct-a/echo"} {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+addr+path, http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
		statuses[path] = resp.StatusCode
	}
	want := map[string]int{
		"/healthz": 404, "/readyz": 404, "/metrics": 404, "/": 404, "/apix": 404, "/mcp/x": 404,
		"/api/accounts/acct-a/echo": 401,
	}
	if diff := cmp.Diff(want, statuses, compare.Options); diff != "" {
		t.Errorf("statuses without the token (-want +got):\n%s", diff)
	}
	if n := calls.Load(); n != 0 {
		t.Errorf("the operation ran %d times without the token", n)
	}
}

// The API server's root configuration type and its own section are pinned field by field, so a new
// value is a visible change (ADR-0078). Each other section's own type is pinned in its package.
func TestTheConfigurationTypeIsPinned(t *testing.T) {
	mustnotcompile.RequireFields(t, "github.com/ppat/mediated-mailbox-mcp/api/app", "Configuration",
		"Process host.Process",
		"Operations host.Operations",
		"Serving host.Serving",
		"Section Section",
		"Database core.Config",
		"Credential core.Config",
		"Scanner scan.Config",
	)
	mustnotcompile.RequireFields(t, "github.com/ppat/mediated-mailbox-mcp/api/app", "Section",
		"TokenFile string",
		"AccountReloadInterval time.Duration",
		"ProviderTimeout time.Duration",
	)
}

func discard() *slog.Logger { return slog.New(slog.DiscardHandler) }

// starter returns the starter of a host that serves nothing, for a test that assembles the API server
// without serving it, so the API server's goroutines are the host's as in its process, and the host
// waits for them when the test ends.
func starter(t *testing.T) host.Starter {
	t.Helper()
	h, err := host.New(discard())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)
	return h.Starter("api")
}

// The API server's defaults are the process's, its two listeners and the info level, a reload each
// minute, the port, its own runtime role, the TLS mode that fails closed, the driver's own pool size,
// the larger of 4 and the processor count, the scanner configuration the application ships and a
// provider timeout of thirty seconds. The token file, the database host, name and password file and
// the key files are required.
func TestTheDefaults(t *testing.T) {
	want := Configuration{
		Process:    host.Process{LogLevel: "info"},
		Operations: host.Operations{ProbeListen: ":8080"},
		Serving:    host.Serving{Listen: ":8443"},
		Section:    Section{AccountReloadInterval: time.Minute, ProviderTimeout: 30 * time.Second},
		Database:   dbconnectcore.Config{Port: 5432, User: "mediated_mailbox_api", SSLMode: "verify-full", PoolSize: max(4, runtime.NumCPU())},
		Scanner:    scan.DefaultConfig(),
	}
	if diff := cmp.Diff(want, defaults(), compare.Options); diff != "" {
		t.Errorf("defaults (-want +got):\n%s", diff)
	}
	err := Run(t.Context(), nil, nil, io.Discard, io.Discard, discard(), new(slog.LevelVar))
	wantErr := "loading the configuration: token_file is required, and neither the file, MEDIATED_MAILBOX_TOKEN_FILE nor --token_file sets it"
	if err == nil || err.Error() != wantErr {
		t.Errorf("Run returned %v, want %q", err, wantErr)
	}
}

// valid returns a configuration the API server starts with.
func valid() Configuration {
	c := defaults()
	c.TokenFile = "/run/token"
	c.Database.Host, c.Database.Name, c.Database.PasswordFile = "db", "mailbox", "/run/password"
	c.Credential.PublicKeyFile, c.Credential.PrivateKeyFiles = "/run/keys/public", []string{"/run/keys/private"}
	return c
}

// The configuration the API server cannot serve with is refused before anything is read or connected,
// past the process's sections, which the host refuses (ADR-0138). No account reaches the API server
// from its configuration (ADR-0080).
func TestAConfigurationTheAPIServerCannotServeWithIsRefused(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Configuration)
		want   string
	}{
		{"a valid configuration", func(*Configuration) {}, ""},
		{"a blank token file path", func(c *Configuration) { c.TokenFile = " " }, "token_file is empty"},
		{"a reload interval of zero", func(c *Configuration) { c.AccountReloadInterval = 0 }, "account_reload_interval 0s is not positive"},
		{"a negative reload interval", func(c *Configuration) { c.AccountReloadInterval = -time.Second }, "account_reload_interval -1s is not positive"},
		{"a provider timeout of zero", func(c *Configuration) { c.ProviderTimeout = 0 }, "provider_timeout 0s is not positive and at most 5m0s"},
		{"a provider timeout above five minutes", func(c *Configuration) { c.ProviderTimeout = 5*time.Minute + time.Second }, "provider_timeout 5m1s is not positive and at most 5m0s"},
		{"an empty database host", func(c *Configuration) { c.Database.Host = "" }, "database.host is empty"},
		{"a database pool size of zero", func(c *Configuration) { c.Database.PoolSize = 0 }, "database.pool_size 0 is not positive"},
		{"no private key", func(c *Configuration) { c.Credential.PrivateKeyFiles = nil }, "credential.private_key_files names no file"},
	}
	for _, c := range cases {
		cfg := valid()
		c.change(&cfg)
		got := ""
		if err := validate(cfg); err != nil {
			got = err.Error()
		}
		if got != c.want {
			t.Errorf("%s: validate reports %q, want %q", c.name, got, c.want)
		}
	}
}

// An argument that is not a configuration flag is refused, so no account reaches the API server from its
// command line (ADR-0080).
func TestAnAccountArgumentIsRefused(t *testing.T) {
	err := Run(t.Context(), []string{"acct-a"}, nil, io.Discard, io.Discard, discard(), new(slog.LevelVar))
	if err == nil || !strings.Contains(err.Error(), "loading the configuration") {
		t.Errorf("Run returned %v, want the argument refused", err)
	}
}

// serving names a token file and no key pair, so a start serving plain HTTP reaches the credential and
// database sections.
var servingArgs = []string{
	"--token_file=/absent/token",
}

// absentKeys names key files that do not exist, for a start meant to stop before the keyring or at it.
var absentKeys = []string{"--credential.public_key_file=/absent/public", "--credential.private_key_files=[/absent/private]"}

// database names a database whose password file does not exist, so a start that gets past the keyring
// stops at the password file, before any connection is made.
var database = []string{"--database.host=db.example", "--database.name=mailbox", "--database.password_file=/absent/password"}

// args joins argument lists into one.
func args(lists ...[]string) []string {
	var out []string
	for _, l := range lists {
		out = append(out, l...)
	}
	return out
}

// The start logs every effective value with the layer that set it, the password file's path and
// never its contents.
func TestTheEffectiveConfigurationIsLogged(t *testing.T) {
	passwordFile := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(passwordFile, []byte("the-secret-itself\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
		if a.Key == slog.TimeKey {
			return slog.Attr{}
		}
		return a
	}}))
	// The key files do not exist, so the start stops at the keyring, after the log and before any
	// connection is made.
	err := Run(t.Context(), args(servingArgs, []string{"--database.host=db.example", "--database.password_file=" + passwordFile}, absentKeys),
		[]string{"MEDIATED_MAILBOX_DATABASE__NAME=mailbox"}, io.Discard, io.Discard, logger, new(slog.LevelVar))
	if err == nil || !strings.HasPrefix(err.Error(), "loading the keyring: ") {
		t.Fatalf("Run returned %v, want the keyring's refusal", err)
	}
	want := []string{
		`level=INFO msg=configuration path=account_reload_interval source=default value=1m0s`,
		`level=INFO msg=configuration path=credential.private_key_files source="flag --credential.private_key_files" value=[/absent/private]`,
		`level=INFO msg=configuration path=credential.public_key_file source="flag --credential.public_key_file" value=/absent/public`,
		`level=INFO msg=configuration path=database.host source="flag --database.host" value=db.example`,
		`level=INFO msg=configuration path=database.name source="environment variable MEDIATED_MAILBOX_DATABASE__NAME" value=mailbox`,
		`level=INFO msg=configuration path=database.password_file source="flag --database.password_file" value=` + passwordFile,
		`level=INFO msg=configuration path=database.pool_size source=default value=` + strconv.Itoa(max(4, runtime.NumCPU())),
		`level=INFO msg=configuration path=database.port source=default value=5432`,
		`level=INFO msg=configuration path=database.sslmode source=default value=verify-full`,
		`level=INFO msg=configuration path=database.sslrootcert source=default value=""`,
		`level=INFO msg=configuration path=database.user source=default value=mediated_mailbox_api`,
		`level=INFO msg=configuration path=listen source=default value=:8443`,
		`level=INFO msg=configuration path=log_level source=default value=info`,
		`level=INFO msg=configuration path=probe_listen source=default value=:8080`,
		`level=INFO msg=configuration path=provider_timeout source=default value=30s`,
		`level=INFO msg=configuration path=scanner.dense_entropy source=default value=3`,
		`level=INFO msg=configuration path=scanner.dense_length source=default value=16`,
		`level=INFO msg=configuration path=scanner.link_params source=default value="[token code key auth t otp reset_password_token confirmation_token unlock_token oobcode verification_code confirmation_code ticket signature]"`,
		`level=INFO msg=configuration path=scanner.link_words source=default value="[token confirm verify reset magic auth password login unlock]"`,
		`level=INFO msg=configuration path=scanner.subject_threshold source=default value=0.6`,
		`level=INFO msg=configuration path=scanner.threshold source=default value=0.6`,
		`level=INFO msg=configuration path=scanner.triggers.en source=default value="[code otp verification verify pin passcode password 2fa two-factor two factor 2-factor one-time one time single-use security code login log in log-in sign-in sign in auth authenticate authentication secret access validate validation tan confirmation]"`,
		`level=INFO msg=configuration path=scanner.weights.entropy source=default value=1`,
		`level=INFO msg=configuration path=scanner.weights.length source=default value=1`,
		`level=INFO msg=configuration path=scanner.weights.mix source=default value=1`,
		`level=INFO msg=configuration path=scanner.weights.position source=default value=1`,
		`level=INFO msg=configuration path=scanner.weights.proximity source=default value=1`,
		`level=INFO msg=configuration path=scanner.window source=default value=8`,
		`level=INFO msg=configuration path=tls_cert source=default value=""`,
		`level=INFO msg=configuration path=tls_key source=default value=""`,
		`level=INFO msg=configuration path=token_file source="flag --token_file" value=/absent/token`,
	}
	got := strings.Split(strings.TrimSpace(out.String()), "\n")
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("log (-want +got):\n%s", diff)
	}
	if strings.Contains(out.String(), "the-secret-itself") {
		t.Errorf("the log holds the password:\n%s", out.String())
	}
}

// No configuration value holds an account or a credential, so a flag or an environment variable naming
// one refuses the start rather than being read (ADR-0080).
func TestACredentialInTheConfigurationRefusesTheStart(t *testing.T) {
	for _, c := range []struct {
		name    string
		args    []string
		environ []string
	}{
		{"a flag", []string{"--accounts.personal.refresh_token=a-token"}, nil},
		{"an environment variable", nil, []string{"MEDIATED_MAILBOX_ACCOUNTS__PERSONAL__REFRESH_TOKEN=a-token"}},
		{"a credential flag", []string{"--credential.refresh_token=a-token"}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := Run(t.Context(), args(servingArgs, database, absentKeys, c.args), c.environ, io.Discard, io.Discard, discard(), new(slog.LevelVar))
			if err == nil || !strings.HasPrefix(err.Error(), "loading the configuration: ") {
				t.Errorf("Run returned %v, want the configuration library's refusal", err)
			}
		})
	}
}

// D3's part of VERIFICATIONS' row for a configuration value that would disable the gate, skip
// masking or weaken deny-by-default. No such value exists, so every layer naming one refuses the
// start, and the scanner's section, whose pattern tier the serve-time check runs, refuses a value that
// would leave the check nothing to match (ADR-0051, ADR-0002). The Configuration type, pinned above,
// holds no field a safety disposition could be read from, so the gate and the check run as they do
// without any of these.
func TestNoConfigurationValueWeakensTheGate(t *testing.T) {
	file := filepath.Join(t.TempDir(), "api.yaml")
	if err := os.WriteFile(file, []byte("redaction:\n  enabled: false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	emptied := filepath.Join(t.TempDir(), "scanner.yaml")
	if err := os.WriteFile(emptied, []byte("scanner:\n  triggers: {}\n  link_words: []\n  link_params: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name    string
		args    []string
		environ []string
		want    string
	}{
		{"a flag disabling the gate", []string{"--gate.enabled=false"}, nil, "loading the configuration: "},
		{"a flag skipping masking", []string{"--masking=false"}, nil, "loading the configuration: "},
		{"a flag releasing restricted bodies", []string{"--release_restricted=true"}, nil, "loading the configuration: "},
		{"a flag skipping the serve-time check", []string{"--scanner.serve_time_check=false"}, nil, "loading the configuration: "},
		{"an environment variable allowing by default", nil, []string{"MEDIATED_MAILBOX_DEFAULT_ALLOW=true"}, "loading the configuration: "},
		{"a file switching redaction off", []string{"--config-file=" + file}, nil, "loading the configuration: "},
		{"a scanner section with nothing to match", []string{"--config-file=" + emptied}, nil, "validating the configuration: scanner: "},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := Run(t.Context(), args(servingArgs, database, absentKeys, c.args), c.environ, io.Discard, io.Discard, discard(), new(slog.LevelVar))
			if err == nil || !strings.HasPrefix(err.Error(), c.want) {
				t.Errorf("Run returned %v, want an error starting %q", err, c.want)
			}
		})
	}
}

// keyFiles writes a generated key pair's files into dir and returns their paths.
func keyFiles(t *testing.T, dir, name string) (private, public string) {
	t.Helper()
	key, err := seal.KEM().GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	seed, err := key.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	private = filepath.Join(dir, name+".private")
	public = filepath.Join(dir, name+".public")
	if err := os.WriteFile(private, seed, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(public, key.PublicKey().Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return private, public
}

// D3's part of VERIFICATIONS' row for a public key matching none of the private keys. The start loads
// the public key and every private key before any connection, refuses it when the public key matches
// none of them, and goes on when it matches any one, the second of two during a key replacement
// included (ADR-0088, ADR-0092).
func TestThePublicKeyMustMatchAPrivateKey(t *testing.T) {
	dir := t.TempDir()
	oldPrivate, oldPublic := keyFiles(t, dir, "old")
	newPrivate, newPublic := keyFiles(t, dir, "new")
	_, otherPublic := keyFiles(t, dir, "other")
	start := func(public string, privates ...string) error {
		keys := []string{"--credential.public_key_file=" + public, "--credential.private_key_files=[" + strings.Join(privates, ", ") + "]"}
		return Run(t.Context(), args(servingArgs, database, keys), nil, io.Discard, io.Discard, discard(), new(slog.LevelVar))
	}
	pastTheKeyring := "configuring the database connection of mediated_mailbox_api: reading the password file: "
	for _, c := range []struct {
		name     string
		public   string
		privates []string
		refused  bool
	}{
		{"a public key matching no private key", otherPublic, []string{oldPrivate, newPrivate}, true},
		{"a public key matching the only private key", oldPublic, []string{oldPrivate}, false},
		{"a public key matching the first of two", oldPublic, []string{oldPrivate, newPrivate}, false},
		{"a public key matching the second of two", newPublic, []string{oldPrivate, newPrivate}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := start(c.public, c.privates...)
			switch {
			case err == nil:
				t.Fatal("run succeeded")
			case c.refused && (!strings.HasPrefix(err.Error(), "loading the keyring: ") || !strings.Contains(err.Error(), "matches none of the private keys")):
				t.Errorf("Run returned %v, want the keyring's refusal", err)
			case !c.refused && !strings.HasPrefix(err.Error(), pastTheKeyring):
				t.Errorf("Run returned %v, want it to pass the keyring and stop at the password file", err)
			}
		})
	}
}

// The API server refuses to start while MCPGODEBUG is set, even to an empty value, before it reads
// its configuration or reaches the database, because the MCP SDK reads it to switch its behaviour
// (ADR-0086).
func TestMCPGODEBUGStopsTheStart(t *testing.T) {
	for _, value := range []string{"allowsessionsinstateless=1", ""} {
		err := Run(t.Context(), nil, []string{"MCPGODEBUG=" + value}, io.Discard, io.Discard, discard(), new(slog.LevelVar))
		if err == nil || !strings.Contains(err.Error(), "MCPGODEBUG is set") {
			t.Errorf("with MCPGODEBUG=%q, run returned %v", value, err)
		}
	}
}

// The API server is assembled only when its bearer token file holds a token, so a mount that cannot
// serve a client stops the start, and its readiness check passes only while the file holds one, so a
// token file emptied or removed while the API server runs leaves it not ready (ADR-0051).
func TestReadyOnlyWhileTheBearerTokenLoads(t *testing.T) {
	dir := t.TempDir()
	absent := filepath.Join(dir, "absent")
	for name, file := range map[string]string{"absent": absent, "holding no token": tokenFile(t, "\n")} {
		reg, _ := counted(t)
		if _, err := parts(&serving{registry: reg, logger: discard()}, Section{TokenFile: file, AccountReloadInterval: time.Hour}, discard()); err == nil {
			t.Errorf("a token file %s was assembled", name)
		}
	}
	token := tokenFile(t, "s3cret")
	reg, _ := counted(t)
	p, err := parts(&serving{registry: reg, logger: discard()}, Section{TokenFile: token, AccountReloadInterval: time.Hour}, discard())
	if err != nil {
		t.Fatal(err)
	}
	ready := func() error { return p.Ready[0].Check(t.Context()) }
	got := map[string]bool{"a token": ready() == nil}
	if err := os.WriteFile(token, []byte(" \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got["emptied"] = ready() == nil
	if err := os.Remove(token); err != nil {
		t.Fatal(err)
	}
	got["removed"] = ready() == nil
	if diff := cmp.Diff(map[string]bool{"a token": true, "emptied": false, "removed": false}, got, compare.Options); diff != "" {
		t.Errorf("ready (-want +got):\n%s", diff)
	}
}

// loopback opens a loopback listener on a port the system picks and keeps it open, so a test learns
// the address from the listener it hands the start and no other process can take the port in between.
func loopback(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return ln
}

// Every response of the client surface carries Cache-Control: no-store and no ETag, whichever root
// or refusal answered it, so no response the gate decided outlives a change in its decision
// (ADR-0087).
func TestEveryResponseIsUncacheable(t *testing.T) {
	reg, _ := counted(t)
	h := mustSurface(t, reg, tokenFile(t, "s3cret"))
	requests := map[string]request{
		"an API call":              apiCall.with("Bearer s3cret"),
		"an MCP call":              mcpCall.with("Bearer s3cret"),
		"a refused bearer token":   apiCall,
		"a path nothing serves":    {method: http.MethodGet, path: "/api/accounts/acct-a/nothing", authorization: "Bearer s3cret"},
		"a HEAD on an API route":   {method: http.MethodHead, path: "/api/accounts/acct-a/echo", authorization: "Bearer s3cret"},
		"a method the route lacks": {method: http.MethodPost, path: "/api/accounts/acct-a/echo", authorization: "Bearer s3cret", body: `{}`},
	}
	for name, r := range requests {
		req := httptest.NewRequestWithContext(t.Context(), r.method, r.path, strings.NewReader(r.body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json, text/event-stream")
		if r.authorization != "" {
			req.Header.Set("Authorization", r.authorization)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s answered %d with Cache-Control %q", name, rec.Code, got)
		}
		if got := rec.Header().Values("ETag"); len(got) != 0 {
			t.Errorf("%s answered %d with ETag %q", name, rec.Code, got)
		}
	}
}

// The caching headers are set as the header is written, so a handler setting its own, a cacheable
// Cache-Control or an ETag, cannot keep them.
func TestNoStoreOverridesAHandlersCaching(t *testing.T) {
	cases := map[string]http.HandlerFunc{
		"a handler writing its status": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Cache-Control", "public, max-age=3600")
			w.Header().Set("ETag", `"v1"`)
			w.WriteHeader(http.StatusOK)
		},
		"a handler writing nothing": func(http.ResponseWriter, *http.Request) {},
		"a handler writing only its body": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Cache-Control", "max-age=60")
			w.Header().Set("ETag", `"v2"`)
			if _, err := w.Write([]byte("{}")); err != nil {
				t.Error(err)
			}
		},
	}
	for name, handler := range cases {
		rec := httptest.NewRecorder()
		noStore(discard(), handler).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s: Cache-Control %q", name, got)
		}
		if got := rec.Header().Values("ETag"); len(got) != 0 {
			t.Errorf("%s: ETag %q", name, got)
		}
	}
}
