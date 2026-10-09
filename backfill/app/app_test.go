package app

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/process/settings"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// Backfill's root configuration type is pinned field by field, so a new section is a visible
// change (ADR-0078). Each section's own type is pinned in its package.
func TestTheConfigurationTypeIsPinned(t *testing.T) {
	mustnotcompile.RequireFields(t, "github.com/ppat/mediated-mailbox-mcp/backfill/app", "Configuration",
		"ProbeListen string",
		"Database core.Config",
		"Credential core.Config",
		"Scanner scan.Config",
		"LogLevel string",
	)
}

func discard() *slog.Logger { return slog.New(slog.DiscardHandler) }

// Backfill's defaults are the port, its own runtime role and the TLS mode that fails closed, and the
// host, the database name and the password file are required.
func TestTheDefaults(t *testing.T) {
	want := Configuration{
		ProbeListen: ":8080",
		Database:    dbconnectcore.Config{Port: 5432, User: "mediated_mailbox_backfill", SSLMode: "verify-full"},
		Scanner:     scan.DefaultConfig(),
		LogLevel:    "info",
	}
	if diff := cmp.Diff(want, defaults(), compare.Options); diff != "" {
		t.Errorf("defaults (-want +got):\n%s", diff)
	}
	err := Run(t.Context(), nil, nil, discard(), new(slog.LevelVar))
	wantErr := "loading the configuration: database.host is required, and neither the file, MEDIATED_MAILBOX_DATABASE__HOST nor --database.host sets it"
	if err == nil || err.Error() != wantErr {
		t.Errorf("Run returned %v, want %q", err, wantErr)
	}
}

// A password variable refuses the start before any configuration is read.
func TestAPasswordVariableRefusesTheStart(t *testing.T) {
	err := Run(t.Context(), nil, []string{"PGPASSWORD="}, discard(), new(slog.LevelVar))
	want := "the environment sets PGPASSWORD, and the database password comes only from the mounted password file"
	if err == nil || err.Error() != want {
		t.Errorf("Run returned %v, want %q", err, want)
	}
}

// The start validates the merged configuration before it configures the connection.
func TestAnEmptyHostRefusesTheStart(t *testing.T) {
	err := Run(t.Context(), append([]string{"--database.host=", "--database.name=mailbox", "--database.password_file=/absent"}, absentKeys...), nil, discard(), new(slog.LevelVar))
	want := "validating the configuration: database.host is empty"
	if err == nil || err.Error() != want {
		t.Errorf("Run returned %v, want %q", err, want)
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
	// The key files do not exist, so the start stops at the keyring, after the log and before any
	// connection is made.
	err := Run(t.Context(), append([]string{"--database.host=db.example", "--database.password_file=" + passwordFile}, absentKeys...),
		[]string{"MEDIATED_MAILBOX_DATABASE__NAME=mailbox"}, logger, new(slog.LevelVar))
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
		`level=INFO msg=configuration path=database.user source=default value=mediated_mailbox_backfill`,
		`level=INFO msg=configuration path=log_level source=default value=info`,
		`level=INFO msg=configuration path=probe_listen source=default value=:8080`,
		`level=INFO msg=configuration path=scanner.window source=default value=8`,
	}
	// Of the scanner's section, whose every value is logged the same way, the window stands for the
	// rest.
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

// absentKeys names key files that do not exist, for a start meant to stop before the keyring or at it.
var absentKeys = []string{"--credential.public_key_file=/absent/public", "--credential.private_key_files=[/absent/private]"}

// database names a database whose password file does not exist, so a start that gets past the keyring
// stops at the password file, before any connection is made.
var database = []string{"--database.host=db.example", "--database.name=mailbox", "--database.password_file=/absent/password"}

// The start validates the credential section before it reads a key file.
func TestAnEmptyListOfPrivateKeysRefusesTheStart(t *testing.T) {
	err := Run(t.Context(), append(slices.Clone(database), "--credential.public_key_file=/absent/public", "--credential.private_key_files=[]"), nil, discard(), new(slog.LevelVar))
	want := "validating the configuration: credential.private_key_files names no file"
	if err == nil || err.Error() != want {
		t.Errorf("Run returned %v, want %q", err, want)
	}
}

// An account is not an argument. An argument that is not a flag refuses the start, an account and a
// flag written without its dashes alike, so no account can come from the command line (ADR-0078,
// ADR-0080).
func TestAnAccountArgumentRefusesTheStart(t *testing.T) {
	for _, arg := range []string{"one@example.com", "database.port=5433"} {
		err := Run(t.Context(), append(slices.Clone(database), arg), nil, discard(), new(slog.LevelVar))
		want := fmt.Sprintf("loading the configuration: argument %q: it is not a flag, and flags are written --path=VALUE", arg)
		if err == nil || err.Error() != want {
			t.Errorf("Run returned %v, want %q", err, want)
		}
	}
}

// D1's part of VERIFICATIONS' row for a credential supplied outside the account's state row. No
// configuration value holds an account or a credential, so a flag or an environment variable naming
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
			err := Run(t.Context(), append(slices.Clone(database), append(slices.Clone(absentKeys), c.args...)...), c.environ, discard(), new(slog.LevelVar))
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

// D1's part of VERIFICATIONS' row for a public key matching none of the private keys. The start
// loads the public key and every private key, refuses it when the public key matches none of them,
// and goes on when it matches any one, the second of two during a key replacement included
// (ADR-0088, ADR-0092).
func TestThePublicKeyMustMatchAPrivateKey(t *testing.T) {
	dir := t.TempDir()
	oldPrivate, oldPublic := keyFiles(t, dir, "old")
	newPrivate, newPublic := keyFiles(t, dir, "new")
	_, otherPublic := keyFiles(t, dir, "other")
	start := func(public string, privates ...string) error {
		args := append(slices.Clone(database), "--credential.public_key_file="+public, "--credential.private_key_files=["+strings.Join(privates, ", ")+"]")
		return Run(t.Context(), args, nil, discard(), new(slog.LevelVar))
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
			case c.refused && !strings.HasPrefix(err.Error(), "loading the keyring: ") || c.refused && !strings.Contains(err.Error(), "matches none of the private keys"):
				t.Errorf("Run returned %v, want the keyring's refusal", err)
			case !c.refused && !strings.HasPrefix(err.Error(), pastTheKeyring):
				t.Errorf("Run returned %v, want it to pass the keyring and stop at the password file", err)
			}
		})
	}
}

// D1's part of VERIFICATIONS' row for a configuration mistake. The scanner's section is validated by
// the scanner itself before any key file is read, so a value outside its designed range refuses the
// start (ADR-0078, ADR-0005).
func TestAnInvalidScannerSectionRefusesTheStart(t *testing.T) {
	for _, c := range []struct{ arg, want string }{
		{"--scanner.window=0", "the window, the dense length and the dense entropy must be positive"},
		{"--scanner.subject_threshold=0.9", "the subject threshold is above the body threshold"},
		{"--scanner.link_words=[]", "no link word or no link parameter is configured"},
	} {
		t.Run(c.arg, func(t *testing.T) {
			err := Run(t.Context(), append(slices.Clone(database), append(slices.Clone(absentKeys), c.arg)...), nil, discard(), new(slog.LevelVar))
			if err == nil || !strings.HasPrefix(err.Error(), "validating the configuration: scanner: ") || !strings.Contains(err.Error(), c.want) {
				t.Errorf("Run returned %v, want the scanner's refusal containing %q", err, c.want)
			}
		})
	}
}

// D1's part of VERIFICATIONS' row for a section's revision. The revision a verdict records follows the
// scanner's effective configuration, whichever layer changed it, and a change to another section
// leaves it as it was (ADR-0078, ADR-0005).
func TestAVerdictsRevisionFollowsTheScannerSection(t *testing.T) {
	file := filepath.Join(t.TempDir(), "backfill.yaml")
	if err := os.WriteFile(file, []byte("scanner:\n  window: 9\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	revision := func(t *testing.T, args, environ []string) string {
		t.Helper()
		loaded, err := settings.Load(defaults(), slices.Concat(database, absentKeys, args), environ)
		if err != nil {
			t.Fatal(err)
		}
		s, err := buildScanner(loaded)
		if err != nil {
			t.Fatal(err)
		}
		return s.Scan("Your code is 419283").Revision()
	}
	base := revision(t, nil, nil)
	if base == "" {
		t.Fatal("the default configuration's verdict records no revision")
	}
	for _, c := range []struct {
		name    string
		args    []string
		environ []string
		changed bool
	}{
		{"a flag changing the scanner", []string{"--scanner.window=9"}, nil, true},
		{"an environment variable changing the scanner", nil, []string{"MEDIATED_MAILBOX_SCANNER__TRIGGERS__DE=[code]"}, true},
		{"the file changing the scanner", []string{"--config-file=" + file}, nil, true},
		{"a flag restating the scanner's default", []string{"--scanner.window=8"}, nil, false},
		{"a flag changing another section", []string{"--database.port=5433"}, nil, false},
		{"a flag changing the probes", []string{"--probe_listen=:9090"}, nil, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := revision(t, c.args, c.environ); (got != base) != c.changed {
				t.Errorf("the revision is %q against the default's %q, want it changed %v", got, base, c.changed)
			}
		})
	}
}

// D2's part of D4's row for the gate and the scanner delta sync shares with backfill. The second pass
// decides under thresholds written out here, which delta sync's own test holds delta sync to as
// well, and scans with the scanner backfill's default section builds, whose version and revision
// are written out here and in delta sync's test, so a workload that differs from the other fails its
// own test (ADR-0120, ADR-0098, ADR-0104).
func TestARunDecidesUnderTheSharedThresholdsAndScanner(t *testing.T) {
	loaded, err := settings.Load(defaults(), slices.Concat(database, absentKeys), nil)
	if err != nil {
		t.Fatal(err)
	}
	scanner, err := buildScanner(loaded)
	if err != nil {
		t.Fatal(err)
	}

	deps := secondDeps(nil, nil, policy.Composed{}, classify.Lookups{}, scanner)

	want := scangate.Config{
		NoReplyLocalParts: []string{"noreply", "no-reply", "security", "accounts", "verify", "auth", "support"},
		SmallBytes:        30 * 1024,
		RecentAgeMillis:   24 * 60 * 60 * 1000,
		LowVolume:         20,
		HighVolume:        500,
	}
	if diff := cmp.Diff(want, deps.Gate, compare.Options); diff != "" {
		t.Errorf("the gate's thresholds (-delta sync's +backfill's):\n%s", diff)
	}
	v := deps.Scanner.Scan("Your code is 419283")
	const syncs = "version 1 revision c9fa3ff13948e4f89ac094b91a27455c"
	if got := fmt.Sprintf("version %d revision %s", v.Version(), v.Revision()); got != syncs {
		t.Errorf("the scanner records %s, want delta sync's %s", got, syncs)
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
		{name: "no level", wantErr: "loading the keyring: ", wantLine: logLevelFrom("default", "info"), wantInfo: true},
		{name: "info", args: []string{"--log_level=info"}, wantErr: "loading the keyring: ", wantLine: logLevelFrom("flag --log_level", "info"), wantInfo: true},
		{name: "warn", args: []string{"--log_level=warn"}, wantErr: "loading the keyring: ", wantLine: logLevelFrom("flag --log_level", "warn")},
		{name: "error, from the environment", environ: []string{"MEDIATED_MAILBOX_LOG_LEVEL=error"}, wantErr: "loading the keyring: ", wantLine: logLevelFrom("environment variable MEDIATED_MAILBOX_LOG_LEVEL", "error")},
		{name: "a name in upper case", args: []string{"--log_level=INFO"}, wantErr: refused("INFO"), wantLine: logLevelFrom("flag --log_level", "INFO"), wantInfo: true},
		{name: "an offset", args: []string{"--log_level=info+2"}, wantErr: refused("info+2"), wantLine: logLevelFrom("flag --log_level", "info+2"), wantInfo: true},
		{name: "warning", environ: []string{"MEDIATED_MAILBOX_LOG_LEVEL=warning"}, wantErr: refused("warning"), wantLine: logLevelFrom("environment variable MEDIATED_MAILBOX_LOG_LEVEL", "warning"), wantInfo: true},
		{name: "a refused value at warn", args: append([]string{"--log_level=warn"}, []string{"--scanner.window=0"}...), wantErr: "validating the configuration: scanner: ", wantLine: `"msg":"configuration","path":"scanner.window","source":"flag --scanner.window","value":0`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var out bytes.Buffer
			level := new(slog.LevelVar)
			logger := slog.New(slog.NewJSONHandler(&out, &slog.HandlerOptions{Level: level}))
			err := Run(t.Context(), append(append(slices.Clone(database), absentKeys...), c.args...), c.environ, logger, level)
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
