//go:build integration

package backfill

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/series"
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
		Store: pass1.NewPostgres(pool), Fetch: f.EnumerateAll, Metadata: f.GetMessageMetadata, PerCall: 3, Policy: fixed(rules.For(account)), Scanner: s, Lookups: lookups,
		RunID: runID, Now: time.Now,
	}
	second := pass2.Deps{
		Store: pass2.NewPostgres(pool), Body: f.GetMessageBody, Policy: fixed(rules.For(account)), Lookups: lookups,
		Gate: scangate.DefaultConfig(), Scanner: s, PageSize: 2, RunID: runID, Now: time.Now,
	}
	return first, second
}

// observing is a second pass store that records, at each read of the backlog, what the account's
// backlog series read before it, which is what the step before set, or -1 when there is no series.
type observing struct {
	pass2.Store
	registry *prometheus.Registry
	seen     []float64
}

func (o *observing) Backlog(ctx context.Context, account string) (int64, error) {
	o.seen = append(o.seen, -1)
	families, err := o.registry.Gather()
	if err != nil {
		return 0, err
	}
	for _, f := range families {
		if f.GetName() == series.BacklogName {
			for _, m := range f.GetMetric() {
				o.seen[len(o.seen)-1] = m.GetGauge().GetValue()
			}
		}
	}
	return o.Store.Backlog(ctx, account)
}

// Every page of the second pass is a unit of work. The account's token is handed over after each page
// and after the step that ends the pass, and the account's scan backlog series is set after each page,
// so the last page leaves it reading the messages left waiting. The step that ends the pass removes it,
// since delta sync's tick emits the account's backlog once the pass has ended (ADR-0082, ADR-0093,
// ADR-0104).
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
	if err := passAccount(t.Context(), first, "personal", metrics1, idleUnit(t, conn, "personal"), schedule.Ask{}, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	registry := prometheus.NewRegistry()
	metrics, err := pass2.NewMetrics(registry)
	if err != nil {
		t.Fatal(err)
	}
	var ends int
	u := connectedUnit(t, conn, "personal", counter(&ends, mail.AuthAttempt{}))

	store := &observing{Store: second.Store, registry: registry}
	second.Store = store

	if err := secondPassAccount(t.Context(), second, "personal", metrics, u, schedule.Ask{}, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}

	if ends != 4 {
		t.Errorf("the units of work ended %d times, want once after each of the three pages and once after the end", ends)
	}
	if len(store.seen) != 4 || store.seen[0] != -1 || store.seen[3] != 1 {
		t.Errorf("the backlog series read %v before each step's read, want none before the first and 1 after the last page, the message whose body the conversion refused", store.seen)
	}
	if got := accountsIn(t, registry, series.BacklogName); len(got) != 0 {
		t.Errorf("the backlog series names %v once the pass ended, want none", got)
	}
}

// A hand-over that fails is returned once the second pass ends, and the pass still ends (ADR-0082).
// The first unit's end fails at recording the attempt its source reports, and the units after it
// report none.
func TestAFailedHandOverFailsTheSecondPassAtItsEnd(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	first, second := passesDeps(t, backfillPool(t), "personal", bodyMailbox(t, "personal"))
	metrics1, err := pass1.NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if err := passAccount(t.Context(), first, "personal", metrics1, idleUnit(t, conn, "personal"), schedule.Ask{}, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	metrics, err := pass2.NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	var ends int
	u := connectedUnit(t, conn, "personal", counter(&ends, mail.AuthAttempt{At: 1_790_000_000_001, Outcome: mail.AuthSucceeded}))
	revoke(t, conn, "UPDATE (last_auth_at, last_auth_outcome) ON account_state")

	err = secondPassAccount(t.Context(), second, "personal", metrics, u, schedule.Ask{}, slog.New(slog.DiscardHandler))

	if err == nil || !strings.Contains(err.Error(), "account personal: recording the last authentication attempt") {
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
	idle := idleUnit(t, conn, "personal")

	if err := passAccount(t.Context(), first, "personal", metrics1, idle, schedule.Ask{}, logger); err != nil {
		t.Fatal(err)
	}
	if err := secondPassAccount(t.Context(), second, "personal", metrics, idle, schedule.Ask{}, logger); err != nil {
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
