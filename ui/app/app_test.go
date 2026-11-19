package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// run runs the UI's spec as its main.go does, until ctx ends or the start fails.
func run(ctx context.Context, args, environ []string, stdout, stderr io.Writer) error {
	return process.RunContext(ctx, args, environ, stdout, stderr, Spec())
}

// The UI's defaults are core/config's, in which the database's host, name, user and password file
// and the key files are required, so a start that names no host is refused at it, before anything
// is read or connected (docs/UI.md section 18.1, ADR-0143).
func TestAStartWithNoDatabaseHostIsRefused(t *testing.T) {
	err := run(t.Context(), []string{"--database.name=mailbox", "--database.user=mediated_mailbox_app", "--database.password_file=/absent"}, nil, io.Discard, io.Discard)
	wantErr := "loading the configuration: database.host is required, and neither the file, MEDIATED_MAILBOX_DATABASE__HOST nor --database.host sets it"
	if err == nil || err.Error() != wantErr {
		t.Errorf("the run returned %v, want %q", err, wantErr)
	}
	err = run(t.Context(), []string{"--database.host=db", "--database.name=mailbox", "--database.password_file=/absent"}, nil, io.Discard, io.Discard)
	wantErr = "loading the configuration: database.user is required, and neither the file, MEDIATED_MAILBOX_DATABASE__USER nor --database.user sets it"
	if err == nil || err.Error() != wantErr {
		t.Errorf("the run returned %v, want %q", err, wantErr)
	}
}

// The database section's pool size is read from any layer and validated before anything else
// starts, so a pool of no connections refuses the start (ADR-0078, VERIFICATIONS, the row for a
// configuration mistake refused).
func TestAPoolSizeOfZeroRefusesTheStart(t *testing.T) {
	args := []string{
		"--database.host=db", "--database.name=mailbox", "--database.user=mediated_mailbox_app", "--database.password_file=/absent", "--database.pool_size=0",
		"--credential.public_key_file=/absent", "--credential.private_key_files=[/absent]",
	}
	err := run(t.Context(), args, nil, io.Discard, io.Discard)
	want := "validating the configuration: database.pool_size 0 is not positive"
	if err == nil || err.Error() != want {
		t.Errorf("the run returned %v, want %q", err, want)
	}
}

// The credential section is validated as the API server's and the worker's is, so an empty list of
// private keys refuses the start before any key file is read (ADR-0078, VERIFICATIONS, the row for a
// configuration mistake refused).
func TestAnEmptyListOfPrivateKeysRefusesTheStart(t *testing.T) {
	args := []string{
		"--database.host=db", "--database.name=mailbox", "--database.user=mediated_mailbox_app", "--database.password_file=/absent", "--tls_cert=/tls/cert",
		"--tls_key=/tls/key", "--credential.public_key_file=/absent", "--credential.private_key_files=[]",
	}
	err := run(t.Context(), args, nil, io.Discard, io.Discard)
	want := "validating the configuration: credential.private_key_files names no file"
	if err == nil || err.Error() != want {
		t.Errorf("the run returned %v, want %q", err, want)
	}
}

// The start logs every effective value with the layer that set it, the password file's path and
// never its contents, and the identifier of the key it seals to (ADR-0092, VERIFICATIONS, the row for
// the UI's log of its key). It stops at the schema check, which reaches no database here, after the log
// and before it serves (ADR-0140).
func TestTheEffectiveConfigurationIsLogged(t *testing.T) {
	passwordFile := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(passwordFile, []byte("the-secret-itself\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	publicKeyFile, privateKeyFile, keyID := writeKeys(t)
	var out bytes.Buffer
	err := run(bounded(t), []string{
		"--database.host=db.example", "--database.user=mediated_mailbox_app", "--database.password_file=" + passwordFile, "--tls_cert=/absent/tls.crt", "--tls_key=/absent/tls.key",
		"--listen=127.0.0.1:0", "--probe_listen=127.0.0.1:0", "--credential.public_key_file=" + publicKeyFile,
		"--credential.private_key_files=[" + privateKeyFile + "]",
	},
		[]string{"MEDIATED_MAILBOX_DATABASE__NAME=mailbox"}, &out, io.Discard)
	if err == nil || !strings.HasPrefix(err.Error(), "building the endpoint /ui/api/: checking the database schema as mediated_mailbox_app: ") {
		t.Fatalf("the run returned %v, want the schema check refused", err)
	}
	want := []string{
		`level=INFO msg=configuration path=attention_backlog_share source=default value=5`,
		`level=INFO msg=configuration path=attention_gap_days source=default value=7`,
		`level=INFO msg=configuration path=attention_mask_count source=default value=20`,
		`level=INFO msg=configuration path=attention_serve_factor source=default value=2`,
		`level=INFO msg=configuration path=consent_redirect source=default value=http://127.0.0.1:47823/`,
		`level=INFO msg=configuration path=credential.private_key_files source="flag --credential.private_key_files" value=[` + privateKeyFile + `]`,
		`level=INFO msg=configuration path=credential.public_key_file source="flag --credential.public_key_file" value=` + publicKeyFile,
		`level=INFO msg=configuration path=database.host source="flag --database.host" value=db.example`,
		`level=INFO msg=configuration path=database.name source="environment variable MEDIATED_MAILBOX_DATABASE__NAME" value=mailbox`,
		`level=INFO msg=configuration path=database.password_file source="flag --database.password_file" value=` + passwordFile,
		`level=INFO msg=configuration path=database.pool_size source=default value=` + strconv.Itoa(max(4, runtime.NumCPU())),
		`level=INFO msg=configuration path=database.port source=default value=5432`,
		`level=INFO msg=configuration path=database.sslmode source=default value=verify-full`,
		`level=INFO msg=configuration path=database.sslrootcert source=default value=""`,
		`level=INFO msg=configuration path=database.user source="flag --database.user" value=mediated_mailbox_app`,
		`level=INFO msg=configuration path=default_theme source=default value=system`,
		`level=INFO msg=configuration path=heuristics_interval source=default value=24h0m0s`,
		`level=INFO msg=configuration path=identity_header source=default value=""`,
		`level=INFO msg=configuration path=listen source="flag --listen" value=127.0.0.1:0`,
		`level=INFO msg=configuration path=log_level source=default value=info`,
		`level=INFO msg=configuration path=operator_name source=default value=operator`,
		`level=INFO msg=configuration path=probe_listen source="flag --probe_listen" value=127.0.0.1:0`,
		`level=INFO msg=configuration path=stream_interval source=default value=2s`,
		`level=INFO msg=configuration path=stream_poll_interval source=default value=5s`,
		`level=INFO msg=configuration path=stream_reconnect_max source=default value=30s`,
		`level=INFO msg=configuration path=sync_interval source=default value=5m0s`,
		`level=INFO msg=configuration path=tls_cert source="flag --tls_cert" value=/absent/tls.crt`,
		`level=INFO msg=configuration path=tls_key source="flag --tls_key" value=/absent/tls.key`,
		`level=INFO msg=configuration path=token_key_file source=default value=""`,
		`level=INFO msg="sealing to the public key" component=ui key_id=` + keyID,
	}
	got := records(t, out.String())
	if len(got) == 0 || got[len(got)-1] != "level=ERROR msg=\"the process failed\"" {
		t.Fatalf("the run's last record is not its failure: %v", got)
	}
	if diff := cmp.Diff(want, got[:len(got)-1], compare.Options); diff != "" {
		t.Errorf("log (-want +got):\n%s", diff)
	}
	if strings.Contains(out.String(), "the-secret-itself") {
		t.Errorf("the log holds the password:\n%s", out.String())
	}
}

// records renders each JSON record in logs as level, message, then the attributes the test reads, in
// the text handler's form, leaving out the time.
func records(t *testing.T, logs string) []string {
	t.Helper()
	var got []string
	for line := range strings.Lines(logs) {
		var r struct {
			Level, Msg, Path, Source, Component string
			Value                               any
			KeyID                               string `json:"key_id"`
		}
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("standard output holds a line that is not a JSON record: %q", line)
		}
		text := "level=" + r.Level + " msg=" + quoted(r.Msg)
		switch r.Msg {
		case "configuration":
			text += " path=" + r.Path + " source=" + quoted(r.Source) + " value=" + quoted(fmt.Sprint(r.Value))
		case "sealing to the public key":
			text += " component=" + r.Component + " key_id=" + r.KeyID
		}
		got = append(got, text)
	}
	return got
}

// quoted quotes s as the text handler does, when it holds a space or a quote or is empty.
func quoted(s string) string {
	if s == "" || strings.ContainsAny(s, " \"=") {
		return strconv.Quote(s)
	}
	return s
}

// writeKeys writes a new key pair's files and returns their paths and the key identifier, the first
// 16 bytes of SHA-256 over the credential code's domain string and the public key, in hexadecimal,
// computed here rather than by the credential code (credential/README.md).
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
			"--database.host=db", "--database.name=mailbox", "--database.user=mediated_mailbox_app", "--database.password_file=/absent", "--tls_cert=/tls/cert",
			"--tls_key=/tls/key", "--credential.public_key_file=/absent", "--credential.private_key_files=[/absent]", "--consent_redirect=" + value,
		}
		err := run(t.Context(), args, nil, io.Discard, io.Discard)
		want := "validating the configuration: consent_redirect is not an http address on a loopback IP literal with an explicit port"
		if err == nil || err.Error() != want {
			t.Errorf("%s: the run returned %v, want %q", value, err, want)
		}
	}
}

// Every endpoint the spec lists sits under the UI's root, /ui/, apart from the one redirect at /, so
// the product's top-level words are its surfaces (ADR-0137, docs/UI.md section 5). The read API's own
// test holds every route it registers under its endpoint.
func TestEveryEndpointSitsUnderTheUIsRoot(t *testing.T) {
	redirects := 0
	for _, e := range Spec().Endpoints {
		switch {
		case e.Pattern == "/{$}":
			redirects++
		case !strings.HasPrefix(e.Pattern, "/ui/"):
			t.Errorf("the spec lists the endpoint %s, outside the UI's root /ui/", e.Pattern)
		}
	}
	if redirects != 1 {
		t.Errorf("the spec lists / %d times, want once, as the redirect to /ui/", redirects)
	}
}
