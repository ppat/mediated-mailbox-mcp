package deltasync

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// D4's row for the gate and the scanner delta sync shares with backfill. A tick decides under
// thresholds written out here, which backfill's own test holds backfill to as well, and scans with
// the scanner the job kind is built with, which the worker builds once from its one scanner section
// for both job kinds, so a job kind that differs from the other fails its own test (ADR-0120,
// ADR-0121, ADR-0104).
func TestATickDecidesUnderBackfillsThresholdsAndScanner(t *testing.T) {
	scanner, err := scan.New(scan.DefaultConfig(), "the-worker's-revision")
	if err != nil {
		t.Fatal(err)
	}
	d, err := build(Config{Scanner: scanner, Registry: prometheus.NewRegistry(), Logger: slog.New(slog.DiscardHandler)}, nil)
	if err != nil {
		t.Fatal(err)
	}

	deps := d.tickDeps("personal", nil, policyload.Snapshot{})

	want := scangate.Config{
		NoReplyLocalParts: []string{"noreply", "no-reply", "security", "accounts", "verify", "auth", "support"},
		SmallBytes:        30 * 1024,
		RecentAgeMillis:   24 * 60 * 60 * 1000,
		LowVolume:         20,
		HighVolume:        500,
	}
	if diff := cmp.Diff(want, deps.Gate, compare.Options); diff != "" {
		t.Errorf("the gate's thresholds (-backfill's +delta sync's):\n%s", diff)
	}
	v := deps.Scanner.Scan("Your code is 419283")
	const given = "version 1 revision the-worker's-revision"
	if got := fmt.Sprintf("version %d revision %s", v.Version(), v.Revision()); got != given {
		t.Errorf("the scanner records %s, want the given scanner's %s", got, given)
	}
}
