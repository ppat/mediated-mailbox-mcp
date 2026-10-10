package backfill

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// D2's part of D4's row for the gate and the scanner delta sync shares with backfill. The second pass
// decides under thresholds written out here, which delta sync's own test holds delta sync to as well,
// and scans with the scanner the job kind is given, which the worker builds once from its one scanner
// section for both job kinds, so a job kind that differs from the other fails its own test (ADR-0120,
// ADR-0121, ADR-0104).
func TestARunDecidesUnderTheSharedThresholdsAndScanner(t *testing.T) {
	scanner, err := scan.New(scan.DefaultConfig(), "the-worker's-revision")
	if err != nil {
		t.Fatal(err)
	}

	deps := secondDeps(nil, nil, nil, classify.Lookups{}, scanner)

	want := scangate.Config{
		NoReplyLocalParts: []string{"noreply", "no-reply", "security", "accounts", "verify", "auth", "support"},
		SmallBytes:        30 * 1024,
		RecentAgeMillis:   24 * 60 * 60 * 1000,
		LowVolume:         20,
		HighVolume:        500,
	}
	if diff := cmp.Diff(want, deps.Gate, compare.Options); diff != "" {
		t.Errorf("the gate's thresholds (-delta sync's +backfill's):\n%s", diff)
	}
	v := deps.Scanner.Scan("Your code is 419283")
	const given = "version 1 revision the-worker's-revision"
	if got := fmt.Sprintf("version %d revision %s", v.Version(), v.Revision()); got != given {
		t.Errorf("the scanner records %s, want the given scanner's %s", got, given)
	}
}
