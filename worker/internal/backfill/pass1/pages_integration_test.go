//go:build integration

package pass1_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// A run of backfill's first pass records a page on each retry of its timeline, the page it asked for
// on a page's retry and zero on a retry of a call fetching subjects again, which concerns no page.
// The other events and failed items of the first pass are held by its other tests, which compare a
// run's timeline and failed items with their pages (ADR-0016).
func TestAFirstPassRecordsAPageOnItsRetries(t *testing.T) {
	w := realWorld(t, mixed)
	fetch, metadata := w.deps.Fetch, w.deps.Metadata
	failPage, failCall := true, false
	w.deps.Fetch = func(ctx context.Context, token mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
		if failPage {
			failPage = false
			return mail.Page[mail.MessageMetadata]{}, fmt.Errorf("fake: %w", mail.ErrProvider)
		}
		return fetch(ctx, token)
	}
	w.deps.Metadata = func(ctx context.Context, ids []string) ([]mail.MessageMetadata, error) {
		if failCall {
			failCall = false
			return nil, fmt.Errorf("fake: %w", mail.ErrProvider)
		}
		return metadata(ctx, ids)
	}
	w.finish(t, 10)
	// A change of scanner has the next run fetch the stale subjects again, and its first call fails.
	w.rescan(t)
	failCall = true
	w.finish(t, 10)

	rows, err := superuser(t).Query(t.Context(), `SELECT run_id || ' ' || kind || ' ' || coalesce(page::text, 'null') FROM job_run_events
		WHERE account_id = $1 AND kind = 'retry' ORDER BY seq`, w.account)
	if err != nil {
		t.Fatal(err)
	}
	retries, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff([]string{"run1 retry 1", "run2 retry 0"}, retries, compare.Options); diff != "" {
		t.Errorf("the first pass's retries and their pages (-want +got):\n%s", diff)
	}
}
