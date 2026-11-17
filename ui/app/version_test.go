package app_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/version"
	"github.com/ppat/mediated-mailbox-mcp/ui/app"
)

// --version prints the release version and exits before any layer of the configuration is read, so
// a flag the configuration would refuse beside it stops nothing, and no configuration is logged
// (ADR-0078).
func TestVersionPrintsTheReleaseVersionAndReadsNoConfiguration(t *testing.T) {
	var stdout bytes.Buffer
	err := process.RunContext(t.Context(), []string{"--version", "--no.such=1"}, nil, &stdout, io.Discard, app.Spec())
	if err != nil {
		t.Fatalf("the run returned %v, want nil", err)
	}
	if want := version.Number + "\n"; stdout.String() != want {
		t.Errorf("the run printed %q and logged nothing else, want %q", stdout.String(), want)
	}
}
