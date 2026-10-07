package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/policyload"
	"github.com/ppat/mediated-mailbox-mcp/settings"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// Delta sync's root configuration type is pinned field by field, so a new value is a visible change
// (ADR-0078). Each section's own type is pinned in its package.
func TestTheConfigurationTypeIsPinned(t *testing.T) {
	mustnotcompile.RequireFields(t, "github.com/ppat/mediated-mailbox-mcp/sync/app", "Configuration",
		"ProbeListen string",
		"SyncInterval time.Duration",
		"FirstWindow time.Duration",
		"DecisionsPerTick int",
		"Database core.Config",
		"Credential core.Config",
		"Scanner scan.Config",
	)
}

func discard() *slog.Logger { return slog.New(slog.DiscardHandler) }

// Delta sync's defaults are ADR-0018's five-minute interval, a first window of seven days, two hundred
// decisions a tick, the port, its own runtime role and the TLS mode that fails closed, and the host,
// the database name and the password file are required (ADR-0103, ADR-0104, ADR-0105).
func TestTheDefaults(t *testing.T) {
	want := Configuration{
		ProbeListen:      ":8080",
		SyncInterval:     5 * time.Minute,
		FirstWindow:      7 * 24 * time.Hour,
		DecisionsPerTick: 200,
		Database:         dbconnectcore.Config{Port: 5432, User: "mediated_mailbox_sync", SSLMode: "verify-full"},
		Scanner:          scan.DefaultConfig(),
	}
	if diff := cmp.Diff(want, defaults(), compare.Options); diff != "" {
		t.Errorf("defaults (-want +got):\n%s", diff)
	}
	err := Run(t.Context(), nil, nil, discard())
	wantErr := "loading the configuration: database.host is required, and neither the file, MEDIATED_MAILBOX_DATABASE__HOST nor --database.host sets it"
	if err == nil || err.Error() != wantErr {
		t.Errorf("Run returned %v, want %q", err, wantErr)
	}
}

// A password variable refuses the start before any configuration is read.
func TestAPasswordVariableRefusesTheStart(t *testing.T) {
	err := Run(t.Context(), nil, []string{"PGPASSWORD="}, discard())
	want := "the environment sets PGPASSWORD, and the database password comes only from the mounted password file"
	if err == nil || err.Error() != want {
		t.Errorf("Run returned %v, want %q", err, want)
	}
}

// absentKeys names key files that do not exist, for a start meant to stop before the keyring or at it.
var absentKeys = []string{"--credential.public_key_file=/absent/public", "--credential.private_key_files=[/absent/private]"}

// database names a database whose password file does not exist, so a start that gets past the keyring
// stops at the password file, before any connection is made.
var database = []string{"--database.host=db.example", "--database.name=mailbox", "--database.password_file=/absent/password"}

// The interval, the first window and the bound on a tick's decisions must be positive, and the start
// refuses one that is not before it reads a key file.
func TestATickThatCannotRunRefusesTheStart(t *testing.T) {
	for _, c := range []struct{ arg, want string }{
		{"--sync_interval=0s", "validating the configuration: sync_interval 0s is not positive"},
		{"--first_window=-1h", "validating the configuration: first_window -1h0m0s is not positive"},
		{"--decisions_per_tick=0", "validating the configuration: decisions_per_tick 0 is not positive"},
	} {
		t.Run(c.arg, func(t *testing.T) {
			err := Run(t.Context(), append(slices.Clone(database), append(slices.Clone(absentKeys), c.arg)...), nil, discard())
			if err == nil || err.Error() != c.want {
				t.Errorf("Run returned %v, want %q", err, c.want)
			}
		})
	}
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
	err := Run(t.Context(), append([]string{"--database.host=db.example", "--database.password_file=" + passwordFile, "--sync_interval=2m"}, absentKeys...),
		[]string{"MEDIATED_MAILBOX_DATABASE__NAME=mailbox"}, logger)
	if err == nil || !strings.HasPrefix(err.Error(), "loading the keyring: ") {
		t.Fatalf("Run returned %v, want the keyring's refusal", err)
	}
	want := []string{
		`level=INFO msg=configuration path=credential.private_key_files source="flag --credential.private_key_files" value=[/absent/private]`,
		`level=INFO msg=configuration path=credential.public_key_file source="flag --credential.public_key_file" value=/absent/public`,
		`level=INFO msg=configuration path=database.host source="flag --database.host" value=db.example`,
		`level=INFO msg=configuration path=database.name source="environment variable MEDIATED_MAILBOX_DATABASE__NAME" value=mailbox`,
		`level=INFO msg=configuration path=database.password_file source="flag --database.password_file" value=` + passwordFile,
		`level=INFO msg=configuration path=database.port source=default value=5432`,
		`level=INFO msg=configuration path=database.sslmode source=default value=verify-full`,
		`level=INFO msg=configuration path=database.sslrootcert source=default value=""`,
		`level=INFO msg=configuration path=database.user source=default value=mediated_mailbox_sync`,
		`level=INFO msg=configuration path=decisions_per_tick source=default value=200`,
		`level=INFO msg=configuration path=first_window source=default value=168h0m0s`,
		`level=INFO msg=configuration path=probe_listen source=default value=:8080`,
		`level=INFO msg=configuration path=scanner.window source=default value=8`,
		`level=INFO msg=configuration path=sync_interval source="flag --sync_interval" value=2m0s`,
	}
	var got []string
	for line := range strings.Lines(strings.TrimSpace(out.String())) {
		line = strings.TrimSuffix(line, "\n")
		if !strings.Contains(line, " path=scanner.") || strings.Contains(line, " path=scanner.window ") {
			got = append(got, line)
		}
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("log (-want +got):\n%s", diff)
	}
	if strings.Contains(out.String(), "the-secret-itself") {
		t.Errorf("the log holds the password:\n%s", out.String())
	}
}

// D4's part of VERIFICATIONS' row for a credential supplied outside the account's state row. No
// configuration value holds an account or a credential, so an argument, a flag or an environment
// variable naming one refuses the start rather than being read (ADR-0080).
func TestACredentialInTheConfigurationRefusesTheStart(t *testing.T) {
	for _, c := range []struct {
		name    string
		args    []string
		environ []string
	}{
		{"an account argument", []string{"one@example.com"}, nil},
		{"a flag", []string{"--accounts.personal.refresh_token=a-token"}, nil},
		{"an environment variable", nil, []string{"MEDIATED_MAILBOX_ACCOUNTS__PERSONAL__REFRESH_TOKEN=a-token"}},
		{"a credential flag", []string{"--credential.refresh_token=a-token"}, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			err := Run(t.Context(), append(slices.Clone(database), append(slices.Clone(absentKeys), c.args...)...), c.environ, discard())
			if err == nil || !strings.HasPrefix(err.Error(), "loading the configuration: ") {
				t.Errorf("Run returned %v, want the configuration library's refusal", err)
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

// D4's part of VERIFICATIONS' row for a public key matching none of the private keys. The start loads
// the public key and every private key, refuses it when the public key matches none of them, and goes
// on when it matches any one, the second of two during a key replacement included (ADR-0088,
// ADR-0092).
func TestThePublicKeyMustMatchAPrivateKey(t *testing.T) {
	dir := t.TempDir()
	oldPrivate, oldPublic := keyFiles(t, dir, "old")
	newPrivate, newPublic := keyFiles(t, dir, "new")
	_, otherPublic := keyFiles(t, dir, "other")
	start := func(public string, privates ...string) error {
		args := append(slices.Clone(database), "--credential.public_key_file="+public, "--credential.private_key_files=["+strings.Join(privates, ", ")+"]")
		return Run(t.Context(), args, nil, discard())
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

// The scanner's section is validated by the scanner itself before any key file is read, so a value
// outside its designed range refuses the start (ADR-0078, ADR-0005).
func TestAnInvalidScannerSectionRefusesTheStart(t *testing.T) {
	err := Run(t.Context(), append(slices.Clone(database), append(slices.Clone(absentKeys), "--scanner.window=0")...), nil, discard())
	if err == nil || !strings.HasPrefix(err.Error(), "validating the configuration: scanner: ") {
		t.Errorf("Run returned %v, want the scanner's refusal", err)
	}
}

// The revision a verdict and a mask record follows the scanner's effective configuration, whichever
// layer changed it, and a change to another section, delta sync's own values included, leaves it as
// it was, so delta sync records the revision backfill records for the same section (ADR-0078,
// ADR-0096).
func TestAVerdictsRevisionFollowsTheScannerSection(t *testing.T) {
	revision := func(t *testing.T, args []string) string {
		t.Helper()
		loaded, err := settings.Load(defaults(), slices.Concat(database, absentKeys, args), nil)
		if err != nil {
			t.Fatal(err)
		}
		s, err := buildScanner(loaded)
		if err != nil {
			t.Fatal(err)
		}
		return s.Scan("Your code is 419283").Revision()
	}
	base := revision(t, nil)
	for _, c := range []struct {
		args    []string
		changed bool
	}{
		{[]string{"--scanner.window=9"}, true},
		{[]string{"--scanner.window=8"}, false},
		{[]string{"--sync_interval=1m"}, false},
		{[]string{"--decisions_per_tick=5"}, false},
	} {
		t.Run(fmt.Sprint(c.args), func(t *testing.T) {
			if got := revision(t, c.args); (got != base) != c.changed {
				t.Errorf("the revision is %q against the default's %q, want it changed %v", got, base, c.changed)
			}
		})
	}
}

// probe returns the probes' answer on path.
func probe(t *testing.T, address, path string) (int, string) {
	t.Helper()
	res, err := http.Get((&url.URL{Scheme: "http", Host: address, Path: path}).String())
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(res.Body)
	if err := errors.Join(err, res.Body.Close()); err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, string(body)
}

// unreachable returns an HTTP client whose every request goes through a proxy nothing listens on, so a
// token refresh or a provider request fails without reaching Google.
func unreachable(t *testing.T) *http.Client {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	proxy := &url.URL{Scheme: "http", Host: ln.Addr().String()}
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxy)}}
}

// held is a token source holding an access token, so the adapter builds and counts a request
// without a token refresh.
type held string

func (h held) AccessToken(context.Context) (string, error) { return string(h), nil }

// D4's row for the gate and the scanner delta sync shares with backfill. A tick decides under
// thresholds written out here, which backfill's own test holds backfill to as well, and scans with
// the scanner its default section builds, whose version and revision are written out here and in
// backfill's test, so a workload that differs from the other fails its own test (ADR-0096, ADR-0098,
// ADR-0104).
func TestATickDecidesUnderBackfillsThresholdsAndScanner(t *testing.T) {
	loaded, err := settings.Load(defaults(), slices.Concat(database, absentKeys), nil)
	if err != nil {
		t.Fatal(err)
	}
	scanner, err := buildScanner(loaded)
	if err != nil {
		t.Fatal(err)
	}
	s, err := newSyncer(nil, nil, scanner, loaded.Config, prometheus.NewRegistry(), discard(), nil)
	if err != nil {
		t.Fatal(err)
	}

	deps := s.tickDeps("personal", nil, policyload.Snapshot{})

	want := scangate.Config{
		NoReplyLocalParts: []string{"noreply", "no-reply", "security", "accounts", "verify", "auth", "support"},
		SmallBytes:        30 * 1024,
		RecentAgeMillis:   24 * 60 * 60 * 1000,
		LowVolume:         20,
		HighVolume:        500,
	}
	if diff := cmp.Diff(want, deps.Gate, compare.Options); diff != "" {
		t.Errorf("the gate's thresholds (-backfill's +delta sync's):\n%s", diff)
	}
	v := deps.Scanner.Scan("Your code is 419283")
	const backfills = "version 1 revision c9fa3ff13948e4f89ac094b91a27455c"
	if got := fmt.Sprintf("version %d revision %s", v.Version(), v.Revision()); got != backfills {
		t.Errorf("the scanner records %s, want backfill's %s", got, backfills)
	}
}
