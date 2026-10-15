package app_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/migrator/app"
	"github.com/ppat/mediated-mailbox-mcp/process/logging"
	"github.com/ppat/mediated-mailbox-mcp/process/version"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

// database is a database section the migrator validates, naming a password file that does not exist,
// so a start that gets past its validation stops at reading the password, before any connection.
var database = []string{"--database.host=db.example", "--database.name=mailbox", "--database.password_file=/absent/password"}

// --version prints the release version and exits before any layer of the configuration is read, so
// a flag the configuration would refuse beside it stops nothing, and no configuration is logged
// (ADR-0078).
func TestVersionPrintsTheReleaseVersionAndReadsNoConfiguration(t *testing.T) {
	var stdout, logs bytes.Buffer
	err := app.Run(t.Context(), []string{"--version", "--no.such=1"}, nil, &stdout, io.Discard, slog.New(slog.NewJSONHandler(&logs, nil)), new(slog.LevelVar))
	if err != nil {
		t.Fatalf("Run returned %v, want nil", err)
	}
	if want := version.Number + "\n"; stdout.String() != want {
		t.Errorf("Run printed %q, want %q", stdout.String(), want)
	}
	if logs.Len() != 0 {
		t.Errorf("Run logged %s, want nothing", logs.String())
	}
}

// The migrator refuses to start while the database driver's password variables are set, before its
// configuration is read, so a stray one never wins over the mounted password file, as every process
// refuses it (ADR-0078).
func TestAPasswordVariableRefusesTheStart(t *testing.T) {
	for _, name := range []string{"PGPASSWORD", "PGSSLPASSWORD"} {
		var logs bytes.Buffer
		err := app.Run(t.Context(), database, []string{name + "="}, io.Discard, io.Discard, slog.New(slog.NewJSONHandler(&logs, nil)), new(slog.LevelVar))
		if err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("with %s set, Run returned %v, want the start refused naming it", name, err)
		}
		if logs.Len() != 0 {
			t.Errorf("with %s set, Run logged %s before refusing, want nothing", name, logs.String())
		}
	}
}

// A start writes the effective configuration at info whatever log_level says, sets the level from
// log_level only then, writes a banner naming the migrator and its release version to the writer
// main.go hands in as standard error, and keeps standard output to JSON records (ADR-0122). The
// password file does not exist, so the start stops at reading it, after the banner and before any
// connection is made.
func TestTheStartLogsItsConfigurationThenSetsItsLevelAndWritesItsBanner(t *testing.T) {
	var stdout, stderr bytes.Buffer
	level := new(slog.LevelVar)
	err := app.Run(t.Context(), append([]string{"--log_level=error"}, database...), nil, &stdout, &stderr, logging.New(&stdout, level), level)
	if err == nil || !strings.Contains(err.Error(), "reading the password file") {
		t.Fatalf("Run returned %v, want the start to stop at reading the password file", err)
	}
	if level.Level() != slog.LevelError {
		t.Errorf("the level is %s, want the one log_level names", level.Level())
	}
	if want := "mediated-mailbox-migrator " + version.Number + "\n"; stderr.String() != want {
		t.Errorf("the banner is %q, want %q", stderr.String(), want)
	}
	var logged []string
	for line := range strings.Lines(stdout.String()) {
		var record struct{ Msg, Path string }
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("standard output holds a line that is not a JSON record: %q", line)
		}
		if record.Msg == "configuration" {
			logged = append(logged, record.Path)
		}
	}
	if !strings.Contains(strings.Join(logged, " "), "log_level") || !strings.Contains(strings.Join(logged, " "), "database.password_file") {
		t.Errorf("the configuration logged is %v, want every effective value, log_level's included", logged)
	}
}

// A log level that names none, and a database section the driver would fill with its own defaults,
// are refused before the banner and before any connection.
func TestAnUnusableConfigurationIsRefused(t *testing.T) {
	for _, c := range []struct {
		args []string
		want string
	}{
		{append([]string{"--log_level=verbose"}, database...), "validating the configuration: "},
		{[]string{"--database.host=db.example", "--database.name=mailbox", "--database.password_file=/absent/password", "--database.sslmode=verify_full"}, "validating the configuration: database.sslmode"},
		{[]string{"--database.name=mailbox", "--database.password_file=/absent/password"}, "loading the configuration: "},
	} {
		var stderr bytes.Buffer
		err := app.Run(t.Context(), c.args, nil, io.Discard, &stderr, slog.New(slog.DiscardHandler), new(slog.LevelVar))
		if err == nil || !strings.HasPrefix(err.Error(), c.want) {
			t.Errorf("%v: Run returned %v, want an error starting %q", c.args, err, c.want)
		}
		if stderr.Len() != 0 {
			t.Errorf("%v: Run wrote the banner %q before refusing", c.args, stderr.String())
		}
	}
}

// The configuration type is pinned field by field, so a new value, one meant to hold a secret
// included, is a visible change (ADR-0078).
func TestTheConfigurationTypeIsPinned(t *testing.T) {
	mustnotcompile.RequireFields(t, "github.com/ppat/mediated-mailbox-mcp/migrator/app", "Configuration",
		"Process host.Process",
		"Database core.Config",
	)
}
