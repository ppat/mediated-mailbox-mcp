package app

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/process/version"
)

// database is a database section the UI validates, naming a password file that does not exist.
var database = []string{
	"--database.host=db.example", "--database.name=mailbox", "--database.user=mediated_mailbox_app", "--database.password_file=/absent/password",
	"--credential.public_key_file=/absent/public.key", "--credential.private_key_files=[/absent/private.key]",
}

// The UI refuses to start while the database driver's password variables are set, even to an empty
// value, so a stray one never wins over the mounted password file, and nothing is logged before the
// refusal (ADR-0078). That the refusal comes before the configuration is read is the process's,
// proven by TestThePasswordVariablesAreRefusedBeforeTheConfigurationIsRead in process.
func TestAPasswordVariableRefusesTheStart(t *testing.T) {
	for _, name := range []string{"PGPASSWORD", "PGSSLPASSWORD"} {
		var stdout, stderr bytes.Buffer
		err := run(t.Context(), database, []string{name + "="}, &stdout, &stderr)
		if err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("with %s set, the run returned %v, want the start refused naming it", name, err)
		}
		if got := messages(t, stdout.String()); len(got) != 1 || got[0] != "the process failed" {
			t.Errorf("with %s set, the run logged %v, want only the refusal and no configuration before it", name, got)
		}
		if stderr.Len() != 0 {
			t.Errorf("with %s set, the run wrote the banner %q before refusing", name, stderr.String())
		}
	}
}

// A start writes the effective configuration at info whatever log_level says, sets the level from
// log_level only then, writes a banner naming the UI and its release version to the writer main.go
// hands in as standard error, and keeps standard output to JSON records (ADR-0122). The public key
// file does not exist, so the start stops at it, after the banner and before any connection.
func TestTheStartLogsItsConfigurationThenSetsItsLevelAndWritesItsBanner(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run(t.Context(), append([]string{"--log_level=error"}, database...), nil, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "loading the public key") {
		t.Fatalf("the run returned %v, want the start to stop at the public key file", err)
	}
	if !strings.HasPrefix(stderr.String(), "mediated-mailbox-ui "+version.Number+"\n") {
		t.Errorf("the banner is %q, want it to name the UI and its release version", stderr.String())
	}
	var logged []string
	for line := range strings.Lines(stdout.String()) {
		var record struct{ Msg, Path, Level string }
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("standard output holds a line that is not a JSON record: %q", line)
		}
		switch {
		case record.Msg == "configuration":
			logged = append(logged, record.Path)
		case record.Level != "ERROR":
			// Past the configuration, the level log_level names holds, so nothing below error is
			// written, the key identifier's line at info among it.
			t.Errorf("the run wrote %q at %s, below the level log_level names", record.Msg, record.Level)
		}
	}
	if !strings.Contains(strings.Join(logged, " "), "log_level") || !strings.Contains(strings.Join(logged, " "), "database.password_file") {
		t.Errorf("the configuration logged is %v, want every effective value, log_level's included", logged)
	}
}

// A log level that names none is refused before the banner, after the effective configuration names
// the layer that set it (ADR-0122).
func TestALevelThatNamesNoneIsRefused(t *testing.T) {
	for _, level := range []string{"verbose", "INFO", "info+2"} {
		var stdout, stderr bytes.Buffer
		err := run(t.Context(), append([]string{"--log_level=" + level}, database...), nil, &stdout, &stderr)
		if err == nil || !strings.HasPrefix(err.Error(), "validating the configuration: ") {
			t.Errorf("%s: the run returned %v, want the configuration refused", level, err)
		}
		if !strings.Contains(stdout.String(), `"path":"log_level","source":"flag --log_level"`) {
			t.Errorf("%s: the log does not name the layer that set log_level:\n%s", level, stdout.String())
		}
		if stderr.Len() != 0 {
			t.Errorf("%s: the run wrote the banner %q before refusing", level, stderr.String())
		}
	}
}

// messages returns the message of each JSON record in logs.
func messages(t *testing.T, logs string) []string {
	t.Helper()
	var got []string
	for line := range strings.Lines(logs) {
		var record struct{ Msg string }
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("the log holds a line that is not a JSON record: %q", line)
		}
		got = append(got, record.Msg)
	}
	return got
}
