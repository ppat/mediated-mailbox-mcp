package app_test

import (
	"bytes"
	"io"
	"log/slog"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/api/app"
	"github.com/ppat/mediated-mailbox-mcp/process/version"
)

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
