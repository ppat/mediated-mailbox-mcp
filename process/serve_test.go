package process_test

import (
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
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/schedule"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// running is a program the process serves on ephemeral loopback ports.
type running struct {
	app, probes string
	stdout      *lockedBuffer
	stop        context.CancelFunc
	done        chan error
	once        sync.Once
	err         error
}

// serveSpec runs spec until the test ends or stop is called, and returns once the serving record is
// written. A start that stops early fails the test.
func serveSpec[C config.Program[C]](t *testing.T, spec process.Spec[C], args ...string) *running {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	r := &running{stdout: &lockedBuffer{}, stop: cancel, done: make(chan error, 1)}
	go func() { r.done <- process.RunContext(ctx, loopbackArgs[C](args), nil, r.stdout, io.Discard, spec) }()
	t.Cleanup(func() {
		cancel()
		if err := r.wait(); err != nil {
			t.Error(err)
		}
	})
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		select {
		case err := <-r.done:
			r.done <- err
			t.Fatalf("the process stopped before it served: %v\n%s", err, r.stdout.String())
		default:
		}
		for _, rec := range records(t, r.stdout.String()) {
			if rec.Msg == "serving" {
				r.app, r.probes = rec.Listen, rec.ProbeListen
				return r
			}
		}
	}
	t.Fatalf("the process wrote no serving record:\n%s", r.stdout.String())
	return nil
}

// wait returns what the run returned, once, and nil after.
func (r *running) wait() error {
	first := false
	r.once.Do(func() {
		first = true
		select {
		case r.err = <-r.done:
		case <-time.After(15 * time.Second):
			r.err = errors.New("the run did not return")
		}
	})
	if !first {
		return nil
	}
	return r.err
}

// get requests url with client and returns the status, the body and the headers, or an error when no
// answer arrives within five seconds.
func get(t *testing.T, client *http.Client, url string) (int, string, http.Header, error) {
	t.Helper()
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

func named(t *testing.T, name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if _, err := io.WriteString(w, name); err != nil {
			t.Error(err)
		}
	})
}

// Each endpoint's handler is served at its pattern and nowhere else, and any other path, the probes'
// included, is not found on the application server (ADR-0142).
func TestEachEndpointIsServedAtItsPattern(t *testing.T) {
	r := serveSpec(t, process.Spec[serving]{Name: "test", Endpoints: []process.Endpoint[serving]{
		endpoint("/one/", named(t, "one")), endpoint("/single", named(t, "single")), endpoint("/two/", named(t, "two")),
	}})
	got := map[string]string{}
	for _, path := range []string{"/one/x", "/single", "/two/y", "/elsewhere", "/healthz"} {
		status, body, _, err := get(t, &http.Client{}, "http://"+r.app+path)
		if err != nil {
			t.Fatal(err)
		}
		got[path] = fmt.Sprintf("%d %s", status, strings.TrimSpace(body))
	}
	want := map[string]string{"/one/x": "200 one", "/single": "200 single", "/two/y": "200 two", "/elsewhere": "404 404 page not found", "/healthz": "404 404 page not found"}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("answers (-want +got):\n%s", diff)
	}
}

// reporter is an endpoint whose Build hands its capabilities back, so the test reports through them.
func reporter(pattern string, got *process.Capabilities) process.Endpoint[serving] {
	return process.Endpoint[serving]{Pattern: pattern, Build: func(_ context.Context, p process.Capabilities, _ serving) (http.Handler, error) {
		*got = p
		return http.NotFoundHandler(), nil
	}}
}

// The operations endpoints answer as plain text with no content security policy, and no other path.
// The readiness probe answers from what was reported and runs nothing. It is ready only while
// everything that reported is ready, and names the first that is not, a failed one with its reason. A
// part that never reports is not waited for. The health probe answers 200 while the process runs
// (ADR-0051, ADR-0142).
func TestTheProbesAnswerFromWhatWasReported(t *testing.T) {
	var a, b process.Capabilities
	r := serveSpec(t, process.Spec[serving]{Name: "test", Endpoints: []process.Endpoint[serving]{
		reporter("/a/", &a), reporter("/b/", &b), endpoint("/silent/", http.NotFoundHandler()),
	}})
	probe := func(path string) string {
		status, body, header, err := get(t, &http.Client{}, "http://"+r.probes+path)
		if err != nil {
			t.Fatal(err)
		}
		if header.Get("Content-Security-Policy") != "" || !strings.HasPrefix(header.Get("Content-Type"), "text/plain") {
			t.Errorf("%s answered with headers %v, want plain text and no content security policy", path, header)
		}
		return fmt.Sprintf("%d %s", status, strings.TrimSpace(body))
	}
	got := map[string]string{"nothing reported": probe("/readyz"), "health": probe("/healthz"), "another path": probe("/debug/pprof/")}
	if metrics := probe("/metrics"); !strings.HasPrefix(metrics, "200 ") || !strings.Contains(metrics, "go_goroutines ") {
		t.Errorf("/metrics answered %.200s, want the registry", metrics)
	}
	a.Report(process.Ready, "")
	b.Report(process.NotReady, "waiting for its first load")
	got["b not ready"] = probe("/readyz")
	got["b not ready, asked again"] = probe("/readyz")
	a.Report(process.Failed, "its file is gone")
	got["a failed"] = probe("/readyz")
	a.Report(process.Ready, "")
	b.Report(process.Ready, "")
	got["both ready"] = probe("/readyz")
	want := map[string]string{
		"nothing reported": "200 ready", "health": "200 ok", "another path": "404 404 page not found",
		"b not ready": "503 not ready: /b/: waiting for its first load", "b not ready, asked again": "503 not ready: /b/: waiting for its first load",
		"a failed": "503 not ready: /a/ failed: its file is gone", "both ready": "200 ready",
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("probes (-want +got):\n%s", diff)
	}
}

// Every series an endpoint or a job kind registers carries the program's name as component, and a
// job kind's its own name as job_kind, so one series registered by two job kinds stays apart, while
// the runtime series carry no component (ADR-0076, ADR-0125, ADR-0142).
func TestEverySeriesIsLabelledWithWhatRegisteredIt(t *testing.T) {
	gauge := func(p process.Capabilities) error {
		return p.Metrics().Register(prometheus.NewGauge(prometheus.GaugeOpts{Name: "mediated_mailbox_test_gauge", Help: "A test gauge."}))
	}
	job := func(name string) process.Job[serving] {
		return process.Job[serving]{Name: name, Build: func(_ context.Context, p process.Capabilities, _ serving) (schedule.Plan, error) {
			return idlePlan(), gauge(p)
		}}
	}
	r := serveSpec(t, process.Spec[serving]{
		Name: "test",
		Endpoints: []process.Endpoint[serving]{{Pattern: "/", Build: func(_ context.Context, p process.Capabilities, _ serving) (http.Handler, error) {
			return http.NotFoundHandler(), p.Metrics().Register(prometheus.NewCounter(prometheus.CounterOpts{Name: "mediated_mailbox_test_total", Help: "A test counter."}))
		}}},
		Jobs: []process.Job[serving]{job("first"), job("second")},
	})
	_, metrics, _, err := get(t, &http.Client{}, "http://"+r.probes+"/metrics")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{
		"\n" + `mediated_mailbox_test_gauge{component="test",job_kind="first"} 0`,
		"\n" + `mediated_mailbox_test_gauge{component="test",job_kind="second"} 0`,
		"\n" + `mediated_mailbox_test_total{component="test"} 0`,
		"\ngo_goroutines ",
	} {
		if !strings.Contains(metrics, line) {
			t.Errorf("/metrics does not serve %s:\n%s", strings.TrimSpace(line), metrics)
		}
	}
	if strings.Contains(metrics, `go_goroutines{`) {
		t.Errorf("the runtime series carry labels:\n%s", metrics)
	}
}

// idlePlan lists no key.
func idlePlan() schedule.Plan {
	return schedule.Plan{
		Keys: func(context.Context) ([]string, error) { return nil, nil },
		Job:  func(string) (schedule.Job, error) { return schedule.Job{}, errors.New("no key is listed") },
	}
}

// A job kind's plan runs on the process's scheduler, which keeps a job for each key the listing finds
// and removes the job of a key it no longer finds, and the job kind reports not ready until its first
// listing has succeeded (ADR-0119, ADR-0142).
func TestAJobKindsPlanRunsOnTheScheduler(t *testing.T) {
	var mu sync.Mutex
	keys := []string{"personal"}
	listingFails := true
	running := map[string]bool{}
	var wake func()
	plan := schedule.Plan{
		Keys: func(context.Context) ([]string, error) {
			mu.Lock()
			defer mu.Unlock()
			if listingFails {
				return nil, errors.New("the accounts cannot be read yet")
			}
			return append([]string(nil), keys...), nil
		},
		Backoff: schedule.Backoff{Base: 20 * time.Millisecond, Max: 20 * time.Millisecond},
		Job: func(key string) (schedule.Job, error) {
			return schedule.Job{Run: func(ctx context.Context, _ schedule.Ask) error {
				mu.Lock()
				running[key] = true
				mu.Unlock()
				<-ctx.Done()
				mu.Lock()
				running[key] = false
				mu.Unlock()
				return nil
			}}, nil
		},
		Woken: func(w func()) { wake = w },
	}
	r := serveSpec(t, process.Spec[jobsOnly]{Name: "test", Jobs: []process.Job[jobsOnly]{{Name: "kind", Build: func(context.Context, process.Capabilities, jobsOnly) (schedule.Plan, error) {
		return plan, nil
	}}}})
	probe := func() string {
		status, body, _, err := get(t, &http.Client{}, "http://"+r.probes+"/readyz")
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("%d %s", status, strings.TrimSpace(body))
	}
	if got := probe(); got != "503 not ready: kind listing: no listing has succeeded yet" {
		t.Errorf("before any listing succeeded the probe answered %q", got)
	}
	eventually := func(what string, cond func() bool) {
		t.Helper()
		for deadline := time.Now().Add(5 * time.Second); !cond(); time.Sleep(10 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("%s never happened", what)
			}
		}
	}
	mu.Lock()
	listingFails = false
	mu.Unlock()
	eventually("the first listing", func() bool { return probe() == "200 ready" })
	eventually("the job of the listed key", func() bool { mu.Lock(); defer mu.Unlock(); return running["personal"] })
	mu.Lock()
	keys = []string{"work"}
	listingFails = true
	mu.Unlock()
	wake()
	time.Sleep(100 * time.Millisecond)
	if got := probe(); got != "200 ready" {
		t.Errorf("a failed listing after the first success left the probe answering %q, want ready", got)
	}
	mu.Lock()
	listingFails = false
	mu.Unlock()
	wake()
	eventually("the dropped key's job removed and the new key's added", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return !running["personal"] && running["work"]
	})
}

// When the process stops, the readiness probe answers not ready, Draining ends, the application server
// lets a request in flight finish, and only then do the jobs' context end and the jobs and every
// goroutine started through Go get waited for, and only then is what the slots hold closed, last built
// first. So an event stream ends on Draining while an ordinary request drains (ADR-0142).
func TestTheProcessStopsInOrder(t *testing.T) {
	var mu sync.Mutex
	var order []string
	record := func(event string) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, event)
	}
	release, arrived, streaming := make(chan struct{}), make(chan struct{}), make(chan struct{})
	first, second := process.NewOnePerProcess[string]("first"), process.NewOnePerProcess[string]("second")
	build := func(ctx context.Context, slot *process.OnePerProcess[string], name string, p process.Capabilities) {
		if _, err := slot.Get(ctx, p, func(context.Context, process.Capabilities) (string, func(), error) {
			return name, func() { record("closed " + name) }, nil
		}); err != nil {
			t.Error(err)
		}
	}
	var draining context.Context
	r := serveSpec(t, process.Spec[serving]{
		Name: "test",
		Endpoints: []process.Endpoint[serving]{{Pattern: "/", Build: func(ctx context.Context, p process.Capabilities, _ serving) (http.Handler, error) {
			build(ctx, first, "first", p)
			build(ctx, second, "second", p)
			draining = p.Draining()
			p.Go("loop", func(ctx context.Context) {
				<-ctx.Done()
				record("goroutine told to stop")
				// A goroutine takes a moment to stop, which a process that did not wait for it would miss.
				time.Sleep(50 * time.Millisecond)
				record("goroutine returned")
			})
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/stream" {
					close(streaming)
					<-draining.Done()
					record("stream ended")
					return
				}
				close(arrived)
				<-release
				record("request finished")
				w.WriteHeader(http.StatusNoContent)
			}), nil
		}}},
		Jobs: []process.Job[serving]{{Name: "kind", Build: func(context.Context, process.Capabilities, serving) (schedule.Plan, error) {
			plan := idlePlan()
			plan.Keys = func(context.Context) ([]string, error) { return []string{"key"}, nil }
			plan.Job = func(string) (schedule.Job, error) {
				return schedule.Job{Run: func(ctx context.Context, _ schedule.Ask) error {
					<-ctx.Done()
					record("job told to stop")
					time.Sleep(50 * time.Millisecond)
					record("job returned")
					return nil
				}}, nil
			}
			return plan, nil
		}}},
	})
	streamDone := make(chan error, 1)
	go func() {
		_, _, _, err := get(t, &http.Client{}, "http://"+r.app+"/stream")
		streamDone <- err
	}()
	requestDone := make(chan int, 1)
	go func() {
		status, _, _, err := get(t, &http.Client{}, "http://"+r.app+"/slow")
		if err != nil {
			status = 0
		}
		requestDone <- status
	}()
	for _, reached := range []chan struct{}{arrived, streaming} {
		select {
		case <-reached:
		case <-time.After(5 * time.Second):
			t.Fatal("a request never reached its handler")
		}
	}
	// The job's run has started once its job exists, which the first listing ensured.
	time.Sleep(100 * time.Millisecond)
	r.stop()
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		status, _, _, err := get(t, &http.Client{Timeout: time.Second}, "http://"+r.probes+"/readyz")
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
	if err := r.wait(); err != nil {
		t.Errorf("the run returned %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	// The job and the goroutine stop at once, so they are compared by what came before and after them.
	stopped := order[3:7]
	if diff := cmp.Diff([]string{"stream ended", "released", "request finished"}, order[:3], compare.Options); diff != "" {
		t.Errorf("the drain (-want +got):\n%s", diff)
	}
	for _, event := range []string{"job told to stop", "job returned", "goroutine told to stop", "goroutine returned"} {
		if !strings.Contains(strings.Join(stopped, ","), event) {
			t.Errorf("the stop's middle %v lacks %q", stopped, event)
		}
	}
	if diff := cmp.Diff([]string{"closed second", "closed first"}, order[7:], compare.Options); diff != "" {
		t.Errorf("the slots closed (-want +got):\n%s", diff)
	}
}

// A goroutine started through Go is the process's. Its panic is the failure of what started it, named
// with the goroutine, and stops the process in order, the stop waits for every such goroutine before
// the slots close, and once the process waits for them Go starts nothing and says so (ADR-0142).
func TestAGoroutineStartedThroughGoIsTheProcesss(t *testing.T) {
	var caps process.Capabilities
	var stopped atomic.Bool
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err := process.RunContext(ctx, loopbackArgs[serving](nil), nil, io.Discard, io.Discard, process.Spec[serving]{Name: "test", Endpoints: []process.Endpoint[serving]{{
		Pattern: "/", Build: func(_ context.Context, p process.Capabilities, _ serving) (http.Handler, error) {
			caps = p
			p.Go("steady", func(ctx context.Context) {
				<-ctx.Done()
				time.Sleep(50 * time.Millisecond)
				stopped.Store(true)
			})
			p.Go("faulty", func(context.Context) {
				// The fault comes once the process serves, so it is the running process that stops.
				time.Sleep(100 * time.Millisecond)
				panic("a fault in a goroutine")
			})
			return http.NotFoundHandler(), nil
		},
	}}})
	if !errors.Is(err, process.ErrPanicked) || !strings.Contains(err.Error(), "/ goroutine faulty: panicked: a fault in a goroutine") {
		t.Errorf("the run returned %v, want the faulty goroutine's panic", err)
	}
	if ctx.Err() != nil {
		t.Error("the process served until its context ended rather than stopping on the panic")
	}
	if !stopped.Load() {
		t.Error("the run returned before the steady goroutine was told to stop and returned")
	}
	var late atomic.Bool
	if caps.Go("late", func(context.Context) { late.Store(true) }) {
		t.Error("Go started a goroutine after the process stopped")
	}
	time.Sleep(50 * time.Millisecond)
	if late.Load() {
		t.Error("a goroutine Go refused ran")
	}
}

// A listener the application server cannot open refuses the start, before anything serves.
func TestAListenerThatCannotBeOpenedRefusesTheStart(t *testing.T) {
	var lc net.ListenConfig
	taken, err := lc.Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := taken.Close(); err != nil {
			t.Error(err)
		}
	}()
	err = runFor(t, process.Spec[serving]{Name: "test", Endpoints: []process.Endpoint[serving]{endpoint("/", http.NotFoundHandler())}}, []string{"--listen=" + taken.Addr().String()}, nil, io.Discard, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "address already in use") {
		t.Errorf("a listener that cannot be opened returned %v, want the start refused", err)
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
		SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "process test"},
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

func servingRecord(t *testing.T, r *running) record {
	for _, rec := range records(t, r.stdout.String()) {
		if rec.Msg == "serving" {
			return rec
		}
	}
	return record{}
}

// Given a key pair, the application server serves TLS, a plain HTTP request never reaches a handler,
// a key pair rotated in its mounted files serves the next handshake, and the serving record, at info,
// says it serves TLS (ADR-0127). The operations endpoints stay plain HTTP.
func TestTheApplicationServerServesTLSWhenGivenAKeyPair(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	first := writeKeyPair(t, certFile, keyFile, 1)
	var calls atomic.Int64
	r := serveSpec(t, process.Spec[serving]{Name: "test", Endpoints: []process.Endpoint[serving]{endpoint("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))}}, "--tls_cert="+certFile, "--tls_key="+keyFile)
	trusting := func(cert *x509.Certificate) *http.Client {
		pool := x509.NewCertPool()
		pool.AddCert(cert)
		return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}}
	}
	if status, _, _, err := get(t, trusting(first), "https://"+r.app+"/"); err != nil || status != http.StatusNoContent {
		t.Fatalf("a TLS request answered %d with %v, want 204", status, err)
	}
	if status, _, _, err := get(t, &http.Client{}, "http://"+r.app+"/"); err == nil && status == http.StatusNoContent {
		t.Error("a plain HTTP request reached the handler")
	}
	second := writeKeyPair(t, certFile, keyFile, 2)
	if status, _, _, err := get(t, trusting(second), "https://"+r.app+"/"); err != nil || status != http.StatusNoContent {
		t.Errorf("after the key pair was rotated a TLS request answered %d with %v, want 204", status, err)
	}
	if calls.Load() != 2 {
		t.Errorf("the handler was reached %d times, want the two TLS requests", calls.Load())
	}
	if status, _, _, err := get(t, &http.Client{}, "http://"+r.probes+"/healthz"); err != nil || status != http.StatusOK {
		t.Errorf("the health probe answered %d with %v over plain HTTP, want 200", status, err)
	}
	if rec := servingRecord(t, r); rec.Level != "INFO" || rec.TLS == nil || !*rec.TLS {
		t.Errorf("the serving record is %+v, want tls true at info", rec)
	}
}

// Given no key pair, the application server serves plain HTTP, a TLS handshake never completes, and
// the serving record says it serves no TLS (ADR-0127).
func TestTheApplicationServerGivenNoKeyPairIsPlain(t *testing.T) {
	for _, args := range [][]string{nil, {"--tls_cert=  ", "--tls_key=  "}} {
		var calls atomic.Int64
		r := serveSpec(t, process.Spec[serving]{Name: "test", Endpoints: []process.Endpoint[serving]{endpoint("/", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusNoContent)
		}))}}, args...)
		insecure := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12}}} //nolint:gosec // The handshake must fail whatever the certificate.
		if _, _, _, err := get(t, insecure, "https://"+r.app+"/"); err == nil {
			t.Errorf("%v: a TLS handshake completed", args)
		}
		if status, _, _, err := get(t, &http.Client{}, "http://"+r.app+"/"); err != nil || status != http.StatusNoContent || calls.Load() != 1 {
			t.Errorf("%v: a plain request answered %d with %v, want 204 from the handler", args, status, err)
		}
		if rec := servingRecord(t, r); rec.TLS == nil || *rec.TLS {
			t.Errorf("%v: the serving record is %+v, want tls false", args, rec)
		}
	}
}

// A key pair that does not load refuses the start before anything is served, and half a key pair is
// refused by the serving part's check before any key file is read (ADR-0127).
func TestAKeyPairThatDoesNotLoadRefusesTheStart(t *testing.T) {
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	spec := process.Spec[serving]{Name: "test", Endpoints: []process.Endpoint[serving]{endpoint("/", http.NotFoundHandler())}}
	if err := runFor(t, spec, []string{"--tls_cert=" + certFile, "--tls_key=" + keyFile}, nil, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "loading the TLS key pair") {
		t.Errorf("a key pair whose files do not exist returned %v, want the start refused", err)
	}
	if err := runFor(t, spec, []string{"--tls_cert=" + certFile}, nil, io.Discard, io.Discard); err == nil || !strings.Contains(err.Error(), "tls_cert is set and tls_key is not") {
		t.Errorf("half a key pair returned %v, want the start refused naming the missing file", err)
	}
}

// The scheduler refuses a plan it could never run, and the refusal fails the start before anything
// serves, with an error naming the job kind, after a goroutine started through Go was told to stop and
// waited for (ADR-0119, ADR-0142).
func TestASchedulerThatRefusesAPlanStopsTheProcessInOrder(t *testing.T) {
	var stopped atomic.Bool
	var stdout lockedBuffer
	err := runFor(t, process.Spec[serving]{
		Name: "test",
		Endpoints: []process.Endpoint[serving]{{Pattern: "/", Build: func(_ context.Context, p process.Capabilities, _ serving) (http.Handler, error) {
			p.Go("steady", func(ctx context.Context) {
				<-ctx.Done()
				time.Sleep(50 * time.Millisecond)
				stopped.Store(true)
			})
			return http.NotFoundHandler(), nil
		}}},
		Jobs: []process.Job[serving]{{Name: "kind", Build: func(context.Context, process.Capabilities, serving) (schedule.Plan, error) {
			return schedule.Plan{}, nil
		}}},
	}, nil, nil, &stdout, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "the scheduler refused the plan of the job kind kind") {
		t.Errorf("the run returned %v, want the scheduler's refusal naming the job kind", err)
	}
	for _, rec := range records(t, stdout.String()) {
		if rec.Msg == "serving" {
			t.Error("the process served before the refusal stopped its start")
		}
	}
	if !stopped.Load() {
		t.Error("the run returned before the steady goroutine was told to stop and returned")
	}
}

// The probes run nothing when asked. However often the readiness probe is asked, no part is built
// again, no slot's thing is built again and no goroutine is started, and each answer is the state
// last reported, which a part changes between asks (ADR-0142).
func TestTheProbesRunNothingWhenAsked(t *testing.T) {
	slot := process.NewOnePerProcess[int]("thing")
	var builds, slotBuilds atomic.Int64
	var caps process.Capabilities
	r := serveSpec(t, process.Spec[serving]{Name: "test", Endpoints: []process.Endpoint[serving]{{Pattern: "/", Build: func(ctx context.Context, p process.Capabilities, _ serving) (http.Handler, error) {
		builds.Add(1)
		caps = p
		_, err := slot.Get(ctx, p, func(context.Context, process.Capabilities) (int, func(), error) {
			slotBuilds.Add(1)
			return 1, nil, nil
		})
		return http.NotFoundHandler(), err
	}}}})
	goroutines := runtime.NumGoroutine()
	for i := range 20 {
		state, want := process.Ready, "200 ready"
		if i%2 == 0 {
			state, want = process.NotReady, fmt.Sprintf("503 not ready: /: ask %d", i)
		}
		caps.Report(state, fmt.Sprintf("ask %d", i))
		status, body, _, err := get(t, &http.Client{}, "http://"+r.probes+"/readyz")
		if err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%d %s", status, strings.TrimSpace(body)); got != want {
			t.Errorf("ask %d answered %q, want %q, the state last reported", i, got, want)
		}
	}
	if builds.Load() != 1 || slotBuilds.Load() != 1 {
		t.Errorf("after the asks the endpoint was built %d times and the slot's thing %d, want each once", builds.Load(), slotBuilds.Load())
	}
	// The client's idle connections hold a goroutine or two of the server's, so the count may grow by
	// those and no more.
	if after := runtime.NumGoroutine(); after > goroutines+4 {
		t.Errorf("the asks left %d goroutines running beside the %d before them", after, goroutines)
	}
}
