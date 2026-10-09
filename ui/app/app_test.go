package app

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
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

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/api"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// bundle is the browser bundle as ui/main.go embeds it, under browser/dist of the UI's directory.
var bundle = os.DirFS("..")

// lookups are the sender classifier's domain functions, as the composition root passes them.
var lookups = classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne}

// The UI's root configuration type is pinned field by field, so a new value is a visible change
// (ADR-0078). The database section's own type is pinned in its package.
func TestTheConfigurationTypeIsPinned(t *testing.T) {
	mustnotcompile.RequireFields(t, "github.com/ppat/mediated-mailbox-mcp/ui/app", "Configuration",
		"Database core.Config", "Listen string", "ProbeListen string", "TLSCert string", "TLSKey string",
		"InsecureHTTP bool", "SyncInterval time.Duration", "HeuristicsInterval time.Duration", "StreamInterval time.Duration",
		"DefaultTheme string", "StreamReconnectMax time.Duration", "StreamPollInterval time.Duration",
		"AttentionBacklogShare float64", "AttentionMaskCount int64", "AttentionServeFactor float64", "AttentionGapDays int64",
		"SealPublicKeyFile string", "PrivateKeyFiles []string", "TokenKeyFile string",
		"ConsentRedirect string", "IdentityHeader string", "OperatorName string", "LogLevel string")
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
		DefaultTheme:       "system",
		StreamReconnectMax: 30 * time.Second,
		StreamPollInterval: 5 * time.Second,
		// The worth-a-look thresholds are docs/UI.md section 8.1's starting values.
		AttentionBacklogShare: 5,
		AttentionMaskCount:    20,
		AttentionServeFactor:  2,
		AttentionGapDays:      7,
		ConsentRedirect:       "http://127.0.0.1:47823/",
		OperatorName:          "operator",
		LogLevel:              "info",
	}
	if diff := cmp.Diff(want, defaults(), compare.Options); diff != "" {
		t.Errorf("defaults (-want +got):\n%s", diff)
	}
	err := Run(t.Context(), []string{"--database.name=mailbox", "--database.password_file=/absent"}, nil, discard(), new(slog.LevelVar), bundle)
	wantErr := "loading the configuration: database.host is required, and neither the file, MEDIATED_MAILBOX_DATABASE__HOST nor --database.host sets it"
	if err == nil || err.Error() != wantErr {
		t.Errorf("Run returned %v, want %q", err, wantErr)
	}
}

// A binary built without the devloop build tag, which every test binary and every image is, refuses
// plain HTTP at start, before it touches the database (docs/UI.md section 18).
func TestPlainHTTPRefusesTheStart(t *testing.T) {
	args := []string{
		"--database.host=db", "--database.name=mailbox", "--database.password_file=/absent",
		"--listen=:8443", "--probe_listen=:8080", "--insecure_http=true", "--seal_public_key_file=/absent",
		"--private_key_files=[/absent]",
	}
	err := Run(t.Context(), args, []string{"MEDIATED_MAILBOX_STREAM_INTERVAL=1s"}, discard(), new(slog.LevelVar), bundle)
	want := "validating the configuration: insecure_http is true, and a binary built without the devloop build tag serves TLS only"
	if err == nil || err.Error() != want {
		t.Errorf("Run returned %v, want %q", err, want)
	}
}

// TLS needs both its files unless plain HTTP is configured.
func TestTLSWithoutItsFilesRefusesTheStart(t *testing.T) {
	args := []string{
		"--database.host=db", "--database.name=mailbox", "--database.password_file=/absent", "--listen=:8443", "--probe_listen=:8080",
		"--tls_cert=/tls/cert", "--seal_public_key_file=/absent", "--private_key_files=[/absent]",
	}
	err := Run(t.Context(), args, nil, discard(), new(slog.LevelVar), bundle)
	want := "validating the configuration: tls_cert and tls_key are both required unless insecure_http is true"
	if err == nil || err.Error() != want {
		t.Errorf("Run returned %v, want %q", err, want)
	}
}

// A password variable refuses the start before any configuration is read.
func TestAPasswordVariableRefusesTheStart(t *testing.T) {
	err := Run(t.Context(), nil, []string{"PGPASSWORD="}, discard(), new(slog.LevelVar), bundle)
	want := "the environment sets PGPASSWORD, and the database password comes only from the mounted password file"
	if err == nil || err.Error() != want {
		t.Errorf("Run returned %v, want %q", err, want)
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
		Bundle: fstest.MapFS{}, Datasets: registry.Datasets(lookups), Logger: discard(),
		Metrics: prometheus.NewRegistry(), Clock: time.Now, StreamInterval: time.Second, TokenKey: make([]byte, api.MinTokenKey),
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
// never its contents, and the identifier of the key it seals to (ADR-0092, VERIFICATIONS, the row for
// the UI's log of its key). It stops at the TLS key pair, after the log and before it listens.
func TestTheEffectiveConfigurationIsLogged(t *testing.T) {
	passwordFile := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(passwordFile, []byte("the-secret-itself\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	publicKeyFile, privateKeyFile, keyID := writeKeys(t)
	var out bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
		if a.Key == slog.TimeKey {
			return slog.Attr{}
		}
		return a
	}}))
	err := Run(bounded(t), []string{
		"--database.host=db.example", "--database.password_file=" + passwordFile, "--tls_cert=/absent/tls.crt", "--tls_key=/absent/tls.key",
		"--listen=127.0.0.1:0", "--probe_listen=127.0.0.1:0", "--seal_public_key_file=" + publicKeyFile,
		"--private_key_files=[" + privateKeyFile + "]",
	},
		[]string{"MEDIATED_MAILBOX_DATABASE__NAME=mailbox"}, logger, new(slog.LevelVar), bundle)
	if err == nil || !strings.HasPrefix(err.Error(), "loading the TLS key pair: ") {
		t.Fatalf("Run returned %v, want the key pair refused", err)
	}
	want := []string{
		`level=INFO msg=configuration path=attention_backlog_share source=default value=5`,
		`level=INFO msg=configuration path=attention_gap_days source=default value=7`,
		`level=INFO msg=configuration path=attention_mask_count source=default value=20`,
		`level=INFO msg=configuration path=attention_serve_factor source=default value=2`,
		`level=INFO msg=configuration path=consent_redirect source=default value=http://127.0.0.1:47823/`,
		`level=INFO msg=configuration path=database.host source="flag --database.host" value=db.example`,
		`level=INFO msg=configuration path=database.name source="environment variable MEDIATED_MAILBOX_DATABASE__NAME" value=mailbox`,
		`level=INFO msg=configuration path=database.password_file source="flag --database.password_file" value=` + passwordFile,
		`level=INFO msg=configuration path=database.port source=default value=5432`,
		`level=INFO msg=configuration path=database.sslmode source=default value=verify-full`,
		`level=INFO msg=configuration path=database.sslrootcert source=default value=""`,
		`level=INFO msg=configuration path=database.user source=default value=mediated_mailbox_ui`,
		`level=INFO msg=configuration path=default_theme source=default value=system`,
		`level=INFO msg=configuration path=heuristics_interval source=default value=24h0m0s`,
		`level=INFO msg=configuration path=identity_header source=default value=""`,
		`level=INFO msg=configuration path=insecure_http source=default value=false`,
		`level=INFO msg=configuration path=listen source="flag --listen" value=127.0.0.1:0`,
		`level=INFO msg=configuration path=log_level source=default value=info`,
		`level=INFO msg=configuration path=operator_name source=default value=operator`,
		`level=INFO msg=configuration path=private_key_files source="flag --private_key_files" value=[` + privateKeyFile + `]`,
		`level=INFO msg=configuration path=probe_listen source="flag --probe_listen" value=127.0.0.1:0`,
		`level=INFO msg=configuration path=seal_public_key_file source="flag --seal_public_key_file" value=` + publicKeyFile,
		`level=INFO msg=configuration path=stream_interval source=default value=2s`,
		`level=INFO msg=configuration path=stream_poll_interval source=default value=5s`,
		`level=INFO msg=configuration path=stream_reconnect_max source=default value=30s`,
		`level=INFO msg=configuration path=sync_interval source=default value=5m0s`,
		`level=INFO msg=configuration path=tls_cert source="flag --tls_cert" value=/absent/tls.crt`,
		`level=INFO msg=configuration path=tls_key source="flag --tls_key" value=/absent/tls.key`,
		`level=INFO msg=configuration path=token_key_file source=default value=""`,
		`level=INFO msg="sealing to the public key" key_id=` + keyID,
	}
	got := strings.Split(strings.TrimSpace(out.String()), "\n")
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("log (-want +got):\n%s", diff)
	}
	if strings.Contains(out.String(), "the-secret-itself") {
		t.Errorf("the log holds the password:\n%s", out.String())
	}
}

// writeKeys writes a new key pair's files and returns their paths and the key identifier, the first
// 16 bytes of SHA-256 over the credential code's domain string and the public key, in hexadecimal,
// computed here rather than by the credential code (executioncontext/README.md).
func writeKeys(t *testing.T) (string, string, string) {
	t.Helper()
	private, err := seal.KEM().GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	seed, err := private.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	key := private.PublicKey().Bytes()
	dir := t.TempDir()
	public, secret := filepath.Join(dir, "public.key"), filepath.Join(dir, "private.key")
	if err := os.WriteFile(public, key, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(secret, seed, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(append([]byte("mediated-mailbox credential key identifier\x00"), key...))
	return public, secret, hex.EncodeToString(sum[:16])
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

// A consent redirect that is not a loopback address with an explicit port refuses the start, before it
// touches the database (docs/UI.md section 18.1).
func TestANonLoopbackConsentRedirectRefusesTheStart(t *testing.T) {
	for _, value := range []string{"http://localhost:47823/", "http://127.0.0.1/", "http://192.0.2.1:47823/"} {
		args := []string{
			"--database.host=db", "--database.name=mailbox", "--database.password_file=/absent", "--tls_cert=/tls/cert",
			"--tls_key=/tls/key", "--seal_public_key_file=/absent", "--private_key_files=[/absent]", "--consent_redirect=" + value,
		}
		err := Run(t.Context(), args, nil, discard(), new(slog.LevelVar), bundle)
		want := "validating the configuration: consent_redirect is not an http address on a loopback IP literal with an explicit port"
		if err == nil || err.Error() != want {
			t.Errorf("%s: Run returned %v, want %q", value, err, want)
		}
	}
}

// The start writes the effective configuration with each value's source before the level applies,
// then sets the level of the logger it was handed from log_level. So whatever the level, a refused
// value's source is written, log_level's own included, and once the level is set no record below it
// is written. A level that names none is refused (ADR-0078, ADR-0122).
func TestTheConfiguredLevelGovernsTheLog(t *testing.T) {
	refused := func(name string) string {
		return `validating the configuration: log_level "` + name + `" is not one of debug, info, warn and error`
	}
	logLevelFrom := func(source, value string) string {
		return `"msg":"configuration","path":"log_level","source":"` + source + `","value":"` + value + `"`
	}
	cases := []struct {
		name    string
		args    []string
		environ []string
		wantErr string
		// wantLine is a record the start must write, whatever the level.
		wantLine string
		// wantInfo is whether an info record is written once the start has returned.
		wantInfo bool
	}{
		{name: "no level", wantErr: "validating the configuration: tls_cert and tls_key are both required unless insecure_http is true", wantLine: logLevelFrom("default", "info"), wantInfo: true},
		{name: "info", args: []string{"--log_level=info"}, wantErr: "validating the configuration: tls_cert and tls_key are both required unless insecure_http is true", wantLine: logLevelFrom("flag --log_level", "info"), wantInfo: true},
		{name: "warn", args: []string{"--log_level=warn"}, wantErr: "validating the configuration: tls_cert and tls_key are both required unless insecure_http is true", wantLine: logLevelFrom("flag --log_level", "warn")},
		{name: "error, from the environment", environ: []string{"MEDIATED_MAILBOX_LOG_LEVEL=error"}, wantErr: "validating the configuration: tls_cert and tls_key are both required unless insecure_http is true", wantLine: logLevelFrom("environment variable MEDIATED_MAILBOX_LOG_LEVEL", "error")},
		{name: "a name in upper case", args: []string{"--log_level=INFO"}, wantErr: refused("INFO"), wantLine: logLevelFrom("flag --log_level", "INFO"), wantInfo: true},
		{name: "an offset", args: []string{"--log_level=info+2"}, wantErr: refused("info+2"), wantLine: logLevelFrom("flag --log_level", "info+2"), wantInfo: true},
		{name: "warning", environ: []string{"MEDIATED_MAILBOX_LOG_LEVEL=warning"}, wantErr: refused("warning"), wantLine: logLevelFrom("environment variable MEDIATED_MAILBOX_LOG_LEVEL", "warning"), wantInfo: true},
		{name: "a refused value at warn", args: append([]string{"--log_level=warn"}, []string{}...), wantErr: "validating the configuration: tls_cert and tls_key are both required unless insecure_http is true", wantLine: `"msg":"configuration","path":"tls_cert","source":"flag --tls_cert","value":"/tls/cert"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out bytes.Buffer
			level := new(slog.LevelVar)
			logger := slog.New(slog.NewJSONHandler(&out, &slog.HandlerOptions{Level: level}))
			err := Run(t.Context(), append([]string{"--database.host=db", "--database.name=mailbox", "--database.password_file=/absent", "--tls_cert=/tls/cert", "--seal_public_key_file=/absent", "--private_key_files=[/absent]"}, c.args...), c.environ, logger, level, bundle)
			if err == nil || !strings.HasPrefix(err.Error(), c.wantErr) {
				t.Fatalf("Run returned %v, want %q", err, c.wantErr)
			}
			if !strings.Contains(out.String(), c.wantLine) {
				t.Errorf("the start did not write %s:\n%s", c.wantLine, out.String())
			}
			out.Reset()
			logger.Info("after the start")
			if written := out.Len() != 0; written != c.wantInfo {
				t.Errorf("an info record after the start was written: %v, want %v", written, c.wantInfo)
			}
		})
	}
}
