package main

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

	"github.com/ppat/mediated-mailbox-mcp/core/scan"

	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/settings"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// Backfill's root configuration type is pinned field by field, so a new section is a visible
// change (ADR-0078). Each section's own type is pinned in its package.
func TestTheConfigurationTypeIsPinned(t *testing.T) {
	mustnotcompile.RequireFields(t, "github.com/ppat/mediated-mailbox-mcp/backfill", "Configuration",
		"ProbeListen string",
		"Database core.Config",
		"Credential core.Config",
		"Scanner scan.Config",
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
	}
	if diff := cmp.Diff(want, defaults(), compare.Options); diff != "" {
		t.Errorf("defaults (-want +got):\n%s", diff)
	}
	err := run(t.Context(), nil, nil, discard())
	wantErr := "loading the configuration: database.host is required, and neither the file, MEDIATED_MAILBOX_DATABASE__HOST nor --database.host sets it"
	if err == nil || err.Error() != wantErr {
		t.Errorf("run returned %v, want %q", err, wantErr)
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

// The start validates the merged configuration before it configures the connection.
func TestAnEmptyHostRefusesTheStart(t *testing.T) {
	err := run(t.Context(), append([]string{"--database.host=", "--database.name=mailbox", "--database.password_file=/absent"}, absentKeys...), nil, discard())
	want := "validating the configuration: database.host is empty"
	if err == nil || err.Error() != want {
		t.Errorf("run returned %v, want %q", err, want)
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
	err := run(t.Context(), append([]string{"--database.host=db.example", "--database.password_file=" + passwordFile}, absentKeys...),
		[]string{"MEDIATED_MAILBOX_DATABASE__NAME=mailbox"}, logger)
	if err == nil || !strings.HasPrefix(err.Error(), "loading the keyring: ") {
		t.Fatalf("run returned %v, want the keyring's refusal", err)
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
	err := run(t.Context(), append(slices.Clone(database), "--credential.public_key_file=/absent/public", "--credential.private_key_files=[]"), nil, discard())
	want := "validating the configuration: credential.private_key_files names no file"
	if err == nil || err.Error() != want {
		t.Errorf("run returned %v, want %q", err, want)
	}
}

// An account is not an argument. An argument that is not a flag refuses the start, an account and a
// flag written without its dashes alike, so no account can come from the command line (ADR-0078,
// ADR-0080).
func TestAnAccountArgumentRefusesTheStart(t *testing.T) {
	for _, arg := range []string{"one@example.com", "database.port=5433"} {
		err := run(t.Context(), append(slices.Clone(database), arg), nil, discard())
		want := fmt.Sprintf("loading the configuration: argument %q: it is not a flag, and flags are written --path=VALUE", arg)
		if err == nil || err.Error() != want {
			t.Errorf("run returned %v, want %q", err, want)
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
			err := run(t.Context(), append(slices.Clone(database), append(slices.Clone(absentKeys), c.args...)...), c.environ, discard())
			if err == nil || !strings.HasPrefix(err.Error(), "loading the configuration: ") {
				t.Errorf("run returned %v, want the configuration library's refusal", err)
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
		return run(t.Context(), args, nil, discard())
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
				t.Errorf("run returned %v, want the keyring's refusal", err)
			case !c.refused && !strings.HasPrefix(err.Error(), pastTheKeyring):
				t.Errorf("run returned %v, want it to pass the keyring and stop at the password file", err)
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
			err := run(t.Context(), append(slices.Clone(database), append(slices.Clone(absentKeys), c.arg)...), nil, discard())
			if err == nil || !strings.HasPrefix(err.Error(), "validating the configuration: scanner: ") || !strings.Contains(err.Error(), c.want) {
				t.Errorf("run returned %v, want the scanner's refusal containing %q", err, c.want)
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
