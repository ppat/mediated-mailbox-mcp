package app_test

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/mediate/app"
	"github.com/ppat/mediated-mailbox-mcp/process/version"
)

// --version prints the release version and exits before any layer of the configuration is read, so
// a flag the configuration would refuse beside it stops nothing, and no configuration is logged
// (ADR-0078).
func TestVersionPrintsTheReleaseVersionAndReadsNoConfiguration(t *testing.T) {
	read, write, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout := os.Stdout
	os.Stdout = write
	var logs bytes.Buffer
	runErr := app.Run(t.Context(), []string{"--version", "--no.such=1"}, nil, io.Discard, slog.New(slog.NewJSONHandler(&logs, nil)), new(slog.LevelVar))
	os.Stdout = stdout
	if err := write.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := io.ReadAll(read)
	if err != nil {
		t.Fatal(err)
	}
	if runErr != nil {
		t.Fatalf("Run returned %v, want nil", runErr)
	}
	if want := version.Number + "\n"; string(out) != want {
		t.Errorf("Run printed %q, want %q", out, want)
	}
	if logs.Len() != 0 {
		t.Errorf("Run logged %s, want nothing", logs.String())
	}
}
