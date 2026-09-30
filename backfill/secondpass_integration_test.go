//go:build integration

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1"
	"github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2"
	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
)

// bodyCode is a one-time code the scanner flags.
const bodyCode = "Your verification code is 419283."

// bodyMailbox returns a provider fake for the account whose every message's body carries a body
// marker, in the text part, the HTML part or both. It holds a restricted sender's message with a code,
// a normal sender's with a code, clean ones, one whose HTML part the conversion refuses, and one whose
// sender has no domain the classifier can read.
func bodyMailbox(t *testing.T, account string) *fake.Fake {
	t.Helper()
	refused := "<p>" + marker.Body("refused") + strings.Repeat(" padding", 512*1024/8) + "</p>"
	bodies := []struct {
		from       string
		text, html string
	}{
		{"alerts@bank.example", marker.Body("bank") + " " + bodyCode, ""},
		{"orders@shop.example", marker.Body("shoptext") + " " + bodyCode, "<p>" + marker.Body("shophtml") + " " + bodyCode + "</p>"},
		{"hello@shop.example", "", "<p>" + marker.Body("welcome") + " Welcome aboard.</p>"},
		{"news@news.example", marker.Body("news") + " This week's news.", ""},
		{"big@news.example", "", refused},
		{"no-domain", marker.Body("nodomain") + " " + bodyCode, ""},
	}
	var messages []fake.Message
	for i, b := range bodies {
		messages = append(messages, fake.Message{
			Metadata: mail.MessageMetadata{
				ID: fmt.Sprintf("m%d", i), ThreadID: fmt.Sprintf("t%d", i), From: mail.Address{Email: b.from},
				Subject: marker.Field("subject"), Date: mail.UnixMilli(1_700_000_000_000 + int64(i)),
			},
			Body: mail.MessageBody{Text: b.text, HTML: b.html},
		})
	}
	f, err := fake.New(fake.Config{Account: account, PageSize: 2}, messages...)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// passesDeps returns the dependencies of both passes over the account through pool, over the fake f,
// under a policy listing the bank.
func passesDeps(t *testing.T, pool *pgxpool.Pool, account string, f *fake.Fake) (pass1.Deps, pass2.Deps) {
	t.Helper()
	s, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	rules, err := policy.Load([]policy.Row{{ID: "rule.bank", Class: policy.Restricted, DomainSuffixes: []string{"bank.example"}}})
	if err != nil {
		t.Fatal(err)
	}
	lookups := classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne}
	n := 0
	runID := func() string { n++; return fmt.Sprintf("run%d", n) }
	first := pass1.Deps{
		Store: pass1.NewPostgres(pool), Fetch: f.EnumerateAll, Policy: rules.For(account), Scanner: s, Lookups: lookups,
		RunID: runID, Now: time.Now,
	}
	second := pass2.Deps{
		Store: pass2.NewPostgres(pool), Body: f.GetMessageBody, Policy: rules.For(account), Lookups: lookups,
		Gate: scangate.DefaultConfig(), Scanner: s, PageSize: 2, RunID: runID, Now: time.Now,
	}
	return first, second
}

// Every page of the second pass is a unit of work. The account's token is handed over after each page
// and after the step that ends the pass, and the account's scan backlog series is set after each, so
// it reads the messages left waiting when the pass ends (ADR-0082, ADR-0093).
func TestEachSecondPassPageIsAUnitOfWork(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	pool := backfillPool(t)
	first, second := passesDeps(t, pool, "personal", bodyMailbox(t, "personal"))
	metrics1, err := pass1.NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if err := passAccount(t.Context(), first, "personal", metrics1, func(context.Context, string) error { return nil }, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	registry := prometheus.NewRegistry()
	metrics, err := pass2.NewMetrics(registry)
	if err != nil {
		t.Fatal(err)
	}
	var handedOver []string
	handOver := func(_ context.Context, account string) error {
		handedOver = append(handedOver, account)
		return nil
	}

	if err := secondPassAccount(t.Context(), second, "personal", metrics, handOver, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff([]string{"personal", "personal", "personal", "personal"}, handedOver, compare.Options); diff != "" {
		t.Errorf("the hand-overs, one after each of the three pages and one after the end (-want +got):\n%s", diff)
	}
	if got := counterValue(t, registry, "mediated_mailbox_backfill_scan_backlog", "personal"); got != 1 {
		t.Errorf("the scan backlog series reads %v, want 1, the message whose body the conversion refused", got)
	}
}

// A hand-over that fails is returned once the second pass ends, and the pass still ends (ADR-0082).
func TestAFailedHandOverFailsTheSecondPassAtItsEnd(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	first, second := passesDeps(t, backfillPool(t), "personal", bodyMailbox(t, "personal"))
	metrics1, err := pass1.NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if err := passAccount(t.Context(), first, "personal", metrics1, func(context.Context, string) error { return nil }, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	metrics, err := pass2.NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	failed := errors.New("the write-back failed")
	calls := 0
	handOver := func(context.Context, string) error {
		calls++
		if calls == 1 {
			return failed
		}
		return nil
	}

	err = secondPassAccount(t.Context(), second, "personal", metrics, handOver, slog.New(slog.DiscardHandler))

	if !errors.Is(err, failed) {
		t.Errorf("secondPassAccount returned %v, want the failed hand-over", err)
	}
	var ended bool
	if err := conn.QueryRow(t.Context(), "SELECT backfill_pass2_complete FROM account_state WHERE account_id = 'personal'").Scan(&ended); err != nil || !ended {
		t.Errorf("the pass did not end after a failed hand-over (%v)", err)
	}
}

// bodyMarkers returns every body marker found in the account's rows of every table that has an
// account column, each row read whole as JSON, and in logs.
func bodyMarkers(t *testing.T, conn *pgx.Conn, account, logs string) []string {
	t.Helper()
	rows, err := conn.Query(t.Context(), `SELECT table_name FROM information_schema.columns
		WHERE table_schema = 'public' AND column_name = 'account_id' ORDER BY table_name`)
	if err != nil {
		t.Fatal(err)
	}
	tables, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) < 10 {
		t.Fatalf("the search reached only the tables %v", tables)
	}
	var found []string
	for _, table := range tables {
		rows, err := conn.Query(t.Context(), "SELECT row_to_json(r)::text FROM "+pgx.Identifier{table}.Sanitize()+" AS r WHERE r.account_id = $1", account)
		if err != nil {
			t.Fatal(err)
		}
		values, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range values {
			if strings.Contains(v, marker.BodyPrefix) {
				found = append(found, table+": "+v)
			}
		}
	}
	for line := range strings.Lines(logs) {
		if strings.Contains(line, marker.BodyPrefix) {
			found = append(found, "log: "+line)
		}
	}
	return found
}

// VERIFICATIONS' row for scanning fixtures and then searching for fixture body text, D2's part, the
// persisted rows and the workload's logs. Both passes run over fixtures whose every body carries a body
// marker, through the steps that scan a body, skip a restricted sender's, refuse one the conversion
// cannot bound and flag codes, with a logger that records everything the workload logs. A search of the
// account's rows in every table with an account column, each row read whole, and of the logs finds no
// body marker. The same search, handed a row and a log line each planted with one, finds both, so its
// silence is evidence (ADR-0009, ADR-0044).
func TestNoBodyTextReachesTheIndexOrTheLogs(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	first, second := passesDeps(t, backfillPool(t), "personal", bodyMailbox(t, "personal"))
	var logs logBuffer
	logger := logs.logger()
	metrics1, err := pass1.NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	metrics, err := pass2.NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	none := func(context.Context, string) error { return nil }

	if err := passAccount(t.Context(), first, "personal", metrics1, none, logger); err != nil {
		t.Fatal(err)
	}
	if err := secondPassAccount(t.Context(), second, "personal", metrics, none, logger); err != nil {
		t.Fatal(err)
	}

	var scanned, refused int
	err = conn.QueryRow(t.Context(), `SELECT count(*) FILTER (WHERE scan_state = 'scanned' AND content_flags <> '{}'),
		(SELECT count(*) FROM job_run_failures WHERE account_id = 'personal' AND item_kind = 'message')
		FROM messages WHERE account_id = 'personal'`).Scan(&scanned, &refused)
	if err != nil || scanned != 1 || refused != 1 {
		t.Fatalf("the run flagged %d bodies and failed %d (%v), want the shop's code flagged and the large body refused", scanned, refused, err)
	}
	if found := bodyMarkers(t, conn, "personal", logs.String()); len(found) != 0 {
		t.Errorf("fixture body text reached the index or the logs:\n%s", strings.Join(found, "\n"))
	}

	must(t, conn, "INSERT INTO job_run_events (account_id, run_id, kind, detail) VALUES ('personal', 'run1', 'retry', $1)", marker.Body("planted"))
	planted := logs.String() + `{"msg":"` + marker.Body("plantedlog") + `"}` + "\n"
	if found := bodyMarkers(t, conn, "personal", planted); len(found) != 2 {
		t.Errorf("the search found %d of the two planted body markers:\n%s", len(found), strings.Join(found, "\n"))
	}
}
