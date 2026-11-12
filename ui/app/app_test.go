package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/process/host"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// The UI's root configuration type and its own section are pinned field by field, so a new value is a
// visible change (ADR-0078). The database section's own type is pinned in its package, and the
// process's sections in theirs.
func TestTheConfigurationTypeIsPinned(t *testing.T) {
	mustnotcompile.RequireFields(t, "github.com/ppat/mediated-mailbox-mcp/ui/app", "Configuration",
		"Process host.Process",
		"Operations host.Operations", "Serving host.Serving", "Section Section", "Database core.Config", "Credential core.Config")
	mustnotcompile.RequireFields(t, "github.com/ppat/mediated-mailbox-mcp/ui/app", "Section",
		"SyncInterval time.Duration", "HeuristicsInterval time.Duration", "StreamInterval time.Duration",
		"DefaultTheme string", "StreamReconnectMax time.Duration", "StreamPollInterval time.Duration",
		"AttentionBacklogShare float64", "AttentionMaskCount int64", "AttentionServeFactor float64", "AttentionGapDays int64",
		"TokenKeyFile string", "ConsentRedirect string", "IdentityHeader string", "OperatorName string")
}

func discard() *slog.Logger { return slog.New(slog.DiscardHandler) }

// The UI's defaults are the process's, its two listeners and the info level, the port, its own runtime
// role, the TLS mode that fails closed, the driver's own pool size, the larger of 4 and the processor
// count, and the records' intervals, and the database's host, name and password file are required
// (docs/UI.md section 18.1).
func TestTheDefaults(t *testing.T) {
	want := Configuration{
		Process:    host.Process{LogLevel: "info"},
		Operations: host.Operations{ProbeListen: ":8080"},
		Serving:    host.Serving{Listen: ":8443"},
		Database:   dbconnectcore.Config{Port: 5432, User: "mediated_mailbox_ui", SSLMode: "verify-full", PoolSize: max(4, runtime.NumCPU())},
		Section: Section{
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
		},
	}
	if diff := cmp.Diff(want, defaults(), compare.Options); diff != "" {
		t.Errorf("defaults (-want +got):\n%s", diff)
	}
	err := Run(t.Context(), []string{"--database.name=mailbox", "--database.password_file=/absent"}, nil, io.Discard, io.Discard, discard(), new(slog.LevelVar))
	wantErr := "loading the configuration: database.host is required, and neither the file, MEDIATED_MAILBOX_DATABASE__HOST nor --database.host sets it"
	if err == nil || err.Error() != wantErr {
		t.Errorf("Run returned %v, want %q", err, wantErr)
	}
}

// The database section's pool size is read from any layer and validated before anything else
// starts, so a pool of no connections refuses the start (ADR-0078, VERIFICATIONS, the row for a
// configuration mistake refused).
func TestAPoolSizeOfZeroRefusesTheStart(t *testing.T) {
	args := []string{
		"--database.host=db", "--database.name=mailbox", "--database.password_file=/absent", "--database.pool_size=0",
		"--credential.public_key_file=/absent", "--credential.private_key_files=[/absent]",
	}
	err := Run(t.Context(), args, nil, io.Discard, io.Discard, discard(), new(slog.LevelVar))
	want := "validating the configuration: database.pool_size 0 is not positive"
	if err == nil || err.Error() != want {
		t.Errorf("Run returned %v, want %q", err, want)
	}
}

// The credential section is validated as the API server's and the worker's is, so an empty list of
// private keys refuses the start before any key file is read (ADR-0078, VERIFICATIONS, the row for a
// configuration mistake refused).
func TestAnEmptyListOfPrivateKeysRefusesTheStart(t *testing.T) {
	args := []string{
		"--database.host=db", "--database.name=mailbox", "--database.password_file=/absent", "--tls_cert=/tls/cert",
		"--tls_key=/tls/key", "--credential.public_key_file=/absent", "--credential.private_key_files=[]",
	}
	err := Run(t.Context(), args, nil, io.Discard, io.Discard, discard(), new(slog.LevelVar))
	want := "validating the configuration: credential.private_key_files names no file"
	if err == nil || err.Error() != want {
		t.Errorf("Run returned %v, want %q", err, want)
	}
}

// The start logs every effective value with the layer that set it, the password file's path and
// never its contents, and the identifier of the key it seals to (ADR-0092, VERIFICATIONS, the row for
// the UI's log of its key). It stops at the TLS key pair, after the log and before it serves.
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
		"--listen=127.0.0.1:0", "--probe_listen=127.0.0.1:0", "--credential.public_key_file=" + publicKeyFile,
		"--credential.private_key_files=[" + privateKeyFile + "]",
	},
		[]string{"MEDIATED_MAILBOX_DATABASE__NAME=mailbox"}, io.Discard, io.Discard, logger, new(slog.LevelVar))
	if err == nil || !strings.HasPrefix(err.Error(), "loading the TLS key pair: ") {
		t.Fatalf("Run returned %v, want the key pair refused", err)
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
		`level=INFO msg=configuration path=database.user source=default value=mediated_mailbox_ui`,
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
			"--tls_key=/tls/key", "--credential.public_key_file=/absent", "--credential.private_key_files=[/absent]", "--consent_redirect=" + value,
		}
		err := Run(t.Context(), args, nil, io.Discard, io.Discard, discard(), new(slog.LevelVar))
		want := "validating the configuration: consent_redirect is not an http address on a loopback IP literal with an explicit port"
		if err == nil || err.Error() != want {
			t.Errorf("%s: Run returned %v, want %q", value, err, want)
		}
	}
}
