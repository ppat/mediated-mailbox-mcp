package main

import (
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
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/readiness"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// counted returns a registry holding one operation, echo, and the count of calls that reached it.
func counted(t *testing.T) (service.Registry, *atomic.Int64) {
	t.Helper()
	var calls atomic.Int64
	reg, err := service.NewRegistry([]string{"acct-a"}, service.Operation{
		Name:        "echo",
		Description: "Returns its arguments.",
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
	apiCall = request{method: http.MethodPost, path: "/api/echo", body: `{"account_id":"acct-a"}`}
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
		{"a path outside both roots, unauthenticated", tokenFile(t, "s3cret\n"), request{method: http.MethodGet, path: "/healthz"}, 401},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			reg, calls := counted(t)
			if got := status(t, surface(reg, c.token), c.request); got != c.want {
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
	h := surface(reg, tokenFile(t, "s3cret\n"))
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
		Subject:      pkix.Name{CommonName: "mediate test"},
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

// The client surface is served over TLS only. A plain HTTP request never reaches a root, a TLS request
// with the token does, and a key pair rotated in its mounted files serves the next handshake.
func TestTheSurfaceIsServedOverTLSOnly(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	first := writeKeyPair(t, certFile, keyFile, 1)
	reg, calls := counted(t)
	srv := &http.Server{Handler: surface(reg, tokenFile(t, "s3cret")), TLSConfig: tlsConfig(certFile, keyFile, false), ReadHeaderTimeout: 5 * time.Second}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	served := make(chan error, 1)
	go func() { served <- serveSurface(srv, ln) }()
	t.Cleanup(func() {
		if err := errors.Join(srv.Shutdown(context.Background()), <-served); err != nil {
			t.Error(err)
		}
	})
	addr := ln.Addr().String()

	post := func(client *http.Client, scheme string) (*http.Response, error) {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, scheme+"://"+addr+"/api/echo", strings.NewReader(`{"account_id":"acct-a"}`))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer s3cret")
		return client.Do(req)
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
}

// The readiness endpoint answers 503 until the mediator is marked ready and after it is marked not
// ready, and 200 between. Health answers 200 and metrics serves the registry.
func TestTheProbes(t *testing.T) {
	metrics := prometheus.NewRegistry()
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{Name: "mediated_mailbox_test_gauge", Help: "A test gauge."})
	metrics.MustRegister(gauge)
	var ready readiness.State
	h := probes(&ready, metrics)
	get := func(path string) string {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
		return fmt.Sprintf("%d %s", rec.Code, strings.TrimSpace(rec.Body.String()))
	}
	got := map[string]string{"before": get("/readyz")}
	ready.MarkReady()
	got["ready"] = get("/readyz")
	ready.MarkNotReady()
	got["shutting down"] = get("/readyz")
	got["health"] = get("/healthz")
	want := map[string]string{"before": "503 not ready", "ready": "200 ready", "shutting down": "503 not ready", "health": "200 ok"}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("probes (-want +got):\n%s", diff)
	}
	if metricsAnswer := get("/metrics"); !strings.Contains(metricsAnswer, "mediated_mailbox_test_gauge 0") {
		t.Errorf("metrics: %q", metricsAnswer)
	}
}

// With an ingress in front declared to terminate TLS, the client surface is served over plain HTTP,
// still behind the bearer check (ADR-0087).
func TestTheSurfaceBehindADeclaredIngressIsPlain(t *testing.T) {
	reg, calls := counted(t)
	srv := &http.Server{Handler: surface(reg, tokenFile(t, "s3cret")), TLSConfig: tlsConfig("", "", true), ReadHeaderTimeout: 5 * time.Second}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	served := make(chan error, 1)
	go func() { served <- serveSurface(srv, ln) }()
	t.Cleanup(func() {
		if err := errors.Join(srv.Shutdown(context.Background()), <-served); err != nil {
			t.Error(err)
		}
	})
	statuses := map[string]int{}
	for name, token := range map[string]string{"with the token": "Bearer s3cret", "without": ""} {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "http://"+ln.Addr().String()+"/api/echo", strings.NewReader(`{"account_id":"acct-a"}`))
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
}

// The settings the mediator cannot run without are required, and it takes no arguments, so no account
// reaches it from its command line (ADR-0080). TLS settings are required unless an ingress is declared
// to terminate TLS, and refused when one is.
func TestTheSettingsNeedWhatTheMediatorCannotRunWithout(t *testing.T) {
	cases := []struct {
		args []string
		want string
	}{
		{[]string{"-tls-cert", "c", "-tls-key", "k", "-token-file", "t"}, ""},
		{[]string{}, "the mediator needs -tls-cert, -tls-key, -token-file"},
		{[]string{"-tls-cert", "c", "-tls-key", "k"}, "the mediator needs -token-file"},
		{[]string{"-tls-cert", "c", "-tls-key", "k", "-token-file", "t", "acct-a"}, `the mediator takes no arguments, and was given ["acct-a"]`},
		{[]string{"-tls-at-ingress", "-token-file", "t"}, ""},
		{[]string{"-tls-at-ingress"}, "the mediator needs -token-file"},
		{[]string{"-tls-at-ingress", "-tls-cert", "c", "-token-file", "t"}, "-tls-at-ingress declares that the mediator serves no TLS, so it takes no -tls-cert or -tls-key"},
	}
	for _, c := range cases {
		got := ""
		if _, err := parse(c.args); err != nil {
			got = err.Error()
		}
		if got != c.want {
			t.Errorf("parse(%q) reports %q, want %q", c.args, got, c.want)
		}
	}
}

// The mediator refuses to start while MCPGODEBUG is set, even to an empty value, before it reads
// its settings or reaches the database, because the MCP SDK reads it to switch its behaviour
// (ADR-0086).
func TestMCPGODEBUGStopsTheStart(t *testing.T) {
	for _, value := range []string{"allowsessionsinstateless=1", ""} {
		t.Setenv("MCPGODEBUG", value)
		err := run(t.Context(), []string{"-tls-at-ingress", "-listen", "127.0.0.1:0", "-probe-listen", "127.0.0.1:0", "-token-file", "t"})
		if err == nil || !strings.Contains(err.Error(), "MCPGODEBUG is set") {
			t.Errorf("with MCPGODEBUG=%q, run returned %v", value, err)
		}
	}
}

// The mediator reports ready only once the TLS key pair, when it serves TLS itself, and the bearer
// token load. A mount that cannot serve a client leaves it not ready and stops the start.
func TestReadyOnlyOnceTheKeysLoad(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	writeKeyPair(t, certFile, keyFile, 1)
	absent := filepath.Join(dir, "absent")
	token, empty := tokenFile(t, "s3cret"), tokenFile(t, "\n")
	cases := []struct {
		name string
		s    settings
		want bool
	}{
		{"TLS at the listener, with the key pair and a token", settings{tlsCert: certFile, tlsKey: keyFile, tokenFile: token}, true},
		{"TLS at an ingress, with a token", settings{tlsAtIngress: true, tokenFile: token}, true},
		{"a certificate that is absent", settings{tlsCert: absent, tlsKey: keyFile, tokenFile: token}, false},
		{"a key that does not match", settings{tlsCert: certFile, tlsKey: token, tokenFile: token}, false},
		{"a token file that is absent", settings{tlsCert: certFile, tlsKey: keyFile, tokenFile: absent}, false},
		{"a token file holding no token", settings{tlsAtIngress: true, tokenFile: empty}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var ready readiness.State
			err := markReadyWhenServable(&ready, c.s)
			if ready.Ready() != c.want || (err == nil) != c.want {
				t.Errorf("ready %v with error %v, want ready %v", ready.Ready(), err, c.want)
			}
		})
	}
}

// The mediator's accounts come from the database (ADR-0080), and the seam they pass through supplies
// none yet. So a call naming any account is refused before an operation runs, on both roots.
func TestTheMediatorServesNoAccountYet(t *testing.T) {
	if got := servedAccounts(); len(got) != 0 {
		t.Fatalf("the mediator serves %q", got)
	}
	var calls atomic.Int64
	reg, err := service.NewRegistry(servedAccounts(), service.Operation{
		Name:        "echo",
		Description: "Returns its arguments.",
		Input:       json.RawMessage(`{"type":"object","properties":{"account_id":{"type":"string"}},"required":["account_id"]}`),
		Output:      json.RawMessage(`{"type":"object"}`),
		Handle: func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			calls.Add(1)
			return json.RawMessage(`{}`), nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	h := surface(reg, tokenFile(t, "s3cret"))
	for _, r := range []request{apiCall.with("Bearer s3cret"), mcpCall.with("Bearer s3cret")} {
		status(t, h, r)
	}
	if n := calls.Load(); n != 0 {
		t.Errorf("an operation ran %d times for an account the mediator does not serve", n)
	}
}

// unsetenv removes names from the environment for the rest of the test and restores them after it.
func unsetenv(t *testing.T, names ...string) {
	t.Helper()
	for _, name := range names {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
}

// The mediator refuses to start while PGPASSWORD or PGSSLPASSWORD is set, even to an empty value,
// because pgx reads either on every parse and it would win over the mounted password file (ADR-0078).
func TestAPasswordInTheEnvironmentStopsTheStart(t *testing.T) {
	for _, name := range []string{"PGPASSWORD", "PGSSLPASSWORD"} {
		for _, value := range []string{"s3cret", ""} {
			unsetenv(t, "MCPGODEBUG", "PGPASSWORD", "PGSSLPASSWORD")
			t.Setenv(name, value)
			err := run(t.Context(), []string{"-tls-at-ingress", "-listen", "127.0.0.1:0", "-probe-listen", "127.0.0.1:0", "-token-file", "t"})
			if err == nil || !strings.Contains(err.Error(), name+" is set") {
				t.Errorf("with %s=%q, run returned %v", name, value, err)
			}
		}
	}
}

// freeAddress returns a loopback address with a port nothing listens on when it returns.
func freeAddress(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return addr
}

// A start whose bearer token file holds no token fails, and the readiness probe never answers ready
// while it runs (ADR-0051). The probe is polled for as long as the start runs.
func TestAStartThatCannotServeFailsAndNeverReportsReady(t *testing.T) {
	unsetenv(t, "MCPGODEBUG", "PGPASSWORD", "PGSSLPASSWORD")
	probe := freeAddress(t)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, []string{"-tls-at-ingress", "-listen", "127.0.0.1:0", "-probe-listen", probe, "-token-file", tokenFile(t, "\n")})
	}()
	client := &http.Client{Timeout: 200 * time.Millisecond}
	sawReady := false
	for running := true; running; {
		select {
		case err := <-done:
			running = false
			if err == nil || !strings.Contains(err.Error(), "holds no token") {
				t.Errorf("run returned %v, want the blank token refused", err)
			}
		default:
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+probe+"/readyz", nil)
			if err != nil {
				t.Fatal(err)
			}
			if resp, err := client.Do(req); err == nil {
				sawReady = sawReady || resp.StatusCode == http.StatusOK
				if err := resp.Body.Close(); err != nil {
					t.Error(err)
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	if sawReady {
		t.Error("the readiness probe answered ready for a start that cannot serve a client")
	}
}
