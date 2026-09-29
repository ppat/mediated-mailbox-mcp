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
	"encoding/pem"
	"errors"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"

	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/api"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// The UI's root configuration type is pinned field by field, so a new value is a visible change
// (ADR-0078). The database section's own type is pinned in its package.
func TestTheConfigurationTypeIsPinned(t *testing.T) {
	mustnotcompile.RequireFields(t, "github.com/ppat/mediated-mailbox-mcp/ui", "Configuration",
		"Database core.Config", "Listen string", "ProbeListen string", "TLSCert string", "TLSKey string",
		"InsecureHTTP bool", "SyncInterval time.Duration", "HeuristicsInterval time.Duration", "StreamInterval time.Duration")
}

func discard() *slog.Logger { return slog.New(slog.DiscardHandler) }

// The UI's defaults are the port, its own runtime role, the TLS mode that fails closed, the mediator's
// listen addresses and the records' intervals, and the database's host, name and password file are
// required (docs/UI.md section 18.1).
func TestTheDefaults(t *testing.T) {
	want := Configuration{
		Database:           dbconnectcore.Config{Port: 5432, User: "mediated_mailbox_ui", SSLMode: "verify-full"},
		Listen:             ":8443",
		ProbeListen:        ":8080",
		SyncInterval:       5 * time.Minute,
		HeuristicsInterval: 24 * time.Hour,
		StreamInterval:     2 * time.Second,
	}
	if diff := cmp.Diff(want, defaults(), compare.Options); diff != "" {
		t.Errorf("defaults (-want +got):\n%s", diff)
	}
	err := run(t.Context(), []string{"--database.name=mailbox", "--database.password_file=/absent"}, nil, discard())
	wantErr := "loading the configuration: database.host is required, and neither the file, MEDIATED_MAILBOX_DATABASE__HOST nor --database.host sets it"
	if err == nil || err.Error() != wantErr {
		t.Errorf("run returned %v, want %q", err, wantErr)
	}
}

// A binary built without the devloop build tag, which every test binary and every image is, refuses
// plain HTTP at start, before it touches the database (docs/UI.md section 18).
func TestPlainHTTPRefusesTheStart(t *testing.T) {
	args := []string{
		"--database.host=db", "--database.name=mailbox", "--database.password_file=/absent",
		"--listen=:8443", "--probe_listen=:8080", "--insecure_http=true",
	}
	err := run(t.Context(), args, []string{"MEDIATED_MAILBOX_STREAM_INTERVAL=1s"}, discard())
	want := "validating the configuration: insecure_http is true, and a binary built without the devloop build tag serves TLS only"
	if err == nil || err.Error() != want {
		t.Errorf("run returned %v, want %q", err, want)
	}
}

// TLS needs both its files unless plain HTTP is configured.
func TestTLSWithoutItsFilesRefusesTheStart(t *testing.T) {
	args := []string{"--database.host=db", "--database.name=mailbox", "--database.password_file=/absent", "--listen=:8443", "--probe_listen=:8080", "--tls_cert=/tls/cert"}
	err := run(t.Context(), args, nil, discard())
	want := "validating the configuration: tls_cert and tls_key are both required unless insecure_http is true"
	if err == nil || err.Error() != want {
		t.Errorf("run returned %v, want %q", err, want)
	}
}

// A password variable refuses the start before any configuration is read.
func TestAPasswordVariableRefusesTheStart(t *testing.T) {
	err := run(t.Context(), nil, []string{"PGPASSWORD="}, discard())
	want := "the environment sets PGPASSWORD, and the database password comes only from the mounted password file"
	if err == nil || err.Error() != want {
		t.Errorf("run returned %v, want %q", err, want)
	}
}

// writeKeyPair writes a self-signed key pair for 127.0.0.1 with the given serial to the two files.
func writeKeyPair(t *testing.T, certFile, keyFile string, serial int64) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "ui test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IsCA: true, BasicConstraintsValid: true,
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

// A key pair rotated in its mounted files serves the next handshake without a restart, as the
// mediator's client surface does (docs/UI.md section 15).
func TestARotatedKeyPairServesTheNextHandshake(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	first := writeKeyPair(t, certFile, keyFile, 1)
	srv := &http.Server{
		Handler:           http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }),
		TLSConfig:         tlsConfig(certFile, keyFile),
		ReadHeaderTimeout: 5 * time.Second,
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	served := make(chan error, 1)
	go func() { served <- srv.ServeTLS(ln, "", "") }()
	t.Cleanup(func() {
		if err := errors.Join(srv.Shutdown(context.Background()), ignoreClosed(<-served)); err != nil {
			t.Error(err)
		}
	})
	for i, cert := range []*x509.Certificate{first, nil} {
		if cert == nil {
			cert = writeKeyPair(t, certFile, keyFile, 2)
		}
		pool := x509.NewCertPool()
		pool.AddCert(cert)
		client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}, DisableKeepAlives: true}}
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://"+ln.Addr().String()+"/", http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("handshake %d with the key pair then in the files: %v", i+1, err)
		}
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
		if got := resp.TLS.PeerCertificates[0].SerialNumber.Int64(); got != cert.SerialNumber.Int64() {
			t.Errorf("handshake %d served serial %d, want %d", i+1, got, cert.SerialNumber.Int64())
		}
	}
}

func ignoreClosed(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// A TLS key pair that does not load refuses the start before the UI serves, so a pod whose mounts
// cannot serve never listens.
func TestAKeyPairThatDoesNotLoadRefusesTheStart(t *testing.T) {
	c := defaults()
	c.Listen, c.ProbeListen = "127.0.0.1:0", "127.0.0.1:0"
	c.TLSCert, c.TLSKey = filepath.Join(t.TempDir(), "absent.crt"), filepath.Join(t.TempDir(), "absent.key")
	s, err := api.New(api.Options{
		Bundle: fstest.MapFS{}, Datasets: registry.Datasets(), Logger: discard(),
		Metrics: prometheus.NewRegistry(), Clock: time.Now, StreamInterval: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	err = serve(bounded(t), c, s, discard())
	if err == nil || !strings.HasPrefix(err.Error(), "loading the TLS key pair: ") {
		t.Fatalf("serve returned %v, want the key pair refused", err)
	}
}

// The start logs every effective value with the layer that set it, the password file's path and
// never its contents. It stops at the TLS key pair, after the log and before it listens.
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
	err := run(bounded(t), []string{
		"--database.host=db.example", "--database.password_file=" + passwordFile, "--tls_cert=/absent/tls.crt", "--tls_key=/absent/tls.key",
		"--listen=127.0.0.1:0", "--probe_listen=127.0.0.1:0",
	},
		[]string{"MEDIATED_MAILBOX_DATABASE__NAME=mailbox"}, logger)
	if err == nil || !strings.HasPrefix(err.Error(), "loading the TLS key pair: ") {
		t.Fatalf("run returned %v, want the key pair refused", err)
	}
	want := []string{
		`level=INFO msg=configuration path=database.host source="flag --database.host" value=db.example`,
		`level=INFO msg=configuration path=database.name source="environment variable MEDIATED_MAILBOX_DATABASE__NAME" value=mailbox`,
		`level=INFO msg=configuration path=database.password_file source="flag --database.password_file" value=` + passwordFile,
		`level=INFO msg=configuration path=database.port source=default value=5432`,
		`level=INFO msg=configuration path=database.sslmode source=default value=verify-full`,
		`level=INFO msg=configuration path=database.sslrootcert source=default value=""`,
		`level=INFO msg=configuration path=database.user source=default value=mediated_mailbox_ui`,
		`level=INFO msg=configuration path=heuristics_interval source=default value=24h0m0s`,
		`level=INFO msg=configuration path=insecure_http source=default value=false`,
		`level=INFO msg=configuration path=listen source="flag --listen" value=127.0.0.1:0`,
		`level=INFO msg=configuration path=probe_listen source="flag --probe_listen" value=127.0.0.1:0`,
		`level=INFO msg=configuration path=stream_interval source=default value=2s`,
		`level=INFO msg=configuration path=sync_interval source=default value=5m0s`,
		`level=INFO msg=configuration path=tls_cert source="flag --tls_cert" value=/absent/tls.crt`,
		`level=INFO msg=configuration path=tls_key source="flag --tls_key" value=/absent/tls.key`,
	}
	got := strings.Split(strings.TrimSpace(out.String()), "\n")
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("log (-want +got):\n%s", diff)
	}
	if strings.Contains(out.String(), "the-secret-itself") {
		t.Errorf("the log holds the password:\n%s", out.String())
	}
}

// bounded is the test's context, ended after five seconds, so a start that gets past the check a test
// expects to stop it fails the test rather than serving until the run times out. The start tests
// listen only on ephemeral loopback ports, so such a start never takes a port another process in the
// pod uses.
func bounded(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}
