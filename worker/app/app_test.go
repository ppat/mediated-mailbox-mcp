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
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/process/settings"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// The worker's root configuration type and each of its own section types are pinned field by field,
// so a new value is a visible change (ADR-0078). Each shared section's own type is pinned in its
// package.
func TestTheConfigurationTypeIsPinned(t *testing.T) {
	const pkg = "github.com/ppat/mediated-mailbox-mcp/worker/app"
	mustnotcompile.RequireFields(t, pkg, "Configuration",
		"ProbeListen string",
		"ReloadInterval time.Duration",
		"Database Database",
		"Credential core.Config",
		"Scanner scan.Config",
		"LogLevel string",
		"Backfill Backfill",
		"Sync Sync",
	)
	mustnotcompile.RequireFields(t, pkg, "Database",
		"Host string",
		"Port int",
		"Name string",
		"SSLMode string",
		"SSLRootCert string",
		"Backfill Role",
		"Sync Role",
	)
	mustnotcompile.RequireFields(t, pkg, "Role",
		"User string",
		"PasswordFile string",
		"PoolSize int",
	)
	mustnotcompile.RequireFields(t, pkg, "Backfill",
		"Concurrency int",
	)
	mustnotcompile.RequireFields(t, pkg, "Sync",
		"SyncInterval time.Duration",
		"FirstWindow time.Duration",
		"DecisionsPerTick int",
		"Concurrency int",
	)
}

func discard() *slog.Logger { return slog.New(slog.DiscardHandler) }

// The worker's defaults are the port, each job kind's own runtime role with a pool of four, the TLS
// mode that fails closed, a reload every minute, ADR-0018's five-minute sync interval, a first window
// of seven days, two hundred decisions a tick, two accounts backfilled at once and four ticking at
// once. The host, the database name and each role's password file are required (ADR-0103, ADR-0104,
// ADR-0105, ADR-0118, ADR-0119).
func TestTheDefaults(t *testing.T) {
	want := Configuration{
		ProbeListen:    ":8080",
		ReloadInterval: time.Minute,
		Database: Database{
			Port: 5432, SSLMode: "verify-full",
			Backfill: Role{User: "mediated_mailbox_backfill", PoolSize: 4},
			Sync:     Role{User: "mediated_mailbox_sync", PoolSize: 4},
		},
		Scanner:  scan.DefaultConfig(),
		LogLevel: "info",
		Backfill: Backfill{Concurrency: 2},
		Sync:     Sync{SyncInterval: 5 * time.Minute, FirstWindow: 7 * 24 * time.Hour, DecisionsPerTick: 200, Concurrency: 4},
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

// absentKeys names key files that do not exist, for a start meant to stop before the keyring or at it.
var absentKeys = []string{"--credential.public_key_file=/absent/public", "--credential.private_key_files=[/absent/private]"}

// database names a database whose password files do not exist, so a start that gets past the keyring
// stops at the first job kind's password file, before any connection is made.
var database = []string{
	"--database.host=db.example", "--database.name=mailbox",
	"--database.backfill.password_file=/absent/backfill-password", "--database.sync.password_file=/absent/sync-password",
}

// The start validates the merged configuration before it configures any connection.
func TestAnEmptyHostRefusesTheStart(t *testing.T) {
	err := Run(t.Context(), append([]string{
		"--database.host=", "--database.name=mailbox",
		"--database.backfill.password_file=/absent", "--database.sync.password_file=/absent",
	}, absentKeys...), nil, discard(), new(slog.LevelVar))
	want := "validating the configuration: database.host is empty"
	if err == nil || err.Error() != want {
		t.Errorf("Run returned %v, want %q", err, want)
	}
}

// Each job kind's pool, the reload interval, the sync interval, the first window, the bound on a tick's
// decisions and each job kind's concurrency must be positive, and the start refuses one that is not
// before it reads a key file.
func TestATickThatCannotRunRefusesTheStart(t *testing.T) {
	for _, c := range []struct{ arg, want string }{
		{"--sync.sync_interval=0s", "validating the configuration: sync.sync_interval 0s is not positive"},
		{"--sync.first_window=-1h", "validating the configuration: sync.first_window -1h0m0s is not positive"},
		{"--sync.decisions_per_tick=0", "validating the configuration: sync.decisions_per_tick 0 is not positive"},
		{"--sync.concurrency=0", "validating the configuration: sync.concurrency 0 is not positive"},
		{"--backfill.concurrency=0", "validating the configuration: backfill.concurrency 0 is not positive"},
		{"--reload_interval=0s", "validating the configuration: reload_interval 0s is not positive"},
		{"--database.backfill.pool_size=0", "validating the configuration: database.backfill.pool_size 0 is not positive"},
		{"--database.sync.pool_size=0", "validating the configuration: database.sync.pool_size 0 is not positive"},
		{"--database.backfill.user=", "validating the configuration: database.backfill.user is empty"},
		{"--database.sync.user=", "validating the configuration: database.sync.user is empty"},
	} {
		t.Run(c.arg, func(t *testing.T) {
			err := Run(t.Context(), append(slices.Clone(database), append(slices.Clone(absentKeys), c.arg)...), nil, discard(), new(slog.LevelVar))
			if err == nil || err.Error() != c.want {
				t.Errorf("Run returned %v, want %q", err, c.want)
			}
		})
	}
	// A job kind's empty password file is refused by its own path.
	for _, kind := range []string{"backfill", "sync"} {
		t.Run(kind+" password file", func(t *testing.T) {
			args := slices.DeleteFunc(slices.Clone(database), func(a string) bool { return strings.HasPrefix(a, "--database."+kind+".password_file=") })
			err := Run(t.Context(), append(append(args, absentKeys...), "--database."+kind+".password_file="), nil, discard(), new(slog.LevelVar))
			want := "validating the configuration: database." + kind + ".password_file is empty"
			if err == nil || err.Error() != want {
				t.Errorf("Run returned %v, want %q", err, want)
			}
		})
	}
}

// The start logs every effective value with the layer that set it, each password file's path and
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
	err := Run(t.Context(), append([]string{
		"--database.host=db.example", "--database.backfill.password_file=" + passwordFile, "--database.sync.password_file=" + passwordFile,
		"--sync.sync_interval=2m",
	}, absentKeys...), []string{"MEDIATED_MAILBOX_DATABASE__NAME=mailbox"}, logger, new(slog.LevelVar))
	if err == nil || !strings.HasPrefix(err.Error(), "loading the keyring: ") {
		t.Fatalf("Run returned %v, want the keyring's refusal", err)
	}
	want := []string{
		`level=INFO msg=configuration path=backfill.concurrency source=default value=2`,
		`level=INFO msg=configuration path=credential.private_key_files source="flag --credential.private_key_files" value=[/absent/private]`,
		`level=INFO msg=configuration path=credential.public_key_file source="flag --credential.public_key_file" value=/absent/public`,
		`level=INFO msg=configuration path=database.backfill.password_file source="flag --database.backfill.password_file" value=` + passwordFile,
		`level=INFO msg=configuration path=database.backfill.pool_size source=default value=4`,
		`level=INFO msg=configuration path=database.backfill.user source=default value=mediated_mailbox_backfill`,
		`level=INFO msg=configuration path=database.host source="flag --database.host" value=db.example`,
		`level=INFO msg=configuration path=database.name source="environment variable MEDIATED_MAILBOX_DATABASE__NAME" value=mailbox`,
		`level=INFO msg=configuration path=database.port source=default value=5432`,
		`level=INFO msg=configuration path=database.sslmode source=default value=verify-full`,
		`level=INFO msg=configuration path=database.sslrootcert source=default value=""`,
		`level=INFO msg=configuration path=database.sync.password_file source="flag --database.sync.password_file" value=` + passwordFile,
		`level=INFO msg=configuration path=database.sync.pool_size source=default value=4`,
		`level=INFO msg=configuration path=database.sync.user source=default value=mediated_mailbox_sync`,
		`level=INFO msg=configuration path=log_level source=default value=info`,
		`level=INFO msg=configuration path=probe_listen source=default value=:8080`,
		`level=INFO msg=configuration path=reload_interval source=default value=1m0s`,
		`level=INFO msg=configuration path=scanner.window source=default value=8`,
		`level=INFO msg=configuration path=sync.concurrency source=default value=4`,
		`level=INFO msg=configuration path=sync.decisions_per_tick source=default value=200`,
		`level=INFO msg=configuration path=sync.first_window source=default value=168h0m0s`,
		`level=INFO msg=configuration path=sync.sync_interval source="flag --sync.sync_interval" value=2m0s`,
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

// D1's and D4's parts of VERIFICATIONS' row for a credential supplied outside the account's state
// row. No configuration value holds an account or a credential, so an argument, a flag or an
// environment variable naming one refuses the start rather than being read (ADR-0080).
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

// D1's and D4's parts of VERIFICATIONS' row for a public key matching none of the private keys. The
// start loads the public key and every private key, refuses it when the public key matches none of
// them, and goes on when it matches any one, the second of two during a key replacement included
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
	pastTheKeyring := "configuring the database connection of mediated_mailbox_backfill: reading the password file: "
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

// D1's part of VERIFICATIONS' row for a section's revision. The revision a verdict and a mask record
// follows the scanner's effective configuration, whichever layer changed it, and a change to another
// section, either job kind's own values included, leaves it as it was (ADR-0078, ADR-0005, ADR-0120).
func TestAVerdictsRevisionFollowsTheScannerSection(t *testing.T) {
	file := filepath.Join(t.TempDir(), "work.yaml")
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
		{"a flag changing the sync interval", []string{"--sync.sync_interval=1m"}, nil, false},
		{"a flag changing a tick's decisions", []string{"--sync.decisions_per_tick=5"}, nil, false},
		{"a flag changing backfill's concurrency", []string{"--backfill.concurrency=3"}, nil, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := revision(t, c.args, c.environ); (got != base) != c.changed {
				t.Errorf("the revision is %q against the default's %q, want it changed %v", got, base, c.changed)
			}
		})
	}
}

// D2's and D4's part of the row for the scanner backfill and delta sync share. The worker builds one
// scanner from its one scanner section, and each job kind's own test holds its gate to the shared
// thresholds and its scans to the scanner it is given. The default section's scanner records the
// version and revision written out here, which both job kinds' verdicts and masks therefore record
// (ADR-0120, ADR-0121, ADR-0104).
func TestTheScannerSectionBuildsTheSharedScanner(t *testing.T) {
	loaded, err := settings.Load(defaults(), slices.Concat(database, absentKeys), nil)
	if err != nil {
		t.Fatal(err)
	}
	scanner, err := buildScanner(loaded)
	if err != nil {
		t.Fatal(err)
	}
	v := scanner.Scan("Your code is 419283")
	const shared = "version 1 revision c9fa3ff13948e4f89ac094b91a27455c"
	if got := fmt.Sprintf("version %d revision %s", v.Version(), v.Revision()); got != shared {
		t.Errorf("the scanner records %s, want %s", got, shared)
	}
}

// Both job kinds are handed the one scanner the worker built from its scanner section, so delta sync
// records backfill's scanner version and revision and neither reopens the other's work, by
// construction rather than by two processes being given the same section (ADR-0117, ADR-0120).
func TestBothJobKindsScanWithTheOneScanner(t *testing.T) {
	loaded, err := settings.Load(defaults(), slices.Concat(database, absentKeys), nil)
	if err != nil {
		t.Fatal(err)
	}
	scanner, err := buildScanner(loaded)
	if err != nil {
		t.Fatal(err)
	}
	b, d := kinds(Pools{}, nil, scanner, loaded.Config, nil, discard(), prometheus.NewRegistry(), nil)
	for name, got := range map[string]interface {
		Version() int
		Revision() string
	}{"backfill": b.Scanner, "delta sync": d.Scanner} {
		if got.Version() != scanner.Version() || got.Revision() != scanner.Revision() {
			t.Errorf("%s scans with version %d revision %s, want the worker's version %d revision %s",
				name, got.Version(), got.Revision(), scanner.Version(), scanner.Revision())
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
		{name: "no level", wantErr: "loading the keyring: ", wantLine: logLevelFrom("default", "info"), wantInfo: true},
		{name: "info", args: []string{"--log_level=info"}, wantErr: "loading the keyring: ", wantLine: logLevelFrom("flag --log_level", "info"), wantInfo: true},
		{name: "warn", args: []string{"--log_level=warn"}, wantErr: "loading the keyring: ", wantLine: logLevelFrom("flag --log_level", "warn")},
		{name: "error, from the environment", environ: []string{"MEDIATED_MAILBOX_LOG_LEVEL=error"}, wantErr: "loading the keyring: ", wantLine: logLevelFrom("environment variable MEDIATED_MAILBOX_LOG_LEVEL", "error")},
		{name: "a name in upper case", args: []string{"--log_level=INFO"}, wantErr: refused("INFO"), wantLine: logLevelFrom("flag --log_level", "INFO"), wantInfo: true},
		{name: "an offset", args: []string{"--log_level=info+2"}, wantErr: refused("info+2"), wantLine: logLevelFrom("flag --log_level", "info+2"), wantInfo: true},
		{name: "warning", environ: []string{"MEDIATED_MAILBOX_LOG_LEVEL=warning"}, wantErr: refused("warning"), wantLine: logLevelFrom("environment variable MEDIATED_MAILBOX_LOG_LEVEL", "warning"), wantInfo: true},
		{name: "a refused value at warn", args: []string{"--log_level=warn", "--scanner.window=0"}, wantErr: "validating the configuration: scanner: ", wantLine: `"msg":"configuration","path":"scanner.window","source":"flag --scanner.window","value":0`},
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

// Each job kind's pool connects as the kind's own runtime role, and each kind is handed its own pool,
// so backfill's statements run under backfill's grants and delta sync's under delta sync's inside the
// one process (ADR-0118). The pools are opened as the start opens them, and open no connection.
func TestEachJobKindIsHandedThePoolOfItsOwnRole(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"backfill", "sync"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(name+"-password"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := settings.Load(defaults(), slices.Concat([]string{
		"--database.host=db.example", "--database.name=mailbox",
		"--database.backfill.password_file=" + filepath.Join(dir, "backfill"),
		"--database.sync.password_file=" + filepath.Join(dir, "sync"),
	}, absentKeys), nil)
	if err != nil {
		t.Fatal(err)
	}
	d := loaded.Config.Database
	pools, err := openPools(t.Context(), d)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pools.Backfill.Close)
	t.Cleanup(pools.Sync.Close)
	if got := pools.Backfill.Config().ConnConfig.User; got != d.Backfill.User || got != "mediated_mailbox_backfill" {
		t.Errorf("backfill's pool connects as %q, want backfill's role %q", got, d.Backfill.User)
	}
	if got := pools.Sync.Config().ConnConfig.User; got != d.Sync.User || got != "mediated_mailbox_sync" {
		t.Errorf("delta sync's pool connects as %q, want delta sync's role %q", got, d.Sync.User)
	}

	scanner, err := buildScanner(loaded)
	if err != nil {
		t.Fatal(err)
	}
	b, s := kinds(pools, nil, scanner, loaded.Config, nil, discard(), prometheus.NewRegistry(), nil)
	if b.Pool != pools.Backfill {
		t.Error("backfill is handed a pool other than the one of its own role")
	}
	if s.Pool != pools.Sync {
		t.Error("delta sync is handed a pool other than the one of its own role")
	}
}
