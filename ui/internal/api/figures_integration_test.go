//go:build integration

package api_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// TestTheCorpusCountsEachThreadOnce reads Home's corpus figures over the seeded corpus with threads
// added beside it, one of three messages, one of one, two whose identifiers differ only in case and two
// whose identifiers differ only in Unicode normal form, with another account holding a message in one
// of them. Each thread counts once, each identifier is a thread of its own, another account's message
// counts nowhere, and an account with no message counts zero of each figure (docs/UI.md section 17.4).
func TestTheCorpusCountsEachThreadOnce(t *testing.T) {
	r := newRig(t)
	b := r.browser()
	type figures struct {
		Messages int64 `json:"messages"`
		Threads  int64 `json:"threads"`
		Unfiled  int64 `json:"unfiled"`
	}
	var got figures
	read(t, b.h, "/ui/api/accounts/{account}/corpus", "/ui/api/accounts/personal/corpus", &got)
	// The seed holds five messages in four threads, three of them unfiled.
	if diff := cmp.Diff(figures{Messages: 5, Threads: 4, Unfiled: 3}, got, compare.Options); diff != "" {
		t.Fatalf("the seeded corpus (-want +got):\n%s", diff)
	}
	r.exec(`INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at, labels, has_attachments, scan_state) VALUES
		($1, 'm-many1', 't-many', 'a@shop.example', 'shop.example', $3, '{}', false, 'scanned'),
		($1, 'm-many2', 't-many', 'a@shop.example', 'shop.example', $3, '{INBOX}', false, 'scanned'),
		($1, 'm-many3', 't-many', 'a@shop.example', 'shop.example', $3, '{INBOX}', false, 'scanned'),
		($1, 'm-one', 't-one', 'a@shop.example', 'shop.example', $3, '{}', false, 'scanned'),
		($1, 'm-upper', 'T-CASE', 'a@shop.example', 'shop.example', $3, '{INBOX}', false, 'scanned'),
		($1, 'm-lower', 't-case', 'a@shop.example', 'shop.example', $3, '{INBOX}', false, 'scanned'),
		($1, 'm-composed', 't-caf`+"é"+`', 'a@shop.example', 'shop.example', $3, '{INBOX}', false, 'scanned'),
		($1, 'm-decomposed', 't-cafe`+"́"+`', 'a@shop.example', 'shop.example', $3, '{INBOX}', false, 'scanned'),
		($2, 'm-other-many', 't-many', 'a@shop.example', 'shop.example', $3, '{}', false, 'scanned')`, personal, other, now)
	read(t, b.h, "/ui/api/accounts/{account}/corpus", "/ui/api/accounts/personal/corpus", &got)
	if diff := cmp.Diff(figures{Messages: 13, Threads: 10, Unfiled: 5}, got, compare.Options); diff != "" {
		t.Errorf("the corpus with the added threads (-want +got):\n%s", diff)
	}
	emptyOther(t)
	read(t, b.h, "/ui/api/accounts/{account}/corpus", "/ui/api/accounts/other/corpus", &got)
	if diff := cmp.Diff(figures{}, got, compare.Options); diff != "" {
		t.Errorf("an account with no message (-want +got):\n%s", diff)
	}
}

// seedRunPages adds fifty-five runs to the seeded ones, filling two pages of the runs dataset, and
// returns each personal run's failures. Pairs of added runs share a start, their durations repeat
// every four runs and their failures every three, so every sort meets ties its run identity breaks,
// and a third of them hold no failure. Another account's run shares a personal run's identifier and
// holds three failures of its own.
func seedRunPages(r *rig) map[string]int64 {
	r.t.Helper()
	failures := map[string]int64{"r-0901": 2, "r-0908": 0, "r-0911": 0, "r-0912": 5, "r-0913": 0, "r-0914": 0}
	for i := range 55 {
		run := fmt.Sprintf("r-old%02d", i)
		// Starts are scattered across the identities, so no sort's order is the identities' order.
		started := now.Add(-time.Duration(20*24+(i*23%55)/2) * time.Hour)
		finished := started.Add(time.Duration(i%4) * time.Minute)
		r.exec(`INSERT INTO job_runs (account_id, run_id, workload, pass, state, started_at, finished_at, heartbeat_at)
			VALUES ($1, $2, 'sync', 'gap_recovery', 'succeeded', $3, $4, $4)`, personal, run, started, finished)
		n := int64(i % 3)
		r.exec(`INSERT INTO job_run_failures (account_id, run_id, seq, item_kind, item_id, page, error_class, attempts, first_at, last_at, disposition)
			SELECT $1, $2, n, 'page', n::text, n, 'throttled', 1, $3, $3, 'pending' FROM generate_series(1, $4::int) AS n`,
			personal, run, started, n)
		failures[run] = n
	}
	r.exec(`INSERT INTO job_runs (account_id, run_id, workload, pass, state, started_at, heartbeat_at)
		VALUES ($1, 'r-0908', 'sync', 'gap_recovery', 'running', $2, $2)`, other, now)
	r.exec(`INSERT INTO job_run_failures (account_id, run_id, seq, item_kind, item_id, page, error_class, attempts, first_at, last_at, disposition)
		SELECT $1, 'r-0908', n, 'page', n::text, n, 'throttled', 1, $2, $2, 'pending' FROM generate_series(1, 3) AS n`, other, now)
	return failures
}

// runPage is one page of the runs dataset's rows, each run's identity and failures.
type runPage struct {
	Pages int `json:"pages"`
	Rows  []struct {
		RunID    string `json:"run_id"`
		Failures *int64 `json:"failures"`
	} `json:"rows"`
}

// runSorts are the runs dataset's sortable columns, each in both directions.
func runSorts() []string {
	return []string{"started_at,desc", "started_at,asc", "duration,desc", "duration,asc", "failures,desc", "failures,asc"}
}

// everyPage reads every page of the runs dataset, all time, under a sort, and requires two.
func everyPage(t *testing.T, b *browser, sort string) []runPage {
	t.Helper()
	var pages []runPage
	for p := 1; p == 1 || p <= pages[0].Pages; p++ {
		var page runPage
		read(t, b.h, "/ui/api/accounts/{account}/lens", fmt.Sprintf("/ui/api/accounts/personal/lens?dataset=runs&level=3&range=all&sort=%s&page=%d", sort, p), &page)
		pages = append(pages, page)
	}
	if pages[0].Pages != 2 {
		t.Errorf("sorted by %s the runs fill %d pages, want 2", sort, pages[0].Pages)
	}
	return pages
}

// TestEachRunCountsItsOwnFailures reads every page of the runs dataset, sorted by each sortable
// column in both directions, over the runs seedRunPages leaves. Every row on every page carries
// exactly its own run's failures, zero rather than nothing for a run with none, and never another
// account's (docs/UI.md section 17.1).
func TestEachRunCountsItsOwnFailures(t *testing.T) {
	r := newRig(t)
	b := r.browser()
	want := seedRunPages(r)
	for _, sort := range runSorts() {
		got := map[string]int64{}
		for _, page := range everyPage(t, b, sort) {
			for _, row := range page.Rows {
				if row.Failures == nil {
					t.Fatalf("sorted by %s, %s carries no failure count", sort, row.RunID)
				}
				got[row.RunID] = *row.Failures
			}
		}
		if diff := cmp.Diff(want, got, compare.Options); diff != "" {
			t.Errorf("sorted by %s, each run's failures (-want +got):\n%s", sort, diff)
		}
	}
}

// TestEachSortPagesThroughTheRunsInItsOrder reads every page of the runs dataset under each sort over
// the runs seedRunPages leaves, and requires the pages, joined in order, to hold every run once in the
// order the sort's key gives, ties broken by run identity, computed here from the stored runs and
// their seeded failures. A running run's duration runs to the server's clock (docs/UI.md section
// 17.1).
func TestEachSortPagesThroughTheRunsInItsOrder(t *testing.T) {
	r := newRig(t)
	b := r.browser()
	failures := seedRunPages(r)
	type run struct {
		id       string
		started  time.Time
		duration time.Duration
	}
	var runs []run
	rows, err := r.db.Query(t.Context(), `SELECT run_id, started_at, coalesce(finished_at, $2) FROM job_runs WHERE account_id = $1`, personal, now)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var x run
		var finished time.Time
		if err := rows.Scan(&x.id, &x.started, &finished); err != nil {
			t.Fatal(err)
		}
		x.duration = finished.Sub(x.started)
		runs = append(runs, x)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	keys := map[string]func(run) int64{
		"started_at": func(x run) int64 { return x.started.UnixNano() },
		"duration":   func(x run) int64 { return int64(x.duration) },
		"failures":   func(x run) int64 { return failures[x.id] },
	}
	for _, sort := range runSorts() {
		column, direction, _ := strings.Cut(sort, ",")
		key := keys[column]
		want := slices.Clone(runs)
		slices.SortFunc(want, func(a, z run) int {
			c := 0
			if ka, kz := key(a), key(z); ka < kz {
				c = -1
			} else if ka > kz {
				c = 1
			}
			if direction == "desc" {
				c = -c
			}
			if c != 0 {
				return c
			}
			return strings.Compare(a.id, z.id)
		})
		var wantIDs, got []string
		for _, x := range want {
			wantIDs = append(wantIDs, x.id)
		}
		for _, page := range everyPage(t, b, sort) {
			for _, row := range page.Rows {
				got = append(got, row.RunID)
			}
		}
		if diff := cmp.Diff(wantIDs, got); diff != "" {
			t.Errorf("sorted by %s, the runs across the pages (-want +got):\n%s", sort, diff)
		}
	}
}
