package main

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	coreconnection "github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/connection"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// Backfill's root configuration type is pinned field by field, so a new section is a visible
// change (ADR-0078). The database section's own type is pinned in its package.
func TestTheConfigurationTypeIsPinned(t *testing.T) {
	mustnotcompile.RequireFields(t, "github.com/ppat/mediated-mailbox-mcp/backfill", "Configuration", "Database connection.Config")
}

func discard() *slog.Logger { return slog.New(slog.DiscardHandler) }

// Backfill's defaults are the port, its own runtime role and the TLS mode that fails closed, and the
// host, the database name and the password file are required.
func TestTheDefaults(t *testing.T) {
	want := Configuration{Database: coreconnection.Config{Port: 5432, User: "mediated_mailbox_backfill", SSLMode: "verify-full"}}
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
	err := run(t.Context(), []string{"--database.host=", "--database.name=mailbox", "--database.password_file=/absent"}, nil, discard())
	want := "validating the configuration: database.host is empty"
	if err == nil || err.Error() != want {
		t.Errorf("run returned %v, want %q", err, want)
	}
}

// An argument starting with a dash is a flag, any other is an account, and one holding an equals
// sign is refused rather than taken as an account.
func TestTheArgumentsSplitIntoFlagsAndAccounts(t *testing.T) {
	flags, accounts, err := splitArguments([]string{"--database.port=5433", "one@example.com", "-h", "two@example.com"})
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"--database.port=5433", "-h"}, flags, compare.Options); diff != "" {
		t.Errorf("flags (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"one@example.com", "two@example.com"}, accounts, compare.Options); diff != "" {
		t.Errorf("accounts (-want +got):\n%s", diff)
	}
	_, _, err = splitArguments([]string{"one@example.com", "database.port=5433"})
	want := `argument "database.port=5433": it is neither an account nor a flag, and flags are written --path=VALUE`
	if err == nil || err.Error() != want {
		t.Errorf("splitArguments returned %v, want %q", err, want)
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
	// No account is given, so the start stops at the policy loader, after the log and before any
	// connection is made.
	err := run(t.Context(), []string{"--database.host=db.example", "--database.password_file=" + passwordFile},
		[]string{"MEDIATED_MAILBOX_DATABASE__NAME=mailbox"}, logger)
	if want := "the policy loader needs an account, because every read runs in an account's transaction"; err == nil || err.Error() != want {
		t.Fatalf("run returned %v, want %q", err, want)
	}
	want := []string{
		`level=INFO msg=configuration path=database.host source="flag --database.host" value=db.example`,
		`level=INFO msg=configuration path=database.name source="environment variable MEDIATED_MAILBOX_DATABASE__NAME" value=mailbox`,
		`level=INFO msg=configuration path=database.password_file source="flag --database.password_file" value=` + passwordFile,
		`level=INFO msg=configuration path=database.port source=default value=5432`,
		`level=INFO msg=configuration path=database.sslmode source=default value=verify-full`,
		`level=INFO msg=configuration path=database.sslrootcert source=default value=""`,
		`level=INFO msg=configuration path=database.user source=default value=mediated_mailbox_backfill`,
	}
	got := strings.Split(strings.TrimSpace(out.String()), "\n")
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("log (-want +got):\n%s", diff)
	}
	if strings.Contains(out.String(), "the-secret-itself") {
		t.Errorf("the log holds the password:\n%s", out.String())
	}
}
