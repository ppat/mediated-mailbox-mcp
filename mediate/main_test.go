package main

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
	"fmt"
	"io"
	"log/slog"
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

	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/readiness"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
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
	h, err := surface(reg, tokenFile)
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
		{"a path outside both roots, unauthenticated", tokenFile(t, "s3cret\n"), request{method: http.MethodGet, path: "/healthz"}, 401},
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
	srv := &http.Server{Handler: mustSurface(t, reg, tokenFile(t, "s3cret")), TLSConfig: tlsConfig(certFile, keyFile, false), ReadHeaderTimeout: 5 * time.Second}
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
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, scheme+"://"+addr+"/api/accounts/acct-a/echo", http.NoBody)
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
	srv := &http.Server{Handler: mustSurface(t, reg, tokenFile(t, "s3cret")), TLSConfig: tlsConfig("", "", true), ReadHeaderTimeout: 5 * time.Second}
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
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+ln.Addr().String()+"/api/accounts/acct-a/echo", http.NoBody)
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

// The mediator's root configuration type is pinned field by field, so a new value is a visible
// change (ADR-0078). Each section's own type is pinned in its package.
func TestTheConfigurationTypeIsPinned(t *testing.T) {
	mustnotcompile.RequireFields(t, "github.com/ppat/mediated-mailbox-mcp/mediate", "Configuration",
		"Listen string",
		"ProbeListen string",
		"TLSAtIngress bool",
		"TLSCert string",
		"TLSKey string",
		"TokenFile string",
		"AccountReloadInterval time.Duration",
		"Database core.Config",
		"Credential core.Config",
		"Scanner scan.Config",
		"ProviderTimeout time.Duration",
	)
}

func discard() *slog.Logger { return slog.New(slog.DiscardHandler) }

// The mediator's defaults are its two listeners, a reload each minute, the port, its own runtime role,
// the TLS mode that fails closed, the scanner configuration the application ships and a provider
// timeout of thirty seconds. The token file, the database host, name and password file and the key
// files are required.
func TestTheDefaults(t *testing.T) {
	want := Configuration{
		Listen: ":8443", ProbeListen: ":8080", AccountReloadInterval: time.Minute,
		Database: dbconnectcore.Config{Port: 5432, User: "mediated_mailbox_mediate", SSLMode: "verify-full"},
		Scanner:  scan.DefaultConfig(), ProviderTimeout: 30 * time.Second,
	}
	if diff := cmp.Diff(want, defaults(), compare.Options); diff != "" {
		t.Errorf("defaults (-want +got):\n%s", diff)
	}
	err := run(t.Context(), nil, nil, discard())
	wantErr := "loading the configuration: token_file is required, and neither the file, MEDIATED_MAILBOX_TOKEN_FILE nor --token_file sets it"
	if err == nil || err.Error() != wantErr {
		t.Errorf("run returned %v, want %q", err, wantErr)
	}
}

// valid returns a configuration the mediator starts with, TLS served at the listener.
func valid() Configuration {
	c := defaults()
	c.TLSCert, c.TLSKey, c.TokenFile = "/run/tls/crt", "/run/tls/key", "/run/token"
	c.Database.Host, c.Database.Name, c.Database.PasswordFile = "db", "mailbox", "/run/password"
	c.Credential.PublicKeyFile, c.Credential.PrivateKeyFiles = "/run/keys/public", []string{"/run/keys/private"}
	return c
}

// The configuration the mediator cannot serve with is refused before anything is read or connected.
// TLS files are required unless an ingress is declared to terminate TLS, and refused when one is. No
// account reaches the mediator from its configuration (ADR-0080).
func TestAConfigurationTheMediatorCannotServeWithIsRefused(t *testing.T) {
	cases := []struct {
		name   string
		change func(*Configuration)
		want   string
	}{
		{"TLS at the listener", func(*Configuration) {}, ""},
		{"TLS at an ingress", func(c *Configuration) { c.TLSAtIngress, c.TLSCert, c.TLSKey = true, "", "" }, ""},
		{"no key pair", func(c *Configuration) { c.TLSCert, c.TLSKey = "", "" }, "the mediator serves TLS unless tls_at_ingress is set, so it needs tls_cert and tls_key"},
		{"no key", func(c *Configuration) { c.TLSKey = "" }, "the mediator serves TLS unless tls_at_ingress is set, so it needs tls_key"},
		{"a certificate beside an ingress", func(c *Configuration) { c.TLSAtIngress, c.TLSKey = true, "" }, "tls_at_ingress declares that the mediator serves no TLS, so it takes no tls_cert or tls_key"},
		{"a blank token file path", func(c *Configuration) { c.TokenFile = " " }, "token_file is empty"},
		{"a reload interval of zero", func(c *Configuration) { c.AccountReloadInterval = 0 }, "account_reload_interval 0s is not positive"},
		{"a negative reload interval", func(c *Configuration) { c.AccountReloadInterval = -time.Second }, "account_reload_interval -1s is not positive"},
		{"a provider timeout of zero", func(c *Configuration) { c.ProviderTimeout = 0 }, "provider_timeout 0s is not positive and at most 5m0s"},
		{"a provider timeout above five minutes", func(c *Configuration) { c.ProviderTimeout = 5*time.Minute + time.Second }, "provider_timeout 5m1s is not positive and at most 5m0s"},
		{"an empty database host", func(c *Configuration) { c.Database.Host = "" }, "database.host is empty"},
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

// An argument that is not a configuration flag is refused, so no account reaches the mediator from its
// command line (ADR-0080).
func TestAnAccountArgumentIsRefused(t *testing.T) {
	err := run(t.Context(), []string{"acct-a"}, nil, discard())
	if err == nil || !strings.Contains(err.Error(), "loading the configuration") {
		t.Errorf("run returned %v, want the argument refused", err)
	}
}

// serving names a token file and declares TLS at an ingress, so a start reaches the credential and
// database sections.
var servingArgs = []string{
	"--tls_at_ingress=true",
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
	err := run(t.Context(), args(servingArgs, []string{"--database.host=db.example", "--database.password_file=" + passwordFile}, absentKeys),
		[]string{"MEDIATED_MAILBOX_DATABASE__NAME=mailbox"}, logger)
	if err == nil || !strings.HasPrefix(err.Error(), "loading the keyring: ") {
		t.Fatalf("run returned %v, want the keyring's refusal", err)
	}
	want := []string{
		`level=INFO msg=configuration path=account_reload_interval source=default value=1m0s`,
		`level=INFO msg=configuration path=credential.private_key_files source="flag --credential.private_key_files" value=[/absent/private]`,
		`level=INFO msg=configuration path=credential.public_key_file source="flag --credential.public_key_file" value=/absent/public`,
		`level=INFO msg=configuration path=database.host source="flag --database.host" value=db.example`,
		`level=INFO msg=configuration path=database.name source="environment variable MEDIATED_MAILBOX_DATABASE__NAME" value=mailbox`,
		`level=INFO msg=configuration path=database.password_file source="flag --database.password_file" value=` + passwordFile,
		`level=INFO msg=configuration path=database.port source=default value=5432`,
		`level=INFO msg=configuration path=database.sslmode source=default value=verify-full`,
		`level=INFO msg=configuration path=database.sslrootcert source=default value=""`,
		`level=INFO msg=configuration path=database.user source=default value=mediated_mailbox_mediate`,
		`level=INFO msg=configuration path=listen source=default value=:8443`,
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
		`level=INFO msg=configuration path=tls_at_ingress source="flag --tls_at_ingress" value=true`,
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
			err := run(t.Context(), args(servingArgs, database, absentKeys, c.args), c.environ, discard())
			if err == nil || !strings.HasPrefix(err.Error(), "loading the configuration: ") {
				t.Errorf("run returned %v, want the configuration library's refusal", err)
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
	file := filepath.Join(t.TempDir(), "mediate.yaml")
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
			err := run(t.Context(), args(servingArgs, database, absentKeys, c.args), c.environ, discard())
			if err == nil || !strings.HasPrefix(err.Error(), c.want) {
				t.Errorf("run returned %v, want an error starting %q", err, c.want)
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
		return run(t.Context(), args(servingArgs, database, keys), nil, discard())
	}
	pastTheKeyring := "configuring the database connection: reading the password file: "
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
				t.Errorf("run returned %v, want the keyring's refusal", err)
			case !c.refused && !strings.HasPrefix(err.Error(), pastTheKeyring):
				t.Errorf("run returned %v, want it to pass the keyring and stop at the password file", err)
			}
		})
	}
}

// The mediator refuses to start while MCPGODEBUG is set, even to an empty value, before it reads
// its configuration or reaches the database, because the MCP SDK reads it to switch its behaviour
// (ADR-0086).
func TestMCPGODEBUGStopsTheStart(t *testing.T) {
	for _, value := range []string{"allowsessionsinstateless=1", ""} {
		err := run(t.Context(), nil, []string{"MCPGODEBUG=" + value}, discard())
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
		c    Configuration
		want bool
	}{
		{"TLS at the listener, with the key pair and a token", Configuration{TLSCert: certFile, TLSKey: keyFile, TokenFile: token}, true},
		{"TLS at an ingress, with a token", Configuration{TLSAtIngress: true, TokenFile: token}, true},
		{"a certificate that is absent", Configuration{TLSCert: absent, TLSKey: keyFile, TokenFile: token}, false},
		{"a key that does not match", Configuration{TLSCert: certFile, TLSKey: token, TokenFile: token}, false},
		{"a token file that is absent", Configuration{TLSCert: certFile, TLSKey: keyFile, TokenFile: absent}, false},
		{"a token file holding no token", Configuration{TLSAtIngress: true, TokenFile: empty}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var ready readiness.State
			err := markReadyWhenServable(&ready, c.c)
			if ready.Ready() != c.want || (err == nil) != c.want {
				t.Errorf("ready %v with error %v, want ready %v", ready.Ready(), err, c.want)
			}
		})
	}
}

// The mediator refuses to start while PGPASSWORD or PGSSLPASSWORD is set, even to an empty value,
// before it reads its configuration, because pgx reads either on every parse and it would win over the
// mounted password file (ADR-0078).
func TestAPasswordInTheEnvironmentStopsTheStart(t *testing.T) {
	for _, name := range []string{"PGPASSWORD", "PGSSLPASSWORD"} {
		for _, value := range []string{"s3cret", ""} {
			err := run(t.Context(), nil, []string{name + "=" + value}, discard())
			if err == nil || !strings.Contains(err.Error(), "the environment sets "+name) {
				t.Errorf("with %s=%q, run returned %v", name, value, err)
			}
		}
	}
}

// The refusal of a password variable comes before the start uses the configuration it read, so a
// start whose environment sets one logs no value and connects nowhere. The configuration file named
// here does not exist, so a refusal moved anywhere after the read's error is checked reports the
// missing file's error instead of the refusal.
func TestAPasswordVariableIsRefusedBeforeTheConfigurationIsRead(t *testing.T) {
	dir := t.TempDir()
	private, public := keyFiles(t, dir, "key")
	password := filepath.Join(dir, "password")
	if err := os.WriteFile(password, []byte("s3cret"), 0o600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&out, nil))
	full := args(servingArgs, []string{
		"--config-file=" + filepath.Join(dir, "absent.yaml"),
		"--database.host=127.0.0.1", "--database.port=1", "--database.name=mailbox",
		"--database.password_file=" + password, "--database.sslmode=disable",
		"--credential.public_key_file=" + public, "--credential.private_key_files=[" + private + "]",
	})
	err := run(t.Context(), full, []string{"PGPASSWORD=s3cret"}, logger)
	want := "the environment sets PGPASSWORD, and the database password comes only from the mounted password file"
	if err == nil || err.Error() != want {
		t.Errorf("run returned %v, want %q", err, want)
	}
	if out.Len() != 0 {
		t.Errorf("the start logged before refusing the password variable:\n%s", out.String())
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

// A start whose bearer token file holds no token fails, and the readiness probe never answers ready
// while it runs (ADR-0051). The probe is polled for as long as the start runs.
func TestAStartThatCannotServeFailsAndNeverReportsReady(t *testing.T) {
	surfaceListener, probeListener := loopback(t), loopback(t)
	probe := probeListener.Addr().String()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	reg, _ := counted(t)
	c := Configuration{TLSAtIngress: true, TokenFile: tokenFile(t, "\n"), AccountReloadInterval: time.Hour}
	done := make(chan error, 1)
	go func() {
		done <- serve(ctx, c, surfaceListener, probeListener, &serving{registry: reg, logger: discard()}, prometheus.NewRegistry(), discard())
	}()
	client := &http.Client{Timeout: 200 * time.Millisecond}
	sawReady := false
	for running := true; running; {
		select {
		case err := <-done:
			running = false
			if err == nil || !strings.Contains(err.Error(), "holds no token") {
				t.Errorf("serve returned %v, want the blank token refused", err)
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
		noStore(handler).ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil))
		if got := rec.Header().Get("Cache-Control"); got != "no-store" {
			t.Errorf("%s: Cache-Control %q", name, got)
		}
		if got := rec.Header().Values("ETag"); len(got) != 0 {
			t.Errorf("%s: ETag %q", name, got)
		}
	}
}
