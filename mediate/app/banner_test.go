package app

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/process/logging"
	"github.com/ppat/mediated-mailbox-mcp/process/version"
)

// The start writes a banner naming the deployable, its release version and where it serves to the
// writer main.go hands in as standard error, and standard output carries only JSON records, the
// banner never among them (ADR-0122). The key files do not exist, so the start stops at the keyring,
// after the banner and before any connection is made.
func TestTheStartBannerGoesToStandardErrorAndStandardOutputStaysJSON(t *testing.T) {
	var stdout, stderr bytes.Buffer
	level := new(slog.LevelVar)
	err := Run(t.Context(), args(servingArgs, []string{"--database.host=db", "--database.name=mailbox", "--database.password_file=/absent"}, absentKeys), nil, &stderr, logging.New(&stdout, level), level)
	if err == nil || !strings.HasPrefix(err.Error(), "loading the keyring: ") {
		t.Fatalf("Run returned %v, want the start to stop at loading the keyring: ", err)
	}
	if want := "mediated-mailbox-mediate " + version.Number + "\n  client surface  http on :8443\n  probes          http on :8080\n"; stderr.String() != want {
		t.Errorf("the banner is\n%s\nwant\n%s", stderr.String(), want)
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) == 0 || lines[0] == "" {
		t.Fatal("the start logged nothing")
	}
	for _, line := range lines {
		if !json.Valid([]byte(line)) {
			t.Errorf("standard output holds a line that is not a JSON record: %q", line)
		}
	}
}
