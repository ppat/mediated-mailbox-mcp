package host_test

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
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/process/host"
	"github.com/ppat/mediated-mailbox-mcp/process/version"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// config is a root configuration type composed as a composition root composes one, the process's
// sections inline beside a component's own.
type config struct {
	host.Process `yaml:",inline"`
	host.Serving `yaml:",inline"`
	Own          own `yaml:"own"`
}

type own struct {
	Value int `yaml:"value"`
}

func defaults() config {
	return config{Process: host.DefaultProcess(), Serving: host.DefaultServing(), Own: own{Value: 1}}
}

// spec returns a spec over config with one component, comp, whose assembly returns parts and records
// that it ran.
func spec(assembled *atomic.Bool, parts host.Parts, refuse func([]string) error) host.Spec[config] {
	return host.Spec[config]{
		Name:     "mediated-mailbox-test",
		Defaults: defaults(),
		Process:  func(c config) host.Process { return c.Process },
		Serving:  func(c config) host.Serving { return c.Serving },
		Validate: func(c config) error {
			if c.Own.Value < 0 {
				return errors.New("own.value is negative")
			}
			return nil
		},
		Components: []host.Component[config]{{
			Name: "comp", Mount: []string{"/"}, RefuseEnvironment: refuse,
			Assemble: func(context.Context, host.Provided[config]) (host.Parts, error) {
				assembled.Store(true)
				return parts, nil
			},
		}},
	}
}

func discard() *slog.Logger { return slog.New(slog.DiscardHandler) }

// run runs spec's process with args and environ under a context ended after five seconds, so a start
// that gets past the check a test expects to stop it fails rather than serving until the run times
// out. Every listener is an ephemeral loopback port.
func run(t *testing.T, s host.Spec[config], args, environ []string, stdout, stderr io.Writer, logger *slog.Logger, level *slog.LevelVar) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	for _, flag := range []string{"--listen=127.0.0.1:0", "--probe_listen=127.0.0.1:0"} {
		name, _, _ := strings.Cut(flag, "=")
		if !slices.ContainsFunc(args, func(a string) bool { return strings.HasPrefix(a, name+"=") }) {
			args = append([]string{flag}, args...)
		}
	}
	return host.Run(ctx, args, environ, stdout, stderr, logger, level, s)
}

// The process refuses to start while PGPASSWORD or PGSSLPASSWORD is set, even to an empty value, or
// while a component refuses the environment, before it reads its configuration, so it logs no value
// and assembles nothing (ADR-0078, ADR-0086). The configuration file named does not exist, so a refusal
// made after the read would report the missing file instead.
func TestTheEnvironmentIsRefusedBeforeTheConfigurationIsRead(t *testing.T) {
	refuse := func(environ []string) error {
		for _, e := range environ {
			if strings.HasPrefix(e, "COMPONENTDEBUG=") {
				return errors.New("COMPONENTDEBUG is set")
			}
		}
		return nil
	}
	for _, c := range []struct{ environ, want string }{
		{"PGPASSWORD=s3cret", "the environment sets PGPASSWORD, and the database password comes only from the mounted password file"},
		{"PGPASSWORD=", "the environment sets PGPASSWORD, and the database password comes only from the mounted password file"},
		{"PGSSLPASSWORD=", "the environment sets PGSSLPASSWORD, and the database password comes only from the mounted password file"},
		{"COMPONENTDEBUG=", "COMPONENTDEBUG is set"},
	} {
		t.Run(c.environ, func(t *testing.T) {
			var out bytes.Buffer
			var assembled atomic.Bool
			err := run(t, spec(&assembled, host.Parts{}, refuse), []string{"--config-file=/absent/config.yaml"}, []string{c.environ},
				io.Discard, io.Discard, slog.New(slog.NewTextHandler(&out, nil)), new(slog.LevelVar))
			if err == nil || err.Error() != c.want {
				t.Errorf("Run returned %v, want %q", err, c.want)
			}
			if out.Len() != 0 || assembled.Load() {
				t.Errorf("the start logged %q or assembled %v before refusing the environment", out.String(), assembled.Load())
			}
		})
	}
}

// --help and --version are answered on the standard output the process is handed, and read no layer,
// so a refused value beside them refuses nothing and nothing is assembled (ADR-0078).
func TestHelpAndVersionAreAnsweredOnStandardOutput(t *testing.T) {
	for _, arg := range []string{"--help", "--version"} {
		t.Run(arg, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			var assembled atomic.Bool
			err := run(t, spec(&assembled, host.Parts{}, nil), []string{arg, "--own.value=not-a-number"}, nil, &stdout, &stderr, discard(), new(slog.LevelVar))
			if err != nil {
				t.Fatal(err)
			}
			want := version.Number + "\n"
			if arg == "--help" {
				want = "probe_listen"
			}
			if !strings.Contains(stdout.String(), want) || stderr.Len() != 0 || assembled.Load() {
				t.Errorf("standard output %q, standard error %q, assembled %v, want %q on standard output alone", stdout.String(), stderr.String(), assembled.Load(), want)
			}
		})
	}
}

// The process writes the effective configuration with each value's source before the level applies,
// then sets the level of the logger it was handed from log_level. So whatever the level, a refused
// value's source is written, log_level's own included, and once the level is set no record below it is
// written. A level that names none is refused (ADR-0078, ADR-0122).
func TestTheConfiguredLevelGovernsTheLog(t *testing.T) {
	refused := func(name string) string {
		return `validating the configuration: log_level "` + name + `" is not one of debug, info, warn and error`
	}
	const ownRefused = "validating the configuration: own.value is negative"
	logLevelFrom := func(source, value string) string {
		return `"msg":"configuration","path":"log_level","source":"` + source + `","value":"` + value + `"`
	}
	cases := []struct {
		name     string
		args     []string
		environ  []string
		wantErr  string
		wantLine string
		wantInfo bool
	}{
		{name: "no level", wantErr: ownRefused, wantLine: logLevelFrom("default", "info"), wantInfo: true},
		{name: "warn", args: []string{"--log_level=warn"}, wantErr: ownRefused, wantLine: logLevelFrom("flag --log_level", "warn")},
		{name: "error, from the environment", environ: []string{"MEDIATED_MAILBOX_LOG_LEVEL=error"}, wantErr: ownRefused, wantLine: logLevelFrom("environment variable MEDIATED_MAILBOX_LOG_LEVEL", "error")},
		{name: "a name in upper case", args: []string{"--log_level=INFO"}, wantErr: refused("INFO"), wantLine: logLevelFrom("flag --log_level", "INFO"), wantInfo: true},
		{name: "an offset", args: []string{"--log_level=info+2"}, wantErr: refused("info+2"), wantLine: logLevelFrom("flag --log_level", "info+2"), wantInfo: true},
		{name: "warning", environ: []string{"MEDIATED_MAILBOX_LOG_LEVEL=warning"}, wantErr: refused("warning"), wantLine: logLevelFrom("environment variable MEDIATED_MAILBOX_LOG_LEVEL", "warning"), wantInfo: true},
		{name: "a refused value at error", args: []string{"--log_level=error"}, wantErr: ownRefused, wantLine: `"msg":"configuration","path":"own.value","source":"flag --own.value","value":-1`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out bytes.Buffer
			level := new(slog.LevelVar)
			logger := slog.New(slog.NewJSONHandler(&out, &slog.HandlerOptions{Level: level}))
			var assembled atomic.Bool
			err := run(t, spec(&assembled, host.Parts{}, nil), append([]string{"--own.value=-1"}, c.args...), c.environ, io.Discard, io.Discard, logger, level)
			if err == nil || err.Error() != c.wantErr {
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
			if assembled.Load() {
				t.Error("a configuration the process refused was assembled")
			}
		})
	}
}

// The process's sections are validated before the root's, and before anything is assembled. The TLS
// files are both set or both unset, a blank path counting as unset (ADR-0127).
func TestTheProcessSectionsAreValidated(t *testing.T) {
	for _, c := range []struct {
		name string
		args []string
		want string
	}{
		{"an empty listen", []string{"--listen= "}, "validating the configuration: listen is empty"},
		{"an empty probe_listen", []string{"--probe_listen="}, "validating the configuration: probe_listen is empty"},
		{"a certificate without its key", []string{"--tls_cert=/tls/cert"}, "validating the configuration: tls_cert is set and tls_key is not, and the process serves TLS with both or plain HTTP with neither"},
		{"a key without its certificate", []string{"--tls_key=/tls/key"}, "validating the configuration: tls_key is set and tls_cert is not, and the process serves TLS with both or plain HTTP with neither"},
		{"a certificate with a blank key", []string{"--tls_cert=/tls/cert", "--tls_key=  "}, "validating the configuration: tls_cert is set and tls_key is not, and the process serves TLS with both or plain HTTP with neither"},
		{"a key with a blank certificate", []string{"--tls_cert= ", "--tls_key=/tls/key"}, "validating the configuration: tls_key is set and tls_cert is not, and the process serves TLS with both or plain HTTP with neither"},
		{"the root's own section", []string{"--own.value=-1"}, "validating the configuration: own.value is negative"},
	} {
		t.Run(c.name, func(t *testing.T) {
			var assembled atomic.Bool
			err := run(t, spec(&assembled, host.Parts{}, nil), c.args, nil, io.Discard, io.Discard, discard(), new(slog.LevelVar))
			if err == nil || err.Error() != c.want || assembled.Load() {
				t.Errorf("Run returned %v having assembled %v, want %q", err, assembled.Load(), c.want)
			}
		})
	}
	for name, files := range map[string][2]string{"both files": {"/tls/cert", "/tls/key"}, "neither file": {"", ""}, "both blank": {" ", "\t"}} {
		if err := host.ValidateServing(host.Serving{Listen: ":8443", TLSCert: files[0], TLSKey: files[1]}); err != nil {
			t.Errorf("%s was refused: %v", name, err)
		}
		if got, want := (host.Serving{TLSCert: files[0], TLSKey: files[1]}).ServesTLS(), name == "both files"; got != want {
			t.Errorf("%s serves TLS %v, want %v", name, got, want)
		}
	}
}

// The start banner goes to the standard error the process is handed and names the process, its
// release version and where it serves, and standard output carries only the logger's records
// (ADR-0122).
func TestTheStartBannerGoesToStandardError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	var assembled atomic.Bool
	logger := slog.New(slog.NewJSONHandler(&stdout, nil))
	err := run(t, spec(&assembled, host.Parts{}, nil), []string{"--tls_cert=/absent/cert", "--tls_key=/absent/key"}, nil, &stdout, &stderr, logger, new(slog.LevelVar))
	if err == nil || !strings.HasPrefix(err.Error(), "loading the TLS key pair: ") {
		t.Fatalf("Run returned %v, want the key pair refused", err)
	}
	want := "mediated-mailbox-test " + version.Number + "\n  serves  https on 127.0.0.1:0\n  probes  http on 127.0.0.1:0\n"
	if stderr.String() != want {
		t.Errorf("standard error %q, want %q", stderr.String(), want)
	}
	for line := range strings.Lines(stdout.String()) {
		if !strings.HasPrefix(line, "{") {
			t.Errorf("standard output holds a line that is no JSON record: %q", line)
		}
	}
}

// writeKeyPair writes a new self-signed certificate for 127.0.0.1 with the given serial and its key to
// certFile and keyFile, and returns the certificate.
func writeKeyPair(t *testing.T, certFile, keyFile string, serial int64) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "host test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		// Self-signed, so the test's client trusts it as its own authority.
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

// lockedBuffer is a log destination the process and the test write and read from different goroutines.
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

func loopback(t *testing.T) net.Listener {
	t.Helper()
	var lc net.ListenConfig
	ln, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	return ln
}

// served is a host serving its components on two loopback listeners.
type served struct {
	app, probes string
	log         *lockedBuffer
	stop        context.CancelFunc
	done        chan error
	once        sync.Once
	err         error
}

// serve serves h on two loopback listeners with s until the test ends or stop is called, and returns
// once the serving record is written. A start that stops early fails the test.
func serve(t *testing.T, h *host.Host, s host.Serving, log *lockedBuffer) *served {
	t.Helper()
	app, probes := loopback(t), loopback(t)
	ctx, cancel := context.WithCancel(t.Context())
	sv := &served{app: app.Addr().String(), probes: probes.Addr().String(), log: log, stop: cancel, done: make(chan error, 1)}
	go func() { sv.done <- h.Serve(ctx, host.Listeners{App: app, Probes: probes}, s) }()
	t.Cleanup(func() {
		cancel()
		if err := sv.wait(t); err != nil {
			t.Error(err)
		}
	})
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		select {
		case err := <-sv.done:
			sv.done <- err
			t.Fatalf("the process stopped before it served: %v", err)
		default:
		}
		if strings.Contains(log.String(), " msg=serving ") {
			return sv
		}
	}
	t.Fatalf("the process wrote no serving record:\n%s", log.String())
	return nil
}

// wait returns what Serve returned, once, and nil after.
func (s *served) wait(t *testing.T) error {
	t.Helper()
	first := false
	s.once.Do(func() {
		first = true
		select {
		case s.err = <-s.done:
		case <-time.After(15 * time.Second):
			s.err = errors.New("serve did not return")
		}
	})
	if !first {
		return nil
	}
	return s.err
}

// newHost returns a host logging to log as text.
func newHost(t *testing.T, log *lockedBuffer) *host.Host {
	t.Helper()
	h, err := host.New(slog.New(slog.NewTextHandler(log, &slog.HandlerOptions{Level: slog.LevelDebug})))
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// get requests url with client and returns the status, the body and the headers, or an error when no
// answer arrives.
func get(t *testing.T, client *http.Client, url string) (int, string, http.Header, error) {
	t.Helper()
	// No answer within five seconds fails the request, so a server that never answers fails the test
	// rather than holding it until the run times out.
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", nil, err
	}
	body, err := io.ReadAll(resp.Body)
	return resp.StatusCode, string(body), resp.Header, errors.Join(err, resp.Body.Close())
}

// noContent answers 204 and counts the requests that reached it.
func noContent(calls *atomic.Int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	})
}

// Given a key pair, the application server serves TLS, a plain HTTP request never reaches a handler,
// a key pair rotated in its mounted files serves the next handshake, and the serving record says it
// serves TLS (ADR-0127). The operations endpoints stay plain HTTP.
func TestTheApplicationServerServesTLSWhenGivenAKeyPair(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	first := writeKeyPair(t, certFile, keyFile, 1)
	log := &lockedBuffer{}
	h := newHost(t, log)
	var calls atomic.Int64
	h.Add("comp", []string{"/"}, host.Parts{Handler: noContent(&calls)})
	sv := serve(t, h, host.Serving{TLSCert: certFile, TLSKey: keyFile}, log)

	if status, _, _, err := get(t, &http.Client{}, "http://"+sv.app+"/"); err == nil && status == http.StatusNoContent {
		t.Error("a plain HTTP request reached the handler")
	}
	for i, cert := range []*x509.Certificate{first, nil} {
		if cert == nil {
			cert = writeKeyPair(t, certFile, keyFile, 2)
		}
		pool := x509.NewCertPool()
		pool.AddCert(cert)
		client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}, DisableKeepAlives: true}}
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+sv.app+"/", http.NoBody)
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
		if got := resp.TLS.PeerCertificates[0].SerialNumber.Int64(); resp.StatusCode != http.StatusNoContent || got != cert.SerialNumber.Int64() {
			t.Errorf("handshake %d answered %d with serial %d, want 204 with serial %d", i+1, resp.StatusCode, got, cert.SerialNumber.Int64())
		}
	}
	if n := calls.Load(); n != 2 {
		t.Errorf("the handler was reached %d times, want 2", n)
	}
	if status, _, _, err := get(t, &http.Client{}, "http://"+sv.probes+"/healthz"); err != nil || status != http.StatusOK {
		t.Errorf("the health probe answered %d with error %v over plain HTTP, want 200", status, err)
	}
	if !strings.Contains(servingRecord(log), " tls=true") {
		t.Errorf("the serving record does not say it serves TLS:\n%s", log.String())
	}
}

func servingRecord(log *lockedBuffer) string {
	for line := range strings.SplitSeq(log.String(), "\n") {
		if strings.Contains(line, " msg=serving ") {
			return line
		}
	}
	return ""
}

// Given no key pair, or two TLS paths of spaces, the application server serves plain HTTP, a TLS
// handshake fails, and the serving record says it does not serve TLS (ADR-0127).
func TestTheApplicationServerGivenNoKeyPairIsPlain(t *testing.T) {
	for name, files := range map[string][2]string{"neither file": {"", ""}, "both blank": {" ", "\t"}} {
		t.Run(name, func(t *testing.T) {
			log := &lockedBuffer{}
			h := newHost(t, log)
			var calls atomic.Int64
			h.Add("comp", []string{"/"}, host.Parts{Handler: noContent(&calls)})
			sv := serve(t, h, host.Serving{TLSCert: files[0], TLSKey: files[1]}, log)
			if status, _, _, err := get(t, &http.Client{}, "http://"+sv.app+"/"); err != nil || status != http.StatusNoContent {
				t.Errorf("a plain HTTP request answered %d with error %v, want 204", status, err)
			}
			insecure := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}, DisableKeepAlives: true}} //nolint:gosec // only whether a handshake completes is asked
			if _, _, _, err := get(t, insecure, "https://"+sv.app+"/"); err == nil {
				t.Error("a TLS request was answered by a process given no key pair")
			}
			if !strings.Contains(servingRecord(log), " tls=false") {
				t.Errorf("the serving record does not say it serves plain HTTP:\n%s", log.String())
			}
		})
	}
}

// A key pair that does not load refuses the start before anything is served, so a process whose
// mounts cannot serve never answers a request, and it closes its listeners (ADR-0127).
func TestAKeyPairThatDoesNotLoadRefusesTheStart(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	writeKeyPair(t, certFile, keyFile, 1)
	for name, s := range map[string]host.Serving{
		"a certificate that is absent": {TLSCert: filepath.Join(dir, "absent"), TLSKey: keyFile},
		"a key that does not match":    {TLSCert: certFile, TLSKey: certFile},
	} {
		t.Run(name, func(t *testing.T) {
			h := newHost(t, &lockedBuffer{})
			var calls atomic.Int64
			h.Add("comp", []string{"/"}, host.Parts{Handler: noContent(&calls)})
			app, probes := loopback(t), loopback(t)
			// A start that got past the key pair would serve until the context ends.
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()
			err := h.Serve(ctx, host.Listeners{App: app, Probes: probes}, s)
			if err == nil || !strings.HasPrefix(err.Error(), "loading the TLS key pair: ") {
				t.Fatalf("Serve returned %v, want the key pair refused", err)
			}
			if _, err := app.Accept(); !errors.Is(err, net.ErrClosed) {
				t.Errorf("the application listener is open after the refusal: %v", err)
			}
		})
	}
}

// The readiness probe answers 200 only while the process serves and every check of every component
// passes, and 503 naming the first that fails. The health
// probe answers 200 while the process runs, the metrics endpoint serves the one registry, each
// component's series labelled with its name, so two components registering one series stay apart, and
// no other path is answered. Every answer is plain text with no content security policy (ADR-0051,
// ADR-0076, ADR-0138). The stop's own readiness is TestTheProcessStopsInOrder's.
func TestTheOperationsEndpoints(t *testing.T) {
	log := &lockedBuffer{}
	h := newHost(t, log)
	var aFails, bFails atomic.Bool
	check := func(fails *atomic.Bool) func(context.Context) error {
		return func(context.Context) error {
			if fails.Load() {
				return errors.New("down")
			}
			return nil
		}
	}
	for _, name := range []string{"a", "b"} {
		gauge := prometheus.NewGauge(prometheus.GaugeOpts{Name: "mediated_mailbox_test_gauge", Help: "A test gauge."})
		if err := h.Registerer(name).Register(gauge); err != nil {
			t.Fatalf("registering %s's series: %v", name, err)
		}
	}
	h.Add("a", nil, host.Parts{Ready: []host.Check{{Name: "first", Check: check(&aFails)}}})
	h.Add("b", nil, host.Parts{Ready: []host.Check{{Name: "second", Check: check(&bFails)}}})
	sv := serve(t, h, host.Serving{}, log)
	probe := func(path string) string {
		status, body, header, err := get(t, &http.Client{Timeout: 5 * time.Second}, "http://"+sv.probes+path)
		if err != nil {
			t.Fatal(err)
		}
		if header.Get("Content-Security-Policy") != "" || !strings.HasPrefix(header.Get("Content-Type"), "text/plain") {
			t.Errorf("%s answered with headers %v, want plain text and no content security policy", path, header)
		}
		if path == "/metrics" {
			return body
		}
		return fmt.Sprintf("%d %s", status, strings.TrimSpace(body))
	}
	got := map[string]string{"all pass": probe("/readyz"), "health": probe("/healthz"), "another path": probe("/debug/pprof/")}
	bFails.Store(true)
	got["b fails"] = probe("/readyz")
	aFails.Store(true)
	got["both fail"] = probe("/readyz")
	aFails.Store(false)
	bFails.Store(false)
	got["both pass again"] = probe("/readyz")
	want := map[string]string{
		"all pass": "200 ready", "health": "200 ok", "another path": "404 404 page not found", "b fails": "503 not ready: b second",
		"both fail": "503 not ready: a first", "both pass again": "200 ready",
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("probes (-want +got):\n%s", diff)
	}
	metrics := probe("/metrics")
	for _, line := range []string{"\n" + `mediated_mailbox_test_gauge{component="a"} 0`, "\n" + `mediated_mailbox_test_gauge{component="b"} 0`, "\ngo_goroutines "} {
		if !strings.Contains(metrics, line) {
			t.Errorf("/metrics does not serve %s:\n%s", line, metrics)
		}
	}
}

// When the process stops, the readiness probe answers not ready, Draining ends, the application server
// lets a request in flight finish, and only then do the tasks' context end, and Serve returns once
// every task has returned. So an event stream ends on Draining while an ordinary request drains.
func TestTheProcessStopsInOrder(t *testing.T) {
	log := &lockedBuffer{}
	h := newHost(t, log)
	var mu sync.Mutex
	var order []string
	record := func(event string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, event)
	}
	release, arrived, streaming := make(chan struct{}), make(chan struct{}), make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/stream" {
			close(streaming)
			<-h.Draining().Done()
			record("stream ended")
			return
		}
		close(arrived)
		<-release
		record("request finished")
		w.WriteHeader(http.StatusNoContent)
	})
	h.Add("comp", []string{"/"}, host.Parts{Handler: handler, Tasks: []host.Task{{Name: "loop", Run: func(ctx context.Context) error {
		<-ctx.Done()
		record("task told to stop")
		// A task takes a moment to stop, which a host that did not wait for it would miss.
		time.Sleep(50 * time.Millisecond)
		record("task returned")
		return nil
	}}}})
	sv := serve(t, h, host.Serving{}, log)
	streamDone := make(chan error, 1)
	go func() {
		_, _, _, err := get(t, &http.Client{}, "http://"+sv.app+"/stream")
		streamDone <- err
	}()
	requestDone := make(chan int, 1)
	go func() {
		status, _, _, err := get(t, &http.Client{}, "http://"+sv.app+"/slow")
		if err != nil {
			status = 0
		}
		requestDone <- status
	}()
	// A request that never reached its handler fails the test rather than holding it.
	for _, reached := range []chan struct{}{arrived, streaming} {
		select {
		case <-reached:
		case <-time.After(5 * time.Second):
			t.Fatal("a request never reached its handler")
		}
	}
	sv.stop()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		status, _, _, err := get(t, &http.Client{Timeout: time.Second}, "http://"+sv.probes+"/readyz")
		if err == nil && status == http.StatusServiceUnavailable {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the readiness probe answered %d with error %v after the stop, want 503", status, err)
		}
	}
	if err := <-streamDone; err != nil {
		t.Errorf("the stream did not end cleanly on draining: %v", err)
	}
	record("released")
	close(release)
	if status := <-requestDone; status != http.StatusNoContent {
		t.Errorf("the request in flight answered %d, want 204 after the drain", status)
	}
	if err := sv.wait(t); err != nil {
		t.Errorf("Serve returned %v", err)
	}
	want := []string{"stream ended", "released", "request finished", "task told to stop", "task returned"}
	mu.Lock()
	defer mu.Unlock()
	if diff := cmp.Diff(want, order, compare.Options); diff != "" {
		t.Errorf("the order of the stop (-want +got):\n%s", diff)
	}
}

// A task that panics is that task's failure. The process stops in order, every other task told to
// stop and waited for, and Serve returns the panic as an error naming the component and the task, so
// one task's panic never stops the process at once (ADR-0119, ADR-0138).
func TestATaskPanicIsItsFailure(t *testing.T) {
	log := &lockedBuffer{}
	h := newHost(t, log)
	var stopped atomic.Bool
	h.Add("comp", nil, host.Parts{Tasks: []host.Task{
		{Name: "steady", Run: func(ctx context.Context) error {
			<-ctx.Done()
			// A task takes a moment to stop, which a host that did not wait for it would miss.
			time.Sleep(50 * time.Millisecond)
			stopped.Store(true)
			return nil
		}},
		{Name: "faulty", Run: func(context.Context) error { panic("a fault in a task") }},
	}})
	// A failure the host missed would leave the process serving until the context ends.
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err := h.Serve(ctx, host.Listeners{Probes: loopback(t)}, host.Serving{})
	if !errors.Is(err, host.ErrPanicked) || !strings.Contains(err.Error(), "comp task faulty: panicked: a fault in a task") {
		t.Errorf("Serve returned %v, want the faulty task's panic", err)
	}
	if !stopped.Load() {
		t.Error("Serve returned before the steady task was told to stop and returned")
	}
}

// A task that fails stops the process with its error, and the other tasks are told to stop and waited
// for.
// A goroutine a component starts through the host's starter is the host's as much as a task. Its
// panic is the component's failure and stops the process in order, the stop waits for every such
// goroutine before the pools close, and once the host waits for them the starter starts nothing and
// says so (ADR-0138).
func TestAStartedGoroutineIsTheHosts(t *testing.T) {
	log := &lockedBuffer{}
	h := newHost(t, log)
	start := h.Starter("comp")
	var stopped atomic.Bool
	if !start("steady", func(ctx context.Context) {
		<-ctx.Done()
		// A goroutine takes a moment to stop, which a host that did not wait for it would miss.
		time.Sleep(50 * time.Millisecond)
		stopped.Store(true)
	}) {
		t.Fatal("the starter refused a goroutine before the process stopped")
	}
	if !start("faulty", func(context.Context) { panic("a fault in a goroutine") }) {
		t.Fatal("the starter refused a goroutine before the process stopped")
	}
	// A failure the host missed would leave the process serving until the context ends.
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err := h.Serve(ctx, host.Listeners{Probes: loopback(t)}, host.Serving{})
	if !errors.Is(err, host.ErrPanicked) || !strings.Contains(err.Error(), "comp goroutine faulty: panicked: a fault in a goroutine") {
		t.Errorf("Serve returned %v, want the faulty goroutine's panic", err)
	}
	if ctx.Err() != nil {
		t.Error("the process served until its context ended rather than stopping on the panic")
	}
	if !stopped.Load() {
		t.Error("Serve returned before the steady goroutine was told to stop and returned")
	}
	var late atomic.Bool
	if start("late", func(context.Context) { late.Store(true) }) {
		t.Error("the starter started a goroutine after the host stopped")
	}
	time.Sleep(50 * time.Millisecond)
	if late.Load() {
		t.Error("a goroutine the starter refused ran")
	}
}

func TestATaskFailureStopsTheProcess(t *testing.T) {
	h := newHost(t, &lockedBuffer{})
	var stopped atomic.Bool
	h.Add("comp", nil, host.Parts{Tasks: []host.Task{
		{Name: "steady", Run: func(ctx context.Context) error {
			<-ctx.Done()
			// A task takes a moment to stop, which a host that did not wait for it would miss.
			time.Sleep(50 * time.Millisecond)
			stopped.Store(true)
			return nil
		}},
		{Name: "failing", Run: func(context.Context) error { return errors.New("it failed") }},
	}})
	// A failure the host missed would leave the process serving until the context ends.
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err := h.Serve(ctx, host.Listeners{Probes: loopback(t)}, host.Serving{})
	if err == nil || err.Error() != "comp task failing: it failed" || !stopped.Load() {
		t.Errorf("Serve returned %v with the steady task stopped %v, want the failing task's error after the steady task stopped", err, stopped.Load())
	}
}

// Each component's handler is served at the patterns it is mounted at and nowhere else, and a path no
// component is mounted at is not found.
func TestEachHandlerIsServedWhereItIsMounted(t *testing.T) {
	log := &lockedBuffer{}
	h := newHost(t, log)
	named := func(name string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			if _, err := io.WriteString(w, name); err != nil {
				t.Error(err)
			}
		})
	}
	h.Add("one", []string{"/one/", "/single"}, host.Parts{Handler: named("one")})
	h.Add("two", []string{"/two/"}, host.Parts{Handler: named("two")})
	sv := serve(t, h, host.Serving{}, log)
	got := map[string]string{}
	for _, path := range []string{"/one/x", "/single", "/two/y", "/elsewhere", "/healthz"} {
		status, body, _, err := get(t, &http.Client{}, "http://"+sv.app+path)
		if err != nil {
			t.Fatal(err)
		}
		got[path] = fmt.Sprintf("%d %s", status, strings.TrimSpace(body))
	}
	want := map[string]string{"/one/x": "200 one", "/single": "200 one", "/two/y": "200 two", "/elsewhere": "404 404 page not found", "/healthz": "404 404 page not found"}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("answers (-want +got):\n%s", diff)
	}
}
